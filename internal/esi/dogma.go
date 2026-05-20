// internal/esi/dogma.go
package esi

import "fmt"

// DogmaAttribute is a single attribute on a type from ESI /universe/types/{id}/.
type DogmaAttribute struct {
	AttributeID int32   `json:"attribute_id"`
	Value       float64 `json:"value"`
}

// FetchTypeAttributes fetches dogma attributes for the given typeID from ESI.
// Used to identify required skills on ship hulls for module candidate derivation.
func (c *Client) FetchTypeAttributes(typeID int32) ([]DogmaAttribute, error) {
	url := fmt.Sprintf("%s/universe/types/%d/?datasource=tranquility", baseURL, typeID)
	var resp struct {
		DogmaAttributes []DogmaAttribute `json:"dogma_attributes"`
	}
	if err := c.GetJSON(url, &resp); err != nil {
		return nil, err
	}
	return resp.DogmaAttributes, nil
}
