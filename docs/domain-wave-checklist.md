# Domain wave checklist

This is the procedure every P3 wave follows to add one or more domains to the
action catalog. It exists because the thing it guards against — the vendored
specification disagreeing with the Portainer server it describes — has
already happened four times in this project's history before
`cmd/audit_spec_reality` existed to look for it systematically (see that
command's own package doc for the four: an optional-looking header that is
actually mandatory, a feature claimed for free that is not delivered, a
documented mutation that silently does not persist, and a field typed wrong
for what it plainly holds), was found by accident every time, and does not
announce itself. `docs/api-divergences.md` catalogues six broader categories
of divergence — a wider scope than just this historical, found-by-accident
count, since it also covers what the systematic route-existence audit itself
now finds. A wave that skips a step here is not saving time; it is choosing
not to look for the next one.

Nothing in Steps 1-5 is optional. Step 6 (moving the coverage ratchet) is
mechanical but must land in the same commit as the domain, never a follow-up.

Every divergence found so far is catalogued in `docs/api-divergences.md`.
Read your domain's entries there before Step 1; that is also where this
wave's own findings are recorded, in Step 6.

## Step 1 — Generate

1. Create `internal/tools/<domain>/<domain>.go` if the domain package does
   not exist yet: package doc comment, a `Specs()` function, and (once the
   generator has run once) a `narrative(operationID string)
   toolutil.ActionNarrative` hook for any `Title`/`Description` override the
   vendored spec's own summary/description does not already say well. See
   `internal/tools/system/system.go` for the shape a domain settles into once
   some of its actions are generated and one is kept hand-written.
2. Confirm `internal/toolutil.DomainTags` already maps the domain's directory
   name to the OpenAPI tag(s) it covers. This table is the only place
   "directory name" and "OpenAPI tag" are reconciled — a domain absent from
   it, or a tag with real operations no domain claims, is a build error the
   next step raises, not a silently empty generated file.
3. Run:

   ```sh
   make gen-action-inputs
   ```

   This regenerates `internal/tools/<domain>/inputs.gen.go` (one Input struct
   per operation) and `actions.gen.go` (one generated `ActionSpec` + handler
   per mechanical operation) for every domain directory it finds under
   `internal/tools`, not only the one this wave is adding. Expect it to
   refuse loudly rather than guess: an ambiguous shape, a wire-type width
   mismatch, or a credential-shaped success response with no declared
   redaction wrapper all abort the whole run, naming the operation. That is
   the generator working as designed (see `cmd/gen_action_inputs`'s package
   doc) — resolve the refusal in the domain file, do not work around it.
4. Wire the new domain's `Specs()` into every place that still collects
   domain packages by hand: `internal/wiring` (the real server), and each of
   `cmd/audit_1to1`, `cmd/audit_e2e_gaps` and `cmd/audit_discovery`'s own
   `allCatalogSpecs`/`allSpecs` functions. There is no single registry yet —
   each of these currently lists the pilot domains by hand, and a wave that
   forgets one gets a build that compiles and an audit that silently ignores
   the new domain.

## Step 2 — Read the generated names

Before running anything else, read `internal/tools/<domain>/actions.gen.go`
top to bottom:

- Does every action name read as what a model would search `portainer_find_action`
  for? Task 1's own naming rules produce a handful of results a human has to
  judge (an operationId that is its own domain prefix, an initialism the
  splitter does not know) — those are refused by name, not silently
  mis-derived, but still worth a second look.
- Did `gen_action_inputs` print any verb-mismatch or weak-description
  warnings for this domain to stderr? Every operation whose path or
  identifier suggests deletion but whose HTTP verb does not is listed there,
  and a wave reviews that list before accepting the generated
  `DestructiveHint` values as correct.
- Does every mutating action either carry no credential-shaped field in its
  success response, or does the domain declare the `redact<OperationID>`
  wrapper the generator now requires for the ones that do? A missing wrapper
  is a generation failure, not a silent gap, but confirm the wrapper actually
  redacts everything credential-shaped, not just the field that was
  reported.

## Step 3 — Diff the generated specs against the previous state

Run `git diff` (or `git status` for a wave adding a brand-new domain) over
`internal/tools/<domain>/*.gen.go` and read every hunk, not just the
line count. A regenerate that only reformats is invisible in a diff line
count and easy to wave through; a regenerate that silently drops a field, a
required flag, or an enum is exactly the kind of change a wave exists to
catch before it merges. If the domain already had hand-written actions being
replaced by generated ones (the pilot swap in P3.1's Task 4/4b is the
worked example), treat every behavioural difference as a finding to
investigate and record — never adjust a passing test to match the generated
output without first deciding, and writing down, which side was right.

## Step 4 — Run `audit_spec_reality`

With a live estate up (`make e2e-up`; bring up the Kubernetes leg too,
`make e2e-k8s-up`, only if this wave's domain is Kubernetes-specific and you
have reason to think its route table could differ by deployment target —
see `cmd/audit_spec_reality`'s own package doc for why the first run found no
such reason):

```sh
make audit-spec-reality
```

This probes every operation the vendored specification documents — not only
this wave's new ones — against the live server, and reports which of them
the running Portainer does not actually serve. It is read-only by
construction (see the package doc for why every probe, on every verb, is
safe) and it reports rather than gates: a divergence is a fact about
Portainer, not a defect in this project's code, so a clean `make check` next
to a non-empty divergence list is the expected, correct outcome. Read the
report anyway, every wave, not only the first time — a divergence
`audit_spec_reality` finds for an operation this wave is about to expose as
an action is exactly the kind of thing to record in
`docs/api-divergences.md` (Step 6) and reflect in the action's own
narrative, rather than discover only once a user reports the tool call is
"broken".

## Step 5 — Exercise every new action on all three surfaces

Every new action needs an e2e test that calls it through `individual`,
`meta` and `dynamic` — the three tool surfaces `test/e2e/suite/sessions_test.go`'s
`Sessions` already builds sessions for — against the live estate, on every
edition the operation applies to (`Sessions.Editions()`/`Estate.Legs()`
derive this; do not hardcode a `[]string{"CE", "EE"}` literal). A mutating
action additionally needs its safe-mode and (where the action requires
Business Edition) safe-mode-EE coverage, proving `tools.Execute` intercepts
it before its handler ever runs.

```sh
make test-e2e
```

must be green, and `make audit-e2e-gaps` should show every new action
referenced (it is informational, not a gate, but a name it reports as
unreferenced is a real gap in this step, not a tool defect).

## Step 6 — Record divergences and move the ratchet, in the same commit

1. Record what this wave's Steps 2-5 found, in whichever of two files it
   belongs to. The division of labour is fixed, not a matter of taste:

   - **`docs/api-divergences.md` — committed, permanent, the destination.**
     Every settled fact about Portainer disagreeing with the documents that
     describe it goes here: an `audit_spec_reality` divergence touching this
     wave's domain, a documented route that answers but does not behave as
     documented, an undocumented or understated required header or field, a
     response that leaks a secret, a defect in the vendored specification
     itself. Follow that file's own conventions — an evidence label on
     every claim (probed live / vendored spec / diagnosed), and an entry in
     its "Open questions" section for anything measured but not explained,
     rather than a guess written as fact. The test is simple: if a
     contributor implementing a different domain later would need the fact
     and could not derive it themselves, it goes here, in this wave's own
     commit.
   - **`plan/carry-forward.md` — gitignored, a working scratch pad, never a
     destination.** In-progress reasoning that is not yet settled enough to
     distil: raw probe transcripts, hypotheses still being tested, a
     generated vs. hand-written disagreement whose resolution is still open,
     an override this wave added and why, decisions deferred to a later
     phase, notes about this project's own code rather than about Portainer.
     Follow the existing entries' shape — dated, specific, reasoned — not a
     bare bullet list. Before the wave ends, anything here that a later wave
     would need must be distilled into `docs/api-divergences.md`: this file
     is gitignored, cannot be committed, and should be assumed to be one
     fresh clone away from not existing.
2. Update `api/coverage-baseline.yaml`'s `ce_covered`/`ee_covered` to the new
   counts `make audit-1to1` (or `make audit-1to1-ratchet`) reports. Never
   lower either number — that would hide a regression instead of catching
   one — and never leave the file stale once coverage has improved: a wave
   that lands new actions but forgets this file leaves the ratchet passing
   for the wrong reason (it improved, not merely held), which defeats the
   one thing the ratchet exists to make visible in a diff.
3. Confirm `make audit-1to1-ratchet` (what CI actually gates on) and
   `make check` are both green before committing.

Commit the domain's hand-written file(s), its `*.gen.go` output, any
`docs/api-divergences.md` entry and the ratchet update together. A wave that
splits these across commits makes it possible for the ratchet to move
without the divergence entry that explains what changed, or vice versa.
(`plan/carry-forward.md` is gitignored and is never part of this commit —
that is precisely why anything permanent has to be distilled out of it
first.)
