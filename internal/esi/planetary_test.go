package esi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCharacterPlanetDetailAcceptsDecimalRouteQuantity(t *testing.T) {
	payload := []byte(`{
		"pins": [],
		"routes": [
			{
				"route_id": 1001,
				"content_type_id": 2393,
				"quantity": 20.0,
				"source_pin_id": 2001,
				"destination_pin_id": 3001
			},
			{
				"route_id": 1002,
				"content_type_id": 2396,
				"quantity": 1585,
				"source_pin_id": 2002,
				"destination_pin_id": 3002
			}
		]
	}`)

	var detail CharacterPlanetDetail
	if err := json.Unmarshal(payload, &detail); err != nil {
		t.Fatalf("unmarshal planet detail: %v", err)
	}
	if got := detail.Routes[0].Quantity; got != 20 {
		t.Fatalf("decimal route quantity = %d, want 20", got)
	}
	if got := detail.Routes[1].Quantity; got != 1585 {
		t.Fatalf("integer route quantity = %d, want 1585", got)
	}
}

func TestCharacterPlanetDetailRejectsFractionalRouteQuantity(t *testing.T) {
	payload := []byte(`{
		"pins": [],
		"routes": [
			{
				"route_id": 1001,
				"content_type_id": 2393,
				"quantity": 20.5,
				"source_pin_id": 2001,
				"destination_pin_id": 3001
			}
		]
	}`)

	var detail CharacterPlanetDetail
	err := json.Unmarshal(payload, &detail)
	if err == nil {
		t.Fatal("expected fractional quantity to fail")
	}
	if !strings.Contains(err.Error(), "not an integer quantity") {
		t.Fatalf("error = %q, want fractional quantity error", err)
	}
}
