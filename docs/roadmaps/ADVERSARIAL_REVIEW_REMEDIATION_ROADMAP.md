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
**COMPLETE 2026-08-10 (PR #16).** Spawned one follow-on slice, **4.5**, which is
not urgent and can be picked up whenever the web surface is next open.

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
| 2.2 | Migrate mob instance persistence | M | 2.1 | 5 | **Done 2026-08-10** |
| 2.3 | Migrate guild and moderation persistence | M | 2.1 | 7 | **Done 2026-08-10** |
| 2.4 | Separate corrupt shop and room state from absence | M | 2.1 | 6, 15 | **Done 2026-08-10** |
| 2.5 | Make authored-content validation fail honestly | M | 1.1, 1.4 | 14, 16 | **Done 2026-08-10** |
| 2.6 | Make builder operations transactional | M | 2.1 | 13 | **Done 2026-08-10** |
| 2.7 | Make autosave outcomes observable | M | 2.1 | 35 | **Done 2026-08-10** |
| 2.8 | Adopt the persistence contract everywhere it was missed | M | 2.1 | New (Wave 2 scope miss) | **DONE** |
| 3.1 | Make global counters race-free | S | — | 4 | **Done 2026-08-08** |
| 3.2 | Make LLM request admission atomic | S | — | 21 | **Done 2026-08-08** |
| 3.3 | Synchronize the room path cache | M | — | 8 | **Done 2026-08-08** |
| 3.4 | Release listener locks before callbacks | S | — | 22 | **Done 2026-08-08** |
| 3.5 | Bound admin world-lock scope | M | — | 34 | **Done 2026-08-10** |
| 3.6a | Measure autosave pauses and set a budget | M | 2.7 | 36 (measure) | **Done 2026-08-10 — remediation REQUIRED** |
| 3.6b | Remediate autosave pauses if required | XL | 3.6a | 36 (conditional) | **3.6b-1 DONE; 3.6b-2 / 3.6b-3 open** |
| 4.1 | Remove admin stored-XSS surfaces | S | — | 17 | **Done 2026-08-10** |
| 4.2 | Harden HTTP server boundaries | M | — | 20 | **Done 2026-08-10 (20a + 20b)** |
| 4.3 | Restore keyboard accessibility | M | — | 18 | **Done 2026-08-10** |
| 4.4 | Remove hot-path GMCP DOM rebuilds | L | — | 19 | **Done 2026-08-10** |
| 4.5 | Skip unchanged map room-token rebuilds | M | 4.4 | 19 (follow-on) | Not started |
| 4.6 | Cache room templates for the autosave compare | S | 3.6b-1 | 36 (follow-on) | **Done 2026-08-10** |
| 4.7 | Amortise plugin saves | M | 3.6b-1, 2.8 | 36 (follow-on) | **Done 2026-08-11** |
| 4.8 | Plugin reads distinguish absent from corrupt | S | 2.1 | New (found in 4.7) | **Done 2026-08-11** |
| 5.1 | Make combat entry transactional | M | 1.2 | 2 | Not started |
| 5.2 | Unify harmful-target authorization | M | — | 3 | **Done 2026-08-10** |
| 5.3 | Fix filtered wandering | S | — | 11 | **Done 2026-08-08** |
| 5.4 | Fix gold-give parsing | S | — | 12 | **Done 2026-08-08** |
| 5.5 | Repair the ANSI wrapper fallback contract | S | — | 32 | **Done 2026-08-08** |
| 5.6 | Converge composition-heavy commands on the parser | M | 1.2 | 27 | Not started |
| 5.7 | Decide post-soft-cap skill effectiveness | M | — | Design decision | Not started |
| 5.8 | Decide opposed-roll variance ownership | L | — | Design decision | **DECIDED 2026-08-11: preserve** |
| 5.9a | Contest floors: stealth, theft, traps, detection | M | 5.8 | 5.8 follow-on | **Done 2026-08-11** |
| 5.9b | Contest floors: combat maneuvers and spells | M | 5.9a | 5.8 follow-on | Not started |
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

**Delivered 2026-08-10.** Both halves of finding 5:

- **Write.** `os.WriteFile` -> `util.Save`, so an interrupted write can no
  longer truncate the file. A mob instance file is the only record of what that
  mob became — stats, skills, mutations, gold, gear, planner state — and cannot
  be rebuilt from the repo.
- **Read.** `LoadMobInstance` returned nil for BOTH "no file" and "corrupt
  file", and nil means "spawn from template". So a torn file silently discarded
  everything the mob had accumulated, and the next save then overwrote the
  damaged file, destroying the only evidence any loss had occurred. It now
  quarantines (never deletes), logs at ERROR naming what was lost and where the
  bytes went, and then spawns from template. Quarantining also frees the path,
  so the next save succeeds rather than failing forever.

Absent stays silent — a mob that has never persisted anything is the ordinary
case and must not generate noise.

**Beyond the finding's letter:** `Mob.Save()` (`internal/mobs/mobs.go`), the
template writer the admin mob builder saves through, had the same bare
`os.WriteFile`. A mob template is authored content, so a truncated file does not
degrade quietly — it panics the next boot on a name/filename mismatch or an
unresolved reference. Same one-line fix, applied.

5 tests including the full corrupt -> quarantine -> reseed -> save -> load
recovery path, plus a control leg proving the round trip still works.

### Chunk 2.3 — Migrate guild and moderation persistence

**Problem:** Guild, ban, and petition operations can publish in-memory success
before durable storage succeeds, while commands discard save failures.

**Outcome:** Guild and moderation state follows the persistence contract, and
administrative success is reported only after durable success.

**Boundary:** Other Mob Aliveness stores are not implicitly included. Any newly
discovered store defect becomes a separately tracked finding.

**Finding:** 7

**Delivered 2026-08-10.** Three defects, not one:

1. **Non-durable writes.** `guilds/persistence.go`, `moderation/bans.go` and
   `moderation/petitions.go` all used bare `os.WriteFile`. Now `util.Save`.
2. **No rollback, so memory diverged from disk.** Every mutator published to
   the in-memory registry and saved afterwards, returning the save error but
   keeping the change. `BanAccount`, `Unban`, `BanIP`, `UnbanIP`,
   `AddMember`, `RemoveMember` and `SetRank` now restore the exact prior state
   when the save fails. `Create()` had always rolled back this way — the
   pattern existed, it just was not applied to the others.
   `AddMember` restores the `byUser` index as well as the member list: leaving
   a stale index behind is worse than the original failure, because the player
   is then treated as already in a guild and cannot join any.
3. **Commands reported success regardless.** `ban` and `unban` discarded the
   error with `_ =` and printed success. Four sites; all now report the failure
   and say plainly that the ban is NOT in effect (or the player is STILL
   banned). The account-ban path also no longer kicks the player when the ban
   did not persist.

11 tests, driven by pointing the data dir at a path whose parent is a regular
file — which makes `MkdirAll` fail on Windows too, unlike chmod tricks.

**Obstacle worth recording:** these error paths were not merely untested, they
were untestable. `mudlog.Error` dereferences a nil logger and panics when the
package has no `TestMain`, so any test that drove a save failure crashed the
binary instead of failing an assertion. `internal/moderation/test_main_test.go`
now initialises it, matching `internal/bounties`.

### Chunk 2.4 — Separate corrupt shop and room state from absence

**Problem:** Shop corruption is treated as a new shop, while corrupt room
overlays can be partially applied.

**Outcome:** Loaders distinguish absence from corruption. Corrupt living state
is preserved or quarantined, never silently reseeded or partially merged.

**Delivered 2026-08-10.**

**Finding 6 (shops).** `loadFromDisk` returned nil for both "no file" and
"unparseable", and `RegisterShop` treats nil as "seed from template at the
abundance level". One malformed byte therefore reset a merchant's stock,
merchant gold and restock timers to opening-day defaults, and the reset was
indistinguishable from normal initialisation. It now quarantines and logs at
ERROR before reseeding.

*Documented deviation:* the review suggested refusing to reseed on corruption.
We still reseed, because a merchant with no inventory is a broken shop, and the
operator policy chosen 2026-08-10 is quarantine + defaults + loud log. Nothing
is destroyed — the original bytes survive in the quarantine file, and
quarantining is also what frees the path so the reseeded shop can save at all.

**Finding 15 (rooms).** `LoadRoomInstance` unmarshalled the overlay directly
ONTO the loaded template. yaml.Unmarshal applies fields as it walks, so a file
that is valid for a while and then breaks left the room already mutated when the
error returned — and the old code logged a Warn and carried on with exactly that
object. The overlay is now applied to a scratch template copy (safe because
`LoadRoomTemplate` re-reads from disk, so it shares no maps or slices with the
returned room) and adopted only if the whole document parsed. Otherwise it is
quarantined and pure template state stands.

8 tests, each with control legs proving valid overlays and absent files still
behave correctly.

**Noted, not changed:** shop and room saves route through
`util.Save(..., CarefulSaveFiles)`, so durability is operator-switchable for
living state. It ships `true` and is a documented, long-standing escape hatch
with 8 consumers, so it was left alone — but the fsync added in 2.1 made the
careful path slower, which gives an operator chasing autosave pauses a reason to
switch off exactly the guarantee this wave is building. Worth a decision in
3.6a.

**Findings:** 6, 15

### Chunk 2.5 — Make authored-content validation fail honestly

**Problem:** Dialogue parse failures are cached as absence, and quest-flag
validation skips files it cannot read or parse.

**Outcome:** Validation reports every file it could not inspect. Runtime caches
do not convert transient parse errors into process-lifetime absence.

**Delivered 2026-08-10** (the two findings; see the boundary note below for
what is explicitly NOT claimed).

**Finding 14 (dialogue).** `Load` stored the nil sentinel for three different
situations: file absent, file unreadable, file unparseable. Only the first is
permanent. Caching the other two meant an author who fixed a YAML typo had a
mute NPC until the whole server restarted — and the builder workflow is exactly
where those typos happen.

The sentinel now records **confirmed absence only**, keeping the optimisation it
exists for (most mobs never talk and should not stat the disk every
interaction). Read and parse failures return nil without caching, so the next
interaction retries. The error log is throttled per mob/zone by message text, so
a broken file does not repeat on every `ask`, while a *changed* error still logs
and a successful load clears the throttle.

**Finding 16 (questengine).** `loadAllDialogueFiles` `continue`d past every read
and parse error, so `ValidateAllFlags` could report success without having
inspected files that may contain undeclared flag keys. Undeclared flags are a
startup panic by design, so a validator that quietly narrows its own input made
that guarantee hollow while still producing confidence. It now returns failures
alongside entries and each becomes a validation error. A missing dialogue
directory is still fine — absence, not corruption.

9 tests. `internal/dialogue` also gained a `TestMain`: like `internal/moderation`
in 2.3, its failure paths were untestable because `mudlog.Error` panics on a nil
logger, and one existing test worked around it by calling `SetupLogger` inside a
helper.

**Still open in this chunk's stated outcome:** wiring the whole-world dialogue
and quest semantic validators into CI and boot smoke, rather than only the
editor-save paths. The two findings are closed; that broader validator-coverage
goal is not, and should be re-scoped as its own chunk. CI and
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

**Delivered 2026-08-10.** The review named two ignored save errors. There were
**four** silent paths to `Ok: true`:

1. `SaveRoomTemplate` error ignored.
2. `SaveZoneConfig` error ignored.
3. `LoadRoomTemplate` returning nil silently skipped the entire entrance-room
   setup — no plane, no title, no description — and still returned success.
4. `GetZoneConfig` returning nil silently skipped the config entirely, leaving
   `RoomId` unset so `GetZoneRoot` returns 0 and the zone renders empty.

Plus an ordering bug: `GetPlaneRegistry().Mark` published the new plane to
memory *before* either save, so a zone that failed to persist still changed
runtime placement rules until restart.

`buildZoneCreate` now applies both contract rules — persist before publishing,
and roll back on failure. `rooms.DeleteZone` is the rollback, which is safe here
because a freshly created zone has no blockers (`ZoneDeletionBlockers` skips the
zone root). A rollback that itself fails is reported in the error message rather
than swallowed, since the operator then has a partial zone on disk to clean up
by hand.

The function was also given the injectable-deps treatment already used by
`buildDeps` in the same file (`zoneCreateDeps` / `realZoneCreateDeps` /
`buildZoneCreateWith`), because none of these failure paths were reachable in a
test otherwise. 7 tests, including one asserting the plane is NOT published when
persistence fails.

### Chunk 2.8 — Adopt the persistence contract everywhere it was missed

**Problem:** Wave 2 established the living-state persistence contract (2.1) and
migrated six write sites across chunks 2.2–2.4. A later sweep found **~20 more**
still writing directly, including the most valuable file in the game.

The miss was a scoping error: the Wave 2 audit grepped `internal/mobs`,
`internal/guilds`, `internal/moderation`, `internal/shops` and `internal/rooms`
only. `internal/users` was never checked.

**Outcome:** Every living-state and authored-content writer routes through
`util.Save`, so atomicity AND durability are uniform. No subsystem hand-rolls
its own careful-save.

**Triage by value:**

*Tier A — irreplaceable player data. Highest value; fix first.*

| Site | Current behaviour |
|---|---|
| `internal/users/users.go:729` | hand-rolled `.new`+rename, **no fsync** |
| `internal/characters/alts.go:69` | direct write, mode **0777** |
| `internal/pets/pets.go:148` | direct write |

*Tier B — accumulated world and economy state, unreproducible.*

| Site | Current behaviour |
|---|---|
| `internal/plugins/plugins.go:364` | **bare write, no atomicity at all** — auction history, leaderboards, weather |
| `internal/warehouse/persistence.go:71` | direct write |
| `internal/caravan/throughput.go:132` | direct write |
| `internal/forager/throughput.go:132` | direct write |
| `internal/sealedcrate/persistence.go:33` | direct write |
| `internal/crimes/persistence.go:95` | direct write |
| `internal/factions/persistence.go:81` | direct write |
| `internal/opinions/persistence.go:98` | direct write |
| `internal/economy/health/persistence.go:53` | direct write |
| `internal/bounties/persistence.go:44` | tmp+rename, no fsync |
| `internal/facts/persistence.go:53,91` | tmp+rename, no fsync |
| `internal/goals/persistence.go:81` | tmp+rename, no fsync |
| `internal/knowledge/persistence.go:51` | tmp+rename, no fsync |

*Tier C — authored content. Recoverable from git, but a torn file PANICS the
next boot, so atomicity still matters.*

| Site | Current behaviour |
|---|---|
| `internal/fileloader/fileloader.go:273,349` | hand-rolled careful save, mode 0777 |
| `internal/behaviortree/save.go:67` | direct write |
| `internal/dialogue/save.go:39` | direct write |
| `internal/quests/save.go:50` | direct write |
| `internal/mutators/mutators.go:307` | direct write |
| `internal/species/species.go:185` | direct write |
| `internal/rooms/zone_rename.go:118` | direct write |

*Tier D — deliberately unchanged.*

- `internal/migration/*` — one-time, at boot, before any player connects; a
  failure is immediately visible and the migration can be re-run.
- `internal/devtools/gridgen.go` — developer tool, not production data.
- `internal/playtestenv/*`, `internal/playtestrun/*` — ephemeral test infra.
- **`internal/playtestprofiles/materialize.go:122` MUST STAY a plain
  `os.WriteFile`.** Chunk 2.3's fix depends on it truncating the pre-created
  file IN PLACE so the inode keeps its host owner. Converting it to an atomic
  rename replaces the inode and silently reintroduces the Linux CI failure.
  This is documented at both ends; do not "fix" it.

**Measured stake (3.6b prep):** a 48KB user file costs 0.696 ms written the
current way and 3.873 ms durably. Users are cheap today *because they are not
durable*.

**Sequencing note:** fixing Tier A roughly doubles the autosave pause at 100
players (70 ms → 387 ms for users). The user accepted this on 2026-08-10 on the
basis that master is not production — the droplet is a manual pull — so the
durability fix can land first and be deployed together with 3.6b-1, which
absorbs the cost by amortising the commit.

**Findings:** New (Wave 2 scope miss).

**Status: DONE (2026-08-10).** All of Tiers A, B and C migrated to `util.Save`.

Two things worth carrying forward:

1. **A 16th site existed that the grep sweep did not find.**
   `internal/users/index_rebuild.go` hand-rolls a temp-and-rename, but it
   streams fixed-size binary records rather than holding one `[]byte`, so it
   never matched the `os.WriteFile` grep the triage table was built from. It
   turned out to be the *best* of the hand-rolled copies — it already did
   `f.Sync()` before close and removed the temp on failure — but it was missing
   the directory sync that makes the rename itself durable on Linux. Without
   that, a power loss can lose the rename and leave the old index in place with
   the `.tmp` surviving beside it. `util.syncDir` was exported as
   `util.SyncDir` and called there.

   The lesson matches this chunk's own premise: this is the *second* time a
   grep-shaped audit of write sites came back short. That is why the guard
   below exists rather than a third audit.

2. **The contract is now enforced by test, not by audit.**
   `durable_write_guard_test.go` (repo root) walks the AST of every non-test
   `.go` file and fails on `os.WriteFile` outside an explicitly-reasoned
   exemption list, and separately fails on any file that builds a `.tmp`/`.new`
   sibling and renames it. Exemptions are per-directory, with a per-file map for
   the streaming case above. Adding a write now requires either using
   `util.Save` or writing down why the data is not living state.

   `SaveRoundCount` is the one deliberate opt-out and is documented at the call
   site: it runs every 10 seconds **with the world lock held**, and an fsync
   measured at ~3.5 ms in 3.6a would be a recurring lock-held cost for a
   ten-byte payload that cannot realistically tear.

Behavioural coverage for the migration landed in
`internal/fileloader/fileloader_save_test.go` (authored-content save path: no
temp litter, complete overwrite, uncareful mode still works, concurrent
fan-out). The `util.SafeSave` semantics themselves were already covered by
chunk 2.1's `internal/util/livingstate_test.go` and are not duplicated.

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

**Delivered 2026-08-10.** The failure was end to end, so the fix had to be:

- `SaveAllRooms` counted failures into a log line then `return nil`
  unconditionally. Now returns an aggregate naming the count and the first
  error.
- `SaveAllUsers` returned nothing at all. Now returns an aggregate. A user save
  carries inventory, gold, progression and quest state.
- `SaveRoomInstance` discarded the `os.Remove` error when clearing an empty
  overlay. That is not harmless: the stale file is re-applied on the next room
  load, resurrecting state the room no longer has.
- `plugins.Save` returned nothing, and the `onSave` callback was `func()`, so a
  plugin *physically could not* report a failed save. The signature is now
  `func() error` and all four in-repo modules (auctions, gmcp/mudlet,
  leaderboards, weather) propagate. They were each discarding a real error:
  `WriteStruct`/`WriteBytes` have always returned one.
- The autosave hook broadcast `Done.` to every connected player after each
  stage regardless of outcome. It now reports `Saved with errors.` and logs at
  ERROR when a stage fails. Shutdown (`main.go`, `world.go`) and copyover
  (`copyover.go`) honour the same outcomes — those are the last chance to
  persist before the process exits, so a silent failure there is permanent.

**Not done here, by boundary:** no background goroutines, no autosave
performance work. The dirty-check question raised in 2.1 stays with 3.6a.

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

**Delivered 2026-08-10.** Three changes, in descending order of severity:

1. **Authentication ran under the world lock.** `RunWithMUDLocked` was the OUTER
   wrapper on all 22 admin routes, so `doBasicAuth` — including **bcrypt**,
   which is expensive by design — executed while holding the lock that stops
   every player and every round. Anyone who could reach an admin URL could
   freeze the game for a bcrypt round *per request, without credentials*. The
   nesting is now inverted on every route, so only authenticated requests reach
   the lock at all. Safe because `users.LoadUser` builds a standalone record
   from disk and never touches the live registry, and those reads are torn-free
   now that user saves are atomic (chunk 2.1).

2. **The response write held the lock.** Handlers wrote straight to the network,
   so a slow or stalled admin client held the world lock for as long as it took
   to accept the bytes. `RunWithMUDLocked` now buffers the handler's output and
   flushes it after unlocking. None of these routes stream and the payloads are
   small, so buffering costs nothing. Note this was partly masked by chunk 4.2's
   `WriteTimeout`, which bounded the freeze at 60s rather than removing it.

3. **Five routes needed no lock at all.** Verified per handler:
   `/admin/static/` (a file server — freezing the game to deliver a stylesheet),
   plus the pure-template pages `adminIndex`, `combatStatsIndex`,
   `progressionIndex` and `economyIndex`, none of which touch world state.

**Route audit.** All 22 were classified by what they actually read. Two nearly
got unlocked by mistake: `economyAPI` and `economySnapshotAPI` look
state-free until you notice `health.CaptureSnapshot` reads live shop, caravan
and forager state. Final state: **16 locked (all with auth outside), 5
unlocked**, zero routes taking the lock before authenticating.

**Not done, deliberately:** per-handler narrowing so the lock covers only the
data-gathering step rather than the whole handler body. The buffering change
removes the unbounded-client risk, which was the availability problem; further
narrowing is a per-route refactor of 16 handlers with much smaller returns.

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

**Disposition 2026-08-10: REMEDIATION REQUIRED. 3.6b is activated.**

Instrumentation added to the autosave hook (per-stage durations, loaded-set
sizes, and `turnsDelayed`), plus benchmarks in `internal/rooms/`
(`autosave_bench_test.go`, `autosave_phase_bench_test.go`).

**Live measurement** — full world, autosave forced to every 8 rounds, ~15
consistent cycles, **0 players connected**, local SSD:

    totalMs=295-300  turnsDelayed=5-6  loadedRooms=1386  activeUsers=0
    usersMs=0        roomsMs=293-299   pluginsMs=0-1

**Whole-set benchmarks** — cost is linear in loaded-set size:

| Scenario | Total | Per room |
|---|---:|---:|
| 1000 clean | 264 ms | 0.26 ms |
| 1000 dirty | 3,564 ms | 3.56 ms |
| 100 clean | 17.6 ms | 0.18 ms |
| 100 dirty | 360 ms | 3.60 ms |

**Per-phase split of one dirty room** (this is the number 3.6b is designed
against):

| Phase | Per room | Share | Can leave the lock? |
|---|---:|---:|---|
| `LoadRoomTemplate` (disk read) | 0.110 ms | 3.0% | Yes — immutable authored data, cacheable |
| Reflection diff | ~0.055 ms | 1.5% | No — reads live state |
| `yaml.Marshal` | 0.007 ms | 0.2% | No, but effectively free |
| `util.Save` (temp + fsync + rename) | **3.459 ms** | **95.3%** | **Yes — input is already immutable bytes** |
| whole dirty save (control) | 3.631 ms | 100% | |

**Why remediation is required rather than a budget.** The pause is not request
latency, it is a full-simulation stop: every player, mid-round, at once. It is
linear in loaded rooms, the world is 49 zones and growing, and it therefore gets
worse with every content addition. A budget would be a number we watch degrade.

**Two corrections to earlier assumptions, recorded so they are not repeated:**

1. "Autosave writes every loaded room" is FALSE. `SaveRoomInstance` computes the
   diff and writes nothing when a room matches its template, so an effective
   dirty check already exists at the write level.
2. "Dirty tracking is the fix" is FALSE, and it was the obvious-looking answer.
   Dirty tracking would only avoid the 1.5% diff. The cost is the fsync.

**Design target for 3.6b, from the evidence:** marshal under the lock (0.007 ms,
free) and move `util.Save` outside it. Bytes are immutable by construction, so
this sidesteps the unsafe-shallow-snapshot hazard the 3.6b boundary warns about
— no deep copy of `Room` is needed at all. Projected worst case (all 1386 rooms
dirty) falls from ~5.0 s to ~0.24 s, and the lock cost becomes independent of
how many rooms are dirty. Template caching would take it to ~0.085 s.

**3.6b must still solve failure reporting** (finding 35 must not regress): never
claim success at dispatch, collect terminal write outcomes and report them at
ERROR within one cycle, and prevent overlapping cycles from queueing stale or
reordered writes.

**Gaps in this evidence:** measured with 0 players, so `SaveAllUsers` is
untested under load; measured on a local Windows SSD, so the droplet's volume is
likely slower. Both matter for 3.6b's acceptance criteria, not for the decision
to do it.

#### Chunk 3.6b — Remediate autosave pauses if required

**Activation:** Execute only if Chunk 3.6a shows the approved budget is
exceeded. Otherwise cancel this chunk with the measurement evidence.

**Outcome:** A separately brainstormed and approved design reduces lock-held
work to the budget without stale overwrites, reordering, lost failures, or
unsafe shallow snapshots.

**Boundary:** Dirty tracking, template caching, precomputed projections, and
immutable snapshots plus background writes are options to evaluate, not
predetermined fixes. Decompose this XL chunk before implementation.

**Decomposed 2026-08-10** into 3.6b-1 (amortised commit), 3.6b-2 (dirty-set
write-behind) and 3.6b-3 (transactional value transfers, conditional). Design:
`docs/superpowers/specs/2026-08-10-autosave-async-writes-design.md`. Plan:
`docs/superpowers/plans/2026-08-10-autosave-amortised-commit-plan.md`.

##### 3.6b-1 — Amortised commit: **DONE 2026-08-10**

Prepare (diff + marshal, everything that reads live state) runs in ONE atomic
pass under the world lock and produces immutable bytes. Commit (the durable
write, 95% of the cost) is spread across later ticks at a bounded rate via the
new `internal/savequeue`.

Measured, 1000 rooms:

| | Fully clean | Fully dirty |
|---|---:|---:|
| Before (`SaveAllRooms`) | 157 ms | **5980 ms** |
| After (prepare only) | 115 ms | **120 ms** |

Dirtiness dependence falls from 38x to 1.05x, and a fully dirty world now costs
less under the lock than a fully clean one did before.

Users, measured headlessly at ~0.145 ms/user (linear across 10/100/1000): 100
users prepare in **15.7 ms**, against **387 ms** previously held in one block.

**Scope changed mid-design, and the reason is worth keeping.** Users were
originally out of 3.6b-1 on the strength of 3.6a's `usersMs=0` — which was
measured with ZERO players connected. Chunk 2.8 then made user saves durable
(0.696 ms → 3.873 ms), making users the LARGER half of the pause at 100
players. Deferring them would have shipped the sub-chunk that bounds the pause
while the bigger half of it grew.

**Two corrections to the design target stated above.** (a) "Marshal under the
lock, move `util.Save` outside" understated what has to stay: the template READ
and the reflection diff also read live state, and the template read is uncached,
making it ~64% of prepare. Template caching is therefore the named prerequisite
for the projected figure holding on the droplet, not a further optimisation.
(b) The projection of ~0.24 s was for rooms alone; the delivered figure is
better than that for rooms but the full pause also carries users.

**Finding 35 did not regress, and this was the main risk.** With commits spread
over ticks, nothing has been written when the prepare tick returns, so the
completion broadcast moved to the drain that empties the queue. A test asserts
partial drains stay silent and exactly one outcome is reported at completion.

**Guards delivered:** G1 one atomic pass (regression test), G2 cancellation
across seven save sites **plus three deletion paths that bypass the save
function entirely** (`ClearRoomCache`, `DeleteRoomTemplate`, `DeleteZone` —
found in adversarial review of the plan, not in the original design), G3
supersede-with-WARN, G4 flush on exit with **different policies per caller**
(copyover ABORTS, shutdown logs and proceeds), G5 report-once-at-completion.

**Known and accepted:** amortised is not free. Three writes per 50 ms turn is a
sustained ~20% MainWorker occupancy while a cycle drains (~23 s at 1386 rooms,
~2.8 min at 10,000). Cost still scales with world size rather than activity.
Removing that is 3.6b-2's job and is the reason it should not be treated as
optional.

**Still owed before the 100-player target is claimed met:** a whole-server load
run with real connected players. The per-file costs are benchmarked and the
arithmetic is direct, but a load run would catch anything else that scales with
player count.

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

**Wave 4 complete 2026-08-10** (findings 18, 19, 20b).

- **20b** the https redirect built its destination from `r.Host`, so anyone
  could hand out a link to this server and bounce visitors elsewhere under our
  domain name, cached as a 301. Fixed with NO new knob by reusing
  `allowedWSOriginHosts` -- the operator has already declared which hostnames
  this server answers to, and that is the same question.
- **18** the client redirected focus to the command bar on every keydown, so Tab
  could not traverse and Space/Enter could not activate a focused control. Only
  printable characters pull focus now, and never from a control the user tabbed
  to. Verified the movement shortcuts were already scoped to the input.
- **19** status/quests/inventory rebuilt on every GMCP push (conditions ship
  with vitals, so the status panel was recreated every round). Renders that
  would change nothing are now skipped. Separately, the map's `deferEdges`
  batching had a hole: `_updateBounds`/`_applyZoom` ran once per room, making a
  Zone.Map push O(n^2).

**Still open on this surface:** the map's per-room token rebuild. Skipping it
needs a signature covering the quest-marker pin, which depends on the centre
room's position -- cross-room state a per-room signature does not capture.

### Chunk 4.6 — Cache room templates for the autosave compare

**Deferred out of 3.6b-1 as "a further optimisation", then promoted on
evidence.** Measured 2026-08-10: `PrepareInstanceWrite` costs 0.1215ms/room, of
which `LoadRoomTemplate` alone is 0.0983ms — **81%**, not the ~64% first
estimated. The template is immutable authored content and was re-read and
re-parsed from disk for every room, every cycle.

**This was taken INSTEAD of 3.6b-2**, on a risk comparison rather than a
preference:

| | 3.6b-2 dirty set | 4.6 template cache |
|---|---|---|
| Correctness surface | **~80 mutation sites** across 10+ packages (50 field writes + 33 in-place slice/map element writes) | **4 template writers**, all in `internal/rooms` |
| Cost of missing one | **Silent non-persistence — permanent data loss** | A stale title until restart; recoverable |

**Results.** Per room 0.1215ms → 0.0115ms. 1000 dirty rooms 120ms → 8.7ms;
1000 clean 115ms → 5.1ms. Live boot, 1386 rooms:

    cycle 1 (cache cold)   prepareMs=207   turnsDelayed=4
    cycle 2 (warm)         prepareMs=7     turnsDelayed=0

**turnsDelayed=0 is the headline**: after the first cycle the autosave pause is
not merely smaller, it is below one turn, so no player can perceive it. Combined
with 100 users at 15.7ms, the whole lock-held pause is well inside a 50ms turn.

**Why the cache is a separate accessor and NOT inside `LoadRoomTemplate`.**
`LoadRoomInstance` calls `LoadRoomTemplate` TWICE and requires two INDEPENDENT
objects: one becomes the live room, the other is a scratch copy the instance
overlay is unmarshalled onto. Sharing a pointer between them would let a
partially applied overlay write into the live room — review finding 15 arriving
by a new route. `templateForCompare` is used by exactly one caller, the diff,
which only ever reads. A test pins that `LoadRoomTemplate` still hands out
independent objects.

**Known and accepted:** the first autosave after boot still pays ~207ms while
the cache is cold, and the cache holds ~1386 parsed templates (single-digit MB).
Pre-warming at boot would trade startup time for one 4-turn hitch per server
lifetime, which is not obviously worth it.

**Consequence for 3.6b-2:** with the sweep at 7ms, a dirty set buys almost
nothing at our scale and still carries the ~80-site correctness surface. It
stays open for forks with far larger worlds, but it is no longer the obvious
next step.

**Finding:** 36 (follow-on)

### Chunk 4.7 — Amortise plugin saves

**A regression introduced by chunk 2.8.** 3.6a measured `pluginsMs=0-1`; 2.8
routed plugin writes through `util.Save` and its fsync, and four plugins at ~5ms
each made plugin saves **20-22ms per cycle** against 3-5ms for the entire
room-and-user prepare. Roughly 6.5KB of data: the cost was fsync count, not
volume. The durability was correct and stayed; only its scheduling was wrong.

**Design:** `plugins.PrepareAll()` activates a registry-wide collector, runs each
plugin's existing `onSave`, and captures the marshalled bytes as
`savequeue.PendingWrite`s that join the SAME atomic set as rooms and users.
`WriteStruct`/`WriteBytes` keep their exact signatures, because that is upstream
GoMud's plugin API and changing it would break third-party modules.

**Measured, live boot with autosave forced to fire every 60s:**

| | Before | After |
|---|---:|---:|
| `prepareMs` (warm) | 7 | 6-8 |
| `pluginsMs` (separate lock-held stage) | 22 | **gone** |
| Total lock-held | ~29 ms | **~7 ms** |

`pluginWrites=4`, `pendingWrites=1390` (1386 rooms + 4 plugins), `failed=0`,
`turnsDelayed=0`, no `.new`/`.tmp` litter, plugin files confirmed rewritten each
cycle.

**Guard G2 was missing from the original design and was added by adversarial
review of the spec.** A plugin can write outside an autosave on its own cadence
(`weather.persistState` does), so a queued entry could commit *after* a newer
synchronous write and roll the state backwards. A synchronous `WriteBytes` now
cancels any pending write for that path. Proven by mutation: removing the cancel
makes the regression test fail.

**Also folded in:** plugins now prepare in the same lock hold as rooms and users,
which closes a real tear window — a bid deducts player gold AND writes auction
history, two files in one logical transaction.

**Guard rail against the new ceiling.** The cost per future plugin moves from
"5ms of fsync" to "whatever its `onSave` computes", which would be invisible in
an aggregate timing. A plugin whose prepare exceeds 5ms is now logged at WARN by
name, and `internal/plugins/context.md` documents the contract as a bound:
`onSave` may gather, but only work proportional to the plugin's own state, never
to the size of the world.

**Found while implementing 4.7, FIXED 2026-08-11 in chunk 4.8** (see below).

**Finding:** 36 (follow-on)

### Chunk 4.8 — Plugin reads distinguish absent from corrupt

**Found while writing 4.7's `context.md`.** `Plugin.ReadIntoStruct` was:

```go
if err = yaml.Unmarshal(b, out); err == nil {
    return err          // nil on success
}
return nil              // ALSO nil on failure
```

It could never report a parse failure. Worse, it got the two cases exactly
backwards: a merely-ABSENT file returned an error (from `ReadBytes`), while a
CORRUPT one returned nil. It reported the harmless case and hid the dangerous
one, and a corrupt data file loaded silently as zero values -- or as a
half-applied hybrid, since `yaml.Unmarshal` populates fields as it walks and
keeps whatever it managed before the fault. That is review finding 15's
template/runtime hybrid, in a different package.

Both callers ignored the return value, which is why it went unnoticed.

**Fixed to the chunk 2.1 living-state contract:** ABSENT returns
`util.ErrStateAbsent` (seed defaults), CORRUPT returns `util.ErrStateCorrupt`
after quarantining the file and resetting `out` to its zero value, so the caller
gets clean defaults rather than a hybrid and the next read sees ABSENT.
`modules/auctions` and `modules/leaderboards` now report corruption at ERROR
instead of discarding the result.

Also extracted `Plugin.dataPath`, since `ReadBytes`, `WriteBytes` and the new
quarantine each derived the same path independently.

**Verified by mutation:** restoring the original implementation makes 4 of the 5
new tests fail. Boot test confirms real plugin data still loads without tripping
the corruption path.

**Finding:** New (found during 4.7).

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

### Chunk 5.8 — DECIDED 2026-08-11: preserve variance ownership, close the floor gap

**Analysis.** `OpposedRollStat(atk, def)` rolls BOTH participants with
`StdDevFor(atk)`, so variance is owned by the initiator. Analytically, with
A ~ N(atk, s) and B ~ N(def, s), P(win) = Phi((atk-def)/(s*sqrt(2))).

Raw, that produces a real role asymmetry — the same pair, different initiator:

| Pair | A wins attacking | A wins defending | Ratio |
|---|---:|---:|---:|
| A=100 B=120 | 17.3% | 21.6% | 1.2x |
| A=100 B=150 | 0.9% | 5.8% | 6.3x |

The underdog is better off being attacked than attacking, because a strong
initiator's high stat gives BOTH rolls large absolute variance and creates
upsets, while a weak initiator's small sigma makes the contest deterministic.

**Two corrections to the finding as written.**

1. It implies variance ownership causes scale-invariance. It does not. All three
   candidate models (initiator-owned, participant-owned, neutral-shared) are
   ratio-only, because sigma is proportional to the stat in every one. 100v150,
   200v300 and 400v600 are identical under all three. That is a property of
   `RollSpread` being a multiplier and belongs to a different question.

2. **It analyses the dice function without the combat layer that bounds it.**
   `MinAttackHitChance` and `MinDefenseChance` (both shipped 0.15,
   `combat_helpers.go`) already floor BOTH ends. Effective hit chance is
   `P(atk wins)*0.85 + P(def wins)*0.15`, which compresses the asymmetry to
   ~1.2x at EVERY gap:

   | Gap | Raw asymmetry | With floors |
   |---:|---:|---:|
   | 1.3 | 1.8x | 1.20x |
   | 1.5 | 6.3x | 1.22x |
   | 2.0 | 7586x | 1.04x |

   At the 1.5 gap, A attacking goes 0.9% -> 15.6% and A avoiding 5.8% -> 19.1%.
   The floors also cap the top: a 200-vs-100 attacker lands 84.4%, not 99.1%.

**Decision: preserve initiator-owned variance.** The floors already deliver what
changing the ownership model would buy, and retuning would touch 34 call sites
across six subsystems for ~2 percentage points where fights are actually
contested (ratio 1.0-1.2).

**But the analysis exposed a real gap, and closing it is chunk 5.9.** The floors
live in `combat_helpers.go`, so they cover combat only. `sneak`, `shadow`,
`steal`, `plant` and `defuse` call `OpposedRollStat` directly with no floor at
either end. Combat says an outmatched actor always keeps a puncher's chance;
stealth and theft say the opposite. That difference was never decided, it was
just never noticed. **User confirmed 2026-08-11 that this was an unintended
miss.**

### Chunk 5.9 — Extend contest floors to non-combat opposed rolls

**Problem:** `MinAttackHitChance` / `MinDefenseChance` bound combat at both ends;
the non-combat consumers of `OpposedRollStat` are unbounded. A stat-100 thief
against a stat-150 mark succeeds 0.9% of the time; a stat-200 thief against a
stat-100 mark succeeds 99.1%. Both ends degenerate.

**Safe to floor:** every affected action has a real failure cost, so a floor
cannot become "guaranteed with patience" — `steal` gets the actor caught and
wakes a sleeping victim, `defuse` SPRINGS THE TRAP on the actor, `sneak` reveals
them. This was checked before selecting the approach.

**Outcome:** one shared, floored contest helper used by the non-combat consumers,
bounded at BOTH ends (an outmatched actor keeps a chance; an overwhelming one is
not certain), with the value(s) in config per the balance-lives-in-config rule.

**Open judgment calls for the implementation spec:**

1. **Both ends, or only the bottom?** Recommended BOTH. Fixing "the weak can
   never succeed" while leaving "the strong always succeeds" repairs one
   degenerate end and not the other, and combat already bounds both.
2. **Reuse 0.15, or a separate knob?** Recommended SEPARATE. A combat floor
   applies per swing and a fight has many swings, so 15% per swing is a small
   cumulative nudge. A steal or defuse is ONE attempt with a large cost, so the
   same 15% is far more generous in effect. Same number, very different meaning.

**Boundary:** decomposed from 5.8 per that chunk's own rule that a retune becomes
a separate arc.

**Split into 5.9a and 5.9b once the real scope was measured.** The floors were
believed to be missing from ~12 stealth/theft sites. They are missing from
**~32**: `resolveAttack` in `combat_helpers.go` is the ONLY place either floor is
applied, and only `avoidance.go` feeds it. Combat MANEUVERS (flee, grapple,
submission, skill moves, taunt) and SPELLS are unfloored too.

**5.9a DONE 2026-08-11** — 17 sites: `steal` (4), `plant` (4), `go` (4, sneak on
move and spotting hidden creatures), `sneak` (2), `shadow` (1+1), `defuse` (1).
New `dice.OpposedRollStatFloored` bounds both ends;
`Balance.MinContestSuccessChance` / `MinContestResistChance` default **0.05**,
set at startup via `dice.SetContestFloors`, mirroring `SetRollSpread`.

0.05 rather than combat's 0.15 on purpose: a combat floor fires on EVERY swing of
a many-swing fight, while these fire ONCE per attempt on actions that already
punish failure (a caught thief, a trap sprung on the fumbler, a revealed sneak).
The same number would be far more generous here.

A floor save returns margin ±1, because a last-resort save is a bare success and
callers that scale an effect by margin must not read it as a rout.

**Verified by mutation:** forcing the floors to zero makes 3 of the 6 new tests
fail. Boot test confirms both knobs load.

**5.9b (not started)** — the remaining 15 sites: `spell_resolution` (4),
`flee` (2), `grapple`, `submission`, `skill_moves`, `combat_taunt`, `throw`,
`charm_spell`, and two mob-tick sites. Held back deliberately: these change
FIGHT math, not just out-of-combat contests, and landing a probability shift
across spells and maneuvers at the same time as the stealth change would make a
regression impossible to attribute. Wants its own playtest.

⚠️ Note for 5.9b: melee hit/avoid IS floored but spells are NOT, so a spell that
can never land against a strong target sits next to a melee swing that always
can. That inconsistency is arguably worse than the one 5.9a fixed.

**Finding:** 5.8 (follow-on).

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
| 5 | 2.2 | **Done** | `util.Save` on both the instance AND template writers; load quarantines corrupt files instead of silently reseeding + 5 tests | Non-atomic and corruption-prone mob instance persistence |
| 6 | 2.4 | **Done** | `loadFromDisk` distinguishes absent from corrupt and quarantines; reseeding no longer destroys the damaged economy + 4 tests | Corrupt shop treated as new |
| 7 | 2.3 | **Done** | `util.Save` on guilds/bans/petitions; rollback on save failure in 7 mutators; 4 admin commands no longer report success on failure + 11 tests | Guild/moderation memory-disk divergence |
| 8 | 3.3 | **Done** | `8dd24e4c8`; RWMutex + 7 accessors, 8 sites (review named 1) + 6 concurrency tests | Unsynchronized room path cache |
| 9 | 1.2 | **Done** | 164 skip-only placeholders deleted (review named 87 in one file; pattern spanned 18) + 4 emptied files removed + go/ast recurrence guard | Phantom position tests |
| 10 | 1.1 | **Done** | Reusable validate.yml consumed by PR, master and tags; only-new-issues verified to work on push | Release CI omits lint/coverage |
| 11 | 5.3 | **Done** | `e23df1071`; `pickWanderExit` + 5 tests | Wander filter ignored |
| 12 | 5.4 | **Done** | `e23df1071`; `parseGoldPhrase` shared by both paths + 2 tests | Gold-give parse mismatch |
| 13 | 2.6 | **Done** | `buildZoneCreate` persists-then-publishes and rolls the zone back on any failure; 4 silent paths closed, not the 2 the review named + 7 tests | Builder ignores save failures |
| 14 | 2.5 | **Done** | nil sentinel now records CONFIRMED absence only; read/parse failures are retryable in-process, with a throttled log + 5 tests | Dialogue parse error cached as absence |
| 15 | 2.4 | **Done** | Overlay applied to a scratch template copy and adopted only on a full parse; corrupt overlays quarantined + 4 tests | Partial room overlay after YAML failure |
| 16 | 2.5 | **Done** | `loadAllDialogueFiles` returns failures alongside entries; unreadable/unparseable files now fail validation instead of being skipped + 4 tests | Quest validation skips unreadable dialogue |
| 17 | 4.1 | **Done** | `admin/static/js/safe-dom.js` default-safe helpers; economy AND progression converted (review named economy only; progression renders player-chosen names) + 12 JS checks | Admin economy stored-XSS surface |
| 18 | 4.3 | **Done** | Focus stealer scoped to printable chars + non-controls; menu-icon div -> button; mut-toast/overlay given roles, keyboard activation and Escape; map buttons named; room cursor no longer advertises a no-op handler. 39 jstest checks | Global keyboard capture |
| 19 | 4.4 | Open | — | Hot GMCP DOM rebuilds |
| 20 | 4.2 | **Done** | 20a: 4 Network timeout knobs, non-zero defaults, applied to both servers; websocket safety proven live (survived 15s vs a 3s WriteTimeout) and slow-header request cut at 3.0s + 6 tests. 20b (Host redirect) remains | HTTP timeout and Host redirect weaknesses |
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
| 34 | 3.5 | **Done** | auth moved outside the lock on all 16 remaining routes (bcrypt no longer runs under it); response buffered and flushed after unlock; 5 no-world-state routes unlocked entirely + 4 tests | Admin routes retain global world lock |
| 35 | 2.7 | **Done** | `SaveAllRooms`/`SaveAllUsers`/`plugins.Save` return aggregates; `onSave` callback signature now returns error (4 modules); autosave, shutdown and copyover all honour them + 2 tests | Autosave failures reported as success |
| 36 | 3.6a–3.6b | **3.6a Done / 3.6b Open** | Measured: 295ms idle at 1386 rooms; dirty room = 3.63ms of which **95% is fsync**; pathological ~5s. Remediation required; design target recorded | Unmeasured autosave world-lock pauses |
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
