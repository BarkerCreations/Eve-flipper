// internal/api/correlation.go
package api

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"eve-flipper/internal/sde"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type correlationCandidate struct {
	TypeID       int32
	TypeName     string
	CandidateSet string // "ingredient" | "rig" | "module" | "shared_mat"
}

type correlationSuggestion struct {
	TypeID            int32     `json:"type_id"`
	TypeName          string    `json:"type_name"`
	CandidateSet      string    `json:"candidate_set"`
	Correlation       float64   `json:"correlation"`
	LagDays           int       `json:"lag_days"`
	PrimaryMoved      bool      `json:"primary_moved"`
	SecondaryMoved    bool      `json:"secondary_moved"`
	Opportunity       bool      `json:"opportunity"`
	OpportunityStatus string    `json:"opportunity_status"`
	PriceTrend7d      float64   `json:"price_trend_7d"`
	VolumeTrend7d     float64   `json:"volume_trend_7d"`
	DataSource        string    `json:"data_source"`
	PriceSeries       []float64 `json:"price_series"`
	VolumeSeries      []float64 `json:"volume_series"`
}

type correlationSuggestionsResponse struct {
	TypeID          int32                   `json:"type_id"`
	TypeName        string                  `json:"type_name"`
	PriceSeries     []float64               `json:"price_series"`
	VolumeSeries    []float64               `json:"volume_series"`
	DataSource      string                  `json:"data_source"`
	CoverageDays    int                     `json:"coverage_days"`
	ExcludedCount   int                     `json:"excluded_count"`
	IngredientCount int                     `json:"ingredient_count"`
	Suggestions     []correlationSuggestion `json:"suggestions"`
}

type correlationPairResponse struct {
	TypeA        int32     `json:"type_a"`
	TypeAName    string    `json:"type_a_name"`
	TypeAPrices  []float64 `json:"type_a_prices"`
	TypeAVolumes []float64 `json:"type_a_volumes"`
	TypeASource  string    `json:"type_a_source"`
	TypeB        int32     `json:"type_b"`
	TypeBName    string    `json:"type_b_name"`
	TypeBPrices  []float64 `json:"type_b_prices"`
	TypeBVolumes []float64 `json:"type_b_volumes"`
	TypeBSource  string    `json:"type_b_source"`
	Correlation  float64   `json:"correlation"`
	LagDays      int       `json:"lag_days"`
}

// ── Math ──────────────────────────────────────────────────────────────────────

const (
	correlationMinThreshold = 0.4
	correlationMinPoints    = 14
	opportunityMovePct      = 5.0
)

var correlationLags = []int{0, 1, 2, 3, 5, 7}

// pearson returns the Pearson correlation coefficient for two equal-length slices.
// Returns NaN if variance is zero or slices are empty/mismatched.
func pearson(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) {
		return math.NaN()
	}
	var mx, my float64
	for i := range x {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var num, dx2, dy2 float64
	for i := range x {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		return math.NaN()
	}
	return num / denom
}

// pearsonWithLag tests Pearson r between a (primary) and b (candidate) at each lag.
// Positive lag means b[lag:] is correlated with a[:n-lag] — primary moves first.
// Returns NaN/0 if no lag meets minPoints.
func pearsonWithLag(a, b []float64, lags []int, minPoints int) (float64, int) {
	best := math.NaN()
	bestLag := 0
	for _, lag := range lags {
		if lag < 0 {
			continue
		}
		na := len(a) - lag
		nb := len(b) - lag
		n := na
		if nb < n {
			n = nb
		}
		if n < minPoints {
			continue
		}
		r := pearson(a[:n], b[lag:lag+n])
		if math.IsNaN(r) {
			continue
		}
		if math.IsNaN(best) || math.Abs(r) > math.Abs(best) {
			best = r
			bestLag = lag
		}
	}
	return best, bestLag
}

// trend7d returns % change over the last 7 entries of a series.
func trend7d(series []float64) float64 {
	if len(series) < 2 {
		return 0
	}
	w := 7
	if w > len(series) {
		w = len(series)
	}
	start := series[len(series)-w]
	end := series[len(series)-1]
	if start == 0 {
		return 0
	}
	return (end - start) / start * 100
}

func priceTrend7d(prices []float64) float64  { return trend7d(prices) }
func volumeTrend7d(volumes []float64) float64 { return trend7d(volumes) }

// classifyOpportunity returns one of: "NOT_MOVED_YET", "FOLLOWING", "ALREADY_MOVED", "NO_SIGNAL".
func classifyOpportunity(primaryPrices, candidatePrices []float64, lagDays int) string {
	primaryMoved := recentMovePct(primaryPrices, 7) > opportunityMovePct
	window := 7 + lagDays
	candidateMoved := recentMovePct(candidatePrices, window) > opportunityMovePct
	if primaryMoved && !candidateMoved {
		return "NOT_MOVED_YET"
	}
	if primaryMoved && candidateMoved {
		return "FOLLOWING"
	}
	if candidateMoved {
		return "ALREADY_MOVED"
	}
	return "NO_SIGNAL"
}

func recentMovePct(prices []float64, window int) float64 {
	if len(prices) < 2 {
		return 0
	}
	if window > len(prices) {
		window = len(prices)
	}
	start := prices[len(prices)-window]
	end := prices[len(prices)-1]
	if start == 0 {
		return 0
	}
	return math.Abs((end-start)/start) * 100
}

// ── Candidate set builders ────────────────────────────────────────────────────

// buildIngredientCandidates returns manufacturing material typeIDs from the SDE blueprint.
// Returns nil if the type has no blueprint in the SDE. No ESI fallback exists: the public
// EVE API does not expose blueprint manufacturing materials; the SDE is the sole source.
func buildIngredientCandidates(sdeData *sde.Data, typeID int32) []int32 {
	if sdeData.Industry == nil {
		return nil
	}
	bpTypeID, ok := sdeData.Industry.ProductToBlueprint[typeID]
	if !ok {
		return nil
	}
	bp, ok := sdeData.Industry.Blueprints[bpTypeID]
	if !ok {
		return nil
	}
	mfg, ok := bp.Activities["manufacturing"]
	if !ok || mfg == nil {
		return nil
	}
	out := make([]int32, 0, len(mfg.Materials))
	for _, m := range mfg.Materials {
		if _, exists := sdeData.Types[m.TypeID]; exists {
			out = append(out, m.TypeID)
		}
	}
	return out
}

// shipGroupToRigSize maps EVE ship group IDs to compatible rig size.
// 1=Small (frigates/destroyers), 2=Medium (cruisers/BCs), 3=Large (battleships), 4=Capital.
// NOTE: Verify all group IDs against current EVE SDE before merging — these are well-known
// but EVE occasionally adds new ship groups.
var shipGroupToRigSize = map[int32]int32{
	// Small
	25: 1, 237: 1, 381: 1, 420: 1, 541: 1, 543: 1, 830: 1, 893: 1,
	// Medium
	26: 2, 419: 2, 540: 2, 831: 2, 894: 2, 898: 2, 900: 2,
	// Large
	27: 3, 513: 3, 833: 3, 834: 3,
	// Capital
	30: 4, 485: 4, 547: 4, 548: 4,
}

// buildRigCandidates returns rig typeIDs whose size matches the given hull, capped at 100.
func buildRigCandidates(sdeData *sde.Data, hullTypeID int32) []int32 {
	hull, ok := sdeData.Types[hullTypeID]
	if !ok {
		return nil
	}
	targetSize, ok := shipGroupToRigSize[hull.GroupID]
	if !ok {
		return nil
	}
	if sdeData.Industry == nil || len(sdeData.Industry.RigSizes) == 0 {
		return nil
	}
	const maxRigs = 100
	out := make([]int32, 0, maxRigs)
	for typeID, size := range sdeData.Industry.RigSizes {
		if size == targetSize {
			out = append(out, typeID)
			if len(out) >= maxRigs {
				break
			}
		}
	}
	return out
}


// buildSharedMaterialCandidates returns module typeIDs that share the top-2 dominant
// blueprint materials (by quantity) with the given module typeID.
func buildSharedMaterialCandidates(sdeData *sde.Data, moduleTypeID int32) []int32 {
	if sdeData.Industry == nil {
		return nil
	}
	bpTypeID, ok := sdeData.Industry.ProductToBlueprint[moduleTypeID]
	if !ok {
		return nil
	}
	bp, ok := sdeData.Industry.Blueprints[bpTypeID]
	if !ok {
		return nil
	}
	mfg, ok := bp.Activities["manufacturing"]
	if !ok || mfg == nil || len(mfg.Materials) == 0 {
		return nil
	}
	// Sort materials by quantity desc, take top 2
	mats := make([]sde.BlueprintMaterial, len(mfg.Materials))
	copy(mats, mfg.Materials)
	sort.Slice(mats, func(i, j int) bool { return mats[i].Quantity > mats[j].Quantity })
	topMats := make(map[int32]bool)
	for i, m := range mats {
		if i >= 2 {
			break
		}
		topMats[m.TypeID] = true
	}
	var out []int32
	for _, bp2 := range sdeData.Industry.Blueprints {
		mfg2, ok2 := bp2.Activities["manufacturing"]
		if !ok2 || mfg2 == nil {
			continue
		}
		var productTypeID int32
		for _, prod := range mfg2.Products {
			if t, exists := sdeData.Types[prod.TypeID]; exists && t.CategoryID == 7 {
				productTypeID = prod.TypeID
				break
			}
		}
		if productTypeID == 0 || productTypeID == moduleTypeID {
			continue
		}
		for _, m2 := range mfg2.Materials {
			if topMats[m2.TypeID] {
				out = append(out, productTypeID)
				break
			}
		}
		if len(out) >= 50 {
			break
		}
	}
	return out
}

// ── Series fetch ──────────────────────────────────────────────────────────────

// fetchHistorySeries returns chronologically sorted daily price and volume series
// for a type in a region, capped at `days` most recent entries.
func (s *Server) fetchHistorySeries(regionID, typeID int32, days int) (prices, volumes []float64, source string, err error) {
	entries, fetchErr := s.cachedMarketHistory(regionID, typeID)
	if fetchErr != nil {
		return nil, nil, "", fetchErr
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Date < entries[j].Date })
	if len(entries) > days {
		entries = entries[len(entries)-days:]
	}
	prices = make([]float64, len(entries))
	volumes = make([]float64, len(entries))
	for i, e := range entries {
		prices[i] = e.Average
		volumes[i] = float64(e.Volume)
	}
	return prices, volumes, "history", nil
}

func last30(s []float64) []float64 {
	if len(s) <= 30 {
		return s
	}
	return s[len(s)-30:]
}

// handleCorrelationSuggestions serves GET /api/correlation/suggestions.
func (s *Server) handleCorrelationSuggestions(w http.ResponseWriter, r *http.Request) {
	typeID, err := parseRequiredInt32Query(r, "type_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid type_id")
		return
	}
	regionID, err := parseRequiredInt32Query(r, "region_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid region_id")
		return
	}

	if !s.isReady() {
		writeError(w, http.StatusServiceUnavailable, "SDE not loaded yet")
		return
	}

	days := 90
	if dRaw := strings.TrimSpace(r.URL.Query().Get("days")); dRaw != "" {
		if d, err2 := strconv.Atoi(dRaw); err2 == nil && (d == 30 || d == 60 || d == 90) {
			days = d
		}
	}

	s.mu.RLock()
	sdeData := s.sdeData
	s.mu.RUnlock()

	item, ok := sdeData.Types[typeID]
	if !ok {
		writeError(w, http.StatusNotFound, "type_id not found")
		return
	}

	// Build candidate set based on item category
	dedupe := make(map[int32]bool)
	var candidates []correlationCandidate

	addCandidates := func(ids []int32, set string) {
		for _, id := range ids {
			if dedupe[id] || id == typeID {
				continue
			}
			t, exists := sdeData.Types[id]
			if !exists {
				continue
			}
			dedupe[id] = true
			candidates = append(candidates, correlationCandidate{TypeID: id, TypeName: t.Name, CandidateSet: set})
		}
	}

	const categoryShip   = int32(6)
	const categoryModule = int32(7)

	ingredientIDs := buildIngredientCandidates(sdeData, typeID)
	addCandidates(ingredientIDs, "ingredient")

	switch item.CategoryID {
	case categoryShip:
		addCandidates(buildRigCandidates(sdeData, typeID), "rig")
	case categoryModule:
		addCandidates(buildSharedMaterialCandidates(sdeData, typeID), "shared_mat")
	}

	primaryPrices, primaryVolumes, primarySource, err := s.fetchHistorySeries(regionID, typeID, days)
	if err != nil || len(primaryPrices) < correlationMinPoints {
		writeJSON(w, correlationSuggestionsResponse{
			TypeID:   typeID,
			TypeName: item.Name,
		})
		return
	}

	var suggestions []correlationSuggestion
	excluded := 0

	for _, c := range candidates {
		cp, cv, csrc, cerr := s.fetchHistorySeries(regionID, c.TypeID, days)
		if cerr != nil || len(cp) < correlationMinPoints {
			excluded++
			continue
		}
		corrVal, lag := pearsonWithLag(primaryPrices, cp, correlationLags, correlationMinPoints)
		if math.IsNaN(corrVal) || math.Abs(corrVal) < correlationMinThreshold {
			excluded++
			continue
		}
		opp := classifyOpportunity(primaryPrices, cp, lag)
		suggestions = append(suggestions, correlationSuggestion{
			TypeID:            c.TypeID,
			TypeName:          c.TypeName,
			CandidateSet:      c.CandidateSet,
			Correlation:       math.Round(corrVal*1000) / 1000,
			LagDays:           lag,
			PrimaryMoved:      recentMovePct(primaryPrices, 7) > opportunityMovePct,
			SecondaryMoved:    recentMovePct(cp, 7) > opportunityMovePct,
			Opportunity:       opp == "NOT_MOVED_YET",
			OpportunityStatus: opp,
			PriceTrend7d:      priceTrend7d(cp),
			VolumeTrend7d:     volumeTrend7d(cv),
			DataSource:        csrc,
			PriceSeries:       last30(cp),
			VolumeSeries:      last30(cv),
		})
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return math.Abs(suggestions[i].Correlation) > math.Abs(suggestions[j].Correlation)
	})

	writeJSON(w, correlationSuggestionsResponse{
		TypeID:          typeID,
		TypeName:        item.Name,
		PriceSeries:     last30(primaryPrices),
		VolumeSeries:    last30(primaryVolumes),
		DataSource:      primarySource,
		CoverageDays:    len(primaryPrices),
		ExcludedCount:   excluded,
		IngredientCount: len(ingredientIDs),
		Suggestions:     suggestions,
	})
}

// parseRequiredInt32Query parses a required positive int32 query param.
func parseRequiredInt32Query(r *http.Request, key string) (int32, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v <= 0 {
		return 0, strconv.ErrRange
	}
	return int32(v), nil
}

// handleCorrelationPair serves GET /api/correlation/pair (implemented in Task 7).
func (s *Server) handleCorrelationPair(w http.ResponseWriter, r *http.Request) {
	typeA, err := parseRequiredInt32Query(r, "type_a")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid type_a")
		return
	}
	typeB, err := parseRequiredInt32Query(r, "type_b")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid type_b")
		return
	}
	regionID, err := parseRequiredInt32Query(r, "region_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid region_id")
		return
	}
	if !s.isReady() {
		writeError(w, http.StatusServiceUnavailable, "SDE not loaded yet")
		return
	}
	days := 90
	if dRaw := strings.TrimSpace(r.URL.Query().Get("days")); dRaw != "" {
		if d, err2 := strconv.Atoi(dRaw); err2 == nil && (d == 30 || d == 60 || d == 90) {
			days = d
		}
	}

	s.mu.RLock()
	sdeData := s.sdeData
	s.mu.RUnlock()

	itemA, okA := sdeData.Types[typeA]
	itemB, okB := sdeData.Types[typeB]
	if !okA || !okB {
		writeError(w, http.StatusNotFound, "one or both type_ids not found")
		return
	}

	pricesA, volsA, srcA, errA := s.fetchHistorySeries(regionID, typeA, days)
	pricesB, volsB, srcB, errB := s.fetchHistorySeries(regionID, typeB, days)
	if errA != nil || errB != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch history")
		return
	}

	corrVal, lag := pearsonWithLag(pricesA, pricesB, correlationLags, correlationMinPoints)
	if math.IsNaN(corrVal) {
		corrVal = 0
	}

	writeJSON(w, correlationPairResponse{
		TypeA:        typeA,
		TypeAName:    itemA.Name,
		TypeAPrices:  pricesA,
		TypeAVolumes: volsA,
		TypeASource:  srcA,
		TypeB:        typeB,
		TypeBName:    itemB.Name,
		TypeBPrices:  pricesB,
		TypeBVolumes: volsB,
		TypeBSource:  srcB,
		Correlation:  math.Round(corrVal*1000) / 1000,
		LagDays:      lag,
	})
}
