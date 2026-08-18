package harness

// The estate's git fixture, as the suite has to address it.
//
// docker-compose.yml's `git` service serves two repositories over smart HTTP
// on the estate's own compose network. Smart HTTP rather than anything
// lighter because it is the only transport BOTH editions clone from:
// measured, dumb HTTP fails on both ("unexpected EOF") and git:// works on
// Business Edition but never on Community Edition, which answers
// "invalid auth method" without so much as resolving the hostname (see that
// file's own comment and docs/api-divergences.md §3.8). These constants are
// what a test passes to custom_templates.create_repository, and what the
// container's own start-up script has to agree with.
//
// They are constants here rather than fields on Estate because nothing about
// them is discovered at provisioning time: the URL is a compose service name,
// a fixed port and a fixed path, identical on every host, local or remote,
// licensed or not. What they cannot do on their own is stay in step with the
// compose file — so TestUnit_GitFixture_ComposeServiceMatchesTheConstants
// reads docker-compose.yml and fails if any of them drifts from what the
// service actually seeds and serves.
const (
	// ComposeProject is the compose project every estate service belongs to
	// (docker-compose.yml's own `name:`). A test that has to reach a service
	// container directly — `docker exec` into the git fixture, say — finds it
	// by this label rather than by a container name compose is free to
	// number differently.
	ComposeProject = "portainer-mcp-e2e"

	// GitFixtureService is the compose service name of the git server. It is
	// also the hostname Portainer reaches it by, which is why the URLs below
	// begin with it.
	GitFixtureService = "git"

	// GitFixtureRepositoryURL is the read-only fixture repository: seeded
	// once at start-up and never written to again, so any number of tests can
	// clone it concurrently and assert on its exact content.
	//
	// The ".git" suffix is sent and Portainer stores the URL without it (
	// measured: GitConfig.URL comes back as ".../cgi-bin/git/repo"), so a
	// test comparing a template's stored URL against this constant must not
	// expect them to be byte-identical.
	GitFixtureRepositoryURL = "http://git:8080/cgi-bin/git/repo.git"

	// GitFixtureMutableRepositoryURL is the second repository, the one
	// /commit.sh pushes into. Only the custom_templates.git_fetch test uses
	// it, and it exists precisely so that test's push cannot change what a
	// concurrent clone of the read-only repository above sees.
	GitFixtureMutableRepositoryURL = "http://git:8080/cgi-bin/git/mutable.git"

	// GitFixtureConfigFilePath is the stack file's path inside both
	// repositories — what a create_repository call passes as
	// composeFilePathInRepository.
	GitFixtureConfigFilePath = "docker-compose.yml"

	// GitFixtureCommitScript is the path, inside the git container, of the
	// script that commits a new stack file (read from stdin) and pushes it
	// into the mutable repository.
	GitFixtureCommitScript = "/commit.sh"

	// GitFixtureStackFile is the exact content of GitFixtureConfigFilePath in
	// the read-only repository, byte for byte, so a test can assert that
	// Portainer really cloned this repository rather than merely answering
	// 200. The compose file writes it with a heredoc; the drift guard in
	// gitfixture_test.go is what keeps the two in step.
	//
	// Its service sleeps rather than exiting, and that is a measured
	// requirement of wave 1 stage C rather than a stylistic choice. Stage B
	// only ever cloned this file into a custom template, which deploys
	// nothing. stacks.create_docker_swarm_repository deploys it as a real
	// Swarm service, and a Swarm task that exits is restarted forever, so
	// the service never converges: measured against this estate, the stack
	// sat at StackStatusDeploying indefinitely while holding Portainer's own
	// stack lock, and every concurrent stack operation queued behind it
	// until it timed out. See docs/api-divergences.md section 2.9 and the
	// note beside the `git` service in test/e2e/docker-compose.yml.
	GitFixtureStackFile = "services:\n" +
		"  hello:\n" +
		"    image: busybox:1.36\n" +
		"    command: [\"sh\", \"-c\", \"echo portainer-mcp e2e git fixture revision one; sleep 86400\"]\n"

	// GitFixtureMutableStackFile is the same for the mutable repository's
	// initial commit — the content a template or stack created from it
	// carries until something pushes over it. It is written the same way as
	// GitFixtureStackFile above, for the reason stated there.
	GitFixtureMutableStackFile = "services:\n" +
		"  hello:\n" +
		"    image: busybox:1.36\n" +
		"    command: [\"sh\", \"-c\", \"echo portainer-mcp e2e mutable fixture, initial revision; sleep 86400\"]\n"
)
