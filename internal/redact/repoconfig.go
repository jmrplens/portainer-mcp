// Package redact removes credential-bearing fields from Portainer API
// responses before they reach a model.
//
// A tool result is read by a model and lands in transcripts, so a credential
// that travels in one is disclosed regardless of what the caller does with
// it. Whether Portainer actually populates a given credential field on a
// given response is not something this code should have to know: these
// functions redact unconditionally.
package redact

import (
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
)

// RepoConfig returns a copy of c with its git credentials removed.
//
// Authentication is dropped whole rather than field by field. It carries
// Password, but also Username and GitCredentialID, and enumerating the
// fields to blank would invite the same omission again the day Portainer
// adds one — the same reasoning internal/tools/registries drops
// ManagementConfiguration.TLSConfig whole. Everything a model has a
// legitimate use for (URL, ReferenceName, ConfigFilePath, ConfigHash,
// TLSSkipVerify) is outside the dropped sub-object.
//
// The same *GithubComPortainerPortainerEeApiGitTypesRepoConfig is reached
// from PortainereeCustomTemplate, PortainereeStack, StacksStackResponse and
// PortainereeEdgeStack, so this one function serves every git-backed
// domain's wrappers.
func RepoConfig(c *apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig) *apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig {
	if c == nil {
		return nil
	}
	scrubbed := *c
	scrubbed.Authentication = nil
	return &scrubbed
}
