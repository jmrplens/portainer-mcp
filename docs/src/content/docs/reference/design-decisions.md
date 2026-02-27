---
title: Design Decisions
description: Key architectural decisions and their rationale.
---

## Decision Log

Design decisions are documented individually in the `docs/design/` directory of the repository. Each document follows a standard template and is named using the convention `YYMMDD-N-short-description.md`.

| Date | Decision | Status |
|:-----|:---------|:-------|
| 2025-07 | Meta-tools grouping (15 meta-tools from 98 tools) | ✅ Implemented |
| 2025-05 | Feature toggles for tool registration modes | ✅ Implemented |

## Key Decisions

### Meta-Tools vs. Granular Tools

**Problem**: LLMs struggle with tool selection accuracy when presented with ~100 tools.

**Decision**: Group tools into 15 domain-specific meta-tools (default mode), with a `--granular-tools` flag for backward compatibility.

**Rationale**:
- Reduces LLM decision space from 98 to 15 options
- Each meta-tool is a natural domain grouping (environments, stacks, Docker, etc.)
- The `action` enum within each meta-tool provides fine-grained control
- Backward compatibility preserved via flag

### Dual Client Architecture

**Problem**: The raw Portainer SDK client is complex and exposes many fields irrelevant to MCP.

**Decision**: Create a wrapper client that simplifies the interface and transforms data into local models.

**Rationale**:
- Clean separation between API layer and MCP handlers
- Local models only contain relevant fields
- Easier to test (mock the wrapper, not the SDK)
- Conversion functions centralize data transformation

### Embedded Tool Definitions

**Problem**: Tool definitions need to be versioned and distributable as part of the binary.

**Decision**: Embed `tools.yaml` in the Go binary using `go:embed`, with optional file override.

**Rationale**:
- Single binary distribution (no external files required)
- Version validation prevents stale definitions
- Override mechanism allows customization without rebuilding
- YAML format is readable and maintainable

### Version-Pinned Compatibility

**Problem**: Portainer API changes between versions can break MCP operations.

**Decision**: Pin each MCP release to a specific Portainer version, with startup validation.

**Rationale**:
- Prevents silent failures from API changes
- Clear error messages when version mismatch detected
- Escape hatch via `--disable-version-check` for advanced users
- Version table in documentation provides upgrade guidance

### Read-Only Mode

**Problem**: AI assistants might accidentally modify infrastructure.

**Decision**: Implement a `-read-only` flag that filters out all write operations at registration time.

**Rationale**:
- Tools are never registered, not just blocked — AI cannot discover them
- Works with both meta-tools and granular modes
- Based on MCP `readOnlyHint` annotations already in tool definitions
- Zero risk of accidental modifications

## Template

When adding new design decisions, use this template:

```markdown
# Title

## Status
Proposed / Accepted / Implemented / Superseded

## Context
What is the issue that we're seeing that is motivating this decision?

## Decision
What is the change that we're proposing?

## Consequences
What becomes easier or more difficult because of this change?
```

Save as `docs/design/YYMMDD-N-short-description.md` and add to the table above.
