# Release Workflow

**When to use**: Cutting a new release, bumping the version, publishing Docker images, or understanding how the release pipeline works.
**Triggers on**: release, version, tag, goreleaser, docker image, ghcr, changelog, publish, v* tag, homebrew
**Covers**: version ldflags, tag naming, GoReleaser config, Docker image publishing, CHANGELOG format, Homebrew tap, CI release job

---

## Overview

Releases are fully automated via GoReleaser, triggered by pushing a `v*` tag to `main`. The pipeline builds multi-platform binaries, creates a GitHub Release with checksums, and pushes Docker images to `ghcr.io`.

---

## Pre-Release Checklist

1. **All tests pass** on `main`:
   ```bash
   make test-all
   ```

2. **Update `CHANGELOG.md`** — add a new section for the version being released. Follow the existing format:
   ```markdown
   ## [v1.2.3] - 2026-04-08

   ### Added
   - ...

   ### Fixed
   - ...
   ```

3. **Verify the version constant** — the binary version is injected at build time via ldflags. No source file needs manual version bumping; the tag drives everything.

4. **Check GoReleaser config** is valid:
   ```bash
   goreleaser check
   ```

---

## Cutting a Release

```bash
# Ensure you are on main with a clean working tree
git checkout main
git pull origin main
git status   # must be clean

# Create and push the tag (triggers the release workflow)
git tag v1.2.3
git push origin v1.2.3
```

The `release.yml` workflow runs automatically on tag push. Monitor it at:
`https://github.com/jmrplens/portainer-mcp-enhanced/actions`

---

## Version Injection (ldflags)

The binary embeds version info at compile time. In `.goreleaser.yaml`:

```yaml
ldflags:
  - -X main.Version={{.Version}}
  - -X main.Commit={{.ShortCommit}}
  - -X main.BuildDate={{.Date}}
```

These map to variables in `cmd/portainer-mcp-enhanced/main.go`. For local builds:

```bash
make build
# Version will be "dev" unless ldflags are set manually
```

---

## GoReleaser Configuration (`.goreleaser.yaml`)

Key sections:

| Section | Purpose |
|---------|---------|
| `builds` | Compiles for linux/darwin/windows × amd64/arm64, `CGO_ENABLED=0` |
| `archives` | Packages binaries as `.tar.gz` (`.zip` on Windows), includes `LICENSE`, `README.md`, `tools.yaml` |
| `checksum` | Generates `checksums.txt` with SHA256 hashes |
| `dockers` | Builds and pushes Docker images to `ghcr.io/jmrplens/portainer-mcp-enhanced` |
| `release` | Creates GitHub Release with auto-generated changelog from commits |

---

## Docker Images

Images are pushed to GitHub Container Registry:

```
ghcr.io/jmrplens/portainer-mcp-enhanced:latest
ghcr.io/jmrplens/portainer-mcp-enhanced:v1.2.3
```

The release workflow authenticates with `GITHUB_TOKEN` (no additional secrets needed for GHCR).

---

## Homebrew Tap

If a Homebrew tap is configured, the `HOMEBREW_TAP_TOKEN` secret must be set in the repository. This is a PAT with write access to the tap repository. Without it, the release still succeeds but the Homebrew formula is not updated.

---

## Required Secrets

| Secret | Purpose | Required |
|--------|---------|---------|
| `GITHUB_TOKEN` | GitHub Release + GHCR push | Auto-provided by Actions |
| `HOMEBREW_TAP_TOKEN` | Update Homebrew tap formula | Optional |

---

## Post-Release

After the release workflow completes:

1. Verify the GitHub Release page has all platform binaries and `checksums.txt`.
2. Verify Docker images are available: `docker pull ghcr.io/jmrplens/portainer-mcp-enhanced:v1.2.3`
3. Update the `README.md` badge if the Portainer version support changed.
4. Close any milestone associated with the release on GitHub Issues.
