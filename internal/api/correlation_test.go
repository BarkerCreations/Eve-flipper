// internal/api/correlation_test.go
package api

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"eve-flipper/internal/config"
	"eve-flipper/internal/esi"
	"eve-flipper/internal/sde"
)

func TestPearson_KnownSeries(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{2, 4, 6, 8, 10} // perfect positive
	r := pearson(a, b)
	if math.Abs(r-1.0) > 1e-9 {
		t.Errorf("pearson perfect positive = %.6f, want 1.0", r)
	}
}

func TestPearson_Constant(t *testing.T) {
	a := []float64{3, 3, 3, 3, 3}
	b := []float64{1, 2, 3, 4, 5}
	r := pearson(a, b)
	if !math.IsNaN(r) {
		t.Errorf("pearson constant series = %.6f, want NaN", r)
	}
}

func TestPearsonWithLag_DetectsLag(t *testing.T) {
	// b is a shifted by 2 days
	a := []float64{1, 3, 5, 7, 9, 11, 13, 15, 17, 19}
	b := []float64{0, 0, 1, 3, 5,  7,  9, 11, 13, 15}
	lags := []int{0, 1, 2, 3}
	r, lag := pearsonWithLag(a, b, lags, 5)
	if lag != 2 {
		t.Errorf("expected lag=2, got lag=%d (r=%.3f)", lag, r)
	}
	if r < 0.99 {
		t.Errorf("expected r≈1.0, got r=%.3f", r)
	}
}

func TestPearsonWithLag_BelowMinPoints(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2, 3}
	r, lag := pearsonWithLag(a, b, []int{0, 1, 2}, 5) // minPoints=5 > len
	if !math.IsNaN(r) || lag != 0 {
		t.Errorf("expected NaN/0 for too-short series, got r=%.3f lag=%d", r, lag)
	}
}

func TestClassifyOpportunity(t *testing.T) {
	// 10 days, primary up >5%, candidate flat
	primary := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 107}
	cand    := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	got := classifyOpportunity(primary, cand, 0)
	if got != "NOT_MOVED_YET" {
		t.Errorf("expected NOT_MOVED_YET, got %s", got)
	}
}

func TestTrend7d(t *testing.T) {
	series := []float64{100, 100, 100, 100, 100, 100, 110} // +10%
	got := trend7d(series)
	if math.Abs(got-10.0) > 0.01 {
		t.Errorf("trend7d = %.2f, want 10.0", got)
	}
}

func TestBuildIngredientCandidates_T1Hull(t *testing.T) {
	// Build minimal SDE with a hull and its blueprint
	sdeData := &sde.Data{
		Types: map[int32]*sde.ItemType{
			587: {ID: 587, Name: "Rifter", GroupID: 25, CategoryID: 6},
			34:  {ID: 34, Name: "Tritanium", GroupID: 18, CategoryID: 4},
		},
		Industry: sde.NewIndustryData(),
	}
	sdeData.Industry.ProductToBlueprint[587] = 2987
	sdeData.Industry.Blueprints[2987] = &sde.Blueprint{
		BlueprintTypeID: 2987,
		ProductTypeID:   587,
		Activities: map[string]*sde.ActivityData{
			"manufacturing": {
				Materials: []sde.BlueprintMaterial{{TypeID: 34, Quantity: 10000}},
			},
		},
	}
	got := buildIngredientCandidates(sdeData, 587)
	if len(got) != 1 || got[0] != 34 {
		t.Errorf("expected [34], got %v", got)
	}
}

func TestBuildIngredientCandidates_NoBP(t *testing.T) {
	sdeData := &sde.Data{
		Types:    map[int32]*sde.ItemType{587: {ID: 587}},
		Industry: sde.NewIndustryData(),
	}
	got := buildIngredientCandidates(sdeData, 587)
	if got != nil {
		t.Errorf("expected nil for unknown typeID, got %v", got)
	}
}

func TestBuildRigCandidates_Frigate(t *testing.T) {
	sdeData := &sde.Data{
		Types: map[int32]*sde.ItemType{
			587:   {ID: 587, GroupID: 25},          // Rifter (frigate)
			31362: {ID: 31362, GroupID: 773},        // some small rig
		},
		Industry: sde.NewIndustryData(),
	}
	sdeData.Industry.RigSizes[31362] = 1 // small rig
	got := buildRigCandidates(sdeData, 587)
	if len(got) != 1 || got[0] != 31362 {
		t.Errorf("expected [31362], got %v", got)
	}
}

func TestClassifyOpportunity_NeitherMoved(t *testing.T) {
	// Both primary and candidate are flat — no signal at all.
	flat := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	got := classifyOpportunity(flat, flat, 0)
	if got != "NO_SIGNAL" {
		t.Errorf("expected NO_SIGNAL for neither moved, got %s", got)
	}
}

func TestBuildRigCandidates_Cap(t *testing.T) {
	sdeData := &sde.Data{
		Types:    map[int32]*sde.ItemType{587: {ID: 587, GroupID: 25}},
		Industry: sde.NewIndustryData(),
	}
	// Add 150 small rigs — expect at most 100 returned.
	for i := int32(30000); i < 30150; i++ {
		sdeData.Types[i] = &sde.ItemType{ID: i, GroupID: 773}
		sdeData.Industry.RigSizes[i] = 1
	}
	got := buildRigCandidates(sdeData, 587)
	if len(got) > 100 {
		t.Errorf("expected at most 100 rig candidates, got %d", len(got))
	}
}

func TestHandleCorrelationSuggestions_UnknownTypeID(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, nil, nil, nil, nil)
	srv.mu.Lock()
	srv.sdeData = &sde.Data{
		Types:    map[int32]*sde.ItemType{587: {ID: 587, Name: "Rifter", GroupID: 25, CategoryID: 6}},
		Industry: sde.NewIndustryData(),
	}
	srv.ready = true
	srv.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/correlation/suggestions?type_id=9999&region_id=10000002", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown type_id: got %d, want 404", rec.Code)
	}
}

func TestBuildSharedMaterialCandidates_FindsSibling(t *testing.T) {
	sdeData := &sde.Data{
		Types: map[int32]*sde.ItemType{
			100: {ID: 100, CategoryID: 7}, // our module
			200: {ID: 200, CategoryID: 7}, // sibling module
			34:  {ID: 34, CategoryID: 4},  // Tritanium (shared mat)
		},
		Industry: sde.NewIndustryData(),
	}
	// Module 100 blueprint: needs Tritanium x1000
	sdeData.Industry.ProductToBlueprint[100] = 1100
	sdeData.Industry.Blueprints[1100] = &sde.Blueprint{
		Activities: map[string]*sde.ActivityData{
			"manufacturing": {
				Materials: []sde.BlueprintMaterial{{TypeID: 34, Quantity: 1000}},
				Products:  []sde.BlueprintProduct{{TypeID: 100, Quantity: 1}},
			},
		},
	}
	// Module 200 blueprint: also needs Tritanium
	sdeData.Industry.ProductToBlueprint[200] = 1200
	sdeData.Industry.Blueprints[1200] = &sde.Blueprint{
		Activities: map[string]*sde.ActivityData{
			"manufacturing": {
				Materials: []sde.BlueprintMaterial{{TypeID: 34, Quantity: 500}},
				Products:  []sde.BlueprintProduct{{TypeID: 200, Quantity: 1}},
			},
		},
	}
	got := buildSharedMaterialCandidates(sdeData, 100)
	if len(got) != 1 || got[0] != 200 {
		t.Errorf("expected [200], got %v", got)
	}
}

func TestHandleCorrelationSuggestions_MissingTypeID(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, &esi.Client{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/correlation/suggestions?region_id=10000002", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing type_id: got %d, want 400", rec.Code)
	}
}

func TestHandleCorrelationSuggestions_MissingRegionID(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, &esi.Client{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/correlation/suggestions?type_id=587", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing region_id: got %d, want 400", rec.Code)
	}
}

func TestHandleCorrelationSuggestions_SDENotReady(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, &esi.Client{}, nil, nil, nil)
	// sdeData is nil → not ready
	req := httptest.NewRequest(http.MethodGet, "/api/correlation/suggestions?type_id=587&region_id=10000002", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("SDE not ready: got %d, want 503", rec.Code)
	}
}

func TestHandleCorrelationPair_MissingParams(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, &esi.Client{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/correlation/pair?type_a=587", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing params: got %d, want 400", rec.Code)
	}
}
