# DOGMud Current Backlog

Last reviewed: 2026-08-31, after the Unified Resolution arc shipped to
production and closed.

This is the compact cross-roadmap memory for planning. It is an index, not a
second requirements document. Follow the linked canonical roadmap/spec/plan for
scope, dependencies, decisions, and verification. Merged code and explicit
tracker status outrank filenames and unchecked boxes in old plans.

## Closed Program: Unified Resolution

Source: [Unified Resolution Roadmap](UNIFIED_RESOLUTION_ROADMAP.md), now marked
**closed**.

✅ **The arc is COMPLETE and DEPLOYED.** Every stage U0–U12 merged, and the whole
arc went to production on **2026-08-30** (962 commits). Prod moved on again the
next day with PRs #98–#101 and was `a1af7269a` on 2026-08-31. The `7c64c228c`
pin this section used to quote is dead, as is its "nothing is deployed" claim.
Per-stage merge evidence stays in the roadmap's Plans table — answer "did stage
X ship?" from there, never from a tick or from memory.

**Three things outlived the arc and are now ordinary backlog:**

- **Five balance knobs silently inert at zero** — `StealCooldown`,
  `StealHiddenBonus`, `ShadowCooldown`, `SneakFailCooldown`, `PackScatterRounds`
  (three of them skullduggery economy). Filed rather than fixed by U11 because
  adding the keys is a live balance change, not a docs edit. Also filed there:
  154 help templates carrying em dashes, and the `gmcp` module's two extra help
  categories. Source:
  [`2026-08-30-u11-filed-findings`](../audits/2026-08-30-u11-filed-findings.md).
- **The Elemental Queen fight was never recorded as run.** It is the designated
  live-verification instrument for quell and for the arc's floor and ceiling at
  veteran power. No longer blocking anything; still worth doing.
- **Buff applier attribution** — `buffs.Buff` has no applier actor, so DoT and
  toxicity deaths name no killer. Verified 2026-08-29 to have no live
  consequence; latent until a bounty guard gets a poison attack. Recorded on the
  U5c row.

**Measurement work that was owed to the pre-deploy playtest** — mob-archer
progression rate, regen tuning, and the progression re-solve (salvage's 0.60 is
the least trustworthy input) — was never a deploy blocker in its own right and
carries over. It lives on `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`, whose
unticked boxes are now post-deploy feel-checks.

## Current Program: none

The next arc has not started. Four are queued; see "Queued Arcs" immediately
below.

## Queued Arcs

All are owner-approved in principle. None has a spec or a plan yet.

✅ **Order settled 2026-08-31: MESSAGING IS NEXT.** The owner resolved the
open spell-scaling-versus-messaging question in messaging's favour. Two arcs
now have work explicitly parked behind them: **behavior** carries chunk 5.13
and, with a quest-mechanism arc, the whole of Wave 7.

Working order: **messaging → behavior → quest mechanisms → config audit**,
with spell scaling unplaced against the tail of that list. Wave 7 (admin
builder) comes after behavior and quest mechanisms.

1. **Spell scaling unification.** Same shape as the resolution arc: many call
   sites hand-rolling one job. Magnitude, effect strength and duration are
   computed independently inside every `case` arm of
   `internal/hooks/spell_resolution.go` (1719 lines, 22 arms, player and mob
   paths duplicating each other). 🔴 **`effect_type: buff` gets no duration
   scaling at all** — the arm calls plain `target.AddBuff(buffId, "spell")`
   (`:1048`, mob twin `:1619`), so `base_folds` is ignored and duration comes
   only from the buff spec. That is the largest effect type, **17 of 56 spells**;
   Skill Attunement sits at a flat 200 rounds no matter who casts it. Divisors
   (dot `/3`, heal `/2`, shield full, buff none), minimum floors (3, 6, 10) and
   crit handling are all per-arm magic numbers, and `magnitude` means something
   different in each arm. `calcSpellDuration` (`:34`) is the nearest existing
   seam, with 7 non-test callers. Do **not** "fix" `SpellData.CasterStatValue`
   — that part is already consistent.
2. **Messaging unification.** Previously filed under "Deferred Follow-ons"; the
   owner promoted it to an arc on 2026-08-26. Two parts. The concrete defect:
   **quell and defy are unnarrated.** `items.DefenseType` defines only dodge,
   parry and block, `defense-messages/` holds only those three files, and
   `sendDefenseMessages` (`internal/combat/combat_helpers.go:1108`) switches on
   the same three, so quelling a spell or defying a taunt falls through to the
   generic verb **"counter"**. Not broken, just generic, and never observed in
   play because the U7 playtest never met a mental or social attacker. The
   larger part: message files sit in five places (`combat-messages/`,
   `defense-messages/`, `taunt-messages/`, `messaging/`, and a bare
   `casting-messages.yaml` at the tree root), combat-state Perception shipped
   dormant with no consumer, and ownership of melee, spell, rhetoric,
   mob-command and data-driven defence narration is scattered. Any consolidation
   must check each loader's `Filepath()` first: a path mismatch is a **startup
   panic**, not a soft failure.
3. **Behavior unification.** Mob behavior is expressed through at least six
   unrelated mechanisms with no single model: legacy AI profiles
   (`internal/combat/ai.go`), behavior trees, `idlecommands`, JS `scriptag`
   scripts, schedules and patrols. Owner's target is **two** systems, not one.
   Related known defect: 114 mobs' `aiprofile` values fall through silently to
   the default.
4. **Quest mechanisms.** ⚠️ **NAMED BY THE OWNER 2026-08-31, NOT YET SCOPED.**
   Raised as a reason to park Wave 7: the admin builder edits quests, so
   reviewing it before quest mechanisms are reshaped reviews a surface that is
   about to move. **Get the scope from the owner before planning anything** —
   nothing in the repo says what "quest mechanism arc" covers, and guessing at
   it would be inventing a roadmap. Likely inputs when it is scoped: the quest
   flags system, the `give.go` transfer-before-handler gotcha, the re-grant
   prevention SOP, and `questRequired` versus the expiry-prone `requires`.
5. **`config.yaml` audit.** ⚠️ **Not the same audit U11 already did.** U11 owned
   an *organisation* pass over the file — grouping, ordering, comments, stale
   keys, drift flagging — explicitly **no value changes**, and it removed six
   orphaned keys and filed five inert knobs. What remains is the *correctness*
   audit: the `if x < 0 || x > 1.0 { x = default }` validator shape, which can
   **never** fire because an absent YAML key unmarshals to `0`, so the knob
   stays at 0.0 forever while its comment advertises a default. That is what
   made all five `SurpriseAttack*Penalty` knobs inert. Sweep for the shape
   repo-wide, then fix the five knobs U11 filed. **Grep the YAML tag, not the Go
   field name** — a `SubGoldLossFraction` claim was retracted for exactly that
   mistake.

## Owner Playtest, 2026-08-30 — feel-checks resolved

Four of the five owed feel-checks are now closed by the owner playing, not by
reasoning. Recorded here so none of them is re-run.

- ✅ **Weather tempo — FINE.** The slowed cadence from weather polish sub-project
  A reads correctly in play. Closed.
- ✅ **ANSI recolors — GOOD.** The WCAG contrast pass on the eight low-legibility
  readable aliases is confirmed by eye. This one could never be closed by the
  harness, which strips color. Closed.
- ✅ **The Elemental Queen — "fine if tough".** The last mandatory pre-deploy
  item, run late. ⚠️ **But it does not close quell's verification.** The Queen
  was chosen as quell's live instrument, and quell **has no messages at all** —
  it falls through to the generic verb "counter" (see the messaging arc). So the
  fight can be survivable and well-tuned while quell stays invisible. Quell
  becomes observable only after the messaging arc gives it a voice.
- 🔴 **NEW: indoor weather prose is written for a house, and most indoor rooms
  are caves.** Owner: the indoor messages "are a bit nonsensical". Verified:
  exactly five biomes carry `indoor: true` — `cave`, `dungeon`, `fort`, `house`,
  `spiderweb` — and the room counts are **cave 123, fort 22, house 15, dungeon 1,
  spiderweb 0**. Every one of the 15 emote files (9 under
  `_datafiles/world/dogmud/weather/emotes/` plus 6 seasonal ones under
  `emotes/seasons/`) authors only an `indoor: default:` pool, and every line in
  those pools assumes a built structure: roof, walls, windowpanes, glass,
  shutters, rafters, eaves, floorboards, nails in wood, a hearth. In a cave,
  "Frost creeps across the inside of the glass in feathered white ferns" and
  "a gritty film settles over everything no matter how tight the shutters" are
  nonsense — and caves are **77% of all indoor rooms**.
  **No code change needed.** `Indoor` is already `map[string]IndoorPool` keyed
  by biome, and `bandedSectionLines`
  (`modules/weather/content/emotes.go:129`) resolves biome then falls back to
  `default`. The fix is authoring a `cave:` pool in each file; `dungeon` and
  `spiderweb` are one room and zero rooms, so they can ride the default.
  Content work, so it ends with the adversarial playtest gate.

The fifth, **newbie aggression**, is still open — see the note under Deferred
Follow-ons.

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
- **5.13: PARKED BEHIND THE BEHAVIOR ARC, and RESTATED — it was never really
  about the Tunnel Shaman.** Owner, 2026-08-31: *"it isn't just the tunnel
  shaman. A lot of mobs mob around faster than you can resolve the windup period
  before kill X becomes 'I'm actually in combat with X'."* So the symptom is not
  one mob with a broken `maxwander: 0`; it is that **mob movement outruns the
  player's engagement windup across the world**, which makes targeting a mob a
  race. Do not resume this as a single-mob hunt. It belongs to the behavior arc,
  which owns mob movement policy, and the narrowed single-mob suspects (goal
  planners issuing `pathto`, combat-chase displacement in `handleAggroAndAssist`)
  are now one input to that arc rather than the whole investigation.

### Performance and web follow-ons

- **4.5:** skip unchanged map room-token rebuilds.

### Debt, in dependency order

- **6.1a-d:** consolidate action, position, mob-command, and test-fixture
  duplication after their prerequisite correctness chunks.
- **6.3:** retire stale cadence config/documentation after the YAML boundary.
- **6.6a-c:** define boot dependency seams, freeze production registration, and
  isolate test callback overrides.
- **6.5:** clear Go and JavaScript lint backlogs last, after earlier deletions.

### Admin builder — PARKED (owner, 2026-08-31)

⚠️ **Do not start either of these until the behavior arc and a quest-mechanism
arc have landed.** Owner's reasoning: the builder edits mob behavior and quests,
so reviewing and indexing it before those systems are reshaped means reviewing a
surface that is about to change underneath. Wave 7 was already sequenced last;
it now sits behind two arcs as well.

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
- **Newbie aggression feel-check (the last unrun one).** Context the owner did
  not have: on 2026-07-10 the `LegacyHostile` export fix **restored
  attack-on-entry aggression world-wide after it had been dead for two months**,
  because a refactor lowercased a yaml-tagged field and silently severed it. 158
  mobs got their aggression back at once, plus 21 deliberate ambusher/lookout
  hostile flips. The mechanic was confirmed working in-game; what was never done
  is **playing a fresh character through the newbie zones to see whether they
  are now too punishing.** That is the whole check. It is a feel question, so it
  wants a naive-newbie playtest persona rather than a code read.
- **Q34 bandit pacing cliff (the other unrun one).** From the 2026-07-13 feel
  test: the **Bandit Scout in room 5232 turned into a roughly 90-round grapple
  slog**. Logged as a difficulty-tune item and deferred ever since. It is a
  pacing problem in the newbie zone, not a correctness bug, and it sits
  naturally with the newbie-aggression check above since both are "how does the
  early game feel now".
- **Unify fragmented combat/action messaging — PROMOTED 2026-08-26.** No longer
  deferred; it is queued arc 2 above. Detail kept here because it is the fuller
  statement of scope: combat-state Perception shipped dormant and has no
  consumer, and a framework would own visibility gating, anonymized infrared
  rendering, look blocking, event-category colors, wrapping, companion-name
  leakage, and the scattered ownership of melee, spell, rhetoric, mob-command
  and data-driven defence narration. U8 only brought quell and defy onto the
  existing defence-message data shape; it did not perform this unification. The
  historical draft is
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
- Unified Resolution is complete end to end: every stage U0-U12 merged, and the
  whole arc deployed to production 2026-08-30. Its roadmap is closed.
- Phase 42 and its first multiplayer playtest bug-fix pass are recorded complete.

## Maintenance Rules

When a task changes status:

1. Update its canonical roadmap or plan status first.
2. Update this index in the same change.
3. Record the date and merge/PR evidence for completed work.
4. Move ambiguous or contradicted work to “Needs triage”; do not guess.
5. Keep implementation details in the canonical source, not here.
