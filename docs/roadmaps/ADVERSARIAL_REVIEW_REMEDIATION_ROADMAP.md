# DOGMud — Adversarial Review Remediation Roadmap

> Living roadmap for resolving every finding in
> `ADVERSARIAL_CODE_REVIEW_2026-08-07.md` plus validated follow-ups from
> `docs/audits/ADVERSARIAL_REVIEW.md` at origin commit `9338a33`.
>
> This is a roadmap, not a specification or implementation plan. Each chunk
> receives its own brainstorm, spec, adversarial spec review, implementation
> plan, adversarial plan review, TDD implementation, verification, and
> adversarial implementation review when that chunk is selected.

## Why this effort exists

The original adversarial review reported 33 issues across release assets, test
reliability, persistence, concurrency, web security, gameplay correctness, and
accumulated technical debt. Finding 1 was later invalidated. An independent
comparison review added four validated findings and two unresolved design-intent
questions. Broad fixes remain unsafe while tests and CI are unreliable, and
cleanup work is wasteful while behavior and persistence contracts are still
changing.

The roadmap was originally written to follow dependency order:

1. Restore a trustworthy release and verification baseline.
2. Protect persistent state and concurrent runtime state.
3. Harden externally reachable web surfaces.
4. Correct player-visible behavior.
5. Consolidate duplicated code and retire legacy debt.

**That ordering was retired 2026-08-08.** It optimized for dependency purity
rather than risk, and delivered zero finding closures across 61 commits. The
phase numbers below are retained as a taxonomy so the finding index still
resolves, but the work order is the wave list in **Execution order (triage
2026-08-08)** immediately after the operating rules.

## Operating rules

- Every review finding is mapped to one owning chunk or explicitly decomposed
  arc. Subchunks may partition a broad finding, but share one finding status.
- Findings may be regrouped as evidence develops, but must never disappear.
- If a finding is disproven, mark it **Invalidated** with the evidence.
- Keep chunks independently reviewable and shippable.
- Prefer a regression test before changing behavior.
- Do not combine unrelated cleanup with a correctness chunk.
- Update the progress tracker and finding coverage index when a chunk changes
  status.
- Newly discovered defects join the narrowest relevant chunk or are added as a
  new chunk with explicit dependencies.

## Execution order (triage 2026-08-08) — READ THIS FIRST

The phase numbering below is a **taxonomy**, not a work order. Following it
literally produced 61 commits and zero finding closures, because Phase 1 is CI
plumbing and the two worst defects in the list sit in Phase 3.

Work the **waves** below. Phase numbers stay as labels so the finding index
still resolves.

### What changed and why

1. **Severity was not driving order.** Finding 8 causes
   `fatal error: concurrent map writes`, which Go does **not** allow you to
   recover from. It kills the whole server for every player. It is reachable
   from tab-completion: `GetAutoComplete` (`world.go:321`) holds only
   `RLockMud` and calls `rooms.LoadRoom` three times, and an uncached load
   writes `roomIdToFileCache` with no synchronization. Two players
   tab-completing into uncached rooms is enough. It was scheduled behind two
   entire phases.
2. **Several dependencies were invented.** Deleting unused slices (6.4) did not
   need parser convergence (5.6). Fixing a nil dereference (1.3) did not need
   CI lint parity (1.1); that was proven by shipping it. Adding one missing
   authorization check (5.2) does not need transactional combat entry (5.1).
3. **"Depends on 0.2" is now noise.** 0.2 is Done. Twenty chunks still list it,
   which made everything look gated.
4. **CI parity is a parallel track, not a gate.** Chunks 1.1/1.2 touch
   `.github/`, `Makefile`, and test files only. Zero file overlap with Wave 1.
   Sequencing them first bought nothing and blocked everything.
5. **There was no cheap-fix lane**, so trivial deletions inherited heavy
   dependencies and never shipped.

### Wave 0 — Done 2026-08-08

Findings 11, 12, 28, 32 (chunks 5.3, 5.4, 5.5, 1.3) plus the repository half of
1.5. Finding 28 turned out to be four sites, not the two the review named.

### Wave 1 — Stop the crashes and the unauthorized actions — COMPLETE 2026-08-10

All eight rows below are closed. Findings 8, 22, 4, 21 (concurrency) shipped
2026-08-08; findings 3 and 17 on 2026-08-10 in PR #7; findings 20a, 29 and 31
on 2026-08-10.

The retriage was worth it: the old dependency ordering had spent 61 commits
without closing a single finding, and reordering by risk closed this whole wave
in three days. The recurring lesson, now seen on findings 8, 28, 3, 17 and 31
alike, is that **a finding's written scope is a lower bound**. Every one of
them turned out to touch more sites than the review named, so verify scope
against the code before estimating.

Track B (chunks 1.1 and 1.2) is still open and was always parallel to this
wave, not gated by it.

All small, all independently shippable. Nothing here blocks anything else.

| Order | Finding | Chunk | Why now |
|---:|---:|---|---|
| 1 | **8** | 3.3 | Unrecoverable `fatal error`, kills the server, reachable from tab-completion |
| 2 | **22** | 3.4 | Listener registration deadlock; a slow handler blocks all registration |
| 3 | **3** | 5.2 | Harmful spells bypass `PlayerAttackImmune`; can break quests and tutorial NPCs. **Dependency on 5.1 dropped** |
| 4 | **4** | 3.1 | Data race on the round counter; `atomic.Uint64` |
| 5 | **21** | 3.2 | Duplicate LLM requests double-mutate dialogue state |
| 6 | **17** | 4.1 | Stored XSS in an admin dashboard; `textContent` swap |
| 7 | **20a** | 4.2 | Server timeouts only. Split from the Host-redirect half |
| 8 | **29, 31** | 6.2, 6.4 | Trivial deletions. **Both invented dependencies dropped** |

**Track B — COMPLETE 2026-08-10.** Chunks 1.1 (findings 10, 25, 26) and 1.2
(findings 9, 24). Ran parallel to Wave 1 on a disjoint file set, as planned.

Two notes for whoever picks up the next chunk:

- **Style-lint backlogs are now the blocker in two languages.** Go has 97
  golangci-lint findings and JavaScript has 1637 jshint findings, so both
  gates ship as new-issues-only (Go) or syntax-only (JS). Chunk 6.5 owns
  clearing them; until then neither can be made fully enforcing.
- **`make test` currently fails**, because it depends on `js-lint`, which runs
  the full jshint backlog. Fold that into 6.5 as well.

### Wave 2 — Stop silent state loss

Sequenced, because 2.1 defines the contract the rest consume. This is the
largest genuine risk cluster after Wave 1: a torn write currently discards mob
progression, resets a shop to template defaults, or resurrects an unbanned
account.

2.1 (contract) → 2.2 (finding 5) → 2.3 (finding 7) → 2.4 (findings 6, 15) →
2.7 (finding 35) → 2.6 (finding 13) → 2.5 (findings 14, 16)

### Wave 3 — Availability

3.5 (finding 34, the global admin write lock) then 3.6a (finding 36, measure
autosave pauses). 3.6b only if 3.6a's evidence demands it. Do not pre-commit to
a persistence rearchitecture.

**Input for 3.6a, measured 2026-08-10 during chunk 2.1.** `SafeSave` now
fsyncs, which costs a measured **+1.45 ms per file** on a local SSD
(0.60 ms → 2.05 ms; `go test ./internal/util/ -bench Save -run '^$'`). Re-measure
on the droplet, where a throttled cloud volume will be worse.

That interacts badly with how autosave works today: `SaveAllRooms`
(`internal/rooms/save_and_load.go:427`) iterates `roomManager.rooms` and writes
**every loaded non-ephemeral room, with no dirty check**. At 1383 world rooms
that is up to ~2 s of added world-lock pause.

**The likely fix is not to drop the fsync.** Skipping unchanged rooms would cut
the write count by orders of magnitude and pays for durability many times over.
Measure first, but treat "autosave writes everything unconditionally" as the
prime suspect rather than the flush.

While in there: that same function always returns `nil` and counts errors into
a log line, which is finding 35 (chunk 2.7).

### Wave 4 — Remaining UX and web surface

4.3 (finding 18), 4.4 (finding 19), and the Host-redirect half of finding 20.

### Wave 5 — Balance decisions (analysis only, no code)

5.7 and 5.8. These are independent of everything above and can be picked up any
time someone wants a design session rather than an implementation session.

### Wave 6 — Debt

5.1 (finding 2), 5.6 (finding 27), 6.1a–6.1d (finding 23), 1.4 (finding 33),
6.3 (finding 30), 6.6a–6.6c (finding 37), then 6.5 (lint backlog) last so
earlier waves delete part of it first.

### Wave 7 — Admin builder: trust, then discoverability

**New work, not from the 2026-08-07 review.** Added 2026-08-10 at the user's
request, and deliberately sequenced LAST: everything above is correctness or
data-loss risk in the live game, while this is tooling.

7.1 (focused adversarial review) then 7.2 (cross-reference index and search).
7.2 depends on 7.1 because the review may change what the payloads should carry.

**Why 7.1 runs after the earlier waves rather than now.** Two open findings are
already in exactly its problem space, and reviewing before they land would just
re-discover them:

- **Finding 13 / chunk 2.6** — builder operations report success after ignored
  save failures. That is the "bad save path" concern, already tracked.
- **Finding 34 / chunk 3.5** — the global admin write lock. That is the
  "mechanism locks up the MUD" concern, already tracked.

Chunks 4.3 (keyboard accessibility) and 4.4 (hot-path GMCP DOM rebuilds) also
touch these pages. 7.1 should treat all four as known and hunt for what they
do not cover.

### Dependency corrections applied

- Dropped `0.2` from every remaining chunk. It is Done; keeping it implied a gate.
- 5.2 no longer depends on 5.1.
- 6.4 no longer depends on 5.6.
- 6.2 no longer depends on 1.3 (1.3 is Done regardless).
- 1.3 never needed 1.1. Proven by shipping it 2026-08-08.

## Status and size

**Status:** Not started · Brainstorming · Spec review · Planning · Plan review ·
Implementing · Implementation review · Done · Blocked · Cancelled · Invalidated

Chunk status tracks delivery. The finding coverage index separately tracks each
finding as Open, Done, or Invalidated and records evidence when it closes.

**Size:** S = contained · M = multi-file · L = subsystem-wide · XL = requires
further decomposition

## Progress tracker

| Chunk | Problem space | Size | Depends on | Findings | Status |
|---|---|---:|---|---|---|
| 0.1 | Restore web-terminal release asset | S | — | 1 | Invalidated |
| 0.2 | Establish a reproducible full-test baseline | S | — | Supporting | Done |
| 0.3a | Build the ephemeral server supervisor | M | — | Supporting | Done |
| 0.3b | Materialize synthetic player profiles | L | 0.3a | Supporting | Done |
| 0.3c | Integrate single-agent ephemeral playtests | M | 0.3a, 0.3b | Supporting | Done |
| 0.3d | Integrate multi-agent ephemeral scenarios | L | 0.3c | Supporting | Done |
| 1.1 | Unify validation across PR, master, and release | M | — | 10, 25, 26 | **Done 2026-08-10** |
| 1.2 | Replace phantom and probabilistic tests | M | — | 9, 24 | **Done 2026-08-10** |
| 1.3 | Eliminate immediate static-analysis crash risks | S | — | 28 | **Done 2026-08-08** |
| 1.4 | Decide and enforce the YAML compatibility boundary | M | — | 33 | Not started |
| 1.5 | Remove tracked playtest credentials | S | — | Security follow-up | **In progress — tree clean, ROTATION OUTSTANDING** |
| 2.1 | Establish the living-state persistence contract | M | 1.4 | Supporting | **Contract Done 2026-08-10; store migrations are 2.2–2.5** |
| 2.2 | Migrate mob instance persistence | M | 2.1 | 5 | Not started |
| 2.3 | Migrate guild and moderation persistence | M | 2.1 | 7 | Not started |
| 2.4 | Separate corrupt shop and room state from absence | M | 2.1 | 6, 15 | Not started |
| 2.5 | Make authored-content validation fail honestly | M | 1.1, 1.4 | 14, 16 | Not started |
| 2.6 | Make builder operations transactional | M | 2.1 | 13 | Not started |
| 2.7 | Make autosave outcomes observable | M | 2.1 | 35 | Not started |
| 3.1 | Make global counters race-free | S | — | 4 | **Done 2026-08-08** |
| 3.2 | Make LLM request admission atomic | S | — | 21 | **Done 2026-08-08** |
| 3.3 | Synchronize the room path cache | M | — | 8 | **Done 2026-08-08** |
| 3.4 | Release listener locks before callbacks | S | — | 22 | **Done 2026-08-08** |
| 3.5 | Bound admin world-lock scope | M | — | 34 | Not started |
| 3.6a | Measure autosave pauses and set a budget | M | 2.7 | 36 (measure) | Not started |
| 3.6b | Remediate autosave pauses if required | XL | 3.6a | 36 (conditional) | Not started |
| 4.1 | Remove admin stored-XSS surfaces | S | — | 17 | **Done 2026-08-10** |
| 4.2 | Harden HTTP server boundaries | M | — | 20 | **20a Done 2026-08-10; Host-redirect half open (Wave 4)** |
| 4.3 | Restore keyboard accessibility | M | — | 18 | Not started |
| 4.4 | Remove hot-path GMCP DOM rebuilds | L | — | 19 | Not started |
| 5.1 | Make combat entry transactional | M | 1.2 | 2 | Not started |
| 5.2 | Unify harmful-target authorization | M | — | 3 | **Done 2026-08-10** |
| 5.3 | Fix filtered wandering | S | — | 11 | **Done 2026-08-08** |
| 5.4 | Fix gold-give parsing | S | — | 12 | **Done 2026-08-08** |
| 5.5 | Repair the ANSI wrapper fallback contract | S | — | 32 | **Done 2026-08-08** |
| 5.6 | Converge composition-heavy commands on the parser | M | 1.2 | 27 | Not started |
| 5.7 | Decide post-soft-cap skill effectiveness | M | — | Design decision | Not started |
| 5.8 | Decide opposed-roll variance ownership | L | — | Design decision | Not started |
| 6.1a | Consolidate action-layer duplication | M | 5.1, 5.2, 5.6 | 23 (actions) | Not started |
| 6.1b | Consolidate position duplication | M | 1.2, 5.1 | 23 (position) | Not started |
| 6.1c | Consolidate mob-command duplication | M | 5.1, 5.2 | 23 (mob commands) | Not started |
| 6.1d | Consolidate duplicated test fixtures | S | 1.2, 6.1a–6.1c | 23 (tests) | Not started |
| 6.2 | Remove the dead progression hook | S | — | 29 | **Done 2026-08-10** |
| 6.3 | Retire stale cadence config and documentation | S | 1.4 | 30 | Not started |
| 6.4 | Remove dead corpse lookup structures | S | — | 31 | **Done 2026-08-10** |
| 6.5 | Retire the remaining lint backlog | L | 6.1a–6.4, 6.6a–6.6c | Cross-cutting | Not started |
| 6.6a | Inventory and define boot dependency seams | M | — | 37 (contract) | Not started |
| 6.6b | Freeze production boot registration | M | 6.6a | 37 (production) | Not started |
| 6.6c | Isolate callback overrides in tests | M | 6.6a | 37 (tests) | Not started |
| 7.1 | Adversarial review of the admin builder pages | L | 2.6, 3.5 | New (not from the review) | Not started |
| 7.2 | Cross-reference index and search for authored content | M | 7.1 | New (not from the review) | Not started |

---

## Phase 0 — Restore release viability

### Chunk 0.1 — Restore web-terminal release asset

**Status:** Invalidated 2026-08-07.

**Original premise:** Shipped HTML references an xterm core runtime that is
absent from the repository and copied release payload.

**Invalidation evidence:** Direct Git-tree inspection shows both `HEAD` and
`origin/master` track
`_datafiles/html/public/static/js/xterm.4.19.0.js` as blob
`fbe149ccd58871e9df6301108a4cbedd7ad9a8a4` (387,768 bytes; SHA-256
`f5d4d231cd6a3f6e9fb49d899427fa9409d7e4dc2344b0a3ee3a8fca15093f4b`).
The release workflow copies the containing `_datafiles` tree. Indexed file
search had hidden the large minified file and produced the false absence claim.

**Disposition:** Finding 1 is invalidated. No implementation is required.
General shipped-asset validation may be proposed separately, but is not
remediation for this finding.

### Chunk 0.2 — Establish a reproducible full-test baseline

**Problem:** The review's full test run was inconclusive because Windows
Defender blocked the generated relationships test executable.

**Outcome:** The project has a documented, repeatable way to execute the
ordinary race-enabled package suite locally and in CI with unambiguous
pass/fail output. Tests behind explicit opt-in environment gates remain visible
as skips. Selected gated tests retain their existing dedicated CI execution;
Chunk 1.1 inventories and assigns any gated tests not currently invoked there.

**Boundary:** This is an enabling chunk, not a review finding. It must not
weaken antivirus, sandbox, or test coverage merely to obtain a green result.

**Status:** Done 2026-08-07.

**Evidence:** The canonical
`docker compose -f compose.test.yml run --build --rm test` invocation completed
with exit `0`, no failed packages, panic, or race report, and explicit skip
output. Docker-context probes excluded credential and runtime state while
preserving required tracked and current authored files; exit status `7`
propagated unchanged; the untargeted production image remained the Alpine
runner with no test-stage `/src`; final adversarial implementation review
reported no blocking or important findings.

### Chunk 0.3 — Ephemeral branch/worktree playtest arc

**Problem:** Local playtests assume one host server at `localhost:55555`, while
the existing Compose service uses fixed names and ports and does not publish the
AI listener. This makes branch-specific isolation, parallel worktree playtests,
reliable cleanup, and unattended report collection cumbersome.

**Arc outcome:** One agent-facing command can build the selected checkout,
prepare suitable synthetic players, start a disposable local DOGMud server,
drive single- or multi-agent playtests, preserve reports and logs, and remove
all run state. Independent worktrees can run concurrently without sharing
ports, accounts, saves, or Compose project state.

**Arc boundary:** The entire arc is local-only. It never targets production,
accepts a remote endpoint, consumes archived production users at runtime,
exports in-game admin/build mutations back into source, or replaces content's
required adversarial playtest review.

#### Chunk 0.3a — Build the ephemeral server supervisor

**Outcome:** A cross-platform Go supervisor plus dedicated Compose definition
can build an explicit checkout, launch a uniquely labelled disposable server on
a Docker-assigned loopback AI port, prove compound readiness, expose
machine-readable lifecycle commands, preserve failure logs, and tear down or
reap only that run's resources.

**Boundary:** This chunk ends at a verified local AI endpoint. It does not
create players, invoke `mudagent`, make gameplay decisions, or produce a
gameplay report. The checkout is never mounted into the server; all admin and
builder mutations remain in a disposable volume that cannot be committed.

**Status (2026-08-08):** Done / verification. Windows Docker integration
(`TestDockerIntegration`) PASS; native
`go test ./cmd/playtestenv ./internal/playtestenv` PASS; managed Docker residue
filters empty. Fingerprint host-independence fixed in `0d499c39a`; Chunk 0.2
`docker compose -f compose.test.yml run --build --rm test` PASS. Production
runner smoke PASS (`Server Ready`) on local npipe context. Linux GitHub
`Playtest Environment Integration` workflow not yet run (no push/PR
authorized). Adversarial implementation review: approve with follow-ups —
no Blocking code findings; remaining gate is remote Linux workflow evidence.

#### Chunk 0.3b — Materialize synthetic player profiles

**Outcome:** Goals can reference tracked, sanitized synthetic profiles and an
explicit validated start room. DOGMud materializes run-scoped offline users
after world loading and migrations but before listeners, with generated
credentials and deterministic supported gameplay state.

**Boundary:** Production archives are design references only, never runtime
inputs. Initial profile scope is identity, role, room, base/training stats,
skills, quest tokens/flags, ordinary inventory, and validated equipment.
Goal→profile binding and mudagent remain 0.3c.

**Status (2026-08-08):** Done on
`feature/stage-0.3b-synthetic-playtest-profiles-v2` — six templates,
`internal/playtestprofiles` materializer, `main.go` boot hook, playtestenv
manifest/overrides/`Artifacts.Creds`, runner `Dockerfile` COPY. Spec/plan:
`docs/superpowers/specs/2026-08-08-synthetic-playtest-profiles-design.md`,
`docs/superpowers/plans/2026-08-08-synthetic-playtest-profiles.md`.
Evidence: package unit tests green; Windows Docker
`TestDockerIntegration/profiles_*` PASS (fresh+creds+AI login,
veteran+heal overlay, bad room fail, empty creation-flow). Adversarial
implementation review: approve-with-follow-ups (prod-name denylist,
quest-flag fail-closed, multi-entry failure test addressed; broader
world-Validate CI coverage remains a follow-up).

#### Chunk 0.3c — Integrate single-agent ephemeral playtests

**Outcome:** One agent command selects or authors goals, binds an appropriate
profile (or explicit creation-flow), starts the ephemeral supervisor, drives
the existing `mudagent` loop, writes a run-scoped gameplay report, and
guarantees cleanup while allowing other work to continue. Each run has an
explicit **wall-clock** budget (Go-enforced). Command spam is paced by
in-engine `AICommandsPerRound`. Token/API limits remain soft driver guidance;
exhaustion or wall-clock expiry produces a structured **incomplete** gameplay
report (partial findings + cleanup outcome), never a false success.

**Boundary:** Explicit user requests and existing SOP-required adversarial
playtests may trigger the command automatically. The command accepts no
production or remote target. Design:
`docs/superpowers/specs/2026-08-08-single-agent-ephemeral-playtests-design.md`.

**Status (2026-08-08):** Done on
`feature/stage-0.3c-single-agent-ephemeral-playtests`. Delivered
`internal/playtestrun` + `cmd/playtestrun` (`run`/`status`/`stop`), goals
`ephemeral:` binding, session sidecar, run-scoped bridge, `/playtest` local
rewire, exemplars (`newbie-naive`, `corpse-looting`, `2026-08-03-prepush-sweep`),
report templates, verbose Human invocation docs. Evidence:
`go test ./internal/playtestrun ./cmd/playtestrun`; Docker
`DOGMUD_PLAYTESTRUN_INTEGRATION=1 go test -run TestDockerPlaytestrun`
(profile + creation-flow ready/stop) PASS. Pre-0.3c local-path dead-code
cleanup deferred (spec Non-goals).

#### Chunk 0.3d — Integrate multi-agent ephemeral scenarios

**Outcome:** Scenario rosters bind each actor to its own goals file and
character loadout (`ephemeral:`), share one ephemeral server, coordinate via
in-game channels plus a file blackboard, isolate per-actor bridges/creds, and
produce one combined report. A scenario **wall-clock** is the hard cut;
per-actor token/turn limits remain soft guidelines. When an actor stops early,
an explicit `on_actor_stop` policy (default **continue**) preserves other
actors' evidence and still guarantees environment cleanup. Non-interacting
parallel work stays multiple 0.3c runs. Design:
`docs/superpowers/specs/2026-08-08-multi-agent-ephemeral-scenarios-design.md`.

**Status: Done (2026-08-08).** Evidence: `playtestrun scenario` +
`ParseScenario` / `RunScenario`; prod-identity denylist + `ForbiddenIdentity`;
`actor_id` creds; migrated party/parallel/pothole scenarios; `/playtest-scenario`
rewritten off ptorch; Docker
`TestDockerPlaytestrunScenario` PASS; live mudagent party-formation smoke
PASS (invite+accept) via `tools/playtest/cmd/party-smoke` (report under
`tools/playtest/reports/`, gitignored). PvP `adversarial-contest` deferred.

**Boundary:** This extends the existing harness scenario protocol; it does not
introduce a new autonomous model runner, require `ptorch`, or general
production-user cloning.

---

## Phase 1 — Make the safety net trustworthy

### Chunk 1.1 — Unify validation across PR, master, and release

**Problem:** Pull requests receive stronger validation than direct master and
release builds; generated-file and JavaScript drift are not gated; local
Makefile targets use stale toolchain and world paths.

**Outcome:** PR, master, and tag workflows consume one validation contract.
Local documented commands exercise the same contract and current DOGMud paths.

**Includes:**

- CI lint and coverage parity.
- Generated-file cleanliness.
- JavaScript validation.
- Current Go toolchain alignment.
- Correct DOGMud instance-clean targets.

**Findings:** 10, 25, 26

### Chunk 1.2 — Replace phantom and probabilistic tests

**Problem:** Position tests claim coverage in a deleted file, and combat tests
skip their assertions when random rolls do not cooperate.

**Outcome:** Every retained behavior-matrix test executes against current APIs,
and random-dependent behavior is driven deterministically.

**Boundary:** Do not preserve placeholder test counts for appearance. A deleted
test is preferable to a permanently skipped fake.

**Findings:** 9, 24

### Chunk 1.3 — Eliminate immediate static-analysis crash risks

**Problem:** Static analysis identified likely nil dereferences in combat setup
and room rendering.

**Outcome:** The plausible panic paths have regression coverage and are either
fixed or invalidated with call-path evidence.

**Boundary:** The rest of the 105-item lint backlog belongs to Chunk 6.5.

**Finding:** 28

### Chunk 1.4 — Decide and enforce the YAML compatibility boundary

**Problem:** Production loaders and stores use YAML v2 and v3 without an
explicit compatibility contract. Deferring this decision until after
persistence work risks reworking the same serialization surfaces.

**Outcome:** The project records and tests a deliberate compatibility boundary
before persistence migrations begin. If full migration is selected, that work
is decomposed before Phase 2 rather than hidden inside a persistence chunk.

**Boundary:** A blind mechanical version migration is explicitly excluded.

**Finding:** 33

### Chunk 1.5 — Remove tracked playtest credentials

**Problem:** `tools/playtest/targets.yaml` tracks operational local and
production playtest credentials in plaintext. Docker context filtering can keep
that file out of new image layers, but it cannot revoke exposed credentials or
remove them from repository history.

**Correction 2026-08-08:** the original finding named one file. There were
**two**. `tools/_archive/testing-pre-harness/testing/targets.yaml` carried the
identical `smoketester` and `aitester` credentials and was missed by both
reviews. The repository is public, so both were world-readable.

**Status: In progress 2026-08-08 (tree clean, rotation OUTSTANDING).**

Done in the working tree:

- `tools/playtest/targets.yaml` untracked via `git rm --cached`; the local copy
  survives on disk so `/playtest prod` keeps working.
- `tools/_archive/testing-pre-harness/testing/targets.yaml` deleted outright.
  The pre-harness stack was retired 2026-06-08 and nothing consumes it.
- `tools/playtest/targets.example.yaml` added as a credential-free template.
- `.gitignore` now ignores `**/targets.yaml` with an explicit negation for the
  example file.
- `.claude/commands/playtest.md` and `CLAUDE.md` document the copy-from-example
  setup step and the never-commit rule.

**Still required, and not satisfiable by any repository change:**

1. **Rotate the `aitester` password on `dogmud.org`.** It was public. Untracking
   does not un-expose it; the blob remains reachable in history and in every
   existing clone and fork.
2. **Rotate the local `smoketester` password** if that account exists anywhere
   reachable.
3. Confirm `aitester` holds no admin role.
4. Decide separately on history rewriting. It is disruptive, it does not
   substitute for rotation, and it cannot reach forks or clones.

**Outcome:** Tracked playtest configuration contains no credentials; local
secrets come from an ignored override or environment-backed mechanism; affected
accounts are rotated; and automated checks reject future credential-bearing
target files.

**Boundary:** Credential rotation requires coordination with the target
accounts. History rewriting is a separate explicit decision because it is
disruptive and does not substitute for rotation.

**Phase 1 exit:** Full tests are reproducible; validation is equivalent across
delivery paths; generated code and JavaScript are checked; no retained test
pretends to execute behavior it always skips; YAML compatibility expectations
are explicit.

---

## Phase 2 — Protect persistent and authored state

### Chunk 2.1 — Establish the living-state persistence contract

**Problem:** Mob, guild, and moderation stores use inconsistent write semantics,
and some commands mutate memory or report success before durable persistence.

**Outcome:** Living-state stores share an explicit contract for atomic writes,
error propagation, memory publication, and rollback behavior.

**Sequencing note:** Define the contract before migrating individual stores.
The spec should compare existing careful-save implementations and select one
canonical pattern.

**Boundary:** This enabling chunk establishes and proves the shared contract. It
does not claim Findings 5 or 7 complete until their store-specific chunks land.

**Delivered 2026-08-10.** The canonical pattern already existed — `util.Save` /
`util.SafeSave`, safe-by-default since 2026-07-31 — so this was hardening and
completing it rather than inventing one.

The contract is four rules, documented in `internal/util/livingstate.go`:

1. **Write atomically AND durably** — `SafeSave` now fsyncs the temp file
   before the rename and syncs the directory after it. It previously did
   neither, so the rename could be recorded while the data sat in the page
   cache: a power loss produced an atomically-renamed *empty* file, the exact
   corruption temp-and-rename exists to prevent. It also cleans up the `.new`
   file on failure and writes 0644 rather than 0777.
2. **Never conflate absent with corrupt** — `ReadLivingState` returns
   `ErrStateAbsent` vs `ErrStateCorrupt`. This is the rule every store broke.
3. **On corruption, quarantine then continue** — `QuarantineCorrupt` moves the
   file aside with a nanosecond-stamped name (repeated corruption cannot
   overwrite earlier evidence), never deletes, and leaves the path reading as
   absent so the caller's normal seed-defaults path takes over. Policy chosen
   by the user 2026-08-10 over refusing to boot, because one bad byte should
   not take the game offline.
4. **Persist before publishing** — an ordering discipline, no helper. Build the
   new state as a value, write it, and mutate the in-memory registry only after
   the write returns nil. This is finding 7's actual defect.

11 tests plus 4 benchmarks. **Adoption is deliberately NOT in this chunk**: six
living-state sites still call `os.WriteFile` directly — `mobs/instance_save.go:161`,
`mobs/mobs.go:1244`, `guilds/persistence.go:45`, `moderation/bans.go:112`,
`moderation/petitions.go:122`, `rooms/zone_rename.go:118`. The review named
four of those; the last two are new. They migrate in 2.2–2.5.

See the Wave 3 note for the measured fsync cost and why it points at autosave's
missing dirty check rather than at the flush.

### Chunk 2.2 — Migrate mob instance persistence

**Problem:** Mob instances are written non-atomically, and malformed instance
files are treated as if no saved progression exists.

**Outcome:** Mob saves follow the persistence contract, and missing, unreadable,
and malformed instance files produce distinct, non-destructive outcomes.

**Finding:** 5

### Chunk 2.3 — Migrate guild and moderation persistence

**Problem:** Guild, ban, and petition operations can publish in-memory success
before durable storage succeeds, while commands discard save failures.

**Outcome:** Guild and moderation state follows the persistence contract, and
administrative success is reported only after durable success.

**Boundary:** Other Mob Aliveness stores are not implicitly included. Any newly
discovered store defect becomes a separately tracked finding.

**Finding:** 7

### Chunk 2.4 — Separate corrupt shop and room state from absence

**Problem:** Shop corruption is treated as a new shop, while corrupt room
overlays can be partially applied.

**Outcome:** Loaders distinguish absence from corruption. Corrupt living state
is preserved or quarantined, never silently reseeded or partially merged.

**Findings:** 6, 15

### Chunk 2.5 — Make authored-content validation fail honestly

**Problem:** Dialogue parse failures are cached as absence, and quest-flag
validation skips files it cannot read or parse.

**Outcome:** Validation reports every file it could not inspect. Runtime caches
do not convert transient parse errors into process-lifetime absence. CI and
boot smoke apply the existing dialogue and quest semantic validators across
every authored file rather than limiting those checks to editor-save paths.

**Boundary:** This chunk defines failure semantics; it does not rewrite authored
dialogue or quests. It owns whole-world semantic validator behavior and
entrypoints plus registering them with the shared validation workflow. Chunk
1.1 owns that workflow framework and the parity contract consumed here.

**Findings:** 14, 16

### Chunk 2.6 — Make builder operations transactional

**Problem:** Zone creation can ignore room/config save failures, update runtime
state, and still report success.

**Outcome:** Builder operations either persist and publish a coherent result or
return a failure without leaving a half-created zone.

**Finding:** 13

**Dependency note:** Builder transactions use the persistence contract's atomic
write, memory-publication, and rollback semantics rather than inventing a
builder-only durability model.

### Chunk 2.7 — Make autosave outcomes observable

**Problem:** Room autosave counts failures but returns `nil`; autosave ignores
that result; empty-instance deletion errors are discarded; and plugin save
callbacks cannot report persistence failures.

**Outcome:** User, room, and plugin autosave paths return aggregate outcomes.
Failures are logged and surfaced without reporting a successful autosave, and
shutdown/copyover callers can enforce the same persistence contract.

**Boundary:** This chunk owns error propagation and outcome reporting. It does
not move persistence to background goroutines or preselect an autosave
performance architecture.

**Finding:** 35

**Phase 2 exit:** A failed save cannot be reported as success; corruption cannot
be mistaken for absence; content validation cannot claim to inspect files it
skipped.

---

## Phase 3 — Make concurrent runtime state explicit

### Chunk 3.1 — Make global counters race-free

**Problem:** Round counters cross goroutine boundaries without synchronization.

**Outcome:** Counter visibility has explicit synchronization semantics with
race-focused tests.

**Finding:** 4

### Chunk 3.2 — Make LLM request admission atomic

**Problem:** LLM pending admission performs separately locked check and set
operations.

**Outcome:** At most one request for a mob can acquire the pending slot.

**Finding:** 21

### Chunk 3.3 — Synchronize the room path cache

**Problem:** The room path cache can be written under concurrent read-side
world access.

**Outcome:** Room-manager ownership is explicit and concurrent first loads are
race-safe.

**Finding:** 8

### Chunk 3.4 — Release listener locks before callbacks

**Problem:** Event dispatch holds its registry lock across arbitrary callbacks.

**Outcome:** Callback execution cannot deadlock listener registration or block
registry access for the full callback duration.

**Finding:** 22

### Chunk 3.5 — Bound admin world-lock scope

**Problem:** Admin routes acquire the global world write lock before
authentication and retain it through disk reads, template rendering, JSON
encoding, static-file serving, and response writes.

**Outcome:** Each admin route declares its world-state access needs. Read routes
and mutation routes retain the world lock only for the minimum shared-state
access required; authentication, disk access, rendering, encoding, static-file
serving, and response writes occur outside it.

**Boundary:** Coordinate with Chunk 4.2 HTTP timeouts, but do not absorb general
HTTP hardening. Autocomplete remains unchanged unless profiling demonstrates a
separate lock-budget problem. Immutable snapshots, MainWorker request/response,
and subsystem-local synchronization are design options, not predetermined
solutions.

**Finding:** 34

### Autosave pause arc 3.6 — Measure first, remediate conditionally

**Problem:** Autosave serially saves users, loaded rooms, and plugin state while
the MainWorker holds the global world write lock. The pause mechanism is
structurally proven, but current severity is not measured.

**Shared outcome:** Autosave behavior is measured against an approved
player-visible pause budget. The arc either closes with evidence that the
current design meets that budget or delivers a separately designed remediation
that does.

**Finding:** 36, decomposed across Chunks 3.6a–3.6b with one shared status.

#### Chunk 3.6a — Measure autosave pauses and set a budget

**Outcome:** Representative benchmarks and runtime instrumentation establish
loaded-set sizes, autosave duration, player-visible tick delay, and an approved
pause budget. The final disposition explicitly records whether remediation is
required.

**Boundary:** This chunk produces evidence and a decision only. It does not
change persistence architecture.

#### Chunk 3.6b — Remediate autosave pauses if required

**Activation:** Execute only if Chunk 3.6a shows the approved budget is
exceeded. Otherwise cancel this chunk with the measurement evidence.

**Outcome:** A separately brainstormed and approved design reduces lock-held
work to the budget without stale overwrites, reordering, lost failures, or
unsafe shallow snapshots.

**Boundary:** Dirty tracking, template caching, precomputed projections, and
immutable snapshots plus background writes are options to evaluate, not
predetermined fixes. Decompose this XL chunk before implementation.

**Phase 3 exit:** The race detector covers each repaired ownership boundary,
callback execution cannot deadlock listener registration, admin response
delivery cannot retain the world lock, and autosave pauses are measured and
accepted against an approved budget or remediated to meet it.

---

## Phase 4 — Harden browser and HTTP surfaces

### Chunk 4.1 — Remove admin stored-XSS surfaces

**Problem:** The economy dashboard inserts authored names through raw HTML.

**Outcome:** Authored and persisted strings cannot be interpreted as executable
markup by admin dashboards.

**Finding:** 17

### Chunk 4.2 — Harden HTTP server boundaries

**Problem:** HTTP servers lack defensive timeouts and redirects derive their
destination host from request input.

**Outcome:** Slow-client resource exposure is bounded and redirects cannot be
steered to an untrusted host.

**Finding:** 20

### Chunk 4.3 — Restore keyboard accessibility

**Problem:** Global key capture and non-semantic controls make dashboard panels
difficult or impossible to operate without a mouse.

**Outcome:** Command shortcuts are scoped, controls are focusable and named, and
keyboard focus remains under user control.

**Finding:** 18

### Chunk 4.4 — Remove hot-path GMCP DOM rebuilds

**Problem:** Map, inventory, and status updates destroy and recreate large DOM
subtrees on frequent GMCP messages.

**Outcome:** Representative large-zone and inventory updates show materially
less DOM work without changing displayed state or interaction behavior.

**Finding:** 19

**Phase 4 exit:** The web surface has explicit security, accessibility, and
performance baselines rather than relying on visual spot checks.

---

## Phase 5 — Correct player-visible behavior

### Chunk 5.1 — Make combat entry transactional

**Problem:** Legacy `Aggro` can be published even when the Combat Phase machine
rejects entry, allowing combat to coexist with blocked activities.

**Outcome:** Combat has one coherent entry result. Activity vetoes, legacy
compatibility, and combat predicates cannot disagree.

**Roadmap relationship:** This chunk is a narrow transactional repair. The
broader deferred `Aggro` compatibility sunset remains owned by
`COMBAT_STATE_ROADMAP.md` and may not be absorbed without a separate roadmap
decision.

**Finding:** 2

### Chunk 5.2 — Unify harmful-target authorization

**Problem:** Melee and harmful spells enforce different protections for
non-combatant and player-attack-immune mobs.

**Outcome:** One authorization policy governs harmful actions and is checked at
both initiation and delayed resolution where state can change.

**Finding:** 3

### Chunk 5.3 — Fix filtered wandering

**Problem:** Filtered wandering computes candidate exits and then ignores them.

**Outcome:** Loot- and player-directed wandering selects only eligible exits,
with explicit behavior when no eligible exit exists.

**Finding:** 11

### Chunk 5.4 — Fix gold-give parsing

**Problem:** Compact gold syntax resolves and executes with different slices of
the input amount.

**Outcome:** Gold resolution and transfer use one parse result for every
accepted syntax.

**Finding:** 12

### Chunk 5.5 — Repair the ANSI wrapper fallback contract

**Problem:** Panic recovery returns an empty string despite promising to return
the original text.

**Outcome:** Malformed or unexpected input cannot silently erase a message.

**Finding:** 32

### Chunk 5.6 — Converge composition-heavy commands on the parser

**Problem:** `get`, `put`, `steal`, and `loot` use different grammars for
multi-word objects, containers, and corpses.

**Outcome:** Composition-heavy commands share parsing/resolution seams while
ownership, discovery, and authorization remain command responsibilities.

**Finding:** 27

### Chunk 5.7 — Decide post-soft-cap skill effectiveness

**Question:** `SkillMultiplier` deliberately clamps at `SkillSoftCap`, while
player skill progression itself remains uncapped. Tests prove this is working
as designed, but no evidence establishes that the ceiling is working as
intended across damage, healing, search, stealth, stealing, and other consumers.

**Outcome:** The game has an explicit post-soft-cap effectiveness contract
supported by analytical examples and player-progression goals. The approved
result is either:

- preserve the multiplier ceiling and clarify terminology/documentation; or
- adopt a diminishing above-cap curve and rebalance every affected consumer.

**Boundary:** Do not infer intent from existing tests. This cycle requires a
balance-design decision and analytical evidence only. If behavior changes, add
a separately decomposed implementation arc to this roadmap before code
planning.

### Chunk 5.8 — Decide opposed-roll variance ownership

**Question:** `OpposedRollStat` deliberately rolls both participants with
attacker-derived standard deviation. This creates role-dependent probability
asymmetry. Documentation proves the behavior is designed, not that initiator-
owned variance is the intended game model.

**Outcome:** Analytical probability fixtures compare initiator-owned,
participant-owned, and neutral shared spread models across representative stat
gaps. The approved result either preserves and documents the current model or
retunes the shared dice contract and all affected systems coherently.

**Boundary:** This is not a local dice-function cleanup. Combat, spells,
grapples, stealth, stealing, and detection share the contract, so no code
changes occur in this decision-only chunk. If retuning is selected, add a
separately decomposed implementation arc before code planning.

**Phase 5 exit:** Combat state, target protection, command parsing, and command
results are consistent across equivalent player actions, and soft-cap and
opposed-roll behavior are verified as working as intended rather than merely
working as designed.

---

## Phase 6 — Consolidate and retire debt

### Duplication arc 6.1 — Consolidate implementation kernels and fixtures

**Problem:** Near-identical production paths encode move policy in copied
control flow.

**Shared outcome:** Equivalent paths have one policy owner, while intentional
behavior differences remain explicit and regression-tested.

**Sequencing note:** Each subchunk follows its exact gameplay dependencies so
consolidation is protected by current behavior tests rather than preserving
accidental bugs. In particular, action-layer steal consolidation follows parser
migration in Chunk 5.6.

**Finding:** 23, decomposed across Chunks 6.1a–6.1d with one shared status.

#### Chunk 6.1a — Consolidate action-layer duplication

**Scope:** Rally/warcry and plant/steal duplication identified by the review.

#### Chunk 6.1b — Consolidate position duplication

**Scope:** Disruption/modifier duplication. This chunk must reconcile its
behavior matrix with `COMBAT_STATE_ROADMAP.md` rather than treating the
completed roadmap as proof that the current tests are valid.

#### Chunk 6.1c — Consolidate mob-command duplication

**Scope:** Drain/maul/rake and howl/taunt duplication identified by the review.

#### Chunk 6.1d — Consolidate duplicated test fixtures

**Scope:** Shared setup and behavior-matrix fixtures duplicated across combat
and action tests. Consolidation must preserve test independence and failure
clarity rather than hiding behavior behind an over-general test framework.

### Chunk 6.2 — Remove the dead progression hook

**Problem:** A deprecated exported progression hook has no call sites and is
retained only for possible future reuse.

**Outcome:** The hook is removed or has a current owner, consumer, and explicit
removal condition.

**Finding:** 29

### Chunk 6.3 — Retire stale cadence config and documentation

**Problem:** Deprecated cadence configuration remains shipped and package
documentation still describes it as active.

**Outcome:** Runtime config, shipped config, compatibility policy, and package
documentation agree.

**Finding:** 30

### Chunk 6.4 — Remove dead corpse lookup structures

**Problem:** `look` builds corpse lookup slices that are never consumed.

**Outcome:** Dead structures are removed or replaced by a single lookup path
that has an active consumer.

**Finding:** 31

### Boot dependency arc 6.6 — Make wiring explicit and immutable

**Problem:** Package-level callback seams avoid import cycles but remain
mutable after startup and degrade through nil-safe fallbacks when wiring is
omitted. Normal boot order prevents the comparison review's claimed nil panics,
but dependency intent is implicit and tests must manually restore global
callbacks.

**Shared outcome:** Callback dependency intent is explicit, production
registration cannot race with workers, and tests cannot leak overrides across
cases.

**Boundary:** Preserve synchronous return and ordering contracts. Do not replace
callbacks mechanically with an asynchronous event bus.

**Finding:** 37, decomposed across Chunks 6.6a–6.6c with one shared status.

#### Chunk 6.6a — Inventory and define boot dependency seams

**Outcome:** Every callback seam has verified callers, a named owner, and an
explicit required-or-optional contract. Nil-safe fallbacks remain valid until
this inventory proves a dependency is required.

#### Chunk 6.6b — Freeze production boot registration

**Outcome:** Dependencies classified as required are validated before workers
start, and production registration cannot mutate after the boot boundary.
Optional seams retain documented fallback behavior.

#### Chunk 6.6c — Isolate callback overrides in tests

**Outcome:** Tests use race-safe scoped override helpers with automatic cleanup
and no cross-test pollution, without weakening production immutability.

### Chunk 6.5 — Retire the remaining lint backlog

**Problem:** New-issue-only enforcement leaves 105 known findings in the tree.

**Outcome:** Bug-finding lint is clean across the repository and CI no longer
needs to grandfather the baseline.

**Sequencing note:** Run this last so earlier correctness and consolidation
chunks naturally delete part of the backlog instead of creating throwaway
cleanup.

**Phase 6 exit:** No known review finding remains open, no bug-finding lint debt
is grandfathered, boot wiring is explicit and immutable, and project
documentation describes the code that actually runs.

---

## Milestones

### Milestone A — Trustworthy delivery

Phases 0 and 1 complete. Clean releases contain their assets, tests execute
honestly, all delivery paths enforce the same baseline, and YAML compatibility
is explicit before persistence work.

### Milestone B — Durable runtime

Phases 2 and 3 complete. Persistent state fails safely, validation reports what
it could not inspect, autosave outcomes are observable, autosave pauses are
accepted against a budget or remediated, and shared runtime state has explicit
concurrency ownership.

### Milestone C — Hardened experience

Phases 4 and 5 complete. Web surfaces are safer and usable, and equivalent
player actions obey consistent rules. Skill ceilings and opposed-roll variance
have explicit, approved intent.

### Milestone D — Clean baseline

Phase 6 complete. Duplicated kernels, dead compatibility surfaces, stale
configuration, and the lint backlog are retired.

## Parallelism guidance

Default to sequential chunks where files or contracts overlap. After Phase 1:

- Phase 2 persistence work and Phase 4 browser work can proceed independently.
- Chunks 3.1, 3.2, 3.4, and 3.5 can proceed beside Phase 2. Chunk 3.3 should
  avoid overlapping room persistence changes in Chunk 2.4. Chunk 3.6a follows
  autosave outcome work in Chunk 2.7; 3.6b activates only from its evidence.
- Chunks 5.3–5.5 can proceed early as independent contained fixes.
- Chunks 5.1 and 5.2 remain ordered. Chunk 5.6 may proceed independently after
  the test baseline is reliable. Chunks 5.7 and 5.8 are independent
  balance-design cycles and must finish design approval before code planning.
- Duplication subchunks 6.1a–6.1d follow their exact tracker dependencies;
  Chunks 6.2 and 6.3 can proceed earlier when their dependencies are complete.
  Arc 6.6 can proceed after the test baseline and before the final lint cleanup.

Parallel agents must use separate worktrees or preassigned file ownership and
must not create competing persistence, parser, or combat abstractions.

## Finding coverage index

Update **Status** and **Evidence** as findings close. A decomposed finding closes
only when all of its subchunks close.

| Finding | Owning chunk | Status | Evidence | Problem |
|---:|---|---|---|---|
| 1 | 0.1 | Invalidated | Git blob `fbe149c`; tracked in `HEAD` and `origin/master` | Missing xterm runtime |
| 2 | 5.1 | Open | — | Combat/activity state divergence |
| 3 | 5.2 | **Done** | `mobs.CheckPlayerHarm` policy + 7 sites; review named 1 (HarmSingle), actual scope was 5 cast-time paths, resolution-time re-check, and `target` + 21 tests | Harmful spells bypass attack immunity |
| 4 | 3.1 | **Done** | `atomic.Uint64`; exactness + monotonicity tests | Unsynchronized round counter |
| 5 | 2.2 | Open | — | Non-atomic and corruption-prone mob instance persistence |
| 6 | 2.4 | Open | — | Corrupt shop treated as new |
| 7 | 2.3 | Open | — | Guild/moderation memory-disk divergence |
| 8 | 3.3 | **Done** | `8dd24e4c8`; RWMutex + 7 accessors, 8 sites (review named 1) + 6 concurrency tests | Unsynchronized room path cache |
| 9 | 1.2 | **Done** | 164 skip-only placeholders deleted (review named 87 in one file; pattern spanned 18) + 4 emptied files removed + go/ast recurrence guard | Phantom position tests |
| 10 | 1.1 | **Done** | Reusable validate.yml consumed by PR, master and tags; only-new-issues verified to work on push | Release CI omits lint/coverage |
| 11 | 5.3 | **Done** | `e23df1071`; `pickWanderExit` + 5 tests | Wander filter ignored |
| 12 | 5.4 | **Done** | `e23df1071`; `parseGoldPhrase` shared by both paths + 2 tests | Gold-give parse mismatch |
| 13 | 2.6 | Open | — | Builder ignores save failures |
| 14 | 2.5 | Open | — | Dialogue parse error cached as absence |
| 15 | 2.4 | Open | — | Partial room overlay after YAML failure |
| 16 | 2.5 | Open | — | Quest validation skips unreadable dialogue |
| 17 | 4.1 | **Done** | `admin/static/js/safe-dom.js` default-safe helpers; economy AND progression converted (review named economy only; progression renders player-chosen names) + 12 JS checks | Admin economy stored-XSS surface |
| 18 | 4.3 | Open | — | Global keyboard capture |
| 19 | 4.4 | Open | — | Hot GMCP DOM rebuilds |
| 20 | 4.2 | **20a Done / 20b Open** | 20a: 4 Network timeout knobs, non-zero defaults, applied to both servers; websocket safety proven live (survived 15s vs a 3s WriteTimeout) and slow-header request cut at 3.0s + 6 tests. 20b (Host redirect) remains | HTTP timeout and Host redirect weaknesses |
| 21 | 3.2 | **Done** | `tryMarkPending` single critical section + 4 tests | LLM check-then-set race |
| 22 | 3.4 | **Done** | snapshot under RLock, invoke unlocked; deadlock reproduced pre-fix | Listener lock held across callbacks |
| 23 | 6.1a–6.1d | Open | — | Production and test implementation duplication |
| 24 | 1.2 | **Done** | 6 sites now Fatal instead of Skip; P(false fire) ~1e-84; verified -count=5 | Probabilistic tests skip assertions |
| 25 | 1.1 | **Done** | git diff --exit-code after go generate; node --check over 20 tracked JS files + safe-dom tests. Style lint deferred: jshint backlog is 1637 | Missing generated/JavaScript CI gates |
| 26 | 1.1 | **Done** | GO_VERSION derived from go.mod; clean-instances targets world/dogmud rooms+mobs instances | Stale Makefile toolchain and world paths |
| 27 | 5.6 | Open | — | Partial parser adoption |
| 28 | 1.3 | **Done** | `e23df1071`; 4 sites fixed (2 named, 2 found by the test) + 3 tests | Plausible nil dereferences |
| 29 | 6.2 | **Done** | `OnLowResource` deleted; the two doc mentions are correct as-is (context.md describes it as replaced, DEVELOPMENT_PLAN entry is a completed-stage record) | Dead progression hook |
| 30 | 6.3 | Open | — | Deprecated live config and stale docs |
| 31 | 6.4 | **Done** | Whole loop removed, not just the 2 slices the review named: both backing maps were dead too, since they only deduped into the unread slices | Dead corpse lookup arrays |
| 32 | 5.5 | **Done** | `e23df1071`; named result + explicit recover assign | Broken ANSI panic fallback |
| 33 | 1.4 | Open | — | Dual YAML major versions |
| 34 | 3.5 | Open | `RunWithMUDLocked` wraps auth, render, and response writes | Admin routes retain global world lock |
| 35 | 2.7 | Open | `SaveAllRooms` suppresses errors; autosave ignores outcomes | Autosave failures reported as success |
| 36 | 3.6a–3.6b | Open | Autosave executes synchronously inside locked `EventLoop` | Unmeasured autosave world-lock pauses |
| 37 | 6.6a–6.6c | Open | Startup callback setters remain mutable after worker launch | Implicit mutable boot dependency wiring |

## Roadmap completion criteria

This roadmap is complete when:

- Every numbered finding is Done or Invalidated with evidence.
- Both design-decision chunks conclude with approved WAI contracts and evidence;
  preserving current behavior is a valid result when explicitly chosen.
- All enabling and cross-cutting chunks are Done or explicitly Cancelled with
  rationale.
- Full tests, race tests selected by the chunk specs, boot smoke, and
  bug-finding lint pass under the unified validation workflow.
- The original adversarial review can be rerun without reproducing any open
  finding.

---

## Phase 7 — Admin builder (new work, added 2026-08-10)

Not from the 2026-08-07 review. Raised by the user after trying to edit the
Path #1 newcomer tutorial through the admin pages and being unable to find what
drove it. Sequenced last: everything above is correctness or data-loss risk in
the live game; this is tooling.

### Chunk 7.1 — Adversarial review of the admin builder pages

**Problem:** The builder writes authored world content directly into the live
server, and has never had a focused adversarial review. Its failure modes are
the expensive kind: a save that reports success but does not land, or an
operation that holds a lock and stalls the game for every connected player.

**Scope:** `/build` and `/build-help`, `modules/gmcp/gmcp.Build*.go`, the
`Build.*` GMCP verb surface, `internal/behaviortree` save paths, and the
editor JS under `_datafiles/html/public/static/js/`
(`builder`, `mobs`, `zones`, `dialogue`, `behaviors`, `quests`, `items`).

**Hunt for, at minimum:**

- **Save paths that can silently lose work.** Partial writes, writes that
  report success on failure, saves racing the autosave, and saves that clobber
  a file another editor session changed.
- **Anything that can stall or deadlock the world.** Long operations under the
  global lock, unbounded loops over rooms/mobs, and GMCP handlers doing
  filesystem work on the event loop.
- **Authorization on every `Build.*` verb**, not just the page. The page is
  admin-gated; confirm each verb re-checks rather than trusting that.
- **Destructive verbs without confirmation or undo** (`Delete` on zones,
  rooms, mobs, behaviors) and what they leave dangling.
- **Validation parity with the YAML loaders.** The builder must not be able to
  write content that panics the next boot. The loaders panic on unresolved
  references, filename/name mismatches and ID collisions; the builder should
  refuse those at save time.

**Known and already tracked — treat as background, hunt for what they miss:**
findings **13** (chunk 2.6, builder reports success after ignored save
failures), **34** (chunk 3.5, global admin write lock), **18** (4.3, keyboard
accessibility), **19** (4.4, hot-path GMCP DOM rebuilds).

**One confirmed defect to fold in.** The behavior-tree editor drops all
hand-written `#` comments on first save, warns about it in the UI, and offers
`note`/`notes` fields as the migration path. Every room-behavior file in the
newcomer tutorial carries its design rationale in exactly those comments. Decide
whether "warn and destroy" is the contract we want, or whether the writer should
round-trip comments.

**Outcome:** A written findings list with evidence, triaged the same way the
2026-08-07 review was, feeding new chunks.

**Findings:** New.

### Chunk 7.2 — Cross-reference index and search for authored content

**Problem:** Authored content cross-links heavily — quests drive rooms and
mobs, room behavior trees gate exits, dialogue hangs off mobs, room nouns are
the targets quest triggers match on — and the editor exposes none of those
links. You can only find a quest if you already know its id or name.

Evidence gathered 2026-08-10 while tracing the Path #1 tutorial:

- Quest search is `"search id or name"`, filtered client-side over the list.
  Searching `dewey`, `6258`, `tutorial` or `effigy` returns nothing.
- `Build.Quest.*` exposes only `Create`, `Delete`, `Get`, `List`, `Update`.
  There is no query verb, so no better search is reachable from the client.
- Nothing computes back-references. The mob inspector's only quest-shaped field
  is `questFlags`, which is the mob's own flags, not "quests that reference this
  mob". `builder.js` and `zones.js` mention quests zero times.
- Room behavior trees ARE editable, but only under Behaviors → kind `room` →
  roomId. The room's own inspector does not link to them, so the thing gating a
  room's exits is invisible from that room.
- The pattern already exists elsewhere: the behavior editor shows archetypes as
  "used by N mobs" and blocks deletion with a `BehaviorRefs` list. Quests, rooms
  and dialogue never got the same treatment.

Worked example of the cost: quest 28 "Waking to Gaius" drives the entire
newcomer tutorial through `room_enter` and `command_issued` triggers, speaking
every one of Dewey's lines via `npc_say`. Standing in room 6258 looking at
Dewey, nothing in the editor names quest 28. The only in-product route is
`questdebug <player>`, walking the tutorial, and reading tokens as they fire.

**Outcome:** Opening a room, mob, item or dialogue shows what references it,
and search finds authored content by what it *does*, not only by its name.

**Sketch, to be confirmed by 7.1:** build the reverse index at load from data
already present in the quest triggers (`room:`, `mob:`, `command:`, `noun:`);
surface it as a `references` field on the existing `Build.*.Get` payloads
rather than a new verb; widen quest search to match trigger contents; link a
room to its room-behavior tree from the room inspector.

**Check first:** whether the dialogue and item editors have the same blind spot
before fixing the payload shape, so the field is designed once.

**Findings:** New.
