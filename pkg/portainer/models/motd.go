package models

import (
	"encoding/json"
)

// MOTD represents the message of the day
type MOTD struct {
	Title         string            `json:"title"`
	Message       string            `json:"message"`
	Style         string            `json:"style"`
	Hash          json.RawMessage   `json:"hash"`
	ContentLayout map[string]string `json:"contentLayout,omitempty"`
}

// ConvertToMOTDFromMap converts a raw map response to a local MOTD.
// This bypasses the SDK model to handle Hash type mismatches between
// SDK versions ([]int64) and newer API versions (string).
func ConvertToMOTDFromMap(raw map[string]any) MOTD {
	motd := MOTD{}

	if v, ok := raw["Title"].(string); ok {
		motd.Title = v
	}
	if v, ok := raw["Message"].(string); ok {
		motd.Message = v
	}
	if v, ok := raw["Style"].(string); ok {
		motd.Style = v
	}
	if v, ok := raw["Hash"]; ok {
		hashJSON, _ := json.Marshal(v)
		motd.Hash = hashJSON
	}
	if v, ok := raw["ContentLayout"].(map[string]any); ok {
		motd.ContentLayout = make(map[string]string, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				motd.ContentLayout[k] = s
			}
		}
	}

	return motd
}
