// Package edition models Portainer's Community and Business editions.
//
// EE exposes 179 operations that CE does not, so the edition decides which
// actions exist at all. It is detected from the server rather than assumed, and
// an unknown answer resolves to CE: presenting fewer operations than exist is
// harmless, while presenting EE-only operations to a CE server produces calls
// that can only fail.
//
// This package is deliberately dependency-free: it imports nothing from this
// project so that both internal/config and internal/portainer can depend on
// it without creating a cycle. Server detection lives on portainer.Client
// (DetectEdition), not here.
package edition

import (
	"fmt"
	"strings"
)

// Edition identifies a Portainer distribution.
type Edition string

// The two editions Portainer ships.
const (
	CE Edition = "CE"
	EE Edition = "EE"
)

// Parse converts a configured value into an Edition. An empty string means
// "not configured" and is returned unchanged so callers can fall back to
// detection.
func Parse(s string) (Edition, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case "CE":
		return CE, nil
	case "EE":
		return EE, nil
	default:
		return "", fmt.Errorf("invalid edition %q: want ce or ee", s)
	}
}

// Includes reports whether an operation requiring the given edition is
// available here. EE is a superset of CE; the converse does not hold.
func (e Edition) Includes(required Edition) bool {
	if required == CE {
		return e == CE || e == EE
	}
	return e == required
}
