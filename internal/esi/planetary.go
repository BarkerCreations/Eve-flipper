package esi

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// CharacterPlanet is the ESI summary row for one character PI colony.
type CharacterPlanet struct {
	PlanetID      int32  `json:"planet_id"`
	PlanetType    string `json:"planet_type"`
	SolarSystemID int32  `json:"solar_system_id"`
	UpgradeLevel  int32  `json:"upgrade_level"`
	NumPins       int32  `json:"num_pins"`
	LastUpdate    string `json:"last_update"`
}

// PlanetaryPinContent is the current content reported for a PI pin.
type PlanetaryPinContent struct {
	TypeID   int32 `json:"type_id"`
	Quantity int64 `json:"amount"`
}

// PlanetaryExtractorDetails is present on extractor control unit pins.
type PlanetaryExtractorDetails struct {
	CycleTime     int64   `json:"cycle_time,omitempty"`
	ProductTypeID int32   `json:"product_type_id,omitempty"`
	QtyPerCycle   int64   `json:"qty_per_cycle,omitempty"`
	HeadRadius    float64 `json:"head_radius,omitempty"`
}

// PlanetaryPin is a pin row from the PI colony detail endpoint.
type PlanetaryPin struct {
	PinID            int64                      `json:"pin_id"`
	TypeID           int32                      `json:"type_id"`
	SchematicID      int32                      `json:"schematic_id,omitempty"`
	LastCycleStart   string                     `json:"last_cycle_start,omitempty"`
	ExpiryTime       string                     `json:"expiry_time,omitempty"`
	InstallTime      string                     `json:"install_time,omitempty"`
	Contents         []PlanetaryPinContent      `json:"contents,omitempty"`
	ExtractorDetails *PlanetaryExtractorDetails `json:"extractor_details,omitempty"`
}

// PlanetaryRoute is a route row from the PI colony detail endpoint.
type PlanetaryRoute struct {
	RouteID          int64 `json:"route_id"`
	ContentTypeID    int32 `json:"content_type_id"`
	Quantity         int64 `json:"quantity"`
	SourcePinID      int64 `json:"source_pin_id"`
	DestinationPinID int64 `json:"destination_pin_id"`
}

// UnmarshalJSON accepts ESI's route quantity as either an integer JSON number
// or an integer-valued decimal such as 20.0.
func (r *PlanetaryRoute) UnmarshalJSON(data []byte) error {
	var raw struct {
		RouteID          int64       `json:"route_id"`
		ContentTypeID    int32       `json:"content_type_id"`
		Quantity         json.Number `json:"quantity"`
		SourcePinID      int64       `json:"source_pin_id"`
		DestinationPinID int64       `json:"destination_pin_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	quantity, err := parsePlanetaryInt64(raw.Quantity)
	if err != nil {
		return fmt.Errorf("quantity: %w", err)
	}
	r.RouteID = raw.RouteID
	r.ContentTypeID = raw.ContentTypeID
	r.Quantity = quantity
	r.SourcePinID = raw.SourcePinID
	r.DestinationPinID = raw.DestinationPinID
	return nil
}

func parsePlanetaryInt64(n json.Number) (int64, error) {
	text := n.String()
	if text == "" {
		return 0, nil
	}
	if i, err := n.Int64(); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f > math.MaxInt64 || f < math.MinInt64 {
		return 0, fmt.Errorf("out of int64 range: %s", text)
	}
	rounded := math.Round(f)
	if math.Abs(f-rounded) > 1e-9 {
		return 0, fmt.Errorf("not an integer quantity: %s", text)
	}
	return int64(rounded), nil
}

// CharacterPlanetDetail is the detailed PI colony layout.
type CharacterPlanetDetail struct {
	Pins   []PlanetaryPin   `json:"pins"`
	Routes []PlanetaryRoute `json:"routes"`
}

// GetCharacterPlanets fetches PI colony summaries for a character.
func (c *Client) GetCharacterPlanets(characterID int64, accessToken string) ([]CharacterPlanet, error) {
	url := fmt.Sprintf("%s/characters/%d/planets/?datasource=tranquility", baseURL, characterID)
	var planets []CharacterPlanet
	if err := c.AuthGetJSON(url, accessToken, &planets); err != nil {
		return nil, fmt.Errorf("character planets: %w", err)
	}
	return planets, nil
}

// GetCharacterPlanetDetail fetches the detailed layout for one PI colony.
func (c *Client) GetCharacterPlanetDetail(characterID int64, planetID int32, accessToken string) (*CharacterPlanetDetail, error) {
	url := fmt.Sprintf("%s/characters/%d/planets/%d/?datasource=tranquility", baseURL, characterID, planetID)
	var detail CharacterPlanetDetail
	if err := c.AuthGetJSON(url, accessToken, &detail); err != nil {
		return nil, fmt.Errorf("character planet detail: %w", err)
	}
	return &detail, nil
}
