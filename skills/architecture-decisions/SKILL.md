# Architecture Decisions

**When to use**: Making or reviewing architectural changes, understanding why something is designed a certain way, or creating a new design decision record.
**Triggers on**: architecture, design decision, ADR, why is, design rationale, architectural change, meta-tool, embedded yaml, read-only, versioning, security
**Covers**: ADR format, existing decisions summary, how to create a new ADR, key architectural constraints

---

## Where Decisions Live

Design decision records (ADRs) are in `docs/design/` with the naming format:

```
YYMMDD-N-short-description.md
```

A summary of all decisions is maintained in `docs/src/content/docs/reference/design-decisions.md` (rendered as the Design Decisions reference page in the docs site).

---

## Existing Decisions Summary

### 202503-1: External Tools File
Tools are defined in `tools.yaml` (external to Go source) rather than hardcoded in Go structs. This allows tool descriptions and parameters to be updated without recompiling, and enables users to supply a custom tools file via `--tools`.

### 202503-2: Tools vs MCP Resources
Tool definitions are exposed as MCP **tools** (callable functions), not MCP **resources** (data sources). This matches how AI assistants interact with Portainer — as actions, not data reads.

### 202503-3: Specific Update Tools
Update operations use dedicated tools per field (e.g., `updateTeamName`, `updateTeamMembers`) rather than a single generic `updateTeam` tool. This gives AI assistants precise control and avoids accidental overwrites of unrelated fields.

### 202504-1: Embedded tools.yaml
`tools.yaml` is embedded into the binary at compile time via `//go:embed` in `internal/tooldef/`. This produces a self-contained binary with no runtime file dependency. Users can still override with `--tools` for customisation.

### 202504-2: tools.yaml Versioning
The tools file carries a `version` field (e.g., `v1.2`). The server enforces a minimum version (`MinimumToolsVersion = "v1.0"`) to reject incompatible custom files. Version format is `v{major}.{minor}`.

### 202504-3: Portainer Version Compatibility
The server validates the connected Portainer instance matches `SupportedPortainerVersion` (major.minor only; patch is flexible). This prevents silent failures from API incompatibilities. Bypass with `--disable-version-check` for development.

### 202504-4: Read-Only Mode
Write tools are excluded at registration time when `--read-only` is passed. This is enforced at the handler registration level, not at the handler call level — write tools simply don't exist in the server's tool registry in read-only mode.

### 202507-1: Meta-Tools
98 individual tools are grouped into 15 meta-tools by default. Each meta-tool accepts an `action` enum parameter. This reduces the tool count visible to AI assistants (which have context limits) while preserving full capability. `--granular-tools` restores individual tool exposure.

### 202602-1: Security Considerations
- No proxy path allowlist — Portainer RBAC provides access control.
- No settings field whitelist — admin tokens already have full access.
- Credentials are never logged by handler code.
- TLS verification is on by default; opt-out via `--skip-tls-verify`.

---

## Key Architectural Constraints

These constraints must be preserved when making changes:

| Constraint | Reason |
|-----------|--------|
| Never log to stdout | stdout is the MCP stdio transport; any non-JSON output breaks the protocol |
| Handlers never touch raw API models | The client layer owns all conversion; handlers only see local models |
| All handlers use `s.cli` interface | Enables mock-based unit testing without HTTP |
| Write tools excluded at registration, not at call time | Cleaner than runtime checks; tools simply don't exist in read-only mode |
| `tools.yaml` uses array `parameters:` format | The YAML parser only recognises this format |
| Tool names are string constants in `schema.go` | Prevents typos and enables refactoring |

---

## Creating a New ADR

1. **Identify the decision** — what architectural choice is being made, and why does it need documenting?

2. **Create the file** in `docs/design/`:
   ```
   YYMMDD-N-short-description.md
   ```
   Use today's date (YYMMDD) and the next sequential number for that month.

3. **Write the ADR** using this template:

   ```markdown
   # <Decision Title>

   ## Date: YYYY-MM

   ## Context

   What problem or situation prompted this decision? What constraints existed?

   ## Decision

   What was decided? State it clearly and directly.

   ## Consequences

   What are the trade-offs? What becomes easier or harder as a result?

   ## Decisions

   | # | Decision | Rationale |
   |---|----------|-----------|
   | D1 | Specific choice made | Why this over alternatives |
   ```

4. **Add a summary entry** to `docs/src/content/docs/reference/design-decisions.md` (the rendered reference page).

5. **Reference the ADR** in relevant code comments or PR descriptions where the decision is implemented.

---

## When to Write an ADR

Write an ADR when:
- A non-obvious architectural choice is made that future contributors might question
- A significant trade-off is accepted (e.g., performance vs. simplicity)
- A pattern is established that should be followed consistently
- An alternative approach was considered and rejected

Do **not** write an ADR for:
- Implementation details that are obvious from the code
- Decisions that can be reversed easily
- Minor style or naming choices
