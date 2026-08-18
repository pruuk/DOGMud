# DOGMud Current Backlog

Last reviewed: 2026-08-18 against the Task 14 validated U8 feature branch.

This is the compact cross-roadmap memory for planning. It is an index, not a
second requirements document. Follow the linked canonical roadmap/spec/plan for
scope, dependencies, decisions, and verification. Merged code and explicit
tracker status outrank filenames and unchecked boxes in old plans.

## Current Program: Unified Resolution

Source: [Unified Resolution Roadmap](UNIFIED_RESOLUTION_ROADMAP.md)

U7b (reservation ceiling) shipped in PR #49. U8 has now passed its final
verification, isolated boot, and adversarial gameplay gate. The sequence is:

1. **U8 - implementation and validation complete; integration pending.** The
   feature branch's unified action-cost surface, admission policy, data-backed
   quell/defy narration, and behavior-specific documentation passed Task 14's
   full verification, isolated boot, and adversarial playtest gate. It has not
   yet been integrated into `master`. Source:
   [U8 design](../superpowers/specs/2026-08-17-u8-unified-action-cost-admission-design.md).
2. **U9 - progression events.** Replace progression side effects with explicit
   events for both participants and decide how spell `primarystat` becomes the
   authoritative resolution stat.
3. **U10 - disruption model.** Make concentration a contest and make knockdown
   and prone recovery opposed rolls.
4. **U12 - targeting audit.** Re-read and simplify target resolution and target
   switching after the resolution flip. Behavioral changes must split out.
5. **U11 - arc closer.** Documentation, `context.md` sweep, config organization,
   help registry/category cleanup, and the final adversarial playtest. It runs
   after U8-U10 and U12; no code slice lands after this closer.

U9, U10, and the U12 audit may be planned independently where their file sets
and decisions do not overlap. Recheck the canonical dependency table first.

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
