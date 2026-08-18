package redact

import (
	"testing"

	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
)

func TestUnit_RepoConfig_DropsTheWholeAuthenticationSubObject(t *testing.T) {
	password := "s3cret"
	username := "deploy-bot"
	credentialID := 7
	url := "https://git.example.com/team/app.git"
	configFilePath := "docker-compose.yml"
	configHash := "bc4c183d756879ea4d173315338110b31004b8e0"
	referenceName := "refs/heads/branch_name"
	tlsSkipVerify := true

	// Every non-credential field on GithubComPortainerPortainerEeApiGitTypesRepoConfig
	// (internal/portainer/gen/types.gen.go) is populated here, so an
	// implementation that reconstructs a struct field-by-field instead of
	// copying and clearing Authentication cannot pass by accident: it would
	// have to name every one of these fields correctly, at which point it is
	// no longer the bug this test exists to catch.
	in := &apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig{
		URL:            &url,
		ConfigFilePath: &configFilePath,
		ConfigHash:     &configHash,
		ReferenceName:  &referenceName,
		TLSSkipVerify:  &tlsSkipVerify,
		Authentication: &apigen.GithubComPortainerPortainerApiGitTypesGitAuthentication{
			Password:        &password,
			Username:        &username,
			GitCredentialID: &credentialID,
		},
	}

	got := RepoConfig(in)

	if got.Authentication != nil {
		t.Errorf("RepoConfig kept Authentication = %+v, want nil", got.Authentication)
	}
	if got.URL == nil || *got.URL != url {
		t.Errorf("RepoConfig dropped URL = %v, want it preserved as %q", got.URL, url)
	}
	if got.ConfigFilePath == nil || *got.ConfigFilePath != configFilePath {
		t.Errorf("RepoConfig dropped ConfigFilePath = %v, want it preserved as %q", got.ConfigFilePath, configFilePath)
	}
	if got.ConfigHash == nil || *got.ConfigHash != configHash {
		t.Errorf("RepoConfig dropped ConfigHash = %v, want it preserved as %q", got.ConfigHash, configHash)
	}
	if got.ReferenceName == nil || *got.ReferenceName != referenceName {
		t.Errorf("RepoConfig dropped ReferenceName = %v, want it preserved as %q", got.ReferenceName, referenceName)
	}
	if got.TLSSkipVerify == nil || *got.TLSSkipVerify != tlsSkipVerify {
		t.Errorf("RepoConfig dropped TLSSkipVerify = %v, want it preserved as %v", got.TLSSkipVerify, tlsSkipVerify)
	}
	if in.Authentication == nil || in.Authentication.Password == nil {
		t.Error("RepoConfig mutated its argument; it must copy")
	}
}

func TestUnit_RepoConfig_NilInput_ReturnsNil(t *testing.T) {
	if got := RepoConfig(nil); got != nil {
		t.Errorf("RepoConfig(nil) = %+v, want nil", got)
	}
}
