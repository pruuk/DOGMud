# Sight Gates Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the sight-gates leftovers from the re-run playtest (#446, #447, #448, #449, the #242 and #216 residue) and make Empathic Shroud a real hide (#444), in one PR on `fix/sight-gates-followups`.

**Architecture:** Every fix reuses a mechanism the codebase already has: `messaging.HideWeapons` (G1), `Room.VisualSnapshot` / `SendTextVisualToSnapshot` (G2), the lightnotice `attribute` counterfactual (G3), `messaging.HideNames` / `ParticipantSight` and `Room.SendTextUnsighted` with `messaging.SoundSpellSputtersOut` (G4, G5), the pipeline's `shouldWrap` and the existing `moveCategories` vars (G6), and for G7 the Awareness machine's `HiddenData`, `hideForStealthRecord`, the `reconcilePerception`-in-`Validate` pattern, and the caster-derived magnitude door in `hooks/light_spell.go`.

**Tech Stack:** Go 1.x, testify, GoMud engine packages under `internal/`, YAML content under `_datafiles/`.

**Spec:** `docs/superpowers/specs/2026-10-09-sight-gates-followups-design.md` (APPROVED 2026-10-09).

**Worktree:** `C:/tmp/dogmud-sight-followups`, branch `fix/sight-gates-followups`. Branch head when this plan was written: `844dc7ff8` (master `7463f7be6` plus the spec commit; no Go code differs from master).

---

## How this plan was verified

Every task below was dry-run on `844dc7ff8` before this plan was written: the code in each step was applied, each new test was run failing against the old code and passing against the new, and the whole tree then passed `go test ./... -count=1`, `go vet ./...` and `golangci-lint run --new-from-rev=HEAD ./...` (0 new issues). The worktree was then reset. Line numbers below are from `844dc7ff8`; where a step's edit shifts a line that a guard test keys by line number, the step says so and gives the new number.

## Facts verified against source (`844dc7ff8`)

Built by grepping and reading for this plan, not copied from the spec.

| Fact | Where (verified) |
|---|---|
| `hideIdentitiesInPersonalLines(result *AttackResult, sourceChar, targetChar *characters.Character, ctx combatContext)` runs `HideWeapons` on both sides with a name keep-list from `heldWeaponNames` | `internal/combat/combat.go:746-761`; `heldWeaponNames` at `:766-777` |
| One production caller of each: `hideIdentitiesInPersonalLines` at `combat.go:693`; `heldWeaponNames` only inside `hideIdentitiesInPersonalLines` | grep over `internal/` |
| Every weapon token in an `AttackResult` is the attacker's: `{itemname}` = `ws.weaponName` (`combat.go:271`) or `sourceChar.Equipment.Weapon.DisplayName()` (`combat.go:315`); `{weapon}`/`{attack}` = `sourceChar.Equipment.Weapon.GetSpec().Name` (`combat_helpers.go:1361`) | read |
| The shipped defence pools use only `{actor}`, `{actee}`, `{attack}`, `{weapon}`, `{stance}`, `{position}`, `{momentum}`, `{bodypart}` and type tokens; `counter-*.yaml` carry no `fg="item"` tag | `_datafiles/world/dogmud/defense-messages/*.yaml` (grep) |
| `messaging.HideWeapons(text string, d SightDecision, keep []string) string`; other production caller passes `nil` | `internal/messaging/hideweapons.go:39`; `pipeline.go:72` |
| `Room.VisualSnapshot() VisualSnapshot`, `Room.SendTextVisualToSnapshot(snap, cat, txt, names, excludeUserIds...)`; with no `names` it delivers exactly as `SendTextVisual` (both reach `deliverVisual` with `names == nil`) | `internal/rooms/rooms.go:400`, `:419`, `:281`, `:372-383` |
| Snapshots today only for a darkness: player equip `usercommands/equip.go:121-124`, `:181-187`; mob equip `mobcommands/equip.go:67-70`, `:94-102`; floor pickup `hooks/mob_equip_best_floor_item.go:63-66`, `:84-94` | read |
| No snapshot: displaced items `usercommands/equip.go:144`, arm-slot "equips" `:157`, wield `:192`; player `remove` `usercommands/remove.go:66`; mob displaced `mobcommands/equip.go:89`, mob wield `:104`; mob `remove` `mobcommands/remove.go:31`; floor wield `mob_equip_best_floor_item.go:96` | read |
| `remove all` (player and mob) sends no room line at all, light or not | `usercommands/remove.go:27-46`, `mobcommands/remove.go:24-27` |
| Every shipped light (`20096` torch .. `20099` sunstone) is `type: light`, `subtype: wearable` with a worn light or darkness condition | `_datafiles/world/dogmud/items/armor-20000/light/*.yaml` |
| The narration guard recognises observer sends by exact method name on a receiver named `room` (`SendTextVisual`, `SendTextVisualToSnapshot`, `SendTextUnsighted`, ...) and the free function `sendVisualRoomText`; NOT `sendVisualElseAudible`. A phrasing-only `if/else` is folded into one event only when an observer call follows it IMMEDIATELY | `messaging_surface_guard_test.go:853-930`, `:962-976`, `:1029-1041` |
| Registry row for the no-target spell event is keyed on its FIRST call, `"hooks/spell_resolution.go\|Your spell erupts outward but finds no targets."`, verdict actor+observer | `messaging_surface_guard_test.go:1288` |
| lightnotice: `observation` `tracker.go:45`, `record` `:61`, `decide` `:86` (stores `next` at `:97`), `attribute` `:150`, `observe` `:331` (sight inputs at `:338`). The record keeps no sight window | read |
| `messaging.Band` runs `BandDark < BandShapes < BandFaces < BandDazzled` | `internal/messaging/band.go:19-26` |
| Target-gone line names the target raw | `hooks/spell_resolution.go:148` |
| No-target room line uses `sendVisualRoomText` only; the caster's name is raw (`user.Character.Name`) | `spell_resolution.go:180-182` |
| `sendVisualElseAudible` (the #445 idiom) = `room.SendTextVisual` + `room.SendTextUnsighted`; `playerSubjectName` hides a hidden player caster | `NewRound_DoCombat_helpers.go:484-490`, `:542-548` |
| Player-facing em dash also at `spell_resolution.go:222` ("The weave unravels ...") | grep |
| `sendDarkRoomCombatFallback` returns early on `room.IsLit()`; sends `u.SendText(CategoryDefault, ...)` to `!messaging.CanSeeShapes` readers | `NewRound_DoCombat_helpers.go:459-474` |
| `SendTextUnsighted` sends to readers whose `visualDecision` is `SightNone`, rendered exactly as `UserRecord.SendText` (audio channel); `visualDecision` is `SightNone` exactly when `CanSeeShapes` is false (both gate on `awake`) | `rooms.go:438-447`, `:546-572`; `messaging/predicates.go:141-143`, `:161`, `:208-214`; `users/userrecord.go:500-514` |
| Login flash at HEAD `_datafiles/config.yaml:74`; `emote` escapes ANSI on the free-text path | `git show HEAD:_datafiles/config.yaml`; `usercommands/emote.go:53` |
| In THIS worktree `git ls-files -v _datafiles/config.yaml` reads `H` (a fresh worktree index); the main checkout reads `S` | run both |
| Dashes in string literals: `usercommands/loot.go:170`, `go.go:132`, `usercommands.go:484`, `rally.go:30`, `warcry.go:31`, `hooks/spell_resolution.go:222`. Every other dash in those six files is in a comment | grep, every hit read |
| `noDashCopyFiles` holds 8 files | `copy_no_dash_test.go:18-27` |
| Personal special-move lines ride `CategorySystem`: `bash.go:23`, `drain.go:19`, `gore.go:19`, `kick.go:19`, `maul.go:19`, `pounce.go:19`, `rake.go:19`, `throttle.go:19` and `:30`, `trip.go:24`, `grapple.go:17-21`; hand-built partial trios `bash.go:113-114`, `drain.go:109-110`, `gore.go:101-102`, `kick.go:132-133`, `maul.go:97-98`, `pounce.go:108-109`, `rake.go:97-98`, `throttle.go:119-120`, `trip.go:139-140`; `grapple.go:146-147`, `:162-163`; `throw.go:334`, `:366`, `:446` | grep `internal/usercommands` |
| Mob side: `mobcommands.sendMoveEvent` takes ONE category for every role; the only `CategorySystem` move line is `mobcommands/throttle.go:85` (cast_interrupt) | `mobcommands/move_narration.go:71-82`; grep |
| `Verbosity.Suppresses` is applied only in `hooks/combat_verbosity.go` (`:310`, `:346`, `:503`, `:590`), never in `messaging.SendTrio`; trio Actor/Actee seats go through `SendText` (audio channel, no sight gate) | grep; `messaging/trio.go:118-123` |
| `send_trio_only_guard_test.go` allows `CategoryKick/Trip/Bash/Submission` only on a line that also holds `messaging.Say(`, `acteeDefenceLine(`, `sendMoveEvent(`, `lineOrNone(` or `moveCategories{` | `send_trio_only_guard_test.go:116-140` |
| `shouldWrap` admits 45 categories; `CategoryLogout` is not one; pinned by `TestShouldWrapMatchesPinnedAllowlist` | `messaging/pipeline.go:122-152`; `pipeline_test.go:57-140`; `messaging/context.md:27` says "45 of the 62" |
| Quit line: `sendVisualRoomText(room, messaging.CategoryLogout, tplTxt)` | `hooks/PlayerDespawn_HandleLeave.go:152` |
| Condition 31: flags `[hidden]` only, 16 rounds, start and end room lines; spell `empathic-shroud`, `primarystat: willpower`, `targeting: single`, school mental | `conditions/31-empathic_shroud.yaml`; `spells/empathic-shroud.yaml` |
| Condition 9: flags `[hidden, cancel-on-combat]` | `conditions/9-hidden.yaml` |
| `hideForStealthRecord(conditionId int)` acts on record 9 only; called by all three add doors | `characters/conditions.go:182-192`; doors `:152`, `:202`, `:223` |
| The Hidden cascade adds a PERMANENT 9 on every entry into Hidden; leaving Hidden runs `CancelConditionsWithFlag(conditions.Hidden)` | `hooks/Awareness_Cascades.go:47-65` |
| `CancelConditionsWithFlag` reveals a still-Hidden machine once no hidden-flag record is live | `characters/conditions.go:33-82` |
| `HiddenData` is `struct{}`; `ResolveConcealment(true)` sets it AFTER the transition, so the Hidden cascade cannot read it; there is no getter | `state/awareness/awareness.go:54`, `:168-183` |
| `Conditions.RemoveCondition` only marks a record expired; the prune pass then narrates its end line. There is no silent removal | `conditions/conditions.go:131-137`; `hooks/NewTurn_PruneConditions.go:39-58` |
| `Conditions.AddConditionMagnitude`: `triggers > 0` overrides, `0` keeps the spec count; a magnitude on a record whose spec has no magnitude effect is inert (`ScaledKind` false, not `TickFromMagnitude`, not a light) | `conditions/conditions.go:347-373`; `conditions/effects.go:88-95` |
| `magnitudeSpellApplication` / `applySpellCondition`: the caster-derived magnitude door, `CasterStatValue(caster.Stats)` + `GetSkillLevel(skills.Spellcasting)` | `hooks/light_spell.go:23-40`, `:56-67` |
| `CalcSneakScore` base = `Dexterity.ValueAdj + Skullduggery rank x SkillWeight + mutation stealth`, then light modifiers | `actions/skill_helpers.go:29-49` |
| `CalcSneakScoreVsObserver` has 11 production call sites (move x4, plant, search, shadow, sneak x2, steal, track); it is the only production caller of `CalcSneakScore` | grep |
| `actions.Sneak` returns `AlreadyHidden` for any hidden actor; the AST guard `TestThrowSneakCostAdmissionOrdering` requires exactly ONE `admitFullCost` in `Sneak`, after `char.IsFree()` and `actor.GetRoom()`, before `TransitionToConcealing` | `actions/sneak.go:132-138`; `actions/sneak_test.go:238-275` |
| Player reply "You're already hidden!"; mob wrapper awards progression win or lose (`AwardResolved`), not only on success as `actions/context.md` says | `usercommands/skill.skullduggery.sneak.go:55-57`; `mobcommands/sneak.go:10-26` |
| `Character.Validate` calls `c.Conditions.Validate()` then `c.reconcilePerception()`, then `RecalculateStats()` (which resets every `ValueAdj` from `Base`) | `characters/validate.go:694-701` |
| Every record-removal path validates: prune (`NewTurn_PruneConditions.go:61`, `:121`), `RemoveCondition` (`characters/conditions.go:238-243`), `CancelConditionsWithFlag` (`:57`), death (`Life_Cascades.go:38` ForceVisible; `:117-118` prune and Validate) | read |
| Awareness is not saved (`Awareness *awareness.Machine yaml:"-"`, `characters/character.go:210`). Logout runs `ForceVisible` before the save (`hooks/Logout_AwarenessCleanup.go`), whose cascade cancels every hidden-flag record | read |
| `condition_apply_path_guard_test.go` keys its allowlist by `file\|line`: `Condition_ApplyConditions.go\|107,109,111`, `Awareness_Cascades.go\|57`, `light_spell.go\|58` (and `rally.go`, `warcry.go` lines). `internal/characters` is exempt (primitive package) | `condition_apply_path_guard_test.go:111-113`, `:145`, `:263`, `:300-304` |
| Windwarden Sylara (mob 241) idle-casts `empathic-shroud` on herself | `mobs/ironwind_steppe/241-windwarden_sylara.yaml:15` |
| `SkillWeight` ships at `5.0` (`config.yaml`); Go default `2.0`. Tests read it through `configs.GetBalanceConfig()`, never a literal | `git show HEAD:_datafiles/config.yaml` line 1155; `configs/config.balance.go:330` |

### Spec facts that were wrong or imprecise

- "Every opposed roll against a hider goes through `CalcSneakScore` ... `actions/skill_helpers.go:66`; 11 call sites": the 11 sites call `CalcSneakScoreVsObserver`, whose body calls `CalcSneakScore` at `:68`.
- "Light lines judged after the change: only a DARKNESS takes a before-snapshot": true, and the floor pickup's wield line (`mob_equip_best_floor_item.go:96`) and the mob wield line (`mobcommands/equip.go:104`) also take none; added to the list.
- "Entering Hidden adds a permanent 9 ... The reveal cascade cancels 31 with 9" (G7 Breaking): with the plan's design a shroud hide carries no record 9 at all (see D4 below); the reveal cascade cancels 31 alone.
- `actions/context.md` says the mob sneak wrapper calls `OnSkillUse` only on success; the code awards win or lose. Task 11 corrects the doc.

### Design points where the code forced a different choice than the spec (for the owner)

- **D1 (G1).** `HideWeapons` keeps its `keep` parameter; after G1 no production caller passes one (the attacker's lines skip `HideWeapons` entirely, the defender's pass `nil`). Removing the parameter would touch nine `hideweapons_test.go` calls for no behaviour change.
- **D2 (G2).** Every equip and remove room line takes the before-snapshot, not only lines for an item that touches light. For an item that touches no light the snapshot reads exactly what the room reads after, so behaviour is unchanged; and the narration guard forces it: a branch between `SendTextVisual` and `SendTextVisualToSnapshot` after the arm-slot `if/else` stops that `if/else` folding into one event and orphans the guard's registry row. No `TouchesLight` helper is needed.
- **D3 (G4).** The harmless line keeps `sendVisualRoomText` and adds `room.SendTextUnsighted(..., messaging.SoundSpellSputtersOut, user.UserId)` beside it, which is what `sendVisualElseAudible` does internally. Calling `sendVisualElseAudible` would hide the event's observer from the narration guard (it does not recognise that helper) and orphan registry row `messaging_surface_guard_test.go:1288`. The line also now names the caster through `playerSubjectName`, the #445 sibling, so a hidden caster reads "Something".
- **D4 (G7).** A shroud hide carries record 31 and NO record 9: the Hidden cascade skips the 9 when the hide's data says shroud. With a 9 beside it, every shroud ending would tell the room two lines ("shimmers back into view" and "emerges from the shadows"), one round apart.
- **D5 (G7).** Condition 31 gains `cancel-on-combat`. Without a 9, a shroud hide would not break on the per-round `CancelCombatConditions` strip, the cross-room shot or death; with the flag it breaks on combat exactly as a sneak hide does.
- **D6 (G7).** One hide replacing another drops the loser's record with a new primitive, `Conditions.Discard` (deletes outright, so no end line), because `RemoveCondition` only expires a record and the prune would then tell the room of a reveal that did not happen.
- **D7 (G7).** A shroud landing on a holder already hidden tells the room nothing (the start line "seems to shimmer and fade from view" would give the hidden holder away); a shroud that loses to the sneak is refused, so neither the holder nor the room hears a start line. The caster still reads their own cast lines.
- **D8 (G7).** A re-cast shroud on a holder the shroud already hides takes the NEW record's score (the record holds one magnitude).
- **D9 (G7).** Ties keep the hide already there (sneak on shroud: "You're already hidden!"; shroud on sneak: dropped).
- **D10 (G7, behaviour change to flag).** Windwarden Sylara (mob 241) idle-casts `empathic-shroud`; she will now actually hide for 16 rounds each time. The playtest checks whether that harms her role.
- **D11 (G7, persistence).** No change needed. Awareness is not saved, and logout's `ForceVisible` cascade cancels 31 (hidden flag) exactly as it cancels 9, so a relog is Visible for both kinds. A crash mid-hide can leave a live 31 or 9 in a save on a Visible character; that is the pre-existing record-9 behaviour and is not changed here.
- **D12 (G5).** A sleeper is in `SendTextUnsighted`'s audience (as in `!CanSeeShapes` today), so a sleeper in a LIT room now also hears "You hear fighting close by."; before, only in a dark room.
- **D13 (G6).** Grapple (`grappleCategories`, the disarm and crit-failure trios) and the mob `throttle` cast_interrupt line get the same treatment as the nine listed moves (sibling paths). `throw`'s own result lines sent with `user.SendText(messaging.CategorySystem, ...)` (`throw.go:378`, `:466`, `:475`, `:496-501`) are self-coloured one-liners outside any trio and are left as they are.
- **D14 (G6).** The login flash keeps its two `⚡` glyphs: `emote @appears before you in a flash of ⚡lightning⚡!` (plain text, no tags).

---

## File map

| File | Change |
|---|---|
| `internal/combat/combat.go` | G1: `hideIdentitiesInPersonalLines` by owner; delete `heldWeaponNames` |
| `internal/combat/darkness_identity_hiding_test.go` | G1 test |
| `internal/usercommands/equip.go`, `remove.go` | G2 |
| `internal/mobcommands/equip.go`, `remove.go` | G2 |
| `internal/hooks/mob_equip_best_floor_item.go` | G2 |
| `internal/usercommands/equip_light_line_test.go`, `internal/mobcommands/equip_darkness_test.go`, `internal/hooks/mob_equip_floor_darkness_test.go` | G2 tests |
| `internal/lightnotice/tracker.go`, `tracker_test.go` | G3 |
| `internal/hooks/spell_resolution.go`, `spell_channel_sight_test.go` | G4 |
| `internal/hooks/NewRound_DoCombat_helpers.go`, `dark_room_fallback_sight_test.go` | G5 |
| `_datafiles/config.yaml` | G6a (HEAD-blob procedure) |
| `internal/usercommands/{loot,go,usercommands,rally,warcry}.go`, `copy_no_dash_test.go` | G6b |
| `internal/usercommands/{bash,drain,gore,grapple,kick,maul,pounce,rake,throttle,throw,trip,move_narration}.go`, `internal/mobcommands/throttle.go`, `internal/messaging/pipeline.go`, `pipeline_test.go` | G6c, G6d |
| `internal/usercommands/special_move_categories_test.go` (new) | G6 test |
| `internal/state/awareness/awareness.go`, `transitions.go`, `awareness_test.go` | G7a |
| `internal/conditions/ids.go`, `conditions.go`, `discard_test.go` (new) | G7a |
| `internal/characters/shroud_hide.go` (new), `conditions.go`, `validate.go`, `shroud_hide_test.go` (new) | G7b |
| `internal/actions/skill_helpers.go`, `sneak.go`, `sneak_shroud_test.go` (new) | G7c |
| `internal/usercommands/skill.skullduggery.sneak.go`, `skill_skullduggery_sneak_shroud_test.go` (new), `internal/mobcommands/sneak.go` | G7c |
| `internal/hooks/Awareness_Cascades.go`, `light_spell.go`, `Condition_ApplyConditions.go`, `shroud_hide_test.go` (new) | G7d |
| `_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml` | G7d |
| `condition_apply_path_guard_test.go` | G7d (line keys) |
| `context.md` in `combat`, `messaging`, `rooms`, `usercommands`, `mobcommands`, `lightnotice`, `hooks`, `state/awareness`, `conditions`, `characters`, `actions` | with each task |

New Go files are package files, not repo-root guards, so `docs/README.md` gets no row for them.

## Rules for every task

- Edit files with the Edit tool. Never a Python read-modify-write. Never `git add -A` or `git add .`; stage named paths only.
- `gofmt -l` every touched Go file must print nothing; run `gofmt -w <file>` if it does.
- Line numbers in a task are the file's numbers before that task's edits; find each edit by its quoted anchor text, not by number alone.
- Run targeted tests: `go test ./internal/<pkg>/ -run '<Name>' -count=1`. The guard tests at the repo root run with `go test . -run '<Name>' -count=1`.
- `grep -c` exits 1 on zero matches. Run an "expect zero" check on its own line, never inside an `&&` chain.
- No em dash or en dash in any Go string literal, comment you write, or YAML text.
- Commit messages end with a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Do not write "closes #N".
- A test fails first: Step 2 of each task is the failing run. If a new test passes before the fix, stop and find out why.

---

### Task 1: G1, weapons hidden by owner, not by name (#446)

**Files:**
- Modify: `internal/combat/combat.go:744-777`
- Test: `internal/combat/darkness_identity_hiding_test.go` (append)
- Docs: `internal/combat/context.md:1136-1140`, `internal/messaging/context.md:428-436`

- [ ] **Step 1: Write the failing test.** Append to `internal/combat/darkness_identity_hiding_test.go` (it already imports `strings`, `testing`, `characters`, `items`, `messaging`):

```go
// #446: weapons are kept by OWNER, not by name. Every weapon in one
// AttackResult is the attacker's, so a defender holding a weapon of the same
// name learned nothing new and still read the attacker's weapon by name.
func TestHideIdentitiesInPersonalLines_SameNamedWeaponsHideByOwner(t *testing.T) {
	atk := characters.New()
	atk.Name = "Ordel"
	atk.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{Name: "iron longsword"}}
	def := characters.New()
	def.Name = "Fold"
	def.Equipment.Weapon = items.Item{ItemId: 999903, Spec: &items.ItemSpec{Name: "iron longsword"}}

	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		res := &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `You slash <ansi fg="username">Fold</ansi> with your <ansi fg="item">iron longsword</ansi>!`},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `<ansi fg="username">Ordel</ansi> slashes you with their <ansi fg="item">iron longsword</ansi>!`},
			},
		}
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: d, targetSight: d})

		if strings.Contains(strings.ToLower(res.MessagesToTarget[0].Text), "longsword") {
			t.Fatalf("sight %d: the defender read the attacker's weapon through their own same-named one: %q", d, res.MessagesToTarget[0].Text)
		}
		if !strings.Contains(res.MessagesToSource[0].Text, `your <ansi fg="item">iron longsword</ansi>`) {
			t.Fatalf("sight %d: the attacker lost their own weapon: %q", d, res.MessagesToSource[0].Text)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/combat/ -run TestHideIdentitiesInPersonalLines_SameNamedWeaponsHideByOwner -count=1`
Expected: FAIL, "the defender read the attacker's weapon through their own same-named one".

- [ ] **Step 3: Implement.** In `internal/combat/combat.go`, replace the last two lines of the doc comment above `hideIdentitiesInPersonalLines` (`// Weapons follow the same rule (spec F2, ruling R8): below full sight a` / `// reader's line names no weapon but their own, which they hold.`), the function, and the whole `heldWeaponNames` function (with its 3-line doc comment) with:

```go
// Weapons follow by OWNER (spec F2, ruling R8; #446). Every weapon token in
// one AttackResult is the ATTACKER's: {itemname} is the swing's weapon,
// {weapon} and {attack} are sourceChar.Equipment.Weapon. So the attacker's
// own lines keep every weapon, and below full sight the defender's lines
// keep none. Matching the defender's own gear by NAME leaked the attacker's
// weapon whenever both held a weapon of the same name. The crit disarm line
// names the victim's weapon outside any AttackResult (criteffects.go) and
// is the accepted known limit.
func hideIdentitiesInPersonalLines(result *AttackResult, sourceChar, targetChar *characters.Character, ctx combatContext) {
	for i := range result.MessagesToSource {
		result.MessagesToSource[i].Text = messaging.HideNames(result.MessagesToSource[i].Text, []string{targetChar.Name}, ctx.sourceSight)
	}
	targetHides := []string{sourceChar.Name}
	if sourceChar.Pet.Exists() {
		targetHides = append(targetHides, sourceChar.Pet.PlainName())
	}
	for i := range result.MessagesToTarget {
		text := messaging.HideNames(result.MessagesToTarget[i].Text, targetHides, ctx.targetSight)
		result.MessagesToTarget[i].Text = messaging.HideWeapons(text, ctx.targetSight, nil)
	}
}
```

- [ ] **Step 4: Run the combat package.**

Run: `go test ./internal/combat/ -count=1` and `go vet ./internal/combat/`
Expected: PASS (the existing `TestHideIdentitiesInPersonalLines_HidesTheOtherPartysWeapon` and `TestShapes_DodgedArmedSwingHidesTheWeaponName` still hold: the attacker keeps their weapon, the defender reads "weapon").

- [ ] **Step 5: Docs.** In `internal/combat/context.md`, replace the paragraph starting `Weapons too, since the sight-gates playtest fixes` (lines 1136-1140) with:

```markdown
Weapons too, since the sight-gates playtest fixes (spec F2, ruling R8) and
#446: every weapon token in one `AttackResult` is the attacker's, so the
attacker's own lines (`MessagesToSource`) keep every weapon and the
defender's lines (`MessagesToTarget`) go through `messaging.HideWeapons` at
the defender's sight with no keep-list. Weapons are kept by owner, never by
name: a defender holding a same-named weapon learns nothing more.
```

In `internal/messaging/context.md` (the `HideWeapons` entry, lines 428-436) replace `and any name in \`keep\` (a participant's own held gear).` with `and any name in \`keep\` (no production caller passes one since #446: \`combat.hideIdentitiesInPersonalLines\` skips the attacker's own lines and passes nil for the defender's).`

- [ ] **Step 6: Commit.**

```bash
git add internal/combat/combat.go internal/combat/darkness_identity_hiding_test.go internal/combat/context.md internal/messaging/context.md
git commit -m "fix(combat): hide weapons by owner, not by name (#446)

Every weapon in one AttackResult is the attacker's, so the attacker keeps
every weapon and the defender keeps none below full sight. Deletes
heldWeaponNames.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: G2, player equip and remove lines judged before the change (#447)

**Files:**
- Modify: `internal/usercommands/equip.go:118-196`, `internal/usercommands/remove.go:48-69`
- Test: `internal/usercommands/equip_light_line_test.go` (append)
- Docs: `internal/usercommands/context.md:533-540`, `internal/rooms/context.md:29-30`

- [ ] **Step 1: Write the failing tests.** In `internal/usercommands/equip_light_line_test.go`, add `"github.com/GoMudEngine/GoMud/internal/rooms"` to the imports, add `equipLightTestStub  = 999977 // a light-slot item that sheds nothing` to the `const` block after `equipLightTestHat`, and append:

```go
// equipLightRoomFixture is equipLightFixture in cave room 2 with no sky and
// no lamp, so the torch is the room's only light, and user 2 watching. It
// adds a stub: a light-slot item that sheds nothing.
func equipLightRoomFixture(t *testing.T) (user, observer *users.UserRecord, room *rooms.Room) {
	t.Helper()
	user = equipLightFixture(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		equipLightTestTorch: {ItemId: equipLightTestTorch, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{equipLightTestCond}},
		equipLightTestHat: {ItemId: equipLightTestHat, Name: "test hat", NameSimple: "hat",
			Type: items.Head, Subtype: items.Wearable},
		equipLightTestStub: {ItemId: equipLightTestStub, Name: "test stub", NameSimple: "stub",
			Type: items.Light, Subtype: items.Wearable},
	}))
	user.Character.StoreItem(items.Item{ItemId: equipLightTestStub})
	room = rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	rooms.LoadRoom(user.Character.RoomId).RemovePlayer(user.UserId)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)
	observer = users.GetByUserId(2)
	require.NotNil(t, observer)
	rooms.LoadRoom(observer.Character.RoomId).RemovePlayer(observer.UserId)
	observer.Character.RoomId = 2
	room.AddPlayer(observer.UserId)
	require.Less(t, room.LightLevel(), 25, "fixture: the room must be dark with no torch")
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(observer.UserId)
	return user, observer, room
}

// #447: a light's equip and remove lines were judged after the change. The
// line announcing a light lands with the room as it was before (owner rule,
// 2026-10-05).
func TestLightLines_JudgedBeforeTheChange(t *testing.T) {
	t.Run("lighting a torch in the dark is not seen", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, room.LightLevel(), 25, "fixture: the torch must light the room")
		got := hoodTestText(observer.UserId)
		require.NotContains(t, got, "puts on their", "the observer was blind when it went on")
		require.NotContains(t, got, user.Character.Name)
	})
	t.Run("taking off the only light is seen", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		hoodTestText(observer.UserId)
		_, err = Remove("test torch", user, room, 0)
		require.NoError(t, err)
		require.Less(t, room.LightLevel(), 25, "fixture: the room must go dark")
		require.Contains(t, hoodTestText(observer.UserId), "removes their",
			"the observer saw the torch taken off before the dark fell")
	})
	t.Run("a displaced light is seen going and the stub coming", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		hoodTestText(observer.UserId)
		_, err = Equip("test stub", user, room, 0)
		require.NoError(t, err)
		require.Equal(t, equipLightTestStub, user.Character.Equipment.Light.ItemId, "fixture: the stub must displace the torch")
		require.Less(t, room.LightLevel(), 25, "fixture: the room must go dark")
		got := hoodTestText(observer.UserId)
		require.Contains(t, got, "removes their", "the displaced torch, seen by its own light")
		require.Contains(t, got, "puts on their", "the stub, seen by the torch it replaced")
	})
}
```

- [ ] **Step 2: Run and see all three subtests fail.**

Run: `go test ./internal/usercommands/ -run TestLightLines_JudgedBeforeTheChange -count=1`
Expected: FAIL in all three subtests.

- [ ] **Step 3: Implement in `internal/usercommands/equip.go`.**

Replace the darkness-only snapshot (lines 118-124, starting `// A darkness's "puts on" line is judged against the room as it was`) with:

```go
		// Every room line below is judged against the room as it was before
		// the equip (owner rule, 2026-10-05; #447): a light or darkness put
		// on, or a light displaced, changes what the room can see, and the
		// line announcing it lands with the state before. An item that
		// touches no light changes no one's sight, so the snapshot reads the
		// same as the room would after; taking it always keeps one call per
		// line, which the narration guard (messaging_surface_guard_test.go)
		// needs to see each line's observer.
		before := room.VisualSnapshot()
```

In the displaced-items loop, replace `room.SendTextVisual(messaging.CategoryEquipment,` ... `user.UserId,` `)` (lines 144-147) with:

```go
					room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, user.Character.Name, oldItem.DisplayName()),
						nil, user.UserId,
					)
```

Replace the arm-slot room line (lines 157-160) with (it must stay the call immediately after the `if/else`):

```go
				room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> equips their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					nil, user.UserId,
				)
```

Replace from `putsOn := fmt.Sprintf(` through the second else-less `if !snapped { ... }` (lines 171-187) with:

```go
				// Judged against the room before it went on (owner rule,
				// 2026-10-05; lighting plan 5d, ruling D6 as amended; #447): a
				// darkness already worn would silence its arrival for everyone
				// it has just blinded, and a light already worn would name the
				// wearer to someone who was blind before it was lit.
				room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> puts on their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					nil, user.UserId)
```

Replace the wield room line (lines 192-195) with:

```go
				room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> wields their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					nil, user.UserId,
				)
```

`conditions` stays imported (the `AnyLightSource` light note at line 165).

- [ ] **Step 4: Implement in `internal/usercommands/remove.go`.** Replace `	result := actions.RemoveEquipment(actor, rest)` (line 48) with:

```go
	// The room line is judged against the room before the item comes off
	// (owner rule, 2026-10-05; #447): taking off the only light must still
	// show the watchers it just left in the dark who took it off.
	before := room.VisualSnapshot()
	result := actions.RemoveEquipment(actor, rest)
```

and the room line at lines 66-69 with:

```go
			room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, user.Character.Name, result.Item.DisplayName()),
				nil, user.UserId,
			)
```

`remove all` sends no room line today and is left as it is.

- [ ] **Step 5: Run.**

Run: `go test ./internal/usercommands/ -count=1` then `go test . -run 'TestNarrationSitesMatchViewpointAudit|TestEveryTrioLiteralNamesAllThreeRoles' -count=1`
Expected: PASS for both (the darkness tests in `darkness_test.go` still pass; the narration guard still sees each event's observer).

- [ ] **Step 6: Docs.** In `internal/usercommands/context.md`, replace the sentence that starts `The wearable room line ("X puts on their Y.") is judged against a` through `and would lose the line's observer.` (lines 532-540) with:

```markdown
  Every equip room line (displaced items, the arm-slot "equips", "puts on",
  "wields") and the single `remove` line is judged against a
  `rooms.VisualSnapshot` taken before the change
  (`room.SendTextVisualToSnapshot`, #447): a light or darkness put on, taken
  off or displaced announces itself with the room as it was before (owner
  rule, 2026-10-05), so an observer blind before a torch was lit learns
  nothing and one the torch's removal leaves in the dark still sees it come
  off. For an item that touches no light the snapshot reads what the room
  reads after. One call per line keeps the viewpoint walk in the root
  `messaging_surface_guard_test.go` seeing each line's observer. `remove all`
  sends no room line.
```

In `internal/rooms/context.md` (lines 29-30), the sentence reads "Lighting plan 5d uses it for a darkness's start line and its equip lines;" across two lines. Change `darkness's start line and its equip lines;` to `darkness's start line, #447 for every equip and remove room line;`, keeping the line break where it is.

- [ ] **Step 7: Commit.**

```bash
git add internal/usercommands/equip.go internal/usercommands/remove.go internal/usercommands/equip_light_line_test.go internal/usercommands/context.md internal/rooms/context.md
git commit -m "fix(equip): judge player equip and remove lines before the change (#447)

Every equip and remove room line takes the before-snapshot, so a light lit
in the dark is not seen and a light taken off is.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: G2, mob equip, mob remove and floor pickup lines judged before the change (#447)

**Files:**
- Modify: `internal/mobcommands/equip.go:64-107`, `internal/mobcommands/remove.go:29-33`, `internal/hooks/mob_equip_best_floor_item.go:7`, `:60-99`
- Test: `internal/mobcommands/equip_darkness_test.go`, `internal/hooks/mob_equip_floor_darkness_test.go` (append)
- Docs: `internal/mobcommands/context.md:104-110`

- [ ] **Step 1: Write the failing tests.** Append to `internal/mobcommands/equip_darkness_test.go` (its imports already cover `fmt`, `strings`, `conditions`, `events`, `items`, `rooms`, `require`):

```go
// #447 on the mob path: a mob taking off the room's only light is seen by the
// players it leaves in the dark, judged by the room before it came off.
func TestMobRemovingTheOnlyLightIsSeenBeforeTheDarkFalls(t *testing.T) {
	const torchCond, torchItem = 9773, 999973
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		torchCond: {ConditionId: torchCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 40}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		torchItem: {ItemId: torchItem, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{torchCond}},
	}))
	mob, room := getTestMobAndRoom(t)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.True(t, mob.Character.StoreItem(items.Item{ItemId: torchItem}))
	_, err := Equip(fmt.Sprintf("!%d", torchItem), mob, room)
	require.NoError(t, err)
	require.GreaterOrEqual(t, room.LightLevel(), 25, "fixture: the torch must light the room")
	events.DrainQueuedMessagesForTest(1)

	_, err = Remove("test torch", mob, room)
	require.NoError(t, err)
	require.Less(t, room.LightLevel(), 25, "fixture: the room must go dark")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(1), "\n"), "removes their",
		"the player the mob just left in the dark missed it taking the torch off")
}
```

Append to `internal/hooks/mob_equip_floor_darkness_test.go`:

```go
// #447 on the floor-loot path: a mob lighting a torch it picked up in a dark
// room is not seen by a player who was blind when it was picked up.
func TestMobDonningAFloorLightIsNotSeenInARoomDarkBeforeIt(t *testing.T) {
	const torchCond, torchItem = 9773, 999973
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		torchCond: {ConditionId: torchCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 40}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		torchItem: {ItemId: torchItem, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{torchCond},
			PhysicalMitigation: 20}, // scores as an upgrade, so the mob picks it
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.BehaviorArchetype = "generic_fighter"
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.Less(t, room.LightLevel(), 25, "fixture: the room must be dark before the torch")
	room.Items = []items.Item{{ItemId: torchItem}}
	drainPlain(1)

	require.True(t, EquipBestFloorItem(mob, room), "fixture: the mob must choose the torch")
	require.GreaterOrEqual(t, room.LightLevel(), 25, "fixture: the torch must light the room")
	got := strings.Join(drainPlain(1), "\n")
	require.NotContains(t, got, "picks up", "a player blind before the torch was lit saw it picked up")
	require.NotContains(t, got, mob.Character.Name)
}
```

- [ ] **Step 2: Run and see both fail.**

Run: `go test ./internal/mobcommands/ -run TestMobRemovingTheOnlyLight -count=1` and `go test ./internal/hooks/ -run TestMobDonningAFloorLight -count=1`
Expected: FAIL in both.

- [ ] **Step 3: Implement `internal/mobcommands/equip.go`.** Replace the darkness-only snapshot (lines 64-70) with:

```go
	// Every equip line is judged against the room as it was before the item
	// goes on (owner rule, 2026-10-05; #447), as the player path: a light or
	// darkness put on, or a light displaced, changes what the room can see.
	before := room.VisualSnapshot()
```

Replace the `if result.Equipped { ... }` body's three sends (lines 87-106: the displaced loop, the wearable `if/else`, and the wield `else`) with:

```go
		for _, oldItem := range result.DisplacedItems {
			if oldItem.ItemId != 0 {
				room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, mob.Character.Name, oldItem.DisplayName()), nil)
			}
		}

		if iSpec.Subtype == items.Wearable {
			room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> puts on <ansi fg="item">%s</ansi>.`, mob.Character.Name, result.Item.DisplayName()), nil)
		} else {
			room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wields <ansi fg="item">%s</ansi>.`, mob.Character.Name, result.Item.DisplayName()), nil)
		}
```

The refusal line ("turns the X over, then sets it aside.") changes no light and stays `SendTextVisual`.

- [ ] **Step 4: Implement `internal/mobcommands/remove.go`.** Replace lines 29-33 with:

```go
	// Judged against the room before the item comes off (owner rule,
	// 2026-10-05; #447), as the player path.
	before := room.VisualSnapshot()
	result := actions.RemoveEquipment(actor, rest)
	if result.Removed {
		room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, mob.Character.Name, result.Item.DisplayName()), nil)
	}
```

- [ ] **Step 5: Implement `internal/hooks/mob_equip_best_floor_item.go`.** Replace the darkness-only snapshot (lines 60-66) with:

```go
	// The pickup line is judged against the room as it was before the item
	// is picked up and worn (owner rule, 2026-10-05; #447), as the equip
	// command: a light or darkness donned changes what the room can see.
	before := room.VisualSnapshot()
```

Replace the `if spec.Subtype == items.Wearable { ... } else { ... }` block (lines 84-99) with:

```go
	if spec.Subtype == items.Wearable {
		room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> picks up <ansi fg="item">%s</ansi> and dons it.`,
			mob.Character.Name, result.Item.DisplayName()), nil)
	} else {
		room.SendTextVisualToSnapshot(before, messaging.CategoryEquipment, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> picks up <ansi fg="item">%s</ansi> and wields it.`,
			mob.Character.Name, result.Item.DisplayName()), nil)
	}
```

Remove the now unused `"github.com/GoMudEngine/GoMud/internal/conditions"` import (line 7).

- [ ] **Step 6: Run.**

Run: `go build ./... && go test ./internal/mobcommands/ ./internal/hooks/ -count=1`
Expected: PASS (the four darkness tests in these two files still pass).

- [ ] **Step 7: Docs.** In `internal/mobcommands/context.md` replace the sentence from `` `equip` judges its`` through `` so\n  does the idle floor pickup, `hooks.EquipBestFloorItem`.`` (lines 104-110) with:

```markdown
  `equip` and `remove` judge every room line (displaced items, "puts on",
  "wields", "removes") against a `rooms.VisualSnapshot` of the room taken
  before the change, sent with `Room.SendTextVisualToSnapshot` (lighting
  plan 5d ruling D6 as amended by the owner on 2026-10-05; #447), the
  siblings of the player's commands; so does the idle floor pickup,
  `hooks.EquipBestFloorItem`.
```

- [ ] **Step 8: Commit.**

```bash
git add internal/mobcommands/equip.go internal/mobcommands/remove.go internal/hooks/mob_equip_best_floor_item.go internal/mobcommands/equip_darkness_test.go internal/hooks/mob_equip_floor_darkness_test.go internal/mobcommands/context.md
git commit -m "fix(equip): judge mob equip, remove and floor pickup lines before the change (#447)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: G3, the sight notice blames the eyes when the eyes moved (#448)

**Files:**
- Modify: `internal/lightnotice/tracker.go` (`observation` :45-58, `record` :61-69, `decide` :97, `attribute` :150-187, `observe` :338-350)
- Test: `internal/lightnotice/tracker_test.go`
- Docs: `internal/lightnotice/context.md:49-60`

- [ ] **Step 1: Write the failing tests.** In `internal/lightnotice/tracker_test.go`, add after `obs` (before `sight`):

```go
// recSight is rec read through a night-sight window of strength (no infra
// reach).
func recSight(room int, b messaging.Band, terms rooms.LightTerms, strength int) record {
	r := rec(room, b, terms)
	r.sight = sightWindow{strength: strength}
	return r
}

// obsSight is obs for an observer whose window is strength (no infra reach):
// bandAt reads through it and the observation carries it.
func obsSight(room int, b messaging.Band, terms rooms.LightTerms, strength int) observation {
	o := obs(room, b, terms, sight(strength))
	o.sight = sightWindow{strength: strength}
	return o
}
```

In `TestAttribution`'s `cases`, after the row `"a room with no sky on either side still attributes the lamp"`, add:

```go

		// #448: the record keeps the sight window. When the eyes changed and
		// the OLD light through the NEW eyes moves the band the way it went
		// (not necessarily all the way), the eyes take the blame even though
		// a light drifted at the same moment.

		{"eyes moved the band and a carried light drifted the same way: eyes",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 40, Carried: true, CarriedLight: 40}, 24),
			obsSight(1, messaging.BandDark, rooms.LightTerms{Level: 20, Carried: true, CarriedLight: 20}, 0),
			CauseEyes},
		{"eyes changed but read the old light the same: the light is named",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Carried: true, CarriedLight: 60}, 10),
			obsSight(1, messaging.BandDark, rooms.LightTerms{Level: 20, Carried: true, CarriedLight: 20}, 0),
			CauseCarried},
		{"eyes moved the band the other way: the light is named",
			recSight(1, messaging.BandFaces, rooms.LightTerms{Level: 60, Sky: 60}, 0),
			obsSight(1, messaging.BandShapes, rooms.LightTerms{Level: 10, Sky: 10}, 24),
			CauseSky},
```

Append at the end of the file:

```go

// The record keeps the sight window it was read through (#448), so the next
// check can tell the eyes changed.
func TestDecideRecordsTheSightWindow(t *testing.T) {
	now := obsSight(1, messaging.BandShapes, termsDim, 12)
	now.sight.reach = 30
	_, _, next := decide(record{}, false, now, TriggerCommand)
	if next.sight != (sightWindow{strength: 12, reach: 30}) {
		t.Errorf("the record must keep the window, got %+v", next.sight)
	}
}
```

- [ ] **Step 2: Run and see it fail.**

Run: `go test ./internal/lightnotice/ -count=1`
Expected: build FAIL, `undefined: sightWindow` (and `r.sight undefined`).

- [ ] **Step 3: Implement.** In `tracker.go`, replace the last field line of `observation` (`	bandAt func(light int) messaging.Band`) and the struct's closing `}` with the lines below, which add a field and declare the new type after the struct:

```go
	bandAt func(light int) messaging.Band
	// sight is the window bandAt reads through. The record keeps it, so
	// attribute can tell that the observer's eyes changed (#448).
	sight sightWindow
}

// sightWindow is an observer's sight inputs: night-sight strength
// (Character.NightVisionStrength) and infra reach (Character.InfraReach).
type sightWindow struct {
	strength int
	reach    int
}
```

In `record`, after `terms  rooms.LightTerms` add:

```go
	// sight is the window the band was read through.
	sight sightWindow
```

In `decide` (line 97):

```go
	next := record{roomId: now.roomId, band: now.band, terms: now.terms, sight: now.sight}
```

In `attribute`, directly after the existing exact counterfactual (`if now.bandAt != nil && now.bandAt(prev.terms.Level) == now.band { return CauseEyes }`), add:

```go
	// The eyes changed too, and on their own they move the band the way it
	// went, even if not all the way: a light that drifted at the same moment
	// does not take the blame (#448). Read through the window the record
	// kept, so a light that moved while the eyes held still is never eyes.
	if now.bandAt != nil && prev.sight != now.sight &&
		movedSameWay(prev.band, now.bandAt(prev.terms.Level), now.band) {
		return CauseEyes
	}
```

Add before `lampAgrees`:

```go
// movedSameWay reports whether via moved off from in the same direction as
// to did. Bands run darkest to brightest (messaging.Band). to differs from
// from: decide filters an unchanged band before attribute runs.
func movedSameWay(from, via, to messaging.Band) bool {
	return via != from && (via > from) == (to > from)
}
```

In `observe`, after the `bandAt:` field of the returned `observation`, add:

```go
		sight: sightWindow{strength: strength, reach: reach},
```

- [ ] **Step 4: Run.**

Run: `go test ./internal/lightnotice/ -count=1`
Expected: PASS. The fixture check at the top of `TestAttribution`'s loop confirms each new row is physically consistent; the first new row read `CauseCarried` before this change.

- [ ] **Step 5: Docs.** In `internal/lightnotice/context.md`, after step 2 of "Attribution order" (the item ending `(a draught wearing off, or taking hold).`), insert:

```markdown
   **2b. Eyes (direction, #448):** each record keeps the sight window it
   was read through (`sightWindow`: night-sight strength and infra reach).
   If the window changed AND the OLD light through the CURRENT sight moves
   the band off the old one in the same direction the band moved
   (`movedSameWay`), the cause is the eyes even when a light term drifted
   too and the eyes alone fall short of the new band. A window that did not
   change never takes this step, so a light moving while the eyes hold
   still is never blamed on them.
```

- [ ] **Step 6: Commit.**

```bash
git add internal/lightnotice/tracker.go internal/lightnotice/tracker_test.go internal/lightnotice/context.md
git commit -m "fix(lightnotice): blame the eyes when the eyes moved the band (#448)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: G4, the spell target-gone and harmless lines (#242 residue)

**Files:**
- Modify: `internal/hooks/spell_resolution.go:146-149`, `:178-182`, `:222`
- Test: `internal/hooks/spell_channel_sight_test.go` (append)
- Guard: `copy_no_dash_test.go` gets `spell_resolution.go` in Task 7

- [ ] **Step 1: Write the failing tests.** In `internal/hooks/spell_channel_sight_test.go` add imports `"github.com/GoMudEngine/GoMud/internal/combatvocab"`, `"github.com/GoMudEngine/GoMud/internal/spells"` and `"github.com/GoMudEngine/GoMud/internal/state/activity"`, and append:

```go
// testHarmSpell is a single-target harm spell with nothing authored beyond
// its axes, enough for resolveSpell to reach its no-target lines.
func testHarmSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-bolt", Name: "Test Bolt", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle, EffectType: "damage"}
}

// #242 residue: the target-gone line named the target raw, to a caster who
// could not make out their own room.
func TestPlayerSpellTargetGone_NamedOnlyAsFarAsTheCasterSees(t *testing.T) {
	room := seedFallbackRoom(t, 0, nightEyesConditionId)
	caster, target := users.GetByUserId(2), users.GetByUserId(1)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(caster.Character, room))
	room.RemovePlayer(target.UserId)
	target.Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(target.UserId)
	drainPlain(caster.UserId)

	resolveSpell(caster, activity.CastingData{SpellId: "test-bolt", TargetUserIds: []int{target.UserId}}, testHarmSpell(), room)

	got := drainPlain(caster.UserId)
	require.Equal(t, 1, countContaining(got, "Your spell dissipates, unspent. Something is no longer here."), "%v", got)
	require.Zero(t, countContaining(got, target.Character.Name), "%v", got)
}

// #242 residue: "crackles through the air harmlessly" went out visual only,
// so a reader who saw nothing heard nothing of a spell spent in the room.
func TestPlayerSpellFindsNothing_HeardByAReaderWhoSeesNothing(t *testing.T) {
	t.Run("sees nothing, hears it sputter out", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		resolveSpell(caster, activity.CastingData{SpellId: "test-bolt"}, testHarmSpell(), room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
		require.Zero(t, countContaining(got, "crackles"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
	t.Run("shapes reader sees a figure, hears no sound line", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		resolveSpell(caster, activity.CastingData{SpellId: "test-bolt"}, testHarmSpell(), room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "crackles through the air harmlessly"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Zero(t, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	})
}
```

- [ ] **Step 2: Run and see both fail.**

Run: `go test ./internal/hooks/ -run 'TestPlayerSpellTargetGone|TestPlayerSpellFindsNothing' -count=1`
Expected: FAIL: the caster reads the target's name; the blind reader hears nothing.

- [ ] **Step 3: Implement.** In `spell_resolution.go`, replace the target-gone block (lines 146-149, `if targetUser.Character.RoomId != room.RoomId {` ... `}`) with:

```go
		if targetUser.Character.RoomId != room.RoomId {
			// Named only as far as the caster can see in the caster's room
			// (#242): a caster who cannot make the room out reads
			// "Something is no longer here."
			user.SendText(messaging.CategorySpellDisruption, messaging.HideNames(
				fmt.Sprintf(`Your spell dissipates, unspent. <ansi fg="username">%s</ansi> is no longer here.`, targetUser.Character.Name),
				[]string{targetUser.Character.Name}, messaging.ParticipantSight(user.Character, room)))
			continue // target left the room before spell resolved
		}
```

Replace the no-target room line (lines 179-182: the `sendVisualRoomText(...)` call after `Your spell erupts outward but finds no targets.`) with:

```go
			sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
				`%s's spell crackles through the air harmlessly.`,
				playerSubjectName(user)), user.UserId)
			// The sound half (#242, owner ruling R4), for a reader who sees
			// nothing: what sendVisualElseAudible sends, kept as two calls so
			// the narration guard still sees this event's observer line.
			room.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundSpellSputtersOut, user.UserId)
```

Line 222: the string literal `` `<ansi fg="red">The weave unravels ... the spell fails to take shape.</ansi>` `` carries an em dash after "unravels". Replace the whole literal with:

```go
`<ansi fg="red">The weave unravels, and the spell fails to take shape.</ansi>`
```

- [ ] **Step 4: Run.**

Run: `go test ./internal/hooks/ -count=1` and `go test . -run TestNarrationSitesMatchViewpointAudit -count=1`
Expected: PASS for both (registry row `"hooks/spell_resolution.go|Your spell erupts outward but finds no targets."` still reads actor+observer).

- [ ] **Step 5: Commit.**

```bash
git add internal/hooks/spell_resolution.go internal/hooks/spell_channel_sight_test.go
git commit -m "fix(spells): target-gone line follows the caster's sight; harmless line is heard (#242)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: G5, the fight sound reaches a Blinded reader in a lit room (#216)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:453-474`
- Test: `internal/hooks/dark_room_fallback_sight_test.go` (append)

- [ ] **Step 1: Write the failing test.** In `internal/hooks/dark_room_fallback_sight_test.go` add imports `"github.com/GoMudEngine/GoMud/internal/state"` and `"github.com/GoMudEngine/GoMud/internal/state/perception"`, and append:

```go
// #216: the fight sound skipped every lit room, so a Blinded reader beside a
// fight in a lit room heard nothing. It goes to every reader who cannot make
// out shapes, whatever the room's light.
func TestDarkRoomCombatFallback_BlindedReaderInALitRoomHearsIt(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	require.True(t, room.IsLit(), "fixture: the room must be lit")
	blind := users.GetByUserId(2)
	saved := blind.Character.Perception
	t.Cleanup(func() { blind.Character.Perception = saved })
	blind.Character.Perception = perception.NewMachine()
	require.NoError(t, blind.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind.Character, room))

	sendDarkRoomCombatFallback(room)

	require.Equal(t, 1, countContaining(drainPlain(2), "You hear fighting close by."), "the blinded reader hears the fight")
	require.Zero(t, countContaining(drainPlain(1), "You hear fighting"), "a reader who sees the fight reads it by eye")
}
```

- [ ] **Step 2: Run and see it fail.**

Run: `go test ./internal/hooks/ -run TestDarkRoomCombatFallback_BlindedReaderInALitRoomHearsIt -count=1`
Expected: FAIL, the blinded reader got 0 lines.

- [ ] **Step 3: Implement.** Replace the doc comment and body of `sendDarkRoomCombatFallback` (lines 453-474) with:

```go
// sendDarkRoomCombatFallback sends a one-time "You hear fighting close by."
// line to every player who cannot follow the fight by eye: one the visual
// pipeline delivers nothing to (Room.SendTextUnsighted, the same audience as
// !messaging.CanSeeShapes, a sleeper included). It used to test the
// nightvision FLAG, which sent the sound to an infravision holder reading
// shapes and withheld it from a nightvision holder whose window reads the
// room as blind (lighting plan 5c). It also skipped every lit room, so a
// Blinded reader in a lit room heard nothing of a fight beside them (#216).
//
// #216: every caller passes the fight's own room, so the reader is IN the
// fight's room; "nearby" said otherwise.
func sendDarkRoomCombatFallback(room *rooms.Room, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextUnsighted(messaging.CategoryDefault, `<ansi fg="yellow">You hear fighting close by.</ansi>`, excludeUserIds...)
}
```

Audience change, stated: a reader who sees nothing in a LIT room (Blinded, and a sleeper, see D12) now hears the line; every reader it reached before still does, rendered identically (`SendTextUnsighted` and `UserRecord.SendText` both render on the audio channel).

- [ ] **Step 4: Run.**

Run: `go test ./internal/hooks/ -count=1` and `go test . -run 'TestCopyFilesHaveNoDashesInStringLiterals|TestNarrationSitesMatchViewpointAudit' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/dark_room_fallback_sight_test.go
git commit -m "fix(combat): fight sound reaches every reader who cannot see, lit room or not (#216)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: G6a and G6b, the login flash and the dash lines (#449)

**Files:**
- Modify: `_datafiles/config.yaml:74` (HEAD-blob procedure below), `internal/usercommands/loot.go:170`, `go.go:132`, `usercommands.go:484`, `rally.go:30`, `warcry.go:31`
- Test: `copy_no_dash_test.go`

`rally.go` and `warcry.go` lines are keyed by line number in `condition_apply_path_guard_test.go`; every edit here replaces one line in place, so no line moves.

- [ ] **Step 1: Widen the guard first (the failing test).** In `copy_no_dash_test.go`, after `"internal/hooks/NewRound_DoCombat_helpers.go",` add:

```go
	// #449, the sight-gates follow-ups.
	"internal/usercommands/loot.go",
	"internal/usercommands/go.go",
	"internal/usercommands/usercommands.go",
	"internal/usercommands/rally.go",
	"internal/usercommands/warcry.go",
	"internal/hooks/spell_resolution.go",
```

- [ ] **Step 2: Run and see it fail.**

Run: `go test . -run TestCopyFilesHaveNoDashesInStringLiterals -count=1`
Expected: FAIL naming `loot.go:170`, `go.go:132`, `usercommands.go:484`, `rally.go:30`, `warcry.go:31` (and `spell_resolution.go:222` only if Task 5 has not run yet).

- [ ] **Step 3: Fix the five lines.** Each is one string literal; replace the whole literal on that line, and nothing else on the line:

| File:line | Literal starts | New literal |
|---|---|---|
| `internal/usercommands/loot.go:170` | `"There's nothing to pass` | `"There's nothing to pass. Loot passing only applies to round-robin looting."` |
| `internal/usercommands/go.go:132` | `"You can't do that` | `"You can't do that while you're dead. Hold on; you'll be pulled back to safety."` |
| `internal/usercommands/usercommands.go:484` | `"You can't do that` | `"You can't do that while you're dead. Hold on; you'll be pulled back to safety."` |
| `internal/usercommands/rally.go:30` | `"You're already rallied` | `"You're already rallied. Save it for when it matters."` |
| `internal/usercommands/warcry.go:31` | `"Your warcry still echoes` | `"Your warcry still echoes. You can't shout it louder."` |

Every other dash in these files is in a comment, which the guard does not read.

- [ ] **Step 4: Run.**

Run: `go test . -run 'TestCopyFilesHaveNoDashesInStringLiterals|TestPlayerConditionsTravelTheEventPath' -count=1` and `go test ./internal/usercommands/ -count=1`
Expected: PASS.

- [ ] **Step 5: The login flash, built from the HEAD blob.** `_datafiles/config.yaml` carries the skip-worktree bit in the owner's checkout and can drift from HEAD in both directions, so the commit is built from the committed blob, never from disk.

```bash
cd /c/tmp/dogmud-sight-followups
git ls-files -v _datafiles/config.yaml        # record the letter: H in this worktree today, S in the main checkout
SCRATCH="$TMP"                                 # your session scratchpad, never C:/tmp
git show HEAD:_datafiles/config.yaml > "$SCRATCH/config.head.yaml"
sed -n 74p "$SCRATCH/config.head.yaml"         # the tagged emote line
```

With the Edit tool, in `$SCRATCH/config.head.yaml` replace line 74 (the `- 'emote @appears before you in a flash of <ansi fg="yellow-bold">...` line) with exactly:

```yaml
  - 'emote @appears before you in a flash of ⚡lightning⚡!'
```

Then:

```bash
diff <(git show HEAD:_datafiles/config.yaml) "$SCRATCH/config.head.yaml"   # exactly one line, 74c74
git update-index --no-skip-worktree _datafiles/config.yaml
cp "$SCRATCH/config.head.yaml" _datafiles/config.yaml
git add _datafiles/config.yaml
git diff --cached --stat -- _datafiles/config.yaml                          # 1 file changed, 1 insertion(+), 1 deletion(-)
git diff --cached -- _datafiles/config.yaml | grep '^[-+]  - '              # the old tagged line out, the plain line in
```

Expected: the cached diff is that one line and nothing else (no line-ending churn).

- [ ] **Step 6: Commit, then restore the bit.**

```bash
git add internal/usercommands/loot.go internal/usercommands/go.go internal/usercommands/usercommands.go internal/usercommands/rally.go internal/usercommands/warcry.go copy_no_dash_test.go
git commit -m "fix(copy): plain login flash; no dashes in the loot, dead, rally and warcry lines (#449)

The emote path escapes ANSI tags, so the tagged login flash printed its
markup. Built from the HEAD blob of config.yaml.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml        # must print: S _datafiles/config.yaml
git show HEAD:_datafiles/config.yaml | sed -n 74p
```

Expected: `S _datafiles/config.yaml`, and line 74 of HEAD is the plain emote.

**Post-merge note (not a task step):** after the PR merges, the owner's main-checkout disk copy `C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/config.yaml` needs the same one-line change at its own `OnLoginCommands` emote line, made with the Edit tool on that file; its `S` bit stays set. The production config is the owner's to update at deploy.

---

### Task 8: G6c and G6d, special-move personal lines and the quit line wrap (#449)

**Files:**
- Modify: `internal/usercommands/{bash,drain,gore,kick,maul,pounce,rake,throttle,trip,grapple,throw,move_narration}.go`, `internal/mobcommands/throttle.go:85`, `internal/messaging/pipeline.go:146-149`
- Test: `internal/usercommands/special_move_categories_test.go` (new), `internal/messaging/pipeline_test.go`
- Docs: `internal/messaging/context.md:27`

- [ ] **Step 1: Write the failing tests.** Create `internal/usercommands/special_move_categories_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

// #449: a special move sent the player's own lines as CategorySystem, which
// never wraps (messaging.shouldWrap), while the same command's defence lines
// already rode the move's category. Every role now takes the move's
// category, so a long kick line wraps and colours like the room's.
func TestSpecialMovePersonalLinesRideTheMoveCategory(t *testing.T) {
	cases := map[string]moveCategories{
		"bash": bashCategories, "drain": drainCategories, "gore": goreCategories,
		"grapple": grappleCategories, "kick": kickCategories, "maul": maulCategories,
		"pounce": pounceCategories, "rake": rakeCategories, "throttle": throttleCategories,
		"throttle cast interrupt": throttleCastInterruptCategories,
		"throw cast interrupt":    throwCastInterruptCategories,
		"trip":                    tripCategories,
	}
	for name, cats := range cases {
		require.NotEqual(t, messaging.CategorySystem, cats.Observer, "%s: fixture", name)
		require.Equal(t, cats.Observer, cats.Actor, "%s: the actor's own line must ride the move's category", name)
		require.Equal(t, cats.Observer, cats.Actee, "%s: the actee's line must ride the move's category", name)
	}
}

var specialMoveTagPattern = regexp.MustCompile(`<[^>]*>`)

func TestKickActorLineWrapsAt80(t *testing.T) {
	long := `You plant your feet and drive a brutal kick into the goblin's knee, ` +
		`and it buckles sideways with a wet crack that echoes off the walls.`
	require.Greater(t, utf8.RuneCountInString(long), 80, "fixture: the line must be over 80")

	out := messaging.RenderForRecipient(messaging.RenderInput{
		Category: kickCategories.Actor, Text: long, Channel: messaging.ChannelAudio, LineWidth: 80,
	})

	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), 1, "a long kick line must wrap: %q", out)
	for _, line := range lines {
		visible := specialMoveTagPattern.ReplaceAllString(line, "")
		require.LessOrEqual(t, utf8.RuneCountInString(visible), 80, "line over 80: %q", visible)
	}
}
```

In `internal/messaging/pipeline_test.go`: add `CategoryLogout:          true,` after `CategoryTip:             true,` in `wrapAllowlist`; in `TestShouldWrapMatchesPinnedAllowlist` change `admitted != 45` to `admitted != 46`, the message `expected exactly 45 categories admitted to wrap` to `expected exactly 46 categories admitted to wrap`, and the doc comment's `the admitted count is exactly 45` to `the admitted count is exactly 46`.

- [ ] **Step 2: Run and see them fail.**

Run: `go test ./internal/usercommands/ -run 'TestSpecialMovePersonalLinesRideTheMoveCategory|TestKickActorLineWrapsAt80' -count=1` and `go test ./internal/messaging/ -run TestShouldWrapMatchesPinnedAllowlist -count=1`
Expected: FAIL in all three.

- [ ] **Step 3: The category vars.** Replace each var and its leading comment (bash and trip keep their "Kept on ONE line" paragraph unchanged below the new first two lines):

`internal/usercommands/bash.go:16-17,23`:
```go
// bashCategories: every role rides CategoryBash, the player's own lines
// included, so they wrap and colour like the room's (#449).
//
// Kept on ONE line (rather than the more usual one-field-per-line struct
// literal): send_trio_only_guard_test.go scans line by line for a guarded
// category (Kick/Trip/Bash) paired with a recognised producer shape on that
// SAME line, and moveCategories{...} is one of those shapes.
var bashCategories = moveCategories{Actor: messaging.CategoryBash, Actee: messaging.CategoryBash, Observer: messaging.CategoryBash}
```

`internal/usercommands/kick.go:17-19`:
```go
// kickCategories: every role rides CategoryKick, the player's own lines
// included, so they wrap and colour like the room's (#449).
var kickCategories = moveCategories{Actor: messaging.CategoryKick, Actee: messaging.CategoryKick, Observer: messaging.CategoryKick}
```

`internal/usercommands/trip.go:17-18,24`:
```go
// tripCategories: every role rides CategoryTrip, the player's own lines
// included, so they wrap and colour like the room's (#449).
//
// Kept on ONE line (rather than the more usual one-field-per-line struct
// literal): send_trio_only_guard_test.go scans line by line for a guarded
// category (Kick/Trip/Bash) paired with a recognised producer shape on that
// SAME line, and moveCategories{...} is one of those shapes.
var tripCategories = moveCategories{Actor: messaging.CategoryTrip, Actee: messaging.CategoryTrip, Observer: messaging.CategoryTrip}
```

`drain.go`, `gore.go`, `maul.go`, `pounce.go`, `rake.go`, `throttle.go` lines 16-19 (shown for drain; for the others change only the var name in both places):
```go
// drainCategories: every role rides CategoryHitNaturalSharp, the room's
// pre-migration category, the player's own lines included, so they wrap
// and colour like the room's (#449).
var drainCategories = moveCategories{Actor: messaging.CategoryHitNaturalSharp, Actee: messaging.CategoryHitNaturalSharp, Observer: messaging.CategoryHitNaturalSharp}
```

`internal/usercommands/throttle.go:30` (keep the comment above it):
```go
var throttleCastInterruptCategories = moveCategories{Actor: messaging.CategorySpellDisruption, Actee: messaging.CategorySpellDisruption, Observer: messaging.CategorySpellDisruption,
```

`internal/usercommands/grapple.go:15-21`:
```go
// grappleCategories: every role rides CategoryGrappleFlow, so the player's
// own lines wrap and colour like the room's (#449).
var grappleCategories = moveCategories{
	Actor:    messaging.CategoryGrappleFlow,
	Actee:    messaging.CategoryGrappleFlow,
	Observer: messaging.CategoryGrappleFlow,
}
```

- [ ] **Step 4: The hand-built trios.** In each partial block, replace `lineOrNone(messaging.CategorySystem, roles.Actor)` with `lineOrNone(<verb>Categories.Actor, roles.Actor)` and `lineOrNone(messaging.CategorySystem, roles.Actee)` with `lineOrNone(<verb>Categories.Actee, roles.Actee)`, where `<verb>` is the file's verb: `bash.go:113-114`, `drain.go:109-110`, `gore.go:101-102`, `kick.go:132-133`, `maul.go:97-98`, `pounce.go:108-109`, `rake.go:97-98`, `throttle.go:119-120`, `trip.go:139-140`. For example `kick.go:132-133` becomes:

```go
			Actor:    lineOrNone(kickCategories.Actor, roles.Actor),
			Actee:    lineOrNone(kickCategories.Actee, roles.Actee),
```

`grapple.go:146-147` and `:162-163`:

```go
				Actor:    messaging.Say(grappleCategories.Actor, result.DisarmResult.Message),
				Actee:    messaging.Say(grappleCategories.Actee, result.DisarmResult.TargetMsg),
```
```go
				Actor:    messaging.Say(grappleCategories.Actor, result.CritFailure.Message),
				Actee:    messaging.Say(grappleCategories.Actee, result.CritFailure.TargetMessage),
```

`throw.go:334` and `:366`: `moveCategories{Actor: messaging.CategorySystem, Observer: messaging.CategoryHitRanged}` becomes `moveCategories{Actor: messaging.CategoryHitRanged, Observer: messaging.CategoryHitRanged}`. `throw.go:446`: `lineOrNone(messaging.CategorySystem, partialRoles.Actor)` becomes `lineOrNone(messaging.CategoryHitRanged, partialRoles.Actor)`.

`internal/mobcommands/throttle.go:85`: `messaging.CategorySystem` becomes `messaging.CategorySpellDisruption` (the actee's cast-interrupt line, sibling of the player's `throttleCastInterruptCategories`).

Confirm nothing is left (run each on its own line, zero matches expected):

```bash
grep -n "lineOrNone(messaging.CategorySystem" internal/usercommands/*.go
grep -n "Actor: messaging.CategorySystem" internal/usercommands/*.go
```

- [ ] **Step 5: The `moveCategories` doc.** In `internal/usercommands/move_narration.go`, replace the paragraph after `// moveCategories lets each rendered role ride its own messaging.Category.` (the one starting `// The player-side pre-migration call sites split their categories by`, through `// category throughout.`) with:

```go
//
// Each special move's personal lines ride the move's own category, the same
// as its room line (#449): CategorySystem never wraps (messaging.shouldWrap),
// so a long actor or actee line ran past 80 columns while the command's
// defence lines, already on the move category, wrapped. A Category decides a
// line's colour (messaging/pipeline.go applyCategoryColor) and whether it
// wraps; combat verbosity (messaging.Verbosity.Suppresses) is applied only to
// the round's combat drains in internal/hooks, never to SendTrio's seats, so
// a light-verbosity player still reads their own move. The struct stays
// because the roles can still differ (throw's actee-less events,
// RemoteObserver). sameMoveCategory below covers a call site whose lines
// share one category throughout.
```

- [ ] **Step 6: `CategoryLogout` wraps.** In `internal/messaging/pipeline.go`, change the last case line of `shouldWrap` (line 148) to:

```go
		CategoryConditionExpire, CategoryMutation, CategoryTip,
		// The quit line (#449): one sentence of narration to the room.
		CategoryLogout:
```

In `internal/messaging/context.md` line 27 change `45 of the 62` to `46 of the 62`.

- [ ] **Step 7: Run.**

Run: `go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/messaging/ ./internal/hooks/ -count=1` then `go test . -run 'TestNarrationTrioOnlyCategoriesLeaveOnlyThroughSendTrio|TestNarrationSitesMatchViewpointAudit|TestEveryTrioLiteralNamesAllThreeRoles|TestMigratedFilesHoldNoNarrationLiterals' -count=1`
Expected: PASS. If a test asserting a long special-move line fails only because the line now folds at 80, compare against `strings.Join(strings.Fields(got), " ")` rather than weakening the assertion; none did in the dry run.

- [ ] **Step 8: Commit.**

```bash
git add internal/usercommands/bash.go internal/usercommands/drain.go internal/usercommands/gore.go internal/usercommands/kick.go internal/usercommands/maul.go internal/usercommands/pounce.go internal/usercommands/rake.go internal/usercommands/throttle.go internal/usercommands/trip.go internal/usercommands/grapple.go internal/usercommands/throw.go internal/usercommands/move_narration.go internal/usercommands/special_move_categories_test.go internal/mobcommands/throttle.go internal/messaging/pipeline.go internal/messaging/pipeline_test.go internal/messaging/context.md
git commit -m "fix(messaging): special-move personal lines and the quit line wrap (#449)

Actor and actee lines ride the move's category like its room line;
CategoryLogout joins shouldWrap.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: G7a, the hide's data and a silent record drop (#444)

**Files:**
- Modify: `internal/state/awareness/awareness.go:51-54`, after `:183`; `internal/state/awareness/transitions.go:37`; `internal/conditions/ids.go:8-9`; `internal/conditions/conditions.go` (before `TriggersLeft`, line 139)
- Test: `internal/state/awareness/awareness_test.go` (append), `internal/conditions/discard_test.go` (new)
- Docs: `internal/state/awareness/context.md`, `internal/conditions/context.md`

- [ ] **Step 1: Write the failing tests.** Append to `internal/state/awareness/awareness_test.go`:

```go

// --- Hide source (Empathic Shroud as a real hide, #444) ---

// ResolveConcealmentAs stores the hide's data before the move to Hidden, so
// the Hidden cascade can tell a shroud from a sneak.
func TestResolveConcealmentAs_DataVisibleToTheHiddenCascade(t *testing.T) {
	A, _ := makePair()
	var seen HiddenData
	var seenOk bool
	A.Inner().AfterTransition("test_hidden_data", func(from, to State, r state.TransitionReason) {
		if to == Hidden {
			seen, seenOk = A.HiddenData()
		}
	})
	require.NoError(t, A.TransitionToConcealing(ConcealingData{}, state.TransitionReason{}))
	A.ResolveConcealmentAs(HiddenData{Source: HideShroud, Score: 180}, state.TransitionReason{})

	require.Equal(t, Hidden, A.State())
	require.True(t, seenOk, "the Hidden cascade must read the hide's data")
	require.Equal(t, HiddenData{Source: HideShroud, Score: 180}, seen)
	got, ok := A.HiddenData()
	require.True(t, ok)
	require.Equal(t, HideShroud, got.Source)
}

// A plain ResolveConcealment hides as a sneak; HiddenData is gone once the
// hide ends, and SetHiddenData only acts while Hidden.
func TestHiddenData_SneakDefaultSwapAndReveal(t *testing.T) {
	A, _ := makePair()
	_, ok := A.HiddenData()
	require.False(t, ok, "a Visible machine has no hide")
	A.SetHiddenData(HiddenData{Source: HideShroud, Score: 50})
	_, ok = A.HiddenData()
	require.False(t, ok, "SetHiddenData must not act outside Hidden")

	require.NoError(t, A.TransitionToConcealing(ConcealingData{}, state.TransitionReason{}))
	A.ResolveConcealment(true, state.TransitionReason{})
	got, ok := A.HiddenData()
	require.True(t, ok)
	require.Equal(t, HideSneak, got.Source)

	A.SetHiddenData(HiddenData{Source: HideShroud, Score: 90})
	got, _ = A.HiddenData()
	require.Equal(t, HiddenData{Source: HideShroud, Score: 90}, got)

	require.NoError(t, A.TransitionToRevealing(state.TransitionReason{Trigger: TriggerShroudEnded}))
	_, ok = A.HiddenData()
	require.False(t, ok, "a revealed machine has no hide")
}
```

Create `internal/conditions/discard_test.go`:

```go
package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Discard deletes a record outright, so the prune pass never narrates its end
// (#444: one hide handing over to another without a reveal). RemoveCondition,
// by contrast, leaves an expired record for the prune.
func TestConditions_DiscardLeavesNothingToPrune(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(100, false) // Haste
	bs.AddCondition(102, false) // Shadow Cloak, Hidden
	require.True(t, bs.HasFlag(Hidden, false))

	require.True(t, bs.Discard(102))
	assert.False(t, bs.HasCondition(102), "a discarded record is gone, not expired")
	assert.False(t, bs.HasFlag(Hidden, false), "the flag lookup was rebuilt")
	assert.True(t, bs.HasFlag(Haste, false), "the other record keeps its flag")
	assert.Empty(t, bs.Prune(), "nothing is left for the prune pass to narrate")
	assert.False(t, bs.Discard(102), "a second discard finds nothing")

	bs.AddCondition(102, false)
	require.True(t, bs.RemoveCondition(102))
	pruned := bs.Prune()
	require.Len(t, pruned, 1, "RemoveCondition still goes through the prune")
	assert.Equal(t, 102, pruned[0].ConditionId)
}
```

- [ ] **Step 2: Run and see them fail.**

Run: `go test ./internal/state/awareness/ ./internal/conditions/ -count=1`
Expected: build FAIL: `undefined: HideShroud`, `A.ResolveConcealmentAs undefined`, `bs.Discard undefined`.

- [ ] **Step 3: Implement the awareness data.** In `awareness.go`, replace the `HiddenData` comment and `type HiddenData struct{}` (lines 51-54) with:

```go
// HideSource says what keeps a Hidden character hidden. Empathic Shroud is a
// real hide (#444, owner 2026-10-09), and only one hide holds at a time: the
// stronger of the two.
type HideSource uint8

const (
	// HideSneak is the default: the sneak command, or a record 9 from a
	// mob's spawn-time conditionids or an admin. It hides at the holder's
	// Dexterity plus Skullduggery x SkillWeight.
	HideSneak HideSource = iota
	// HideShroud is Empathic Shroud (condition 31). It hides at Score: the
	// caster's spell stat plus Spellcasting x SkillWeight.
	HideShroud
)

// HiddenData carries hidden-state metadata: what is hiding the character,
// and for a shroud the score it hides at.
type HiddenData struct {
	Source HideSource
	// Score is a shroud hide's base score, in place of Dexterity plus
	// Skullduggery x SkillWeight. Unused for HideSneak.
	Score float64
}
```

After the closing brace of `ResolveConcealment`, add:

```go

// ResolveConcealmentAs is ResolveConcealment(true, r) for a hide whose data
// matters. d is stored BEFORE the move to Hidden, so the Hidden cascade
// (internal/hooks/Awareness_Cascades.go) can read what is hiding the
// character. No-op if not currently Concealing.
func (m *Machine) ResolveConcealmentAs(d HiddenData, r state.TransitionReason) {
	if m.State() != Concealing {
		return
	}
	m.hidden = &d
	if err := m.inner.TransitionTo(Hidden, r); err != nil {
		m.hidden = nil
		return
	}
	m.concealing = nil
}

// HiddenData returns the hide's data while Hidden. ok is false in any other
// state, and inside ResolveConcealment's own Hidden cascade, which runs
// before that call stores its data; a caller reads a missing value as a
// sneak.
func (m *Machine) HiddenData() (HiddenData, bool) {
	if m.State() != Hidden || m.hidden == nil {
		return HiddenData{}, false
	}
	return *m.hidden, true
}

// SetHiddenData replaces the hide's data while Hidden, for one hide taking
// over from another without a reveal. A no-op in any other state.
func (m *Machine) SetHiddenData(d HiddenData) {
	if m.State() != Hidden {
		return
	}
	m.hidden = &d
}
```

In `transitions.go`, after `TriggerConditionApplied = "condition_applied"` add:

```go
	// TriggerShroudEnded is a shroud hide ending because no live Empathic
	// Shroud record (condition 31) remains: it ran out, was purged or
	// removed. Character.Validate reveals it.
	TriggerShroudEnded = "shroud_ended"
```

- [ ] **Step 4: Implement the id and `Discard`.** In `internal/conditions/ids.go`, make the first entries of the `const` block:

```go
const (
	// ConditionIdEmpathicShroud is the Empathic Shroud spell's hide. It is a
	// real hide (#444): characters.hideForStealthRecord enters Hidden on it.
	ConditionIdEmpathicShroud    = 31
	ConditionIdWarcry            = 79
```

In `internal/conditions/conditions.go`, add before `func (bs *Conditions) TriggersLeft(`:

```go
// Discard deletes a held record outright. Unlike RemoveCondition, which only
// marks it expired, the prune pass never sees it, so no end line is told. It
// is for one hide handing over to another without a reveal (Empathic Shroud
// and the sneak's record 9, characters.hideForStealthRecord), where the end
// line ("emerges from the shadows") would tell the room something that did
// not happen. Reports whether a record was held.
func (bs *Conditions) Discard(conditionId int) bool {
	idx, ok := bs.conditionIds[conditionId]
	if !ok {
		return false
	}
	bs.List = append(bs.List[:idx], bs.List[idx+1:]...)
	bs.Validate(true)
	return true
}
```

- [ ] **Step 5: Run.**

Run: `go test ./internal/state/awareness/ ./internal/conditions/ -count=1` and `go build ./...`
Expected: PASS; the build is clean.

- [ ] **Step 6: Docs.** In `internal/state/awareness/context.md`:
  - In "Per-state data structs", replace the `type HiddenData struct{}` entry and its two comment lines with:

```go
type HideSource uint8 // HideSneak (default) or HideShroud (#444)

type HiddenData struct {
    Source HideSource // what keeps the character hidden
    Score  float64    // a shroud's base score; unused for a sneak
}
```

  - After the "Detection resolution" section add:

```markdown
### Hide data (#444)

```go
func (m *Machine) ResolveConcealmentAs(d HiddenData, r state.TransitionReason)
func (m *Machine) HiddenData() (HiddenData, bool)
func (m *Machine) SetHiddenData(d HiddenData)
```

`ResolveConcealmentAs` is `ResolveConcealment(true, r)` that stores `d`
BEFORE the move to Hidden, so the Hidden cascade
(`hooks/Awareness_Cascades.go`) can read it: a shroud hide (`HideShroud`)
carries record 31 and no record 9. `HiddenData` reports the hide's data only
while Hidden (inside a plain `ResolveConcealment`'s cascade it is not yet
stored; read that as a sneak). `SetHiddenData` swaps it while Hidden, for one
hide taking over from another without a reveal
(`characters.SneakOverShroud`, `characters.hideForStealthRecord`).
```

  - Add two rows to the trigger table: `| \`TriggerConditionApplied\` | \`"condition_applied"\` |` and `| \`TriggerShroudEnded\` | \`"shroud_ended"\` |`.

In `internal/conditions/context.md`, change `` (`ConditionIdWarcry` through `` to `` (`ConditionIdEmpathicShroud`, then `ConditionIdWarcry` through ``, and append to "Facts worth knowing":

```markdown
- **`Discard` drops a record with no end line.** `RemoveCondition` only marks
  a record expired, and the prune pass then narrates its end. `Discard`
  deletes it outright and rebuilds the lookups, so nothing is told. It exists
  for one hide handing over to another (#444): the loser's record (9 or 31)
  goes without telling the room of a reveal that did not happen.
```

- [ ] **Step 7: Commit.**

```bash
git add internal/state/awareness/awareness.go internal/state/awareness/transitions.go internal/state/awareness/awareness_test.go internal/state/awareness/context.md internal/conditions/ids.go internal/conditions/conditions.go internal/conditions/discard_test.go internal/conditions/context.md
git commit -m "feat(awareness): hide data says sneak or shroud; Conditions.Discard (#444)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: G7b, record 31 hides, the stronger hide holds, and the end reveals (#444)

**Files:**
- Create: `internal/characters/shroud_hide.go`
- Modify: `internal/characters/conditions.go:152-161`, `:167-192`, `:202-211`, `:223-234`; `internal/characters/validate.go:695`
- Test: `internal/characters/shroud_hide_test.go` (new)
- Docs: `internal/characters/context.md` (section "Cascade pattern: Awareness to Condition #9", line 1424)

- [ ] **Step 1: Write the failing tests.** Create `internal/characters/shroud_hide_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09). The Awareness
// cascade that adds record 9 lives in internal/hooks and is not wired here,
// so these tests see exactly what this package does.

// seedStealthRecords seeds the two stealth records with their shipped flags.
func seedStealthRecords(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
}

// newHider is a visible character with a bare Awareness machine and a known
// sneak base: Dexterity dex, Skullduggery 0. Stats are set on Base, because
// every add door runs Validate, which recalculates ValueAdj from Base.
func newHider(dex int) *Character {
	c := New()
	c.Awareness = awareness.NewMachine()
	c.Stats.Dexterity.Base = dex
	_ = c.Validate()
	return c
}

// sneakHide hides c as a sneak, the way the sneak command and its cascade
// leave it: Hidden as a sneak, holding a permanent record 9.
func sneakHide(t *testing.T, c *Character) {
	t.Helper()
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	c.Awareness.ResolveConcealment(true, r)
	require.NoError(t, c.AddCondition(9, true))
	require.True(t, c.IsHidden())
	require.False(t, c.HiddenByShroud())
}

func TestShroudRecord_EntersHiddenAtTheRecordScore(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
	require.True(t, c.IsHidden(), "record 31 must hide a visible holder")
	require.True(t, c.HiddenByShroud())
	require.Equal(t, 180.0, c.HideBaseScore(), "the shroud hides at the caster's score, not Dex plus Skullduggery")
	sw := float64(configs.GetBalanceConfig().SkillWeight)
	require.Equal(t, 50+float64(c.GetSkillLevel(skills.Skullduggery))*sw, c.SneakBaseScore())
}

func TestShroudRecord_NoMagnitudeUsesTheHoldersOwnScore(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	c.Stats.Willpower.Base = 70
	c.SetSkill("spellcasting", 4)
	require.NoError(t, c.Validate())
	require.NoError(t, c.AddCondition(conditions.ConditionIdEmpathicShroud, false))
	require.True(t, c.HiddenByShroud())
	want := 70 + 4*float64(configs.GetBalanceConfig().SkillWeight)
	require.Equal(t, want, c.HideBaseScore())
	require.Equal(t, want, c.OwnShroudScore())
}

// Ending: once no live 31 remains, Validate reveals the shroud hide. A sneak
// hide with no 31 is left alone.
func TestShroudHide_EndsWhenNoLiveRecordRemains(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
	require.True(t, c.IsHidden())

	c.RemoveCondition(conditions.ConditionIdEmpathicShroud) // expires it, then validates
	require.False(t, c.IsHidden(), "a shroud hide must end with its record")

	sneaker := newHider(50)
	sneakHide(t, sneaker)
	require.NoError(t, sneaker.Validate())
	require.True(t, sneaker.IsHidden(), "a sneak hide needs no record 31")
}

// One hide at a time, the stronger (owner, 2026-10-09): the shroud landing on
// a sneak hide.
func TestShroudLanding_OnASneakHide(t *testing.T) {
	t.Run("stronger shroud replaces the sneak", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		sneakHide(t, c)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 200, "spell"))
		require.True(t, c.IsHidden(), "the hide never lapses")
		require.True(t, c.HiddenByShroud())
		require.Equal(t, 200.0, c.HideBaseScore())
		require.False(t, c.Conditions.HasCondition(9), "record 9 is discarded, not cancelled, so no end line")
	})
	t.Run("weaker shroud is dropped", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		sneakHide(t, c)
		require.Error(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"),
			"a dropped shroud is refused so no start line is told")
		require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud))
		require.True(t, c.IsHidden())
		require.False(t, c.HiddenByShroud())
		require.True(t, c.holdsLiveStealthRecord(), "the sneak keeps its record 9")
	})
	t.Run("a tie keeps the sneak already there", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(100)
		sneakHide(t, c)
		require.Error(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, c.SneakBaseScore(), "spell"))
		require.False(t, c.HiddenByShroud())
	})
}

// The other direction through the record door: a 9 landing on a shroud hide.
func TestStealthRecordLanding_OnAShroudHide(t *testing.T) {
	t.Run("stronger sneak base takes over", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
		require.NoError(t, c.AddCondition(9, true))
		require.True(t, c.IsHidden())
		require.False(t, c.HiddenByShroud())
		require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, not cancelled")
		require.NoError(t, c.Validate())
		require.True(t, c.IsHidden(), "with 31 gone the sneak hide holds")
	})
	t.Run("weaker sneak base is dropped", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 200, "spell"))
		require.Error(t, c.AddCondition(9, true))
		require.True(t, c.HiddenByShroud())
		require.False(t, c.Conditions.HasCondition(9))
	})
}

func TestSneakOverShroud_SwapsWithoutAReveal(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(300)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
	require.True(t, c.SneakOverShroud())
	require.True(t, c.IsHidden(), "no reveal")
	require.False(t, c.HiddenByShroud())
	require.Equal(t, c.SneakBaseScore(), c.HideBaseScore(), "the hide now scores as a sneak")
	require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, so its end line is not told")
	require.True(t, c.holdsLiveStealthRecord(), "record 9 carries the sneak hide")
	require.False(t, c.SneakOverShroud(), "nothing left to take over")
}

// A re-cast while shroud-hidden reads the new record's score.
func TestShroudRecast_TakesTheNewScore(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 120, "spell"))
	require.True(t, c.HiddenByShroud())
	require.Equal(t, 120.0, c.HideBaseScore())
}
```

- [ ] **Step 2: Run and see it fail.**

Run: `go test ./internal/characters/ -run 'Shroud|StealthRecordLanding|SneakOverShroud' -count=1`
Expected: build FAIL: `c.HiddenByShroud undefined` and the other new methods.

- [ ] **Step 3: Create `internal/characters/shroud_hide.go`:**

```go
package characters

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Empathic Shroud is a real hide (#444, owner ruling 2026-10-09): "a real
// hide/sneak with the spellcasting rank and stat replacing dex +
// skullduggery for the future opposed rolls." Only one hide holds at a time,
// the stronger: "Only one or the other should exist (the strongest)."
//
// A shroud hide carries record 31 and no record 9 (Awareness_Cascades.go
// skips the 9 for it), so its end is told once, by 31's own end line.

// shroudSpellId is the spell whose stat a shroud with no magnitude (an admin
// setcondition) reads for the holder's own score.
const shroudSpellId = "empathic-shroud"

// ShroudScore is an Empathic Shroud hide score: the spell's stat plus
// Spellcasting rank x SkillWeight. The spell hook stamps the CASTER's score
// on the record as its magnitude (hooks.applySpellCondition).
func ShroudScore(stat, spellcastingRank int) float64 {
	return float64(stat) + float64(spellcastingRank)*float64(configs.GetBalanceConfig().SkillWeight)
}

// OwnShroudScore is the score c would cast the shroud at: the spell's stat
// (willpower as shipped; willpower too if the spell is not loaded) plus c's
// Spellcasting rank x SkillWeight.
func (c *Character) OwnShroudScore() float64 {
	stat := c.Stats.Willpower.ValueAdj
	if sd := spells.GetSpell(shroudSpellId); sd != nil {
		stat = sd.CasterStatValue(c.Stats)
	}
	return ShroudScore(stat, c.GetSkillLevel(skills.Spellcasting))
}

// SneakBaseScore is the base of a sneak hide's score: Dexterity plus
// Skullduggery rank x SkillWeight. Mutation stealth and the light modifiers
// apply on top in actions.CalcSneakScore, to a sneak and a shroud alike, so
// the stronger-hide comparison leaves them out.
func (c *Character) SneakBaseScore() float64 {
	return float64(c.Stats.Dexterity.ValueAdj) +
		float64(c.GetSkillLevel(skills.Skullduggery))*float64(configs.GetBalanceConfig().SkillWeight)
}

// HideBaseScore is the base score c hides at in every opposed roll
// (actions.CalcSneakScore): the shroud's score while the shroud hides c,
// SneakBaseScore otherwise.
func (c *Character) HideBaseScore() float64 {
	if d, ok := c.hiddenData(); ok && d.Source == awareness.HideShroud {
		return d.Score
	}
	return c.SneakBaseScore()
}

// HiddenByShroud reports whether Empathic Shroud, not a sneak, is what keeps
// c hidden.
func (c *Character) HiddenByShroud() bool {
	d, ok := c.hiddenData()
	return ok && d.Source == awareness.HideShroud
}

// SneakOverShroud turns a shroud hide into a sneak hide with no reveal and no
// roll (the holder is already hidden): record 31 is discarded without its end
// line, the hide becomes a sneak, and record 9 carries it from here. The
// caller decides the sneak is the stronger and charges the sneak's cost
// (actions.Sneak). Reports whether there was a shroud hide to take over.
func (c *Character) SneakOverShroud() bool {
	if !c.HiddenByShroud() {
		return false
	}
	c.Conditions.Discard(conditions.ConditionIdEmpathicShroud)
	c.Awareness.SetHiddenData(awareness.HiddenData{Source: awareness.HideSneak})
	_ = c.AddCondition(conditionIdHidden, true)
	return true
}

func (c *Character) hiddenData() (awareness.HiddenData, bool) {
	if c.Awareness == nil {
		return awareness.HiddenData{}, false
	}
	return c.Awareness.HiddenData()
}

// holdsLiveShroudRecord reports a live (unexpired) record 31. HasCondition
// cannot answer this: it also counts an expired record the prune pass has not
// yet removed.
func (c *Character) holdsLiveShroudRecord() bool {
	return len(c.Conditions.GetConditions(conditions.ConditionIdEmpathicShroud)) > 0
}

// shroudRecordScore is the score the held record 31 hides at: its magnitude
// (the caster's score, stamped by the spell), or the holder's own score for a
// record with none.
func (c *Character) shroudRecordScore() float64 {
	for _, rec := range c.Conditions.GetConditions(conditions.ConditionIdEmpathicShroud) {
		if rec.Magnitude > 0 {
			return rec.Magnitude
		}
	}
	return c.OwnShroudScore()
}

// hideScoreOf is the base score a hide holds c at.
func (c *Character) hideScoreOf(d awareness.HiddenData) float64 {
	if d.Source == awareness.HideShroud {
		return d.Score
	}
	return c.SneakBaseScore()
}

// settleHide resolves a stealth record landing on a character already
// Hidden. The same kind keeps the hide (a re-cast shroud takes the new
// record's score). A different kind: the stronger base score holds and the
// incumbent wins a tie. The loser's record is discarded, not cancelled, so no
// end line tells the room of a reveal that did not happen. refused reports
// that the incoming record was the one dropped.
func (c *Character) settleHide(conditionId int, incoming awareness.HiddenData) (refused bool) {
	held, _ := c.Awareness.HiddenData()
	if held.Source == incoming.Source {
		if incoming.Source == awareness.HideShroud {
			c.Awareness.SetHiddenData(incoming)
		}
		return false
	}
	if c.hideScoreOf(incoming) <= c.hideScoreOf(held) {
		c.Conditions.Discard(conditionId)
		return true
	}
	if held.Source == awareness.HideShroud {
		c.Conditions.Discard(conditions.ConditionIdEmpathicShroud)
	} else {
		c.Conditions.Discard(conditionIdHidden)
		c.RemovePermanentCondition(conditionIdHidden)
	}
	c.Awareness.SetHiddenData(incoming)
	return false
}

// reconcileShroudHide ends a shroud hide once no live record 31 remains: it
// ran out (the prune pass validates after pruning), was removed, purged or
// cancelled. Validate calls it beside reconcilePerception. The reveal runs
// the Awareness cascade, which cancels any other hidden-flag record; with no
// record left that cascade's cancel is a no-op, so this cannot recurse.
func (c *Character) reconcileShroudHide() {
	if !c.HiddenByShroud() || c.holdsLiveShroudRecord() {
		return
	}
	_ = c.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerShroudEnded})
}
```

- [ ] **Step 4: `hideForStealthRecord` handles both records.** In `internal/characters/conditions.go`, replace the doc comment and body of `hideForStealthRecord` (lines 167-192) with:

```go
// hideForStealthRecord is the other half of that mirror, for both stealth
// records. Record 9 (a mob's spawn-time conditionids, an admin setcondition)
// or record 31 (Empathic Shroud, #444) added to a Visible character drives
// Awareness into Hidden, so IsHidden agrees with the record (sight gates
// playtest fixes, F5). The hide's data says which: a 9 hides as a sneak, a
// 31 as a shroud at shroudRecordScore. A sneak in flight is Concealing and
// is left alone.
//
// On a character already Hidden, settleHide keeps one hide, the stronger
// (owner, 2026-10-09). The cascade's own re-add of 9 arrives while the
// machine is Hidden as a sneak, so it settles as "same kind" and is not
// driven twice.
//
// A refused hide (a busy character's Visible to Concealing is vetoed): a 9
// is taken back off, so no record 9 claims hidden on a character the machine
// kept Visible, and a permanent 9 stays in the id list and tries again at the
// next Validate(true); a 31 is discarded and reported refused.
//
// refused is true when the incoming record was dropped; the add doors then
// return an error so the event path tells no start line for it.
func (c *Character) hideForStealthRecord(conditionId int) (refused bool) {
	if c.Awareness == nil {
		return false
	}
	var incoming awareness.HiddenData
	switch conditionId {
	case conditionIdHidden:
		incoming = awareness.HiddenData{Source: awareness.HideSneak}
	case conditions.ConditionIdEmpathicShroud:
		incoming = awareness.HiddenData{Source: awareness.HideShroud, Score: c.shroudRecordScore()}
	default:
		return false
	}
	switch c.Awareness.State() {
	case awareness.Visible:
		reason := state.TransitionReason{Trigger: awareness.TriggerConditionApplied}
		if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
			if conditionId == conditionIdHidden {
				c.Conditions.RemoveCondition(conditionIdHidden)
				return false
			}
			c.Conditions.Discard(conditionId)
			return true
		}
		c.Awareness.ResolveConcealmentAs(incoming, reason)
	case awareness.Hidden:
		return c.settleHide(conditionId, incoming)
	}
	return false
}

// errStealthRecordRefused is what an add door returns when hideForStealthRecord
// dropped the record it was adding.
func errStealthRecordRefused(name string, conditionId int) error {
	return fmt.Errorf(`stealth record dropped, a stronger hide holds. target: "%s" conditionId: %d`, name, conditionId)
}
```

In each of the three doors, replace `c.hideForStealthRecord(conditionId)` with `refused := c.hideForStealthRecord(conditionId)`, and replace the door's final `return nil` with:

```go
	if refused {
		return errStealthRecordRefused(c.Name, conditionId)
	}
	return nil
```

(keep each door's `_ = c.Validate()` where it is, before this).

In `internal/characters/validate.go`, after `c.reconcilePerception()` (line 695) add:

```go
	c.reconcileShroudHide()
```

- [ ] **Step 5: Run.**

Run: `go test ./internal/characters/ -count=1` and `go vet ./internal/characters/`
Expected: PASS, including the existing `TestStealthRecord_EveryAddDoorHides` and `TestCancelCombatConditions_DrivesAwarenessOutOfHidden`.

- [ ] **Step 6: Docs.** In `internal/characters/context.md`, after the bullets of "### Cascade pattern: Awareness to Condition #9" (ending `the hook removes condition #9.`), add:

```markdown
**Empathic Shroud is a real hide (#444, `shroud_hide.go`).** Record 31 landing
on a Visible holder enters Hidden as a shroud (`hideForStealthRecord`, through
`awareness.ResolveConcealmentAs`), and the cascade adds NO record 9 for it: a
shroud hide is Hidden plus a live 31. Its score is the record's magnitude,
which the spell stamps as the CASTER's `ShroudScore(stat, spellcastingRank)`
(spell stat plus Spellcasting x `SkillWeight`); a 31 with none uses the
holder's `OwnShroudScore()`. `HideBaseScore()` is what `actions.CalcSneakScore`
starts from: the shroud's score while `HiddenByShroud()`, else
`SneakBaseScore()` (Dexterity plus Skullduggery x `SkillWeight`).

One hide at a time, the stronger (owner, 2026-10-09). A 9 or a 31 landing on a
holder already Hidden goes through `settleHide`: the same kind keeps the hide
(a re-cast 31 takes its new score), a different kind keeps the higher base
score, the incumbent winning a tie. The loser's record goes by
`Conditions.Discard`, so no end line is told; a dropped incoming record makes
its add door return an error so no start line is told either.
`SneakOverShroud()` is the sneak command's takeover. `Validate` calls
`reconcileShroudHide()` beside `reconcilePerception()`: a shroud hide with no
live 31 (expired, removed, purged, cancelled) is revealed with
`awareness.TriggerShroudEnded`. Breaking: an observer winning a roll, combat
(31 carries `cancel-on-combat`), death and logout reveal it like a sneak hide,
and the reveal cascade cancels 31.
```

- [ ] **Step 7: Commit.**

```bash
git add internal/characters/shroud_hide.go internal/characters/shroud_hide_test.go internal/characters/conditions.go internal/characters/validate.go internal/characters/context.md
git commit -m "feat(characters): Empathic Shroud hides; the stronger hide holds; Validate ends it (#444)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: G7c, the shroud score in every roll, and sneak while shrouded (#444)

**Files:**
- Modify: `internal/actions/skill_helpers.go:32-34`, `internal/actions/sneak.go:15-35`, `:135-165`; `internal/usercommands/skill.skullduggery.sneak.go:59-61`; `internal/mobcommands/sneak.go:13-15`
- Test: `internal/actions/sneak_shroud_test.go` (new), `internal/usercommands/skill_skullduggery_sneak_shroud_test.go` (new)
- Docs: `internal/actions/context.md` (Sneak section, lines 707-756)

- [ ] **Step 1: Write the failing tests.** Create `internal/actions/sneak_shroud_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09): while it hides a
// character, the spell's score replaces Dexterity plus Skullduggery in every
// opposed roll, and only one hide holds, the stronger.

func seedShroudRecords(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
}

// shroudedChar is a character the shroud hides at score, with Dexterity dex
// on Base (the add door validates, which recalculates ValueAdj from Base) and
// stamina to pay for a sneak.
func shroudedChar(t *testing.T, dex int, score float64) *characters.Character {
	t.Helper()
	c := newTestChar()
	c.Stats.Dexterity.Base = dex
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, score, "spell"))
	require.True(t, c.HiddenByShroud(), "fixture: the shroud must hide the character")
	// After the add: its Validate recalculates the pool maxima.
	c.StaminaMax.Value = 100
	c.Stamina = 100
	return c
}

func TestCalcSneakScore_ShroudScoreReplacesDexAndSkullduggery(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 40, 250)
	require.Less(t, c.SneakBaseScore(), 250.0, "fixture: the shroud must differ from the sneak base")
	assert.Equal(t, 250.0, CalcSneakScore(c, false), "dark room, no light carried: the bare base")

	plain := newTestChar()
	plain.Stats.Dexterity.Base = 40
	require.NoError(t, plain.Validate())
	assert.Equal(t, plain.SneakBaseScore(), CalcSneakScore(plain, false), "an unshrouded hider keeps Dex plus Skullduggery")
}

func TestSneak_WhileShroudedStrongerShroudIsAlreadyHidden(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 40, 250)
	stamina := c.Stamina

	result := Sneak(newStubActor(c, newTestRoom()))

	assert.True(t, result.AlreadyHidden)
	assert.False(t, result.ReplacedShroud)
	assert.True(t, c.HiddenByShroud(), "the stronger shroud stays")
	assert.Equal(t, stamina, c.Stamina, "a refusal costs nothing")
}

func TestSneak_WhileShroudedStrongerSneakTakesOver(t *testing.T) {
	seedShroudRecords(t)
	c := shroudedChar(t, 300, 100)

	result := Sneak(newStubActor(c, newTestRoom()))

	assert.True(t, result.Success)
	assert.True(t, result.ReplacedShroud)
	assert.False(t, result.AlreadyHidden)
	assert.False(t, result.RollHappened, "already hidden: no roll, so no practice")
	assert.Equal(t, characters.CostPaid, result.Cost.Status, "the normal sneak cost applies")
	assert.Positive(t, result.Cost.Charged)
	assert.True(t, c.IsHidden(), "no reveal")
	assert.False(t, c.HiddenByShroud())
	assert.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, not cancelled")
	assert.Equal(t, true, c.GetMiscData("sneaking"))
}
```

Create `internal/usercommands/skill_skullduggery_sneak_shroud_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #444: a player hidden by a weaker Empathic Shroud who sneaks takes the hide
// over and is told so; under a stronger shroud they are told they are
// already hidden, as before.
func TestSneakCommand_UnderAShroud(t *testing.T) {
	cases := []struct {
		name        string
		dex         int
		shroud      float64
		want        string
		shroudHolds bool
	}{
		{"weaker shroud: the sneak takes over", 400, 10, "You let the shroud fall away and slip into the shadows on your own.", false},
		{"stronger shroud: already hidden", 1, 9999, "You're already hidden!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
				9: {ConditionId: 9, Name: "Hidden",
					Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
				conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
					TriggerCount: 16, RoundInterval: 1,
					Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
			}))
			room := &rooms.Room{RoomId: 9950, Zone: "SneakShroud", Lamp: rooms.LampPtr(90)}
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9950: room}, map[string]*rooms.ZoneConfig{}))
			sneaker := users.NewTestUser(9951, "shrouded", "Shrouded", 0)
			sneaker.Character.Skills = map[string]int{string(skills.Skullduggery): 1}
			sneaker.Character.Stats.Dexterity.Base = tc.dex
			sneaker.Character.RoomId = room.RoomId
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9951: sneaker}))
			room.AddPlayer(9951)
			require.NoError(t, sneaker.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, tc.shroud, "spell"))
			require.True(t, sneaker.Character.HiddenByShroud(), "fixture: the shroud must hide the sneaker")
			sneaker.Character.StaminaMax.Value = 100
			sneaker.Character.Stamina = 100
			events.DrainQueuedMessagesForTest(9951)

			handled, err := Sneak("", sneaker, room, 0)
			require.True(t, handled)
			require.NoError(t, err)

			out := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(
				strings.Join(events.DrainQueuedMessagesForTest(9951), "\n"), "")
			require.Contains(t, out, tc.want)
			require.True(t, sneaker.Character.IsHidden())
			require.Equal(t, tc.shroudHolds, sneaker.Character.HiddenByShroud())
		})
	}
}
```

- [ ] **Step 2: Run and see them fail.**

Run: `go test ./internal/actions/ -run 'Shroud' -count=1` and `go test ./internal/usercommands/ -run TestSneakCommand_UnderAShroud -count=1`
Expected: build FAIL on `result.ReplacedShroud`; once that exists, the score and takeover assertions fail.

- [ ] **Step 3: `CalcSneakScore`.** In `internal/actions/skill_helpers.go`, replace the `base := ...` statement (lines 32-34) with:

```go
	// HideBaseScore is Dexterity plus Skullduggery x SkillWeight, or, while
	// Empathic Shroud hides c, the shroud's score in their place (#444).
	base := c.HideBaseScore() + mutations.GetStealthBonus(c.Mutations)
```

(`skills` stays imported; other functions in the file use it.)

- [ ] **Step 4: `actions.Sneak`.** In `internal/actions/sneak.go`, add to `SneakResult` after `AlreadyHidden bool`:

```go
	// ReplacedShroud is true when the actor was hidden by Empathic Shroud and
	// their own sneak is the stronger, so the sneak took the hide over with
	// no reveal and no roll (#444). Success is true with it, RollHappened
	// false: there was no contest, so no practice.
	ReplacedShroud bool
```

Replace the one-line comment above `if char.IsHidden() {` and that `if` block (lines 135-138) with:

```go
	// Already hidden: nothing to do, unless the shroud hides the actor and
	// their own sneak is the stronger (owner, 2026-10-09: one hide, the
	// strongest). Then the sneak takes the hide over for the normal sneak
	// cost, with no reveal and no roll.
	takeOver := false
	if char.IsHidden() {
		if !char.HiddenByShroud() || char.SneakBaseScore() <= char.HideBaseScore() {
			return SneakResult{AlreadyHidden: true}
		}
		takeOver = true
	}
```

Change the readiness check two statements below to:

```go
	if !char.IsFree() || char.Awareness == nil || (!takeOver && char.Awareness.State() != awareness.Visible) {
		return SneakResult{}
	}
```

After the existing cost refusal (`if cost.Status == characters.CostRefused { return SneakResult{Cost: cost} }`, after the one `admitFullCost` call) add:

```go
	if takeOver {
		char.SneakOverShroud()
		char.SetMiscData(`sneaking`, true)
		return SneakResult{Cost: cost, Success: true, ReplacedShroud: true}
	}
```

The takeover goes through the one `admitFullCost` call, so `TestThrowSneakCostAdmissionOrdering` (exactly one admission, after `IsFree` and `GetRoom`, before `TransitionToConcealing`) still holds.

- [ ] **Step 5: The wrappers.** In `internal/usercommands/skill.skullduggery.sneak.go`, after the `case result.InCombat:` arm add:

```go

	case result.ReplacedShroud:
		// Already hidden by a weaker Empathic Shroud: the sneak took the hide
		// over with no roll (#444), so there is nothing to practise.
		user.SendText(messaging.CategorySystem, `You let the shroud fall away and slip into the shadows on your own.`)
		return true, nil
```

In `internal/mobcommands/sneak.go`, after the `CostRefused` return add:

```go
	// A sneak that took over a weaker Empathic Shroud ran no contest (#444),
	// so there is nothing to practise, as on the player path.
	if result.ReplacedShroud {
		return true, nil
	}
```

The mob behaviour tree (`behaviortree/actions_skullduggery.go:32`) already reads `Success || AlreadyHidden` as Success and needs no change.

- [ ] **Step 6: Run.**

Run: `go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/behaviortree/ -count=1`
Expected: PASS, including `TestThrowSneakCostAdmissionOrdering`.

- [ ] **Step 7: Docs.** In `internal/actions/context.md`, Sneak section:
  - Change the **Roll** bullet's first sentence to: `The sneaker uses \`CalcSneakScore\`: \`Character.HideBaseScore()\` (effective Dexterity plus Skullduggery x \`SkillWeight\`, or while Empathic Shroud hides the actor the shroud's score in their place, #444) plus stealth bonuses, modified by light conditions per observer. The same score is every opposed roll against a hider: the 11 \`CalcSneakScoreVsObserver\` sites.`
  - Add a bullet after **Success/failure**: `- **Under a shroud (#444):** an actor the shroud hides is \`AlreadyHidden\` unless their \`SneakBaseScore()\` beats the shroud's score; then the sneak pays the normal admission and takes the hide over with \`Character.SneakOverShroud()\` (record 31 dropped without its end line, record 9 added, no reveal, no roll) and returns \`Success\` with \`ReplacedShroud\` and \`RollHappened\` false.`
  - Replace the **Mob wrapper ownership** bullet's last sentence (`only a successful paid attempt calls \`OnSkillUse("skullduggery", 0)\`.`) with `a paid attempt awards Skullduggery through \`AwardResolved\` win or lose, except a \`ReplacedShroud\` takeover, which ran no contest.`
  - In **Player wrapper ownership**, add: `A \`ReplacedShroud\` result reads "You let the shroud fall away and slip into the shadows on your own." and awards nothing.`

- [ ] **Step 8: Commit.**

```bash
git add internal/actions/skill_helpers.go internal/actions/sneak.go internal/actions/sneak_shroud_test.go internal/actions/context.md internal/usercommands/skill.skullduggery.sneak.go internal/usercommands/skill_skullduggery_sneak_shroud_test.go internal/mobcommands/sneak.go
git commit -m "feat(sneak): the shroud's score in every roll; a stronger sneak takes a shroud hide over (#444)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: G7d, the spell stamps the caster's score; the cascade and the room lines (#444)

**Files:**
- Modify: `internal/hooks/light_spell.go:42`, `:56-57`; `internal/hooks/Awareness_Cascades.go:47`; `internal/hooks/Condition_ApplyConditions.go:3-4`, `:86`, `:168`; `_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml:6-7`; `condition_apply_path_guard_test.go:111-113`, `:145`, `:263`
- Test: `internal/hooks/shroud_hide_test.go` (new)
- Docs: `internal/hooks/context.md:1017-1045`

- [ ] **Step 1: Write the failing tests.** Create `internal/hooks/shroud_hide_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09). These tests run
// with the Awareness cascade wired (Awareness_Cascades.go), which the
// characters package tests cannot.

const shroudTestStartLine = "seems to shimmer and fade from view"

// shroudLines is everything queued for userId, tags stripped, as one string.
func shroudLines(userId int) string { return strings.Join(drainPlain(userId), "\n") }

// shroudFixture seeds records 9 and 31 with their shipped flags and text,
// puts users 1 and 2 in cave room 2 under a lamp of 90, and returns them
// visible. User 1's cascade is proven wired: a sneak hide adds record 9 and
// its reveal cancels it.
func shroudFixture(t *testing.T) (holder, other *users.UserRecord, room *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee_plain} disappears into the shadows.",
			EndRoomText:   "{actee_plain} emerges from the shadows."},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee} seems to shimmer and fade from view.",
			EndRoomText:   "{actee} shimmers back into view."},
	}))
	room = rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(90)
	holder, other = users.GetByUserId(1), users.GetByUserId(2)
	for _, u := range []*users.UserRecord{holder, other} {
		rooms.LoadRoom(1).RemovePlayer(u.UserId)
		u.Character.RoomId = 2
		room.AddPlayer(u.UserId)
		require.NoError(t, u.Character.Validate()) // wires the cascades once
		u := u
		t.Cleanup(func() {
			u.Character.Awareness.ForceVisible(state.TransitionReason{Trigger: "test cleanup"})
		})
	}

	// Probe: the cascade must be live, or "no record 9" below proves nothing.
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, holder.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	holder.Character.Awareness.ResolveConcealment(true, r)
	require.NotEmpty(t, holder.Character.GetConditions(9), "fixture: the Hidden cascade must add record 9")
	require.NoError(t, holder.Character.Awareness.TransitionToRevealing(r))
	require.Empty(t, holder.Character.GetConditions(9), "fixture: the reveal cascade must cancel record 9")
	holder.Character.Conditions.Prune()

	for _, uid := range []int{1, 2} {
		events.DrainQueuedMessagesForTest(uid)
		events.DrainQueuedConditionsForTest(uid) // left by earlier tests in the package
	}
	return holder, other, room
}

// The spell stamps the caster's score on record 31; landing, it hides the
// holder at that score with no record 9.
func TestShroudSpell_HidesAtTheCastersScoreWithNoRecord9(t *testing.T) {
	holder, caster, _ := shroudFixture(t)
	caster.Character.SetSkill("spellcasting", 5)
	spell := &spells.SpellData{SpellId: "empathic-shroud", Name: "Empathic Shroud", PrimaryStat: "willpower",
		EffectType: "condition", ConditionIds: []int{conditions.ConditionIdEmpathicShroud}}
	want := characters.ShroudScore(spell.CasterStatValue(caster.Character.Stats), caster.Character.GetSkillLevel(skills.Spellcasting))
	require.Positive(t, want)

	applySpellCondition(holder, spell, caster.Character, conditions.ConditionIdEmpathicShroud)
	q := events.DrainQueuedConditionsForTest(holder.UserId)
	require.Len(t, q, 1)
	require.Equal(t, want, q[0].Magnitude, "the record carries the CASTER's score")
	require.Zero(t, q[0].Triggers, "the authored duration stands")

	require.Equal(t, events.Continue, ApplyConditions(q[0]))
	require.True(t, holder.Character.IsHidden())
	require.True(t, holder.Character.HiddenByShroud())
	require.Equal(t, want, holder.Character.HideBaseScore())
	require.Empty(t, holder.Character.GetConditions(9), "a shroud hide carries no record 9")
	require.Contains(t, shroudLines(caster.UserId), shroudTestStartLine, "the room watched the holder fade")
}

// Breaking: an observer spotting the hide, logging out, and the combat strip
// all reveal it, and record 31 ends with the hide.
func TestShroudHide_RevealCancelsRecord31(t *testing.T) {
	cases := map[string]func(u *users.UserRecord){
		"spotted": func(u *users.UserRecord) {
			_ = u.Character.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		},
		"logout": func(u *users.UserRecord) {
			onPlayerDespawnForAwareness(events.PlayerDespawn{UserId: u.UserId})
		},
		"combat strip": func(u *users.UserRecord) {
			u.Character.CancelCombatConditions()
		},
	}
	for name, reveal := range cases {
		t.Run(name, func(t *testing.T) {
			holder, _, _ := shroudFixture(t)
			require.NoError(t, holder.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
			require.True(t, holder.Character.HiddenByShroud())

			reveal(holder)

			require.False(t, holder.Character.IsHidden())
			require.Empty(t, holder.Character.GetConditions(conditions.ConditionIdEmpathicShroud), "record 31 must end with the hide")
		})
	}
}

// A stronger shroud taking over a sneak hide: the holder was already hidden,
// so nobody watches them fade, and the room line is not told.
func TestShroudOverSneak_TellsTheRoomNothing(t *testing.T) {
	holder, other, _ := shroudFixture(t)
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, holder.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	holder.Character.Awareness.ResolveConcealment(true, r)
	require.NotEmpty(t, holder.Character.GetConditions(9))
	events.DrainQueuedMessagesForTest(other.UserId)

	holder.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 9999, "spell")
	q := events.DrainQueuedConditionsForTest(holder.UserId)
	require.Len(t, q, 1)
	require.Equal(t, events.Continue, ApplyConditions(q[0]))

	require.True(t, holder.Character.HiddenByShroud(), "the stronger shroud took over")
	require.False(t, holder.Character.Conditions.HasCondition(9), "record 9 is discarded, so no end line")
	require.NotContains(t, shroudLines(other.UserId), shroudTestStartLine,
		"a holder already hidden fades in front of nobody")
}
```

- [ ] **Step 2: Run and see them fail.**

Run: `go test ./internal/hooks/ -run 'Shroud' -count=1`
Expected: FAIL: the queued magnitude is 0 (no shroud door yet), a record 9 sits beside the shroud hide, and the start line reaches the observer for a holder already hidden. (The "combat strip" subtest passes already only because the test seeds 31 with `cancel-on-combat`; Step 6 puts that flag in the shipped YAML.)

- [ ] **Step 3: The spell door.** In `internal/hooks/light_spell.go`, insert before the `// spellConditionTarget is what a spell condition lands on` comment:

```go
// shroudSpellApplication reports the magnitude Empathic Shroud's record 31
// carries: the CASTER's shroud score, the spell's stat plus Spellcasting rank
// x SkillWeight (#444, owner 2026-10-09). The holder hides at it in place of
// Dexterity plus Skullduggery (characters.HideBaseScore). The duration stays
// the authored one (triggers 0 on AddConditionMagnitude). ok is false for
// every other condition.
func shroudSpellApplication(spellData *spells.SpellData, caster *characters.Character, conditionId int) (magnitude float64, ok bool) {
	if conditionId != conditions.ConditionIdEmpathicShroud || spellData == nil || caster == nil {
		return 0, false
	}
	return characters.ShroudScore(spellData.CasterStatValue(caster.Stats), caster.GetSkillLevel(skills.Spellcasting)), true
}

```

and make the first statement of `applySpellCondition`:

```go
	if mag, ok := shroudSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, 0, mag, "spell")
		return
	}
```

- [ ] **Step 4: The cascade skips record 9 for a shroud.** In `internal/hooks/Awareness_Cascades.go`, directly after the line `case to == awareness.Hidden:` (line 47) insert:

```go
				// A shroud hide carries record 31, not 9 (#444): its end
				// is told once, by 31's own end line, and a timed 31 must
				// not pin a permanent 9 that outlives it. Its data is
				// stored before this cascade runs (ResolveConcealmentAs).
				if d, ok := c.Awareness.HiddenData(); ok && d.Source == awareness.HideShroud {
					return
				}
```

- [ ] **Step 5: No start line for a hide on a holder already hidden.** In `internal/hooks/Condition_ApplyConditions.go`, add `"slices"` as the first import followed by a blank line. After `wasAlreadyActive := targetChar.HasCondition(evt.ConditionId)` (line 86) add:

```go
	// A hide landing on a holder already hidden (Empathic Shroud taking over
	// a sneak, #444) is seen by nobody: no one watched them vanish, so its
	// start room line ("seems to shimmer and fade from view") would only
	// give the hidden holder away.
	hideOnHidden := targetChar.IsHidden() && slices.Contains(conditionInfo.Flags, conditions.Hidden)
```

and change `if roles.Observer != "" {` (line 168) to `if roles.Observer != "" && !hideOnHidden {`.

- [ ] **Step 6: Condition 31 breaks on combat.** In `_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml`, make the flags:

```yaml
flags:
  - hidden
  # A real hide (#444): it breaks on combat the way the sneak's record 9 does.
  - cancel-on-combat
```

- [ ] **Step 7: Re-key the condition-path allowlist.** These edits move lines that `condition_apply_path_guard_test.go` keys by number. Run `go test . -run TestPlayerConditionsTravelTheEventPath -count=1`; it names each stale key and each new line. With the code exactly as above the keys become:

| Old key | New key |
|---|---|
| `internal/hooks/Condition_ApplyConditions.go\|107` | `internal/hooks/Condition_ApplyConditions.go\|114` |
| `internal/hooks/Condition_ApplyConditions.go\|109` | `internal/hooks/Condition_ApplyConditions.go\|116` |
| `internal/hooks/Condition_ApplyConditions.go\|111` | `internal/hooks/Condition_ApplyConditions.go\|118` |
| `internal/hooks/Awareness_Cascades.go\|57` | `internal/hooks/Awareness_Cascades.go\|64` |
| `internal/hooks/light_spell.go\|58` | `internal/hooks/light_spell.go\|75` |

(The `\|` above is a table escape; the keys in the Go file use a plain `|`.)

and add a new entry directly above the `light_spell.go|75` one:

```go
	"internal/hooks/light_spell.go|71": "Empathic Shroud at the caster's shroud score (#444): the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",
```

If the guard reports different numbers, use the guard's numbers.

- [ ] **Step 8: Run.**

Run: `go test ./internal/hooks/ -count=1`, then `go test . -run 'TestPlayerConditionsTravelTheEventPath|TestNarrationSitesMatchViewpointAudit' -count=1`, then `gofmt -l internal/hooks condition_apply_path_guard_test.go` (no output).
Expected: PASS.

- [ ] **Step 9: Docs.** In `internal/hooks/context.md`, in "### Awareness_Cascades.go", after the first paragraph add:

```markdown
A shroud hide (#444) is the exception: when the hide's data says
`awareness.HideShroud` (stored by `ResolveConcealmentAs` before the
transition), the Hidden cascade adds no record 9, so the hide is Hidden plus
record 31 and its end is told once, by 31's end line. `light_spell.go`'s
`shroudSpellApplication` stamps the caster's `characters.ShroudScore` on
record 31 as its magnitude, and `ApplyConditions` tells no start room line
for a hidden-flag record landing on a holder already hidden.
```

and change the bullet `- Awareness \`Visible → Hidden\`: apply condition #9 + room text "sneaks away"` to `- Awareness \`Visible → Hidden\`: apply condition #9 (not for a shroud hide) + room text "sneaks away"`.

- [ ] **Step 10: Commit.**

```bash
git add internal/hooks/light_spell.go internal/hooks/Awareness_Cascades.go internal/hooks/Condition_ApplyConditions.go internal/hooks/shroud_hide_test.go internal/hooks/context.md _datafiles/world/dogmud/conditions/31-empathic_shroud.yaml condition_apply_path_guard_test.go
git commit -m "feat(spells): Empathic Shroud carries the caster's score and hides without a record 9 (#444)

Condition 31 gains cancel-on-combat, so a shroud hide breaks on combat as a
sneak hide does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Whole-tree verification

- [ ] **Step 1: Format.**

Run: `gofmt -l ./internal ./modules *.go`
Expected: no output.

- [ ] **Step 2: Vet.**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Full suite.**

Run: `go test ./... -count=1` (about two minutes)
Expected: every package `ok`. A failure is a finding: fix it in the task it belongs to, do not skip it.

- [ ] **Step 4: Lint, new issues only (as CI's `only-new-issues`).**

Run: `golangci-lint run --new-from-rev=master ./...`
Expected: `0 issues.` (errcheck is on: assign an unused error return with `_ =`.)

- [ ] **Step 5: No dashes in what this branch added.**

Run (on its own line; zero matches expected): `git diff master -- '*.go' '*.yaml' | grep '^+' | grep -nP '[\x{2013}\x{2014}]'`
Expected: no output.

- [ ] **Step 6: The config bit.**

Run: `git ls-files -v _datafiles/config.yaml`
Expected: `S _datafiles/config.yaml`.

---

### Task 14: Playtest (executed by the controller, not a subagent)

Run against a local build of this branch with the harness, following `dogmud-playtesting`; harness lessons from the last two sight-gates runs are in memory `project-sight-closeout-playtest-harness-lessons-2026-10-08`. Goals, not mechanics:

- **G1 (#446):** two fighters holding identically named weapons, below full sight. The defender's lines read "weapon"; the attacker's name their own.
- **G2 (#447):** a room lit only by a carried torch or Sunstone. Taking it off is seen by watchers it leaves in the dark; lighting one in pitch dark is not seen; a light displaced by another light-slot item is seen going. Repeat once with a mob (`command <mob> remove`, or a mob donning a floor torch).
- **G3 (#448):** take or lose a night-sight draught while a carried light dims; the band notice names the eyes.
- **G4 (#242):** a target who walks out mid-fold, caster in a dark room: "Something is no longer here." A harm spell with no target, a reader who sees nothing in the room: "A half-formed spell sputters out."
- **G5 (#216):** a Blinded reader beside a fight in a lit room hears "You hear fighting close by."
- **G6 (#449):** the login flash reads plain with its two glyphs; a long kick or grapple line wraps at 80 in the attacker's and defender's own text; the quit line wraps; the five copy lines read without dashes.
- **G7 (#444):** cast `empathic-shroud` on yourself and on another player: the room sees the fade, the holder is hidden and searches and arrivals roll against the caster's score; after 16 rounds one line ("shimmers back into view") and the holder is visible; `sneak` while shrouded, both directions; a shroud on a sneak-hidden ally, both directions; attacking while shrouded reveals. Watch Windwarden Sylara (mob 241, Ironwind Steppe) idle-cast it and judge whether her now real hide hurts her role (D10).
- **#251 scout case:** a scout mob next to a dark room and next to a hidden player: does it see into the dark room or pick the hidden player as its soft target?
- **A TargetGone fizzle:** a mob caster whose target leaves mid-fold: the room reads the fizzle by sight or by sound, and nothing names the caster to a reader who cannot see.

File every finding as a GitHub issue on `pruuk/DOGMud` (`--repo pruuk/DOGMud`). If no leak is found, the PR body lists #446, #447, #448, #242, #216, #251, #444 and #382 as resolved, in words, without a closing keyword.

---

## Self-review against the spec

- G1 Task 1; G2 Tasks 2 and 3; G3 Task 4; G4 Task 5; G5 Task 6; G6 login flash and dashes Task 7, categories and logout Task 8 (admin echoes over 80 stay open in #449, per spec); G7 entering, score, one hide, breaking, ending: Tasks 9 to 12; testing and close: Tasks 13 and 14.
- Out of scope, per spec: room prose dashes (#248), the `achievements` and `craft list` table separators.
