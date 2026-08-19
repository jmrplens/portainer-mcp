# Domain wave checklist

This is the procedure for adding a domain to the action catalog, owning it
afterwards, and upgrading the whole catalog to a newer Portainer release. It
exists because the thing it guards against — the vendored specification
disagreeing with the Portainer server it describes, or the catalog quietly
drifting from the vendored specification — has already happened several
times in this project's history before the audits that now watch for it
systematically existed (see `cmd/audit_spec_reality`'s own package doc for
four cases found by accident, and `cmd/audit_spec_drift`'s for the
parameter-shape and credential-redaction class), and neither announces
itself. `docs/api-divergences.md` catalogues six broader categories of
divergence. A wave, or an upgrade, that skips a step here is not saving
time; it is choosing not to look for the next one.

## Before and after every estate cycle: check this host's edge agent

This machine runs a `portainer_edge_agent` that reports to the owner's Portainer
at `192.168.0.40`. It is not part of this project and must keep working.

On 2026-08-04 its tunnel stopped connecting and the owner fixed it by restarting
the container. No mechanism was ever proven — the agent's API port lives inside
its own network namespace, so an ephemeral container cannot take it, and the
agent answered correctly on 9001 throughout. But the tunnel made no connection
attempt between 2026-08-02 12:05 and 2026-08-04 00:37, which overlaps heavy
estate work, and the tunnel is on-demand so the silence has an innocent reading
too. Cause unresolved.

Rather than argue it, check it. Before `make e2e-up` and after `make e2e-down`:

```sh
docker logs --since 15m portainer_edge_agent 2>&1 | grep -E "client: (Connected|Connection error)" | tail -3
```

A `Connection error` or a long silence where there was activity before is worth
stopping for. The failure the owner saw was invisible from this side: check-in
kept succeeding and only the tunnel was broken, so a heartbeat check would have
said everything was fine.

When the estate runs on a remote Docker host (`make e2e-up-remote` and/or
`make e2e-k8s-up-remote` — see README.md's "Running the estate on another
machine"), check the remote host too — it is somebody's real machine, and the
estate is a guest on it. The two legs record where they went separately
(`test/e2e/.docker-host` for the compose legs, `test/e2e/.docker-host-kubernetes`
for the Kubernetes leg), because the two `-remote` targets are independent
opt-ins and can legitimately name different hosts.

**Capture both markers BEFORE tearing anything down, in the same shell
session you will run the `make` commands in.** Both `down.sh` and
`k3d-down.sh` delete their own marker as part of a normal teardown; reading
them only *afterward* finds nothing for either leg, so every check below
silently skips — printing nothing rather than failing — even when a remote
estate genuinely never came down. That is not hypothetical: an earlier
version of this checklist did exactly that, read the markers after `make
e2e-k8s-down && make e2e-down`, and its own "must report nothing left behind"
line held vacuously, by construction, on every run:

```sh
dest=$(cat test/e2e/.docker-host 2>/dev/null)
k8s_dest=$(cat test/e2e/.docker-host-kubernetes 2>/dev/null)
```

Then tear down as usual:

```sh
make e2e-k8s-down && make e2e-down
```

Then check the CAPTURED destinations — `$dest`/`$k8s_dest` from above, not a
fresh read of the now-deleted marker files:

```sh
if [ -n "$dest" ]; then
    if names=$(ssh "$dest" 'docker ps -a --filter name=portainer-mcp-e2e --format "{{.Names}}"'); then
        echo "compose leg ($dest): $(printf '%s' "$names" | grep -c . || true) container(s) left"
    else
        echo "COULD NOT CHECK compose leg ($dest): ssh or docker failed -- this is NOT the same as clean, check by hand" >&2
    fi
fi
if [ -n "$k8s_dest" ]; then
    if names=$(ssh "$k8s_dest" 'docker ps -a --format "{{.Names}}"'); then
        echo "kubernetes leg ($k8s_dest): $(printf '%s' "$names" | grep -c k3d-portainer-mcp-e2e || true) container(s) left"
    else
        echo "COULD NOT CHECK kubernetes leg ($k8s_dest): ssh or docker failed -- this is NOT the same as clean, check by hand" >&2
    fi
fi
# The literal path below must match cdi_spec_path() in test/e2e/scripts/lib.sh
# exactly -- this checklist is a manual runbook and cannot source that file.
[ -n "$dest" ] && ssh "$dest" 'test -f /tmp/portainer-mcp-e2e-cdi-nvidia.yaml && echo "LEFTOVER cdi spec" || echo clean'
```

(Each leg's `docker ps` runs alone over ssh, with nothing piped into it
remotely: piping straight into `wc -l` or `grep -c` on the remote side, as an
earlier version of this checklist did, means a failing `ssh` or a failing
remote `docker` (wrong context, daemon not running, permission denied) leaves
the pipe's stdin empty and `wc -l`/`grep -c` still print a reassuring `0` —
indistinguishable from "checked, genuinely nothing left". Checking the `ssh`
call's own exit status first, before ever counting anything, is what makes
"could not look" print as a loud, separate line instead of a `0` that reads
as clean. The two `grep -c ... || true` calls that count locally, once the
remote call is already known to have succeeded, use `|| true` for the same
reason the previous version of this checklist noted: `grep -c` already prints
`0` on its own when nothing matches, and only exits non-zero for that same
case, so the `|| true` exists purely to keep that non-zero status from
tripping a `set -e` shell that runs this line — never to print a second `0`.)

Every check that ran (a leg whose marker never existed means that leg never
left this machine, and is skipped above rather than checked against nothing)
must report nothing left behind. A non-zero container count means teardown
did not reach the remote daemon it should have — most often because the
marker file was removed by hand before teardown ran, or because the estate
was brought up by something other than the matching `-remote` target.

## One Business Edition licence at a time

`GET /api/licenses` on this estate's key reports `nodes: 3` — a licence good
for three nodes. `GET /api/system/nodes` on the compose leg's own server
(measured live, 2026-08-18) reports `2`: the compose estate's own
provisioning already consumes two of the three nodes by itself. The
Kubernetes leg is created with `--agents 1` (`k3d-up.sh`), two more nodes.
Both legs running at once is therefore 4 nodes against an allowance of 3 —
over the limit on node count alone. This does not rest on
`multiuseInstancesCount` (also measured `0` on this estate): that counter is
read the opposite way in `plan/carry-forward.md`'s own nine-cycle
measurement, where it never moved across nine up/down cycles and so does not
seem to be counted per instance at all — contested, and not needed to make
the case above, so this checklist does not lean on it. Both legs read the
SAME key out of the same gitignored `.env`, and both give it back on
teardown (`down.sh`, `k3d-down.sh`). Two failure modes both happened for real
on 2026-08-18: activating the licence on both legs at once, and tearing one
leg down — releasing the licence — while the other leg was still using it.

The rule: **bring one leg down before bringing the other up — including
re-running the SAME leg.** A second `up.sh` (or `k3d-up.sh`) now refuses
exactly like the other leg's would, while its own first run's lock is still
held: the licence has to be freed with a teardown, not simply rerun over.
`test/e2e/scripts/lib.sh`'s `take_licence_lock`/`release_licence_lock`
enforce this — `up.sh` and `k3d-up.sh` take a lock
(`test/e2e/.licence.lock`) before activating anything, and a second leg's
`up` refuses, naming the leg already holding it, when it was taken, and the
exact command (`make e2e-down` or `make e2e-k8s-down`) that frees it. A lock
that names a leg no longer running is reported as stale, never
auto-removed; `make e2e-licence-release` (`licence-check.sh`) clears both
the stranded licence and the stale lock together, but only once it has
confirmed the recorded holder does not actually look like it is still
running. The lock is only a guard, though — the licence's own one-instance
limit is the real constraint underneath it, and holds regardless of whether
the lock is in place.

**In CI the lock cannot help at all**, and this is the part that is easy to
get wrong. `test/e2e/.licence.lock` is a file on one runner's own
filesystem: it refuses a second activation by the run that holds it and is
blind to every other runner and every other workflow run. So
`.github/workflows/e2e.yml` serialises the two legs a second way, and both
mechanisms are load bearing:

- the Kubernetes job declares `needs: compose`, so the two legs never run at
  the same time on two runners (removing that to "parallelise" reintroduces
  exactly the double activation this section is about, with every test still
  green);
- the workflow declares a repository-wide `concurrency` group with
  `cancel-in-progress: false`, so two pull requests cannot each hold the
  licence, and a queued run is never cancelled mid-estate — a cancellation
  races its own teardown and can strand the key.

`test/e2e/harness/e2e_workflow_test.go` pins both, plus each job's
`if: always()` teardown, against the committed workflow file.

## The model, in one paragraph

A domain is scaffolded once, from the vendored specification, by
`make scaffold-domain`. From the moment it writes `internal/tools/<domain>/actions.go`
and `inputs.go`, that domain owns those files exactly like every other
source file in this repository: no `DO NOT EDIT` header, no regeneration,
no CI freshness check comparing them against a fresh run. `scaffold-domain`
itself refuses to touch a domain directory that already has one of these
files unless told `FORCE=1`, because the whole point of scaffolding once is
that the files may have been edited since. What replaces the freshness
check is `make audit-spec-drift`: a standing, gating comparison between
every declared action and the vendored specification it names, which
catches drift *however it arose* — a hand edit, a spec that moved out from
under the catalog, or (its own two structural checks) a credential-shaped
response with no redaction wrapper, or an identifier-shaped path parameter
missing its `minimum: 1`. Run it after every hand edit to an owned domain,
not only when adding one.

This document has three parts: **scaffolding a new domain** (Steps 1-6,
done once per domain), **owning a scaffolded domain** (the day-to-day
workflow from that point on), and **upgrading to a new Portainer version**
(the human procedure that replaces "regenerate everything").

Every divergence found so far is catalogued in `docs/api-divergences.md`.
Read your domain's entries there before Step 1; that is also where a wave's
own findings are recorded, in Step 6.

---

## Part A — Scaffolding a new domain

Nothing in Steps 1-5 is optional. Step 6 (moving the coverage ratchet) is
mechanical but must land in the same commit as the domain, never a follow-up.

### Step 1 — Scaffold

1. Create `internal/tools/<domain>/<domain>.go` if the domain package does
   not exist yet: package doc comment, a `Specs()` function, and (once the
   domain has at least one generated action) a `narrative(operationID string)
   toolutil.ActionNarrative` hook for any `Title`/`Description` override the
   vendored spec's own summary/description does not already say well. See
   `internal/tools/system/system.go` for the shape a domain settles into once
   some of its actions are generated and one is kept hand-written — and note
   that *every* action in that file, generated or hand-written, is built
   through `toolutil.WithNarrative`, never a literal `Title`/`Description`
   assignment: see "Narrative overrides are structural, not a policy
   exception" below for why that is load-bearing, not a style preference.
2. Confirm `internal/toolutil.DomainTags` already maps the domain's directory
   name to the OpenAPI tag(s) it covers. This table is the only place
   "directory name" and "OpenAPI tag" are reconciled — a domain absent from
   it, or a tag with real operations no domain claims, is a build error the
   next step raises, not a silently empty generated file.
3. Run:

   ```sh
   make scaffold-domain
   ```

   This writes `internal/tools/<domain>/inputs.go` (one Input struct per
   operation) and `actions.go` (one generated `ActionSpec` + handler per
   mechanical operation) for every domain directory it finds under
   `internal/tools` that does **not** already have one of these files (or
   the pre-freeze `*.gen.go` names) — which, for every domain scaffolded
   before this one, is all of them: `scaffold-domain` silently does nothing
   to a domain it has already written to, unless you pass `FORCE=1`.

   Be precise about what `FORCE=1` actually does, because it is narrower
   than it sounds and narrower than an earlier version of this checklist
   claimed: it bypasses the "already scaffolded" skip, nothing more. It
   does **not** discard the hand edits made since that domain was
   scaffolded. `cmd/gen_action_inputs`'s `scanHandOverrides` treats every
   operationId already declared in a domain's own `actions.go`/`inputs.go`
   as a hand-written override to leave alone — correctly, once those files
   stopped being suffixed `.gen.go` and became owned source like any
   other — which means regeneration with `FORCE=1` skips every operation
   already declared there and generates only genuinely new ones the
   domain's tag has gained since it was last scaffolded. There is
   currently no flag that discards an owned domain's accumulated hand
   edits and starts over; see `docs/api-divergences.md` §9.2 for the full
   account of why, and the ordinary path (scaffold once, hand-edit
   forever, `FORCE=1` never used) is entirely unaffected by this. Whoever
   genuinely needs to start a domain over should delete its
   `actions.go`/`inputs.go` by hand first, then run `make scaffold-domain`
   without `FORCE`.

   Expect a first-time scaffold to refuse loudly rather than guess: an
   ambiguous shape, a wire-type width mismatch, or a credential-shaped
   success response with no declared redaction wrapper each refuse
   generation for *that one operation*, naming it — not the whole domain.
   Since P3.3 Task 3, a refusal costs only the refused operation: every
   other operation in the same domain still generates normally, and the
   domain's `actions.go`/`inputs.go` are still written from whatever
   succeeded. `run()` still exits non-zero when any refusal occurred (a
   refused operation gets no handler, no Input struct and no ActionSpec
   entry, and that is worth failing the build over), and every refusal is
   named in the report printed at the end of the run — this is
   `cmd/gen_action_inputs` working as designed (see its own package doc);
   resolve each named refusal in the domain file, do not work around it. An
   earlier revision of this checklist described the pre-Task-3 behaviour,
   where any one of these three refusals aborted generation for the entire
   domain.
4. Wire the new domain's `Specs()` into every place that still collects
   domain packages by hand: `internal/wiring` (the real server), and each of
   `cmd/audit_1to1`, `cmd/audit_e2e_gaps` and `cmd/audit_discovery`'s own
   `allCatalogSpecs`/`allSpecs` functions. That is four places, no more —
   there is no single registry yet, and each of these currently lists domains
   by hand.

   Forgetting one is no longer silent, and this checklist said it was for
   longer than it was true. `internal/wiring/registration_parity_test.go`
   (wave 1) parses each of the three audits' own spec-collecting function and
   compares the set of domains it names against `wiring.AllSpecs()`, failing
   with both sets printed and naming the file that is out of step. It catches
   the omission in either direction — a domain wired into the server but
   missing from an audit, and a domain in an audit but missing from the
   server. It does not remove the need to edit all four places; it removes
   the possibility of finding out three waves later.

   `internal/wiring/server_test.go` is deliberately **not** a fifth place to
   edit: its meta-surface test derives its expected tool list from
   `AllSpecs()` itself (one tool per `catalog.Domains()` entry, plus status),
   so it tracks whatever domains step 4 already registered without a further
   hand edit. It briefly was a fifth, undocumented site — a hard-coded
   literal there went stale on every wave and failed the build with no note
   pointing back at itself — before being fixed to derive the expectation
   instead of hard-coding it.


### Step 2 — Read the generated names

Before running anything else, read `internal/tools/<domain>/actions.go`
top to bottom:

- Does every action name read as what a model would search `portainer_find_action`
  for? The naming rules produce a handful of results a human has to judge
  (an operationId that is its own domain prefix, an initialism the splitter
  does not know) — those are refused by name, not silently mis-derived, but
  still worth a second look.
- Did `scaffold-domain` print any verb-mismatch or weak-description
  warnings for this domain to stderr? Every operation whose path or
  identifier suggests deletion but whose HTTP verb does not is listed there,
  and a wave reviews that list before accepting the generated
  `DestructiveHint` values as correct.
- Does every mutating action either carry no credential-shaped field in its
  success response, or does the domain declare the `redact<OperationID>`
  wrapper the generator requires for the ones that do? A missing wrapper is
  a generation failure, not a silent gap, but confirm the wrapper actually
  redacts everything credential-shaped, not just the field that was
  reported — `redaction_test.go`'s generated guard is what proves that, and
  it stays part of this domain's test suite forever (it is not itself
  regenerated once the domain is owned, but it does not need to be: a
  wrapper does not stop existing just because nobody is running the
  scaffolder over it any more).

### Step 3 — Diff the generated files against the previous state

Run `git status` (a brand-new domain has nothing to diff against) or, for a
domain that had hand-written actions being replaced by generated ones, `git
diff` over `internal/tools/<domain>/*.go` and read every hunk, not just the
line count. Treat every behavioural difference from what existed before as
a finding to investigate and record — never adjust a passing test to match
the generated output without first deciding, and writing down, which side
was right.

### Step 4 — Run `audit_spec_reality`

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

### Step 5 — Exercise every new action on all three surfaces

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

**Proving a guard by mutating it: mutate the domain's own files, not shared
code.** A guard is only proven when it has been seen to fail, so mutating
one and re-running is right. But the mutation runs against the live shared
estate. In wave 2 stage A, `internal/tools/register.go` was mutated to prove
one domain's guard; every domain shares that file, so the mutation disabled
the check for all of them at once and three teams leaked into the live
estate, to be deleted by hand afterwards. Prefer mutating the file the guard
lives beside; if the proof genuinely needs shared code, revert it and check
the estate immediately afterwards, before running anything else.

### Step 6 — Record divergences and move the ratchet, in the same commit

1. Record what this wave's Steps 2-5 found, in whichever of three files it
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
   - **`docs/open-follow-ups.md` — committed, the work rather than the
     fact.** A measurement that a later wave has to *act* on, rather than
     merely know, gets an entry here naming the evidence and what closing it
     requires, cross-referenced to its `docs/api-divergences.md` entry
     rather than restating it. Delete the entry when the work lands. This
     file exists because a stage's real findings had been living only in
     gitignored working notes, where the point below applies to them.
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
3. Confirm `make audit-1to1-ratchet` and `make audit-spec-drift` (both of
   which CI gates on) and `make check` are all green before committing.

Commit the domain's file(s) (hand-written and scaffolded alike — there is
no separate "generated output" category to split out any more), any
`docs/api-divergences.md` entry and the ratchet update together. A wave
that splits these across commits makes it possible for the ratchet to move
without the divergence entry that explains what changed, or vice versa.
(`plan/carry-forward.md` is gitignored and is never part of this commit —
that is precisely why anything permanent has to be distilled out of it
first.)

---

## Part B — Owning a scaffolded domain

Once `internal/tools/<domain>/actions.go` and `inputs.go` exist, they are
ordinary Go source: hand-edit them like any other file in this repository.
There is nothing left that regenerates them, nothing that diffs them
against a fresh run, and no header that exempts them from lint. The
guarantees that used to depend on the generator having *just run* now
depend on standing, independently-gating checks instead:

- **`make audit-spec-drift`** — run this after any change that touches a
  parameter's shape (a field's type, requiredness, enum, origin), a
  redaction wrapper, an identifier's minimum bound, or a Title/Description.
  It fails the build the moment any of those disagrees with the vendored
  specification, with three exceptions it treats differently, not three
  gaps it misses:
  - A **cosmetic** description-only difference where the specification
    itself never had text to drift from is reported but does not gate — see
    `isGating`'s own doc comment.
  - A **deliberate narrative override** (see the next section) does not
    gate either, structurally, not via an exception list.
  - A **credential-shaped response with no redaction wrapper call**, or an
    **identifier-shaped path parameter with no `minimum: 1`**, always gates
    and is never allow-listable — these are the two structural guarantees
    that used to be enforced only at generation time (see
    `cmd/audit_spec_drift`'s own package doc for why moving them here, not
    trusting the generator's refusal alone, is what survives the freeze).
- **`internal/tools/<domain>/redaction_test.go`**, if this domain has one,
  is part of `go test ./...` forever: it proves each `redact<OperationID>`
  wrapper actually strips what it claims to, which `audit_spec_drift`'s own
  static check (does the handler call the wrapper at all) cannot.
- **`api/spec-drift-allowlist.yaml`** exists for a genuinely narrower type on
  a hand-written field or two — not for Title/Description any more (see
  below). Adding an entry requires a reason and a date; a stale one (nothing
  it excuses is still gating) is itself a build failure, not a warning.

### Narrative overrides are structural, not a policy exception

Every action whose Title or Description deliberately improves on the
vendored specification's own wording is built through
`toolutil.WithNarrative`, which sets
`ActionSpec.TitleOverridden`/`DescriptionOverridden` whenever its own
`ActionNarrative.Title`/`Description` are non-empty. `audit_spec_drift`
reads that fact (`specdiff.FieldChange.AfterOverridden`, produced by
`Compare` and consumed by `isGating`) and never gates a Title/Description
finding whose catalog side was deliberately overridden — with **no
allow-list entry required**. This replaced a 35-entry
`spec-drift-allowlist.yaml` exception list (one recording a decision that
was, in every case, already recorded once in the domain's own
`narrative()` hook or `WithNarrative` call) with a fact recorded exactly
once, at the point the override is made.

The consequence for a domain author: **never assign `Title`/`Description`
directly in an `ActionSpec` literal.** Always route through
`toolutil.WithNarrative`, even for an action with no other narrative field
to set:

```go
toolutil.WithNarrative(toolutil.ActionSpec{
    Name: "registries.configure", Domain: "registries", OperationID: "RegistryConfigure",
    // ... no Title/Description here ...
}, narrative("RegistryConfigure"))
```

An `ActionSpec` literal that assigns `Title`/`Description` directly instead
leaves `TitleOverridden`/`DescriptionOverridden` false, and the next
`audit_spec_drift` run has no way to tell that apart from accidental drift
— it will gate, correctly, on prose that was actually a deliberate choice.

---

## Part C — Upgrading to a new Portainer version

Regenerating the whole catalog from a new specification is no longer the
procedure — every domain owns its own files, and a regeneration would
discard every hand edit made since. Upgrading is a **human procedure**,
domain by domain, driven by `cmd/audit_spec_delta`'s work list.

1. **Fetch and bundle the candidate specification**, without touching the
   vendored one:

   ```sh
   python3 plan/research/specs/bundle.py ee 2.45.0 /tmp/ee-2.45.0.json
   ```

   `api/specs/` stays vendored to the version the catalog was last verified
   against throughout this whole procedure; nothing here overwrites it
   until Step 6.

2. **Run the delta audit** to get the work list:

   ```sh
   make audit-spec-delta BEFORE=api/specs/ee-2.44.0.json AFTER=/tmp/ee-2.45.0.json
   ```

   Read it grouped by domain, in the order it already presents: added
   operations (JUDGEMENT — a person names the action, writes its narrative,
   and decides whether its response needs a redaction wrapper), removed
   operations (MECHANICAL — delete the action), struct-touching changes
   (JUDGEMENT — a field appeared, disappeared, changed type, changed
   requiredness, or moved between path/query/body: decide how the owned Go
   code should change), and cosmetic changes (MECHANICAL — a description or
   enum text copy-paste). `JSON=1` gives the identical grouping as
   machine-readable output if you are scripting part of this.

3. **Work the list one domain at a time**, hand-editing that domain's owned
   `actions.go`/`inputs.go`/`<domain>.go` directly:
   - For a MECHANICAL entry, make the textual change described.
   - For a JUDGEMENT entry, decide the change and make it — this is exactly
     the kind of decision Part A's Steps 1-5 exist to support for a brand
     new operation (naming, redaction, edition, narrative), so apply the
     identical judgement here for a changed or added one.
   - A domain with no entries in the work list needs no attention this
     upgrade; do not touch it.
4. **Run `make audit-spec-drift` against each domain you touched**, as you
   finish it, not only at the end — it will not gate until Step 6 (below),
   since the vendored specification is still the old version, but it is
   otherwise the correct tool to confirm your hand edit actually matches
   the *candidate* specification: point it at the candidate temporarily
   (`SPEC_VERSION=2.45.0`, with `/tmp/ee-2.45.0.json`/`/tmp/ce-2.45.0.json`
   copied to where `-spec-version` expects them, or pass the paths
   directly) rather than waiting until the vendored spec is swapped to find
   out you got a field wrong.
5. **Run the full verification suite** once every domain the work list
   named has been addressed: `make check`, `make audit-1to1-ratchet`,
   `make test-e2e` against a live estate (see Part A's edge-agent check —
   it applies here too, since an upgrade cycle is exactly the kind of heavy
   estate work that motivated it), and `make audit-spec-reality` against
   that estate if the new version might have changed which routes exist.
6. **Adopt the candidate specification as vendored**, only now:

   ```sh
   make update-spec SPEC_VERSION=2.45.0
   ```

   then update `SPEC_VERSION` in the `Makefile` (or however it is invoked in
   CI) to the new default, and confirm `make audit-spec-drift` is clean
   against the newly-vendored files with no `-spec-version` override.
7. **Record what changed**, in `docs/api-divergences.md`, the same way
   Part A's Step 6 does for a new domain: any new divergence between the
   new Portainer version and its own specification, any judgement call this
   upgrade made that a later reader would need and could not re-derive from
   the diff alone.

Commit the whole upgrade — every domain's hand edits, the vendored spec
swap, and the `docs/api-divergences.md` entry — together, for the identical
reason Part A's Step 6 commits a new domain's pieces together: splitting
them makes it possible for the vendored spec to move without the record of
what that move required.
