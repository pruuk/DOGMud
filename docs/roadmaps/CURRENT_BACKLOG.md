# DOGMud Current Backlog

Last reviewed: 2026-08-29, after U10b-3 merged.

This is the compact cross-roadmap memory for planning. It is an index, not a
second requirements document. Follow the linked canonical roadmap/spec/plan for
scope, dependencies, decisions, and verification. Merged code and explicit
tracker status outrank filenames and unchecked boxes in old plans.

## Current Program: Unified Resolution

Source: [Unified Resolution Roadmap](UNIFIED_RESOLUTION_ROADMAP.md)

**Everything through U10d is MERGED.** U8 integrated as `15a5fc94d` (PR #51) on
2026-08-18, the same day this file last claimed it was pending. U9 (2026-08-19),
U10 (2026-08-21), U10c (2026-08-24), U10d (2026-08-25), and all five U10b
sub-slices (U10b-0 PRs #55-#60, U10b-1 #70, U10b-1b #74, U10b-2 #75, U10b-3 #76)
have followed. Per-stage merge evidence lives in the roadmap's Plans table.

**Two stages remain, in this order:**

1. **U12 - targeting audit.** Re-read and simplify target resolution and target
   switching after the resolution flip. Behavioural changes must split out.
2. **U11 - arc closer.** Documentation, `context.md` sweep, config organisation,
   help registry/category cleanup, and the final adversarial playtest. No code
   slice lands after this closer.

🚫 **Nothing is deployed.** Prod is still `7c64c228c`. Merging to master is not a
deploy trigger; the gate is the whole arc plus a playtest.

**Owed to that playtest, not to a slice:** mob-archer progression rate, regen
tuning, and the progression re-solve (salvage's 0.60 is the least trustworthy
input). All three are measurement questions and are on
`docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`.

**One disclosed gap owned by no stage:** buff applier attribution — `buffs.Buff`
has no applier actor, so DoT and toxicity deaths name no killer. Verified
2026-08-29 to have no live consequence; latent until a bounty guard gets a
poison attack. Recorded on the U5c row.

## Adversarial Review Remediation

Source: [Adversarial Review Remediation Roadmap](ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md)

### External or partially complete

- **1.5:** repository credential cleanup is complete; production credential
  rotation remains an external owner action. Never expose the credentials.
- **3.6b:** autosave remediation slice 1 shipped; slices 2 and 3 remain open.

### Correctness and behavior

- **1.4:** decide and enforce the YAML compatibility boundary.
- **5.1:** make combat entry transactional.
- **5.6:** converge composition-heavy commands on `internal/parser`.
- **5.11f-2:** design dedicated reflect resistance and its cap.
- **5.11h:** close the skill/crit arc documentation and adversarial playtest.
- **5.12:** correct phantom APIs in package `context.md` files.
- **5.13:** Tunnel Shaman constant movement is parked; suspects are narrowed,
  but root cause is not established.

### Performance and web follow-ons

- **4.5:** skip unchanged map room-token rebuilds.

### Debt, in dependency order

- **6.1a-d:** consolidate action, position, mob-command, and test-fixture
  duplication after their prerequisite correctness chunks.
- **6.3:** retire stale cadence config/documentation after the YAML boundary.
- **6.6a-c:** define boot dependency seams, freeze production registration, and
  isolate test callback overrides.
- **6.5:** clear Go and JavaScript lint backlogs last, after earlier deletions.

### Admin builder

- **7.1:** adversarially review builder pages after the known persistence/lock
  fixes; hunt beyond already-closed findings.
- **7.2:** add authored-content cross-reference indexing/search after 7.1
  establishes trustworthy payloads.

## Independent Approved Work

- **Architecture diagrams tab:** approved design, implementation not started.
  Sources: [design](../superpowers/specs/2026-08-04-archify-diagrams-tab-design.md)
  and [plan](../superpowers/plans/2026-08-04-archify-diagrams-tab.md).

## Deferred Follow-ons

- **The lint gate silently inverts on any PR over 20,000 diff lines.**
  `only-new-issues` asks the GitHub API for the PR patch and feeds it to
  `--new-from-patch`. The API refuses any diff above 20,000 lines with
  `code: too_large`, and when that call fails `golangci-lint-action` cannot
  tell new findings from old, so it reports the **entire** grandfathered
  backlog and the gate fails on code the branch never touched. U8
  (2026-08-18, PR #51) was the first change large enough to hit this; its
  lint failure was entirely this, verified by `--new-from-rev=master`
  reporting 0 and by CI's own count falling to master's exact 96 baseline.
  `fetch-depth: 0` on the checkout was tried and does **not** help: the
  action has no local-diff fallback on `pull_request` events (reverted in
  `dab4006ff`, so the workflow is unchanged). A real fix means passing
  `--new-from-merge-base` explicitly, which needs care because
  `validate.yml` is shared by the PR and push-to-master callers and that
  flag would compare master against itself on a push. Until then:
  **when the lint gate fails, check `golangci-lint run --new-from-rev=master`
  locally before believing it**, and keep large changes split where practical.
- **Defender name rendered from the wrong viewer's perspective (cosmetic):**
  `mobcommands/taunt.go` and `hooks/spell_resolution.go` build a defender's
  display name with `GetPlayerName(defender.UserId)` rather than the id of the
  player who will read the line, so the `(aggro)` suffix is computed from the
  defender's own perspective instead of the reader's. Wrong decoration only; the
  name itself is correct. Found in the U8 review, 2026-08-18. Belongs with the
  combat/action messaging unification below rather than as a one-off patch.
- **Audit and remove dead mutation active command skills:** inventory command
  registration, mutation definitions, implementations, tests, help, and config
  knobs after the mutation removals. Delete only entries proven unreachable or
  obsolete. This remains explicitly outside U8.
- **NPC maintenance routines (Mob Aliveness 3.5):** the only deferred item in an
  otherwise complete 45-chunk roadmap. Source:
  [Mob Aliveness Roadmap](MOB_ALIVENESS_ROADMAP.md).
- **Unify fragmented combat/action messaging:** combat-state Perception shipped dormant
  and has no consumer. A future framework would own visibility gating,
  anonymized infrared rendering, look blocking, event-category colors, wrapping,
  companion-name leakage, and the scattered ownership of melee, spell,
  rhetoric, mob-command and data-driven defence narration. U8 only brings quell
  and defy onto the existing defence-message data shape; it does not perform
  this unification. The historical draft is
  [messaging framework design](../superpowers/specs/completed/2026-05-19-messaging-framework-design.md);
  it requires fresh brainstorming and repository verification before planning.
- **NPC conversation gossip/opinion use:** generic NPC conversations shipped;
  “spoken about you” gossip and conversation-driven opinion changes remain
  intentionally deferred from that slice. Re-scope against current NPC systems
  rather than reviving the old design verbatim.

## Needs Triage, Not Yet Actionable

- `DEVELOPMENT_PLAN.md` says Phase 42 and its playtest pass are complete and
  points next to “Future Expansion planning.” Its unscheduled economy, crafting,
  faction, quest, world, and PvP bullets overlap heavily with systems shipped
  later. Reconcile each against current code and newer roadmaps before promoting
  it to active work.
- Top-level files under `docs/superpowers/plans/` and `specs/` are not backlog by
  location alone. Several remain outside `completed/` after shipping, and some
  status headers are stale. Promote one here only after checking its canonical
  tracker and merged commits.
- Old remote branches are not evidence of pending work. Verify intent, ancestry,
  and current implementation before resurrecting one.

## Completed Arcs That Should Not Re-enter the Backlog

- Combat state machines chunks 0-6 are complete; only the separately deferred
  messaging consumer remains.
- Mob Aliveness reports 45/45 complete plus deferred maintenance routines 3.5.
- Unified Resolution U0-U7b shipped through the reservation-ceiling merge.
- Phase 42 and its first multiplayer playtest bug-fix pass are recorded complete.

## Maintenance Rules

When a task changes status:

1. Update its canonical roadmap or plan status first.
2. Update this index in the same change.
3. Record the date and merge/PR evidence for completed work.
4. Move ambiguous or contradicted work to “Needs triage”; do not guess.
5. Keep implementation details in the canonical source, not here.
