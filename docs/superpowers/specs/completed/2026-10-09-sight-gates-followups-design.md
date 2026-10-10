# Sight gates follow-ups (#382)

**Status:** APPROVED by the owner, 2026-10-09. G1-G6 are as drafted. G7 follows the owner's
rulings that day: the caster's score, and the stronger hide wins.
**Parent:** `specs/completed/2026-10-08-sight-gates-playtest-fixes-design.md` (shipped in #445).
The re-run playtest (run `a7219436241e24e7`, master `c7520f387`) left #446, #447, #448 and
#449, and the residue of #242 and #216. This slice fixes all six in one PR, and adds the owner's
Empathic Shroud ruling (#444) as G7. #251 needs only a scout playtest case, run in this slice's
playtest.

## Facts verified against source (master `7463f7be6`)

| Fact | Where |
|---|---|
| Participants keep every weapon whose NAME matches one they hold | `internal/combat/combat.go:746-776` (`hideIdentitiesInPersonalLines`, `heldWeaponNames`) |
| Every weapon token in an `AttackResult` is the ATTACKER's: `{itemname}` from `ws.weaponName`, `{weapon}` and `{attack}` from `sourceChar.Equipment.Weapon` | `combat_helpers.go:1676`, `:1357-1369`; `combat.go:271`, `:315` |
| Double fumble and pet lines name no weapon | `combat_helpers.go:919-960`; `applyPetDamage` |
| Crit disarm names the victim's weapon on its own Actor line, outside `AttackResult` | `combat/criteffects.go:48-58` |
| Light lines judged after the change: only a DARKNESS takes a before-snapshot | `usercommands/equip.go:118-124`, `:177-187`; `mobcommands/equip.go:64-70`, `:93-100`; `hooks/mob_equip_best_floor_item.go` |
| No snapshot at all on: displaced items, the arm-slot "equips", player and mob `remove` | `usercommands/equip.go:144`, `:157`; `usercommands/remove.go:66`; `mobcommands/remove.go:31` |
| `Room.VisualSnapshot` / `SendTextVisualToSnapshot` exist for exactly this rule | `internal/rooms/rooms.go:400`, `:419` |
| Sunstone is `type: light`, `subtype: wearable`, worn condition 134 | `items/armor-20000/light/20099-sunstone.yaml` |
| Notice attribution: "eyes" only when the OLD light through the CURRENT sight gives exactly the new band; otherwise the first light term that moved wins, carried before sky | `internal/lightnotice/tracker.go:150-187` |
| A record stores room, band and light terms, not the observer's sight window | `tracker.go:61-69`; sight inputs `NightVisionStrength()`, `InfraReach()` at `:338` |
| Target-gone line names the target raw | `hooks/spell_resolution.go:148` |
| "crackles through the air harmlessly" goes out visual only | `spell_resolution.go:180-182` (`sendVisualRoomText`) |
| Fight sound skips every lit room, so a Blinded reader in a lit room hears nothing | `hooks/NewRound_DoCombat_helpers.go:459-474` (`room.IsLit()` guard) |
| `Room.SendTextUnsighted` sends to exactly the SightNone readers | `rooms.go:546` |
| Login flash: `emote` escapes ANSI tags, so the tagged config line prints "< ansi" | `config.yaml:74` (HEAD); `usercommands/emote.go:53` |
| Em dash at `loot.go:170`; the dash guard is a hand list of 8 files that omits loot | `copy_no_dash_test.go` |
| Special moves send the player's own lines as `CategorySystem`, which never wraps; the same command's defence lines already use the move category | `usercommands/kick.go:19` vs `:140`; peers `bash, drain, gore, maul, pounce, rake, throttle, trip, throw` |
| Quit line goes out as `CategoryLogout`, not in `shouldWrap` | `hooks/PlayerDespawn_HandleLeave.go:151-152`; `messaging/pipeline.go:122` |
| Condition 31: `hidden` flag, 16 rounds, cast by `empathic-shroud` (willpower, mental) | `conditions/31-empathic_shroud.yaml`; `spells/empathic-shroud.yaml` |
| Only record 9 enters Hidden on apply; 31 is excluded on purpose | `characters/conditions.go:163-192` (`hideForStealthRecord`) |
| Entering Hidden adds a permanent 9; leaving cancels every hidden-flag record | `hooks/Awareness_Cascades.go` (`awareness_condition_mirror`) |
| `HiddenData` is empty today; `ResolveConcealment(true)` sets it | `state/awareness/awareness.go:168-183` |
| Hider score is Dex + Skullduggery × `SkillWeight` + mutation stealth, then light modifiers | `actions/skill_helpers.go:29-49` (`CalcSneakScore`) |
| A spell condition can carry a caster-derived magnitude: `CasterStatValue` + Spellcasting | `hooks/light_spell.go:23-40`, `:56-66` |
| `sneak` while hidden returns `AlreadyHidden`: "You're already hidden!" | `actions/sneak.go:136-138`; `usercommands/skill.skullduggery.sneak.go:55-57` |
| Every opposed roll against a hider goes through `CalcSneakScore` (move, sneak, search, track, shadow, steal, plant) | `actions/skill_helpers.go:66`; 11 call sites |

## Fixes

- **G1 Weapons by owner, not name (#446).** Inside one `AttackResult` every weapon tag is the
  attacker's. So the attacker's own lines keep every weapon, and the defender's lines keep
  none. Delete `heldWeaponNames`. The crit disarm line stays the accepted known limit.
- **G2 Light lines judged before the change (#447).** Every equip and remove line for an
  item that touches light (`type: light`, or a light or darkness worn condition) is
  snapshotted before the change and sent with `SendTextVisualToSnapshot`. This covers
  player and mob equip, displaced items, the arm-slot line, and player and mob remove.
  A reader who saw nothing before gets nothing; there is no sound line.
- **G3 Blame the eyes when the eyes moved (#448).** Record the sight window (strength,
  reach) with each notice. If it changed and the old light through the new sight moves the
  band the same way the band moved, the cause is "eyes", even when a light also drifted.
- **G4 #242 residue.** The target-gone line hides the target's name at the caster's sight.
  The "harmless" line gains the sound half: `SoundSpellSputtersOut` via `SendTextUnsighted`.
- **G5 #216.** Drop the lit-room guard. The fight sound goes to every reader who cannot make
  out shapes, through `SendTextUnsighted`.
- **G6 Copy (#449).**
  - The login flash becomes plain "in a flash of lightning!". This is a HEAD-blob edit, and
    the owner's local copy gets the same line.
  - Remove the dash from the loot, dead, rally and warcry lines, and add their files to the
    guard.
  - Special-move personal lines take the move's category, so they wrap and colour like that
    command's defence lines.
  - `CategoryLogout` wraps.
  - The admin echoes over 80 stay open in #449.

- **G7 Empathic Shroud is a real hide (#444, owner ruling 2026-10-09).** The ruling: "a real
  hide/sneak with the spellcasting rank and stat replacing dex + skullduggery for the future
  opposed rolls."
  - **Entering.** Record 31 landing on a Visible holder enters Hidden, as F5 does for 9.
    The Hidden data notes that the shroud is the source.
  - **Score.** At cast, the record's magnitude takes the CASTER's shroud score: the spell's
    stat (`CasterStatValue`, willpower) plus Spellcasting rank × `SkillWeight`. While
    hidden by the shroud, `CalcSneakScore` uses that score in place of Dex plus
    Skullduggery × `SkillWeight`. Mutation stealth and the light modifiers still apply. A 31
    with no magnitude (admin `setcondition`) uses the holder's own spell score.
  - **One hide at a time, the stronger (owner, 2026-10-09).** "Only one or the other should
    exist (the strongest)." The comparison is the two base scores: the shroud score against
    Dex plus Skullduggery × `SkillWeight`. Mutation and light apply to both alike, so they
    are left out.
    - `sneak` while shroud-hidden, shroud stronger: refused with the existing "You're
      already hidden!".
    - `sneak` while shroud-hidden, sneak stronger: the sneak replaces the shroud. Record 31
      is removed without a reveal and the hide becomes a sneak hide. There is no new roll,
      since the holder is already hidden; the normal sneak cost applies.
    - The shroud landing on a sneak-hidden holder: if it is stronger, it replaces the sneak.
      If not, record 31 is dropped and the sneak stays.
  - **Breaking.** A shroud hide breaks the way a sneak hide does: when an observer wins an
    opposed roll against it, and on entering combat. The reveal cascade cancels 31 with 9.
  - **Ending.** When no live 31 remains (expiry, purge, death), `Validate` reveals a
    shroud-sourced hide.

**Out of scope:** room prose dashes (#248), and the table separators in `achievements` and
`craft list`.

## Testing and close

Each fix gets a failing test first:
- G1: `combat/darkness_identity_hiding_test.go`, with same-named weapons.
- G2: `usercommands/equip_light_line_test.go`.
- G3: `lightnotice/tracker_test.go` table rows.
- G4: `hooks/spell_channel_sight_test.go`.
- G5: a Blinded reader in a lit room.
- G6: the dash guard, plus a wrap test on a kick line.
- G7: in `characters`, 31 enters Hidden and its end reveals. In `actions`, the shroud score
  replaces Dex plus Skullduggery. Both stronger-wins directions are tested, as is a spotted
  shroud hide cancelling 31.

One PR. The playtest re-runs the failing cases. It adds a scout case (#251), a TargetGone
fizzle, and a shroud cast: hidden, then visible when the shroud ends. If no leak is found,
#446-#448, #242, #216, #251 and #444 close, along with #382.
