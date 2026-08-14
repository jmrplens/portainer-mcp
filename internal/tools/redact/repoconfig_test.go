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

	in := &apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig{
		URL: &url,
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
	if in.Authentication == nil || in.Authentication.Password == nil {
		t.Error("RepoConfig mutated its argument; it must copy")
	}
}

func TestUnit_RepoConfig_NilInput_ReturnsNil(t *testing.T) {
	if got := RepoConfig(nil); got != nil {
		t.Errorf("RepoConfig(nil) = %+v, want nil", got)
	}
}
