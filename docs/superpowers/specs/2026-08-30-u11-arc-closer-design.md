# U11 — the arc closer

**Date:** 2026-08-30
**Status:** approved, ready for planning
**Roadmap row:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, U11
**Depends on:** U6b, U8–U10d, U12 (all merged)

U11 is the last slice of the unified resolution arc. It runs after U12 by
design: it is the arc's closing gate, so no code slice may land after it.

---

## Facts verified against source

Every row below was read from the tree at `66930eba1` on 2026-08-30, not
recalled from the roadmap. Four roadmap claims did not survive the check and are
marked STALE.

| Claim | Verified value | Where |
|---|---|---|
| `dice.OpposedRoll*` has production callers outside `internal/dice` | **No.** Only references are inside `internal/dice` itself (`contest_floors.go`, `dice.go`) | `grep -rn OpposedRoll internal/ modules/ --include=*.go \| grep -v _test.go` |
| `TrySpellDeflection` / `TryStoicResolve` still exist | **No.** Comment-only references in `combat/defence_multiplier.go:157,295` and `configs/config.balance.go:319` | same grep |
| `quell.template` / `defy.template` missing — **STALE** | Both exist | `_datafiles/world/dogmud/templates/help/` |
| quell/defy unregistered in `keywords.yaml` — **STALE** | Registered: lines 81, 91, 244, 245 | `_datafiles/world/dogmud/keywords.yaml` |
| quell/defy have no cross-links — **STALE** | Referenced from `combat`, `conviction`, `defense`, `taunt` templates | grep over `templates/help/` |
| quell/defy absent from `u8ActionHelpPaths` — **STALE** | Both present | `internal/templates/u8_help_test.go:30-31` |
| `u8ActionHelpPaths` entry count | **28** | `internal/templates/u8_help_test.go:19-52` |
| help template file count | **454** | `ls _datafiles/world/dogmud/templates/help/ \| wc -l` |
| `combatphase.RegisterMachine` production callers | **Zero.** Every call site is `_test.go`; only non-test mention is a comment at `actions/combat_fire.go:411` | `grep -rn RegisterMachine internal/ modules/ *.go` |
| Refs built from `c.userId` alone | **Exactly one line**: `internal/characters/engagement_storage.go:167` | `grep -rn 'ActorRef{UserId: c\.userId}'` |
| `Character.MobInstanceId` set before `Validate()` on spawn | **Yes**, `internal/mobs/mobs.go:358` precedes the spawn-path `Validate()` | `internal/mobs/mobs.go:340-380` |
| Mob teardown entry point | `mobs.DestroyInstance(instanceId)` | `internal/mobs/mobs.go:798` |
| Player teardown entry point | `users.LogOutUserByConnectionId` | `internal/users/users.go:388` |
| `SetUserId` call sites | `users/users.go:308,461,551`, `users/userrecord.go:835`, `playtestprofiles/persist.go:23` | grep |
| `recoveryContest` call sites | **Two, both round-tick**: `hooks/NewRound_UserRoundTick.go:246`, `hooks/NewRound_MobRoundTick.go:197` | grep |
| Stale cooldown comment (Go) | `// Shared cooldown rounds for bash/trip/kick (default 5)` | `internal/configs/config.balance.go:246` |
| Stale cooldown comment (yaml) | "Bash, trip, and kick all share a cooldown … Spellcasting also shares this cooldown slot" | `_datafiles/config.yaml:660-664` |
| Files referencing `special-move` | **46** non-test `.go` under `internal/` + `modules/` | `grep -rl special-move … \| grep -v _test.go \| wc -l` |
| `auctions` overlay files `auction` under | `shop` (singular) | `modules/auctions/files/data-overlays/keywords.yaml:3` |
| `cleanup` overlay files `bury`/`trash` under | `information` | `modules/cleanup/files/data-overlays/keywords.yaml:3-5` |
| `context.md` phantom symbols | **56 across 20 packages** of 129 checked; of the six arc packages only `internal/hooks` (2) and `internal/usercommands` (1) are dirty | `python tools/context_md_audit.py` |
| Surviving contest guard tests | `internal/combat/contest_site_guard_test.go` only — `contest_floor_guard_test.go` and `floor_pair_guard_test.go` no longer exist | `ls internal/combat/*guard*_test.go` |

**Consequence of the four STALE rows:** U8 already discharged the entire
"player helpfiles for quell and defy" section of the U11 roadmap row (items 1
through 4). What remains of the help work is the *category* cleanup and the
`u8ActionHelpPaths` inversion, not the helpfiles themselves.

---

## 1. Scope

Three phases, in order, on one branch, shipped as one PR. Only Phase A changes
behaviour.

| Phase | Contents | Behaviour change? |
|---|---|---|
| A | Attacker-registry wiring, help-allowlist inversion, cooldown comment pair, Done-when guards | **Yes, A1 only** |
| B | `config.yaml` organisation audit, `context.md` sweep, help category cleanup, patch notes, roadmap close | No |
| C | Adversarial playtest gate, then owner handoff | No |

Sequencing is A → B → C → owner manual playtest. The owner's pass covers
helpfiles and confirms no config edit broke anything, which is why B precedes C
rather than being deferred to a second PR.

---

## 2. Phase A1 — wire the attacker registry

### 2.1 What is broken

`combatphase.RegisterMachine` has no production callers, so `machineRegistry` is
permanently empty, `lookupMachine` always returns nil, and the single
`RecordInboundAttacker` call site (`combatphase.go:359`, behind
`if target := lookupMachine(d.Target); target != nil`) never runs.
`Character.Attackers()` therefore always returns an empty slice while its
docstring promises it *"replaces room-scan loops for 'who's attacking me?'"*.

A second, independent break sits behind the first:
`internal/characters/engagement_storage.go:167` passes
`Actor: state.ActorRef{UserId: c.userId}`. Nothing calls `SetUserId` on a mob, so
a mob's ref is the zero value and `RecordInboundAttacker` early-returns on
`ActorRef.IsZero()`. **Repairing only the registry would still never record a
mob attacker.**

### 2.2 What it costs today

The one live consumer is `recoveryContest`
(`internal/hooks/recovery_contest.go`), which iterates `ch.Attackers()`, finds
nothing, and returns `nil`. That function documents `nil` as *a free stand*.

`recoveryContest` is passed only to `Character.AttemptRecovery`
(`internal/characters/skills.go:53`), from the two round ticks. It is the
**automatic** stand-up-on-your-own path, not the manual `stand` command — the
docstring is explicit that *"the manual stand command is uncontested by design"*.
Manual `stand` is out of scope and unchanged.

Two things are consequently dead, not one:

1. **Auto-recovery has never been contested for anyone**, whatever U10 intended.
   Once `MinRecoveryRounds` are consumed, every prone or supine actor stands for
   free.
2. **Auto-recovery has never awarded progression.** `AwardResolved`
   (`skills.go:88`) sits *inside* the `if contestWin != nil` guard, with a
   U10b-1 comment noting that failing to stand is exactly what should teach you.
   With `contestWin` effectively never non-nil in a contested sense, unarmed
   combat has earned nothing from scrambles.

Also dead, and in scope for the same fix:
`CombatPhase_CompanionAssist.go`'s reactive `SubscribeAttackersChange` path
(companion assist runs on a polling fallback), `NotifyTargetDied` and
`NotifySelfDied`.

### 2.3 Design

**Identity.** Add `func (c *Character) ActorRef() state.ActorRef` returning
`{UserId: c.userId, MobInstanceId: c.MobInstanceId}`. Replace the hand-built ref
at `engagement_storage.go:167`. This is the only production site building a ref
from `userId` alone, so the accessor is the fix rather than a convenience.

**Registration.** Register from the existing once-per-Character seam in
`Validate()` (`internal/characters/validate.go:625-631`, guarded by
`combatPhaseWired`), which already exists precisely to run per-Character setup
exactly once and is already re-fired for mob instances after
`ResetForMobInstance`.

**Ordering risk, and it gates the task.** On the mob path the ordering is
verified safe: `mobs.go:358` sets `MobInstanceId` before the spawn-path
`Validate()`. On the player path `SetUserId` is called from `users.go:308`,
`users.go:461`, `users.go:551` and `userrecord.go:835`, and it is **not yet
verified** that any of those precede the `Validate()` that fires
`fireCharacterCreated`. If they do not, players would register under a zero ref
— the exact bug being fixed, in the other direction.

> **Task one of implementation is to determine this empirically**, by
> instrumenting or reading the login path end to end. If `SetUserId` lands after
> `Validate()`, registration moves to `SetUserId` for players (re-registering
> under the correct ref, and unregistering any zero-ref binding) rather than
> living in `Validate()` for both actor types.

**Teardown.** `machineRegistry` is a process-global `map[state.ActorRef]*Machine`
guarded by `registryMu`. Nothing in the finding mentions teardown, but without
it every mob instance ever spawned is retained for the life of the server, and a
long-running droplet accumulates them indefinitely. Unregister at:

- `mobs.DestroyInstance` (`internal/mobs/mobs.go:798`)
- `users.LogOutUserByConnectionId` (`internal/users/users.go:388`)

All five machines expose `RegisterMachine` / `UnregisterMachine`
(`activity.go:154`, `awareness.go:124`, `combatphase.go:274`, `life.go:121`,
`position.go:277`). Register and unregister all five together; a partial wiring
is how the next reader concludes the whole mechanism is dead again.

**Retarget hygiene is already written.** `TransitionToEngaging`
(`combatphase.go:344-356`) already moves the inbound entry off the previous
target, with a comment saying it is inert today *"but without it a retarget
would leak an entry on the old target the day that registry is wired up"*. That
day is now; the code needs no change, but the retarget case needs a test.

### 2.4 Consequences to state plainly

- **Knockdown gets stickier** when a living, same-room attacker is holding you
  down. `RunContest` applies `ContestFloor` (0.125), so no one is pinned
  forever, and the paid manual `stand` remains an unconditional exit.
- **Unarmed combat starts earning** from auto-recovery attempts, win or lose.
- The owner's prior finding that there is **no prone/stand death spiral** was
  measured against the *uncontested* auto-stand and does not carry over
  unexamined. Re-checking it is an explicit goal of the Phase C playtest.
- Adjacent open bug: the companion prone lock. The companion-assist
  `SubscribeAttackersChange` path going live may interact with it. Watch for it;
  do not attempt to fix it in this slice.

---

## 3. Phase A2 — invert `u8ActionHelpPaths`

`internal/templates/u8_help_test.go` lists 28 paths against 454 help templates.

> **Correction to the roadmap's inbox #4.** It claims *"FOUR tests iterate only
> that list, not three"* and then names three. Read at `66930eba1`, the truth is
> **three tests across four iteration sites**: `u8ActionHelpPaths` is ranged at
> lines 171, 214, 223 and 242, and 214 and 223 both sit inside
> `TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning` (182-231). The three
> tests are `TestU8ActionAdmissionHelpTemplatesProcess` (168),
> `TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning` (182) and
> `TestU8ActionHelpCrossReferencesResolve` (239). The count does not change the
> finding or the fix; it is corrected here so the record is right.

So **426 templates get no parse check, no numeric-disclosure check and no
cross-reference check at all.** Both files U10d edited that fell outside the
list were exactly where its surviving copy defects lived. Structurally this is
the same failure as `stow` going invisible in the 2026-08-03 helpfile audit.

**Fix:** walk every `help/*.template` and keep a shrinking, commented exception
list instead of an allowlist. The action-admission *expectations map* stays
keyed to the specific commands that document a stamina policy; only the
iteration set inverts. All four iteration sites move together — converting
three and leaving one on the allowlist reproduces the bug in miniature.

Expect the inversion to fail on first run. Every failure is either a real defect
to fix or an exception to add with a written reason. **An exception added
without a reason is the allowlist growing back.**

---

## 4. Phase A3 — the cooldown comment pair

Two comments disagree with each other and with the code:

- `internal/configs/config.balance.go:246` — "Shared cooldown rounds for
  bash/trip/kick (default 5)"
- `_datafiles/config.yaml:660-664` — bash/trip/kick plus spellcasting

Neither is right: **46** non-test `.go` files under `internal/` and `modules/`
reference the `special-move` key, and U10d added the melee and ranged ambush
openers to that population. The `config.yaml` comment is the worse of the two
because it is what a tuner reads.

Fix both **together** — fixing one leaves the pair still disagreeing.
`_datafiles/config.yaml` carries `skip-worktree`, so the commit must be built
from the `git show HEAD:` blob rather than from disk, per the standing SOP.

---

## 5. Phase A4 — the Done-when list as tests

The U11 row requires the "Done when" list be shipped as tests, because U6 was
declared done with two criteria false and, being prose, nothing failed.

| # | Criterion | Disposition |
|---|---|---|
| 1 | `dice.OpposedRoll*` called from the core and nowhere else in `actions`/`combat`/`hooks`/`usercommands` | **Met in fact, unguarded.** Both former floor-guard files are gone. Ship a new source-walk guard that fails any such caller. |
| 2 | Defence skill weight ×5 everywhere; `SpellAttackSkillFactor` gone | Already pinned by the U6b Task 18 guards. No new test. |
| 3 | `TrySpellDeflection` / `TryStoicResolve` no longer exist as parallel mechanisms | **Met in fact, unguarded.** Fold a symbol check into the same new guard file. |
| 4 | Adding a contest requires declaring scores, a defence set and a channel — no new resolution code | Substantially held by `TestEveryContestSiteIsOwned`. Make the criterion explicit in that test's docstring and owner map header rather than inventing a second, weaker test. |
| 5 | Parity damage-per-swing within ±10% of today at light/mid/BIS armour | **Not applicable — closed with a written reason.** See §5.1. |
| 6 | Documentation current | Phase B. |
| 7 | Adversarial playtest gate | Phase C. |

### 5.1 Why criterion 5 is closed as not-applicable

The criterion asks for parity against "today", meaning the pre-arc model. U6 was
"THE FLIP" and deliberately retuned damage, defence and mitigation; U6b deleted
the legacy per-channel parameters outright. There is no preserved pre-arc
baseline to compare against, and the one candidate source
(`_datafiles/logs/combat-analytics.jsonl`) spans the arc's own retunes, so any
baseline reconstructed from it would be contaminated by the very changes it was
meant to measure.

A criterion that cannot be evaluated must not be silently dropped, and must not
be discharged by a test that measures something else while wearing its name.
**It is recorded in the roadmap as superseded by U6, with this reasoning, and
closed.** The owner decided this on 2026-08-30.

---

## 6. Phase B — documentation

### 6.1 `config.yaml` organisation audit

A documentation and ergonomics pass over `_datafiles/config.yaml` as a file, not
over the knobs this arc touched. **No value changes.** Any retune found along
the way is filed, not applied — a config edit inside a docs chunk is how an
unreviewed balance change ships.

- **Grouping** — related knobs adjacent, with the shared principle stated once
  above the block.
- **Ordering** — sections follow the shape of the systems they configure, not
  the order chunks happened to land.
- **Comments** — what it does, what changing it costs, and the rationale for a
  non-obvious live value. A comment describing the removed pre-arc model is
  worse than no comment.
- **Stale and orphaned keys** — knobs whose readers this arc deleted. Sweep;
  do not trust a list.
- **Drift flags** — any knob whose shipped value differs sharply from its Go
  default without an explaining comment (`SpellDamageScale` ships 3.12 against a
  default of 1.0).
- **The inert-validator sweep**, carried over from inbox #3: a knob with a
  non-zero advertised default, a `if x < 0 || x > 1.0 { x = default }`-shaped
  validator, and **no key in `config.yaml`** is silently inert at zero, because
  the zero an absent key unmarshals to passes the validator untouched. That is
  how the five `SurpriseAttack*Penalty` knobs came to auto-hit every limb. The
  earlier sweep covered only that one validator shape; widen it.

Two traps to respect. `MinContestSuccessChance`, `MinSpellHitChance` and
`MinManeuverHitChance` **all ship at 0.05 by coincidence, not by rule** — do not
"simplify" them into one knob during a tidy-up. And **a Go test binary never
loads this file**, so config-read knobs measure their struct zero value under
test, not the shipped value and not necessarily the documented default.

**Explicitly not in scope:** `SubGoldLossFraction`. Inbox #3 is a correction, not
a defect. `sub_gold_loss_fraction: 0.20` **is present** at `_datafiles/config.yaml:925`
and the subdue/cripple transfer runs at the intended 0.20. Do not "fix" it.

### 6.2 `context.md` sweep

Done-when 6 requires zero phantom symbols in the six arc-touched packages:
`internal/combat`, `internal/actions`, `internal/hooks`, `internal/characters`,
`internal/dice`, `internal/usercommands`. Current state: `combat`, `actions`,
`characters` and `dice` are clean; `internal/hooks` has 2 phantoms
(`handleSomething`, `SystemMaintenance`) and `internal/usercommands` has 1
(`Foo`). Several of these read as prose examples rather than API claims, so
triage before deleting.

That is the gate. The other 53 phantoms across 18 packages are **reported, not
fixed** — expanding a docs gate into a repo-wide rewrite is how a closing slice
stops closing. File them.

Also update, as part of A1: `internal/state/combatphase/context.md` and
`internal/state/awareness/context.md`, which currently document the registry as
inert. When it stops being inert those files become actively misleading.

### 6.3 Help category cleanup

`help` groups topics by category and the categories have drifted into
near-duplicates, caused by module overlays merging last-write-wins onto a flat
map. Two confirmed:

- `modules/auctions/files/data-overlays/keywords.yaml:3` files `auction` under
  **`shop`** (singular) while the main file uses **`shops`** (plural), so the
  rendered index shows two headings and the second holds one entry.
- `modules/cleanup/files/data-overlays/keywords.yaml:3-5` re-files `bury` and
  `trash` under **`information`**, overriding the main file's `character` and
  `items` placements and leaving those main-file entries dead.

Sweep the full category set rather than fixing only these two.

### 6.4 Patch notes and roadmap close

- `docs/PATCH_NOTES.md` gets a dated, player-facing entry. The only
  player-visible change is contested auto-recovery: describe the *feel* (someone
  standing over you makes it harder to scramble up; the `stand` command still
  works) with **no raw numbers and no em dashes**.
- The U11 roadmap row closes with merge evidence, the four STALE claims
  corrected, inbox items #1/#4/#5 marked resolved, and Done-when #5 recorded as
  not-applicable per §5.1.

---

## 7. Phase C — the adversarial playtest gate

Per the standing content SOP, run the harness with an explicitly critical
mandate against an ephemeral local checkout:

```
/playtest local --checkout <abs> bug-finder <goals>.yaml
```

Two focus areas:

1. **Knockdown and prone**, the one behaviour change. Get knocked down next to
   something that survives more than one round, and confirm: auto-stand
   sometimes fails; the failure is narrated sensibly; the manual `stand` still
   always works; nothing gets pinned indefinitely; and the prior no-death-spiral
   finding still holds under contest.
2. **Helpfiles**, since A2 will have brought 426 templates under test for the
   first time and B3 will have moved categories around. Walk the `help` index
   and spot-check the moved topics.

> **Fixture warning, learned twice already.** U10d left four checks UNVERIFIED
> and U12c-2 hid a flee regression for a day, both because **every practice
> target in the game died in one round**. Contested recovery is invisible
> against a target that cannot hold you down. Use the Straw Effigy in the Drill
> Yard (`227e96088`), or a mob that survives several rounds. A one-round dummy
> will produce a clean report that proves nothing.

Findings get fixed and the run repeated if needed. Only then does it go to the
owner, who runs the manual pass with Meirok per
`docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`.

---

## 8. Testing strategy

| Area | Test |
|---|---|
| `Character.ActorRef()` | Unit: player ref, mob ref, and that a mob's ref is non-zero — the exact condition that made `RecordInboundAttacker` early-return |
| Registration | Machines resolvable via `lookupMachine` after `Validate()` for both a player and a spawned mob instance |
| Teardown | Registry entry gone after `DestroyInstance` and after logout; no residual binding under a zero ref |
| Retarget | Inbound entry moves off the previous target and does not leak, exercising the path `combatphase.go:344-356` was written blind for |
| `recoveryContest` | Returns non-nil with a living same-room attacker registered; nil when the attacker is dead, in another room, or absent. Picks the strongest holder |
| Auto-recovery | Contested outcome respects `ContestFloor`; `AwardResolved` fires on both win and loss |
| Done-when 1 & 3 | New source-walk guard over the four packages |
| Done-when 4 | `TestEveryContestSiteIsOwned` docstring carries the criterion |
| Help templates | Inverted walk over all 454, exceptions commented |
| Boot | Isolated detached worktree per SOP, `Server Ready`, exit 124 is success |

Registration and teardown are the risk surface: the registry is process-global
and reached from combat. A leak or a stale binding would not fail a unit test,
which is why the boot test and the Phase C gate both matter.

---

## 9. Out of scope

- The manual `stand` command. Uncontested by design; untouched.
- The other 53 `context.md` phantoms outside the six arc packages. Reported.
- `SubGoldLossFraction`. Inbox #3 is a correction; the knob is live and correct.
- Restoring `EngagingData.Reason`. Inbox #6 closed by U12c-2, and the struct
  comment warns explicitly against reinstating it.
- The companion prone lock. Adjacent, may be perturbed by A1, filed separately.
- Any balance retune surfaced by the config audit. Filed, not applied.
