package harness

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// composeFile is the slice of docker-compose.yml this file reads: the one
// service it guards, and of that service only what the constants in
// gitfixture.go have to agree with.
type composeFile struct {
	Services map[string]struct {
		Image       string   `yaml:"image"`
		Command     []string `yaml:"command"`
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck"`
	} `yaml:"services"`
}

// loadComposeFile reads and parses test/e2e/docker-compose.yml, resolved
// relative to this package's own directory rather than the process's working
// directory — the same way loadNvidiaDevicePluginManifest resolves its
// manifest, so this test finds the file whether `go test` is invoked from the
// repository root or from inside this package.
func loadComposeFile(t *testing.T) composeFile {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "docker-compose.yml"))
	if err != nil {
		t.Fatalf("resolving docker-compose.yml path: %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // fixed, repo-relative path; not user input
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var parsed composeFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return parsed
}

// TestUnit_GitFixture_ComposeServiceMatchesTheConstants is the drift guard
// for gitfixture.go. Those constants describe a container the Go code never
// builds: the URL a test hands Portainer, the byte-exact stack file it then
// asserts came back, and the path of the script it execs are all decided by
// a shell script embedded in docker-compose.yml, and nothing but this test
// connects the two.
//
// Every check below is one way the pair can silently disagree, and each has
// a failure mode worse than a compile error, because the estate still comes
// up and the e2e suite still runs:
//
//   - a changed URL, port or path makes every create_repository call answer
//     500 from the live server, which reads as a Portainer defect rather
//     than a fixture one;
//   - a changed seed file makes the content assertions fail with a diff
//     nobody can attribute;
//   - a push into the read-only repository rather than the mutable one makes
//     the git_fetch test corrupt what every other test clones, intermittently
//     and only under parallelism.
//
// The two package checks are not decoration either: both were established by
// measurement (Alpine's git package ships no git-http-backend, and the base
// image's busybox has no httpd applet), and dropping either one leaves a
// service that starts, passes `docker compose config`, and fails only when a
// container actually runs.
func TestUnit_GitFixture_ComposeServiceMatchesTheConstants(t *testing.T) {
	compose := loadComposeFile(t)

	service, ok := compose.Services[GitFixtureService]
	if !ok {
		t.Fatalf("docker-compose.yml declares no %q service: the estate cannot serve a git repository", GitFixtureService)
	}

	// A floating tag would let the fixture's git version change under a
	// `docker compose pull` on any host, which is exactly the kind of
	// difference that makes an e2e failure irreproducible.
	if !strings.HasPrefix(service.Image, "alpine:3.") {
		t.Errorf("git service image = %q, want a pinned alpine:3.x tag", service.Image)
	}

	if len(service.Command) != 3 || service.Command[0] != "sh" || service.Command[1] != "-euc" {
		t.Fatalf("git service command = %v, want [sh -euc <script>]", service.Command)
	}
	script := service.Command[2]

	// Compose interpolates this string before the shell ever sees it, so a
	// literal dollar has to be written "$$" — and a "$VAR" that slipped in
	// unescaped is either replaced by an empty string or rejected outright at
	// `up` time. The script is written to need none at all; this keeps it
	// that way.
	if strings.Contains(script, "$") {
		t.Error("the git service's command contains a '$': compose interpolates it before the shell runs, so it must be written '$$' or, better, avoided entirely")
	}

	for _, pkg := range []string{"git", "git-daemon", "busybox-extras"} {
		if !strings.Contains(script, pkg) {
			t.Errorf("the git service's command does not install %q: measured, git-http-backend lives in git-daemon and httpd in busybox-extras, and neither is in the base image", pkg)
		}
	}

	// The two seeded stack files, byte for byte. A test asserting a cloned
	// template's content against these constants proves nothing if the
	// repository was seeded with something else.
	if !strings.Contains(script, GitFixtureStackFile) {
		t.Errorf("the git service does not seed the read-only repository with GitFixtureStackFile:\nwant to find:\n%s\nin:\n%s", GitFixtureStackFile, script)
	}
	if !strings.Contains(script, GitFixtureMutableStackFile) {
		t.Errorf("the git service does not seed the mutable repository with GitFixtureMutableStackFile:\nwant to find:\n%s\nin:\n%s", GitFixtureMutableStackFile, script)
	}

	// Both repositories are addressed by the path of their own URL: the last
	// segment is the bare repository's directory name under the served root.
	readOnlyRepo := gitFixtureRepoName(t, GitFixtureRepositoryURL)
	mutableRepo := gitFixtureRepoName(t, GitFixtureMutableRepositoryURL)
	if readOnlyRepo == mutableRepo {
		t.Fatalf("GitFixtureRepositoryURL and GitFixtureMutableRepositoryURL name the same repository %q: the read-only/mutable split is what stops the git_fetch test's push from changing what every other test clones", readOnlyRepo)
	}
	for _, repo := range []string{readOnlyRepo, mutableRepo} {
		if !strings.Contains(script, "git init --bare -q -b main /srv/"+repo) {
			t.Errorf("the git service does not create the bare repository /srv/%s that its URL names", repo)
		}
	}

	// The commit script must exist at the path the suite execs, and must push
	// into the mutable repository only.
	if !strings.Contains(script, "cat > "+GitFixtureCommitScript+" ") {
		t.Errorf("the git service writes no %s: the git_fetch test execs exactly that path", GitFixtureCommitScript)
	}
	commitScript, ok := gitFixtureCommitBody(script)
	if !ok {
		t.Fatalf("could not read the body of %s out of the git service's command", GitFixtureCommitScript)
	}
	if !strings.Contains(commitScript, "git push -q /srv/"+mutableRepo+" main") {
		t.Errorf("%s does not push into the mutable repository /srv/%s:\n%s", GitFixtureCommitScript, mutableRepo, commitScript)
	}
	if strings.Contains(commitScript, "/srv/"+readOnlyRepo) {
		t.Errorf("%s writes to the read-only repository /srv/%s, which every other test clones concurrently:\n%s", GitFixtureCommitScript, readOnlyRepo, commitScript)
	}

	// The served port comes from the URLs the suite uses, so a port changed
	// in one place and not the other is caught here rather than as a
	// connection refused inside Portainer.
	port := gitFixturePort(t, GitFixtureRepositoryURL)
	if !strings.Contains(script, "httpd -f -p "+port+" -h /www") {
		t.Errorf("the git service does not serve /www on port %s, which is the port GitFixtureRepositoryURL names", port)
	}

	// The healthcheck is the estate's own proof that this whole path works
	// before any test runs, so it must exercise the same smart-HTTP path
	// Portainer uses — the CGI script and the seeded repository included —
	// against the loopback address rather than the service name.
	health := strings.Join(service.Healthcheck.Test, " ")
	wantHealthPath := gitFixturePathWithHost(t, GitFixtureRepositoryURL, "127.0.0.1")
	if !strings.Contains(health, "git ls-remote") || !strings.Contains(health, wantHealthPath) {
		t.Errorf("git service healthcheck = %q, want a `git ls-remote` against %s so a broken CGI or an unseeded repository fails the estate at `up` time", health, wantHealthPath)
	}
}

// gitFixtureRepoName returns the bare repository directory a fixture URL
// names — "repo.git" out of ".../cgi-bin/git/repo.git".
func gitFixtureRepoName(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return segments[len(segments)-1]
}

// gitFixturePort returns the port a fixture URL names.
func gitFixturePort(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	if parsed.Port() == "" {
		t.Fatalf("%q names no port: the compose healthcheck and the served port are both derived from it", raw)
	}
	return parsed.Port()
}

// gitFixturePathWithHost rewrites a fixture URL's host, keeping its port and
// path — how the same repository is addressed from inside its own container.
func gitFixturePathWithHost(t *testing.T, raw, host string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	parsed.Host = host + ":" + parsed.Port()
	return parsed.String()
}

// gitFixtureCommitBody extracts the body of the commit script out of the
// service's start-up script — everything between the heredoc that writes it
// and that heredoc's terminator.
func gitFixtureCommitBody(script string) (string, bool) {
	const marker = "cat > " + GitFixtureCommitScript + " <<'SH'\n"
	_, rest, ok := strings.Cut(script, marker)
	if !ok {
		return "", false
	}
	body, _, ok := strings.Cut(rest, "\nSH\n")
	return body, ok
}
