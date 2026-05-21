package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"eve-flipper/internal/config"
)

func TestAgentQueueRejectsWrongKey(t *testing.T) {
	cfg := config.Default()
	cfg.AgentAPIKey = "correct-key"
	srv := NewServer(cfg, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/queue", nil)
	req.Header.Set("X-Agent-Key", "wrong-key")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}


func TestAgentQueueAcceptsCorrectKey(t *testing.T) {
	cfg := config.Default()
	cfg.AgentAPIKey = "correct-key"
	srv := NewServer(cfg, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/queue", nil)
	req.Header.Set("X-Agent-Key", "correct-key")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Auth passed — expect 503 (no session), not 401
	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentConfirmWritesCooldown(t *testing.T) {
	cfg := config.Default()
	cfg.AgentAPIKey = "testkey"
	d := openAPITestDB(t)
	defer d.Close()
	srv := NewServer(cfg, nil, d, nil, nil)

	body := `{"order_id":999,"type_id":34,"executed_price":5.01,"ok":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent/confirm", strings.NewReader(body))
	req.Header.Set("X-Agent-Key", "testkey")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		SuppressedUntil string `json:"suppressed_until"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SuppressedUntil == "" {
		t.Fatal("expected suppressed_until in response")
	}

	if !d.IsAgentCooldownActive(999, 34) {
		t.Fatal("expected cooldown to be active after confirm")
	}
}

func TestAgentConfirmRejectsMissingOrderID(t *testing.T) {
	cfg := config.Default()
	cfg.AgentAPIKey = "testkey"
	d := openAPITestDB(t)
	defer d.Close()
	srv := NewServer(cfg, nil, d, nil, nil)

	body := `{"order_id":0,"type_id":34,"executed_price":5.01,"ok":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent/confirm", strings.NewReader(body))
	req.Header.Set("X-Agent-Key", "testkey")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing order_id, got %d", w.Code)
	}
}
