# Docs Authoring

**When to use**: Adding or updating pages in the Starlight/Astro documentation site, writing design decision records, or understanding the docs structure.
**Triggers on**: docs, documentation, starlight, astro, pnpm, mdx, design decision, ADR, docs site, content page
**Covers**: docs directory structure, pnpm commands, frontmatter format, Starlight components, page locations, design decision record format

---

## Setup

The documentation site lives in `docs/` and uses [Starlight](https://starlight.astro.build/) (Astro). It is managed with `pnpm` — **do not use `npm`**.

```bash
cd docs
pnpm install        # install dependencies
pnpm run dev        # local dev server (localhost:4321)
pnpm run build      # production build → docs/dist/
pnpm run preview    # preview production build locally
```

---

## Directory Structure

```text
docs/
  src/
    content/
      docs/                   # All documentation pages
        index.mdx             # Home page
        getting-started.mdx
        configuration.mdx
        development/          # Developer guides
          adding-tools.mdx
          testing.mdx
          contributing.mdx
          workflow.mdx
          project-structure.mdx
          ci-cd.mdx
          dependencies.mdx
        guides/               # User guides
          meta-tools.mdx
          security.md
          troubleshooting.mdx
        reference/            # Reference docs
          architecture.md
          api-reference.md
          clients-and-models.md
          design-decisions.md
    assets/                   # Images, GIFs
    styles/                   # Custom CSS
  design/                     # Design decision records (ADRs)
  astro.config.mjs            # Starlight/Astro config (sidebar, nav)
  package.json
  pnpm-lock.yaml
```

---

## Page Frontmatter

Every page requires a frontmatter block:

```yaml
---
title: Page Title
description: One-sentence description for SEO and sidebar tooltips.
---
```

For `.mdx` files, you can also import Starlight components after the frontmatter.

---

## Starlight Components

Import components at the top of `.mdx` files (after frontmatter):

```mdx
import { Aside, Steps, Tabs, TabItem } from '@astrojs/starlight/components';
```

### `<Aside>` — callout boxes

```mdx
<Aside type="note">
Informational note.
</Aside>

<Aside type="tip">
Helpful tip.
</Aside>

<Aside type="caution">
Warning about a potential issue.
</Aside>

<Aside type="danger">
Critical warning.
</Aside>
```

### `<Steps>` — numbered procedure

```mdx
<Steps>
1. ### First step heading
   Step content here.

2. ### Second step heading
   More content.
</Steps>
```

### `<Tabs>` / `<TabItem>` — tabbed content

```mdx
<Tabs>
<TabItem label="Go install">
```bash
go install ...
```
</TabItem>
<TabItem label="Docker">
```bash
docker pull ...
```
</TabItem>
</Tabs>
```

---

## Adding a New Page

1. Create the `.mdx` (or `.md`) file in the appropriate subdirectory under `docs/src/content/docs/`.
2. Add frontmatter with `title` and `description`.
3. Register the page in the sidebar in `docs/astro.config.mjs`:

```js
// In the sidebar array, find the right group and add:
{ label: 'My New Page', slug: 'development/my-new-page' }
```

4. Run `pnpm run dev` to verify it renders correctly.

---

## Design Decision Records (ADRs)

Architectural decisions are documented in `docs/design/` using the naming format:

```
YYMMDD-N-short-description.md
```

Examples:
- `202503-1-external-tools-file.md`
- `202504-2-tools-yaml-versioning.md`
- `202602-1-security-considerations.md`

### ADR Template

```markdown
# <Decision Title>

## Date: YYYY-MM

## Context

What problem or situation prompted this decision?

## Decision

What was decided?

## Consequences

What are the trade-offs, benefits, and drawbacks?

## Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | ... | ... |
```

After writing an ADR, add a summary entry to `docs/src/content/docs/reference/design-decisions.md`.

---

## Deployment

The docs site deploys automatically to GitHub Pages via `.github/workflows/deploy-docs.yml` on every push to `main`. No manual deployment is needed.
