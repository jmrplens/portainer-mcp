package portainer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// systemVersion is the subset of GET /system/version this package needs.
type systemVersion struct {
	ServerVersion string `json:"ServerVersion"`
	ServerEdition string `json:"ServerEdition"`
}

// DetectEdition asks the server for its edition and version.
//
// It issues the request directly through Get rather than the generated
// client because it must run before anything else is wired up, and because
// the two fields it needs are stable across every published version of the
// API. A server that does not identify its edition resolves to CE: showing
// fewer operations than the server actually supports is harmless, showing
// EE-only operations to a CE server is not.
func (c *Client) DetectEdition(ctx context.Context) (edition.Edition, string, error) {
	if c == nil {
		return "", "", errors.New("detect edition: a client is required")
	}

	resp, err := c.Get(ctx, "/system/version")
	if err != nil {
		return "", "", fmt.Errorf("detect edition: query system version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("detect edition: read system version: %w", err)
	}
	if err := ClassifyResponse(resp.StatusCode, body); err != nil {
		return "", "", fmt.Errorf("detect edition: query system version: %w", err)
	}

	var parsed systemVersion
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("detect edition: decode system version: %w", err)
	}

	resolved, err := edition.Parse(parsed.ServerEdition)
	if err != nil || resolved == "" {
		// A server that does not identify its edition is treated as CE.
		return edition.CE, parsed.ServerVersion, nil
	}
	return resolved, parsed.ServerVersion, nil
}
