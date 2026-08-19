# Open follow-ups

Work this project already knows it owes, each entry carrying the measurement
that found it and what closing it would mean.

## Why this file exists, and what does not belong in it

Wave 2 stage A caught a finding that existed only in `.superpowers/sdd/`,
which is gitignored and — as `docs/domain-wave-checklist.md` Step 6 puts it —
"one fresh clone away from not existing". Recording the *fact* somewhere
durable was only half the fix: a fact nobody is going to act on and a piece
of work nobody has done are different things, and a file of facts is the
wrong place to track the second.

The division, so that neither file swallows the other:

- A **settled fact** about Portainer, or about this project's own tooling,
  goes to `docs/api-divergences.md` (§9 for the tooling ones) — and is cited
  from here rather than restated.
- Something **measured but not explained, or not measured at all**, goes to
  `docs/api-divergences.md` §8 "Open questions" — and is cited from here.
- A **procedure that must change** goes to `docs/domain-wave-checklist.md`.
- The **work**, and what "done" looks like for it, lives here.

Rules for an entry: name the evidence, not an impression; state what closing
it requires; delete the entry when the work lands. An entry nobody can
re-derive is worse than no entry, because it will be believed.

Everything below was measured on 2026-08-19 against the branch that closed
wave 2 stage A, unless the entry says otherwise.

---

## 1. `bodyJSONTag` publishes five field names Portainer would never send

**Fact:** `docs/api-divergences.md` §9.6 (root cause, evidence, the exact
five fields). **Open question about its true size:** §8 item 9.

Five published body fields carry a stray capital `S` — `endpoints`
(`tagIdS`, `endpointIdS`), `endpoint_groups` (`tagIdS` twice),
`resource_controls` (`subResourceIdS`). A caller writing the natural
`tagIds` is refused by this project's own input schema before Portainer sees
the call. Two of the five are already on `main`.

**What closing it requires**, in one commit:

1. Give `bodyJSONTag` the lone-trailing-`s` branch `goFieldName` already has
   — or, better, stop deriving the tag and emit the specification's property
   name verbatim, which removes the whole class rather than this instance.
2. Change **both** copies together: `cmd/gen_action_inputs/naming.go` and
   `internal/specdiff/naming.go`.
   `TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation` pins them to each
   other over every operation in both vendored documents.
3. Rename all five published fields in the same commit. Renaming one
   domain's field alone makes `cmd/audit_spec_drift` report a bogus
   added/removed pair on an operation nobody touched (measured) — the
   allow-list trap that audit's own doc comment warns about.
4. Update the two `resource_controls` narratives (create and update) that
   currently spell the mangled name out for the caller as a workaround, and
   `TestResourceControls_SubResourceIdsFieldName_IsRefusedUnlessSpelledAsPublished`,
   which asserts today's spelling on every surface. No `endpoint_groups` or
   `endpoints` narrative mentions its mangled field at all — those two
   domains publish it silently, which is the worse case of the two.

Neither the audits nor the e2e suite can catch a regression here: both write
the same derivation they are checking. A test that compares a published tag
against the *raw* specification property name is what would close the
§8 item 9 question at the same time.

---

## 2. Sweep the seven pre-stage domains for narratives that describe the raw API

**Open question:** `docs/api-divergences.md` §8 item 10.

A narrative can be a true statement about Portainer and a false statement
about the action a model can actually reach, because the catalog refuses
some calls before Portainer sees them — per-field edition pruning, an
`EnumParams` constraint, `additionalProperties: false`. Six narratives
shipped that way in this stage alone and were corrected in `3619869`
(`teams.create`, `teams.update`, `team_memberships.create`) and `de44649`
(`resource_controls.create`, `resource_controls.update`, `roles.list`).

Only the five stage-A domains have been swept. The seven written before it —
`system`, `tags`, `registries`, `docker`, `custom_templates`, `stacks`,
`endpoints` — have never been checked for this at all. Every domain carrying
an `edition:"EE"` field or an `EnumParams` constraint is a candidate.

**What closing it requires:** for each candidate action, issue the call
through the catalog (not with `curl`) on every edition it is published for,
and confirm the narrative describes what came back. A claim quoting
`docs/api-divergences.md` needs the extra step: that file describes the raw
API deliberately, so quoting it in a narrative is exactly the mistake.

---

## 3. Six domains have no `TestUnit_EveryDeclaredAction_HasANarrative`

Deleting a `narrative()` case is invisible to `cmd/audit_spec_drift`: with
the override gone, catalog and document agree again and the audit reports
"No drift". The per-domain guard is the only thing that catches a lost
measured fact. It exists in `endpoint_groups`, `endpoints`,
`resource_controls`, `roles`, `team_memberships` and `teams`.

Measured by adding the guard to each of the six domains that lack it and
running it (the probe files were removed afterwards):

| Domain | Result |
|---|---|
| `custom_templates` | passes as-is |
| `registries` | passes as-is |
| `stacks` | passes as-is |
| `system` | passes as-is |
| `tags` | passes as-is |
| `docker` | **fails on all 8 actions** — none carries a Title override, and five carry neither Title nor Description |

`custom_templates` and `stacks` do have a
`TestUnit_Narrative_GivesEveryActionADistinctTitleAndDescription`, which is a
different guard: it catches two actions colliding, not one action losing its
narrative.

**What closing it requires:** add the guard to the five that pass, in one
commit. `docker` is a decision, not a paste — either its eight actions get
narratives, or the domain records why the vendored wording is right for them
and carries a guard shaped to that ruling. Do not weaken the guard to make
`docker` green.

---

## 4. `orphanSweeps` covers four resource kinds; the suite creates six more

`test/e2e/suite/fixtures_test.go`'s `orphanSweeps` registers `tags`,
`registries`, `custom templates` and `stacks`. `cleanupOrphans` is the net
for a run that dies between creating a fixture and its own cleanup — on an
estate every session in the matrix shares.

Six kinds have no sweep behind them. Five arrived with wave 2 stage A —
`createEndpointGroupFixture` (`endpoint_groups_test.go`),
`createTeamFixture`, `createUserFixture`, `createMembershipFixture`
(`teams_test.go`) and `createVolumeFixture` (`resource_controls_test.go`) —
and the sixth predates it: `endpoints_test.go` creates environments,
including edge environments.

Environments are the expensive one: on Business Edition each consumes a
licence node, and the licence this project runs on has three (two of which
the estate itself uses — see `docs/domain-wave-checklist.md`, "One Business
Edition licence at a time"). A crashed run burns node allowance until
somebody sweeps by hand.

**What closing it requires:** one `orphanSweeps` entry per kind, appended
from the domain's own suite file where the cleanup logic wants to stay local
— that is what the registration point exists for, and
`TestCleanupOrphans_CallsEveryRegisteredSweeper` already proves
`cleanupOrphans` needs no knowledge of the kinds it sweeps. Environments
first.

---

## 5. `Usage` is empty for all 109 actions

`toolutil.ActionSpec.Usage` — "one or two sentences of model-facing guidance
on when to reach for this action" — is assigned nowhere in
`internal/tools/`. `make audit-discovery` consequently reports **13 of 13**
sibling clusters as indistinguishable, every one of them for the same
reason: `share identical Usage: (no Usage text)`.

The worst clusters are the ones a model is most likely to meet:
`stacks` (base "create") has nine siblings, `endpoints` (base "update") has
four.

**What closing it requires:** `Usage` text written per action, through
`toolutil.WithNarrative` like every other narrative field, in the domain's
own commit. `audit-discovery` reports and never gates, deliberately —
gating on it invites filler text that satisfies the audit and helps nobody —
so the measure of done is the cluster list shrinking, not the exit code
changing.
