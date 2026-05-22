package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"eve-flipper/internal/engine"
	"eve-flipper/internal/esi"
)

// agentKeyMiddleware checks the X-Agent-Key header against the configured key.
// Returns false and writes an error response if auth fails.
func (s *Server) agentKeyMiddleware(w http.ResponseWriter, r *http.Request) bool {
	key := s.cfg.AgentAPIKey
	if key == "" {
		writeError(w, http.StatusServiceUnavailable, "agent_api_key not configured")
		return false
	}
	if r.Header.Get("X-Agent-Key") != key {
		writeError(w, http.StatusUnauthorized, "invalid agent key")
		return false
	}
	return true
}

// AgentQueueItem is one reprice action for the agent to execute.
type AgentQueueItem struct {
	Rank        int     `json:"rank"`
	TypeID      int32   `json:"type_id"`
	TypeName    string  `json:"type_name"`
	OrderID     int64   `json:"order_id"`
	IsBuy       bool    `json:"is_buy_order"`
	BestBid     float64 `json:"best_bid"`
	BestAsk     float64 `json:"best_ask"`
	TargetPrice float64 `json:"target_price"`
	Priority    int     `json:"priority"`
	Reason      string  `json:"reason"`
}

// handleAgentQueue returns a prioritised list of reprice actions derived from
// the latest cached station scan and the character's live orders.
// GET /api/agent/queue
func (s *Server) handleAgentQueue(w http.ResponseWriter, r *http.Request) {
	if !s.agentKeyMiddleware(w, r) {
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "db unavailable")
		return
	}
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "no authenticated session")
		return
	}

	scanID, ok := s.db.GetLatestStationScanID()
	if !ok {
		writeJSON(w, map[string]interface{}{"generated_at": time.Now().UTC().Format(time.RFC3339), "count": 0, "items": []AgentQueueItem{}})
		return
	}

	trades := s.db.GetStationResults(scanID)
	if len(trades) == 0 {
		writeJSON(w, map[string]interface{}{"generated_at": time.Now().UTC().Format(time.RFC3339), "count": 0, "items": []AgentQueueItem{}})
		return
	}

	sess := s.sessions.GetAny()
	if sess == nil {
		writeError(w, http.StatusServiceUnavailable, "no authenticated session")
		return
	}
	token, err := s.sessions.EnsureValidTokenAny(s.sso)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "token refresh failed")
		return
	}

	activeOrders, err := s.esi.GetCharacterOrders(sess.CharacterID, token)
	if err != nil {
		log.Printf("[agent] GetCharacterOrders error: %v", err)
		activeOrders = nil
	}

	cmd := engine.BuildStationCommand(trades, activeOrders, nil)

	ordersByType := make(map[int32][]esi.CharacterOrder)
	for _, o := range activeOrders {
		ordersByType[o.TypeID] = append(ordersByType[o.TypeID], o)
	}

	var items []AgentQueueItem
	for _, row := range cmd.Rows {
		if row.RecommendedAction != engine.StationActionReprice {
			continue
		}
		t := row.Trade
		orders := ordersByType[t.TypeID]
		if len(orders) == 0 {
			continue
		}
		order := orders[0]
		if s.db.IsAgentCooldownActive(order.OrderID, t.TypeID) {
			continue
		}

		var target float64
		if order.IsBuyOrder {
			target = t.BuyPrice + 0.01
		} else {
			target = t.SellPrice - 0.01
		}

		items = append(items, AgentQueueItem{
			Rank:        len(items) + 1,
			TypeID:      t.TypeID,
			TypeName:    t.TypeName,
			OrderID:     order.OrderID,
			IsBuy:       order.IsBuyOrder,
			BestBid:     t.BuyPrice,
			BestAsk:     t.SellPrice,
			TargetPrice: target,
			Priority:    row.Priority,
			Reason:      row.ActionReason,
		})
	}

	if items == nil {
		items = []AgentQueueItem{}
	}

	writeJSON(w, map[string]interface{}{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"count":        len(items),
		"items":        items,
	})
}

// handleAgentConfirm marks an order as executed and starts its 6-minute cooldown.
// POST /api/agent/confirm
func (s *Server) handleAgentConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.agentKeyMiddleware(w, r) {
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "db unavailable")
		return
	}

	var req struct {
		OrderID       int64   `json:"order_id"`
		TypeID        int32   `json:"type_id"`
		ExecutedPrice float64 `json:"executed_price"`
		OK            bool    `json:"ok"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.OrderID == 0 {
		writeError(w, http.StatusBadRequest, "order_id required")
		return
	}

	const cooldown = 6 * time.Minute
	until := time.Now().Add(cooldown)
	s.db.SetAgentCooldownUntil(req.OrderID, req.TypeID, until)

	log.Printf("[agent] confirm order_id=%d type_id=%d price=%.2f ok=%v suppressed_until=%s",
		req.OrderID, req.TypeID, req.ExecutedPrice, req.OK, until.Format(time.RFC3339))

	writeJSON(w, map[string]string{
		"suppressed_until": until.UTC().Format(time.RFC3339),
	})
}

// handleAgentState returns wallet balance, open order count, and actionable queue depth.
// GET /api/agent/state
func (s *Server) handleAgentState(w http.ResponseWriter, r *http.Request) {
	if !s.agentKeyMiddleware(w, r) {
		return
	}
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "no authenticated session")
		return
	}

	sess := s.sessions.GetAny()
	if sess == nil {
		writeError(w, http.StatusServiceUnavailable, "no authenticated session")
		return
	}
	token, err := s.sessions.EnsureValidTokenAny(s.sso)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "token refresh failed")
		return
	}

	var walletISK float64
	if balance, err := s.esi.GetWalletBalance(sess.CharacterID, token); err == nil {
		walletISK = balance
	}

	activeOrders, _ := s.esi.GetCharacterOrders(sess.CharacterID, token)

	queueDepth := 0
	if s.db != nil {
		if scanID, ok := s.db.GetLatestStationScanID(); ok {
			trades := s.db.GetStationResults(scanID)
			if len(trades) > 0 {
				cmd := engine.BuildStationCommand(trades, activeOrders, nil)
				ordersByType := make(map[int32][]esi.CharacterOrder)
				for _, o := range activeOrders {
					ordersByType[o.TypeID] = append(ordersByType[o.TypeID], o)
				}
				for _, row := range cmd.Rows {
					if row.RecommendedAction != engine.StationActionReprice {
						continue
					}
					os := ordersByType[row.Trade.TypeID]
					if len(os) > 0 && !s.db.IsAgentCooldownActive(os[0].OrderID, row.Trade.TypeID) {
						queueDepth++
					}
				}
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"wallet_isk":       walletISK,
		"open_order_count": len(activeOrders),
		"queue_depth":      queueDepth,
	})
}
