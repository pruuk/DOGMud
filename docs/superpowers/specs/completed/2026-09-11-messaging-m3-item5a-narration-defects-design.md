# Messaging M3 Item 5a: Buff, Spell and Quest Narration Defects

Design for the first half of M3 item 5 in the messaging unification arc
([`2026-08-31-messaging-unification-design.md`](2026-08-31-messaging-unification-design.md)).
The arc spec lists item 5 as "buffs, spells, quests: the Actee slot appears
here". Reading the source showed that item 5 is really a cluster of live
defects that share one cause, so the owner split it:

- **5a (this spec):** fix the six defects on the paths as they exist today.
- **5b (later):** move buffs, spells and quests onto the `internal/narration`
  core as a pure refactor with byte-identical goldens.

The split keeps behaviour changes and refactoring in separate diffs, the rule
items 1 to 4 followed.

## Facts verified against source, 2026-09-11

Read from master `e1e0fed74`. Nothing here is recalled.

### Delivery

| Fact | Evidence |
|---|---|
| Every buff and spell phase line goes through one two-role function: a user line and a room line, no actee | `internal/textutil/spelltext.go:21` `SendPhaseText` |
| `rooms.Room.SendText` is the **audio** channel: never sight-gated, never anonymized, and blind observers still receive it | `internal/rooms/rooms.go:276-291` |
| Buff start text sends its room line with `r.SendText` | `internal/hooks/Buff_ApplyBuffs.go:105` |
| Buff trigger text sends its room line with `r.SendText` | `internal/hooks/NewRound_UserRoundTick.go:288` |
| Buff end text sends its room line with `r.SendText`, for player holders and mob holders | `internal/hooks/NewTurn_PruneBuffs.go:53`, `:110` |
| The quest bridge sends `room_text` with `room.SendText`, raw, with no token substitution | `internal/questengine/bridge.go:205-211`, called from `internal/questengine/actions.go:76` |
| `GameBridge.RoomText` is the only production implementation; the other two are test mocks | `questengine/bridge.go:205`; `actions_test.go:68`, `action_abort_test.go:31` |
| The behaviour tree reads the **same** `room_text` key and does it correctly: substitutes tokens, sends with `SendTextVisual` | `internal/behaviortree/actions_dialogue.go:44` |
| The quest reward `roommessage` path is already visual | `internal/hooks/Quest_HandleQuestUpdate.go:267` |

### Who the buff text is about

| Fact | Evidence |
|---|---|
| In buff phase text, the "user" line goes to the buff's **holder**, and `{source}` is the **holder's own name** | `Buff_ApplyBuffs.go:80-107`; `NewRound_UserRoundTick.go:281-293` |
| The buff event carries no caster id at all | `internal/events/eventtypes.go:20-25` (`UserId`, `MobInstanceId`, `BuffId`, `Source string`) |
| A spell applying a buff queues that event, so `Buff_ApplyBuffs` fires its start text **as well as** the spell's own lines | `spell_resolution.go:1050` `target.AddBuff`; `users/userrecord.go:422`; `hooks/hooks.go:15` |
| Mob buff ticks read no trigger text at all; only the player tick does | `NewRound_MobRoundTick.go:218` `tickMobBuffs`; trigger text is read only at `NewRound_UserRoundTick.go:279` |

### The spell lines

| Fact | Evidence |
|---|---|
| A help spell cast with no target is a **self-cast** by default | `internal/actions/cast.go:227` `// default to self` |
| An area help spell puts **every** player in the room in its target list, caster included | `spell_resolution.go:109-110` |
| Mass Mend and Cleansing Wave are area help spells | `spells/mass-mend.yaml`, `spells/cleansing-wave.yaml` (`type: helparea`) |
| `case "purge"`: the caster line names the target even on a self-cast, and a self-cast also gets a second self line | `spell_resolution.go:1005`, `:1015` |
| `case "heal"`: same shape | `spell_resolution.go:1035`, `:1045` |
| `case "buff"`: the caster line and the room line both name the target even on a self-cast | `spell_resolution.go:1076`, `:1090` |
| `default:` caster line names the target even on a self-cast | `spell_resolution.go:1123` |
| **`case "shield"` already handles self-cast correctly**: the target line is unconditional, the caster's third-person line is sent only when caster and target differ | `spell_resolution.go:1112`, `:1119` |
| **The mob self-cast path is correct too** | `spell_resolution.go:1392` `%s channels restorative magic.` |
| `resolvePurgeAffliction` handles self-cast with its own branch | `internal/hooks/spell_purgeaffliction.go:34`, `:37` |
| The buff room line at `:1090` was added by M2 | commit `092e5f11a`, merged in #113 |

### Quests

| Fact | Evidence |
|---|---|
| A room interaction fires on **any** command, before normal dispatch | `internal/usercommands/usercommands.go:382` |
| A trigger with no verb matches every verb | `internal/questengine/engine.go:182` |
| Quest triggers are validated at startup by a function that already walks every action and panics on bad data | `internal/questengine/loader.go:106` `ValidateAllFlags` |
| Buff and spell loaders **only warn** on an unknown token | `buffs/buffspec.go:217-218`; `spells/spells.go:305-306` |
| `ValidateTokens` flags unknown tokens only; `{source}` is a known token, so it would not have caught D5 | `internal/textutil/tokens.go:33-42` |

### Names and anonymization

| Fact | Evidence |
|---|---|
| `GetCharacterName(true)` returns the name wrapped in a `username` tag, with any adjectives appended **outside** that tag | `internal/characters/formattedname.go:164-168`, `:72-93` |
| So `messaging.Anonymize`, which matches `[^<]+` inside a `username`, `mobname` or `petname` tag, strips it correctly | `internal/messaging/anonymize.go` |
| The buff start text's mob-holder branch also uses `GetCharacterName(true)`, which tags a **mob** with the player colour | `Buff_ApplyBuffs.go:90` |
| The `{source_plain}` leak is held by the owner for M5 | arc spec `:91`, `:333`, `:588` |

### Test coverage today

| Fact | Evidence |
|---|---|
| The self-cast tests call the function and **assert nothing** | `internal/hooks/hooks_test.go:2411` `PurgeSelf`, `:2442` `HealSelf`, `:2489` `BuffSelf` |
| The `ApplyBuffs` tests check return codes only, never delivered text | `hooks_test.go:940-1001` |
| Nothing asserts on quest `RoomText` output; the mock only records it | `questengine/actions_test.go:68` |
| Message capture works in hooks tests | `events.DrainQueuedMessagesForTest`, e.g. `hooks/channel_defence_routing_test.go:149` |

## The reframing

The arc spec says the actee slot "appears here". For buffs that is backwards.
A buff's "user" line already goes to the one **acted upon**, the holder, so the
viewpoint actually missing from buff text is the **caster**. The buff event has
no caster id to carry it.

That is not fixed in 5a. It is the prerequisite for the D3 merged line, which
is deferred to M6.

## The six defects

| # | Defect | Reach |
|---|---|---|
| **D1** | Buff room lines go out on the audio channel although the text is visual ("A warm glow surrounds Alice"), so they reach blind and unsighted observers | start, trigger and end phases; 43 authored lines |
| **D2** | Quest `room_text` goes out on the audio channel although the text is visual ("unlocks the strongbox") | 22 lines |
| **D3** | A spell that buffs a player is narrated twice to every audience: once by the spell code, once by the buff's start text. Self-casts also read badly ("Alice's Chrysalis Glow settles over Alice") | 17 of 19 spell-to-buff links; 5 of them also double the room line, which M2 introduced |
| **D4** | Mob buff trigger text never fires | 7 buffs |
| **D5** | Quest 77 sends a literal `{source}` to players | 1 line |
| **D6** | Quest `room_text` never names who acted: "unlocks the strongbox and pulls out a leather journal." | 21 lines |

### A correction to the M1 audit

The M1 viewpoint audit ruled the buff case at `spell_resolution.go:1075` a room
"gap" (audit `:284`). That was only partly right. `Buff_ApplyBuffs` was already
narrating the room through `start_room_text`, via the async buff event that
neither the audit nor M2 followed. M2's fix was correct for the 12 buff links
with no authored room line and doubled the room line for the other 5.

## Owner rulings, 2026-09-11

- **Split:** defects first (5a), migration second (5b).
- **D3:** the right end state is one merged line per audience, with the code
  supplying who and the data supplying what. That needs a caster in the buff
  event and a rewrite of 17 buffs, so it moves to **M6**. In 5a, D3 fixes
  **only the self-cast wording**.
- **D6:** rewrite the 21 fragments to start with `{source}` now, so every quest
  line uses one convention, the same one the behaviour tree and quest 77 use.
  This brings the content playtest gate into 5a.

## Design

### D1: buff room lines become visual

All four buff room sends switch from `SendText` to `SendTextVisual`:
`Buff_ApplyBuffs.go:105`, `NewRound_UserRoundTick.go:288`,
`NewTurn_PruneBuffs.go:53` and `:110`.

While those lines are open, the mob-holder branches tag the mob with
`mobDisplayName` instead of `GetCharacterName(true)`, so a mob's name gets the
mob colour. `mobDisplayName` is already used for the same purpose at
`spell_resolution.go:1392`.

**Accepted behaviour change:** someone blind, asleep, or in an unlit room
without night vision stops receiving buff room lines. An infrared-only observer
receives "a figure". The 14 buffs using `{source_plain}` still name the holder
to an infrared-only observer; that leak is held for M5 and not touched here.
It is still no worse than today, because on the audio channel everyone
receives the plain name.

### D4: mob buff trigger text

`tickMobBuffs` sends a triggered buff's `trigger_room_text` the way the player
tick does, on the visual channel from D1, excluding no one. A mob has no
client, so there is no user line.

### D2, D5, D6: quest room text

`GameBridge.RoomText` copies what the behaviour tree already does: substitute
tokens, then send with `SendTextVisual`. `{source}` is the triggering player's
name from `GetCharacterName(true)`, so it carries the `username` tag and
anonymizes correctly.

The 21 fragment lines are rewritten to start with `{source} `. All 21 begin
with a lowercase verb and read correctly after a name. Quest 77 already uses
`{source}` and needs no text change; D5 is fixed by the substitution.

| Quest | Line |
|---|---|
| `14-the_undertow` | `:118` |
| `63-dock_rat` | `:71` |
| `65-the_street_sweepers_secret` | `:78` |
| `68-the_cooperage_circle` | `:69`, `:103` |
| `69-the_gallery_cipher` | `:86` |
| `70-the_pre_founding_web` | `:72` |
| `71-the_tribute` | `:91` |
| `72-the_water_dispute` | `:58` |
| `73-the_margin_notation` | `:76`, `:101`, `:127` |
| `74-the_undercroft` | `:96`, `:120`, `:145`, `:177` |
| `75-the_surveyors_report` | `:73`, `:107` |
| `76-the_disc` | `:54`, `:85`, `:127` |
| `77-the_truth` | `:79` (already correct) |

### D3: self-cast wording

Purge, heal, buff and the `default:` case copy the pattern `case "shield"` and
the mob self-cast path already use: when caster and target are the same, the
caster gets **one** line and the room gets **one** line that names them once.

| Case | Caster, self-cast | Room, self-cast |
|---|---|---|
| purge | `You purge the afflictions from your body.` (the existing self line becomes the only line) | `Cleansing Wave cleanses Alice of afflictions.` |
| heal | `A warm glow of healing magic envelops you. Your wounds begin to mend.` (existing, now the only line) | `Alice channels restorative magic.` (the mob self-cast wording) |
| buff | `Your Chrysalis Glow takes effect.` | `Chrysalis Glow settles over Alice.` |
| `default:` | `Your <spell> takes effect.` | no room line today; none added |

The buff caster line must stay rather than be dropped, because Veil Sight has
no authored start text and a self-caster would otherwise read nothing.

Cross-casts, where caster and target differ, are unchanged. The wording avoids
pronouns.

This matters more than it first appears, because area spells include the
caster in their own target list. Every Mass Mend or Cleansing Wave cast hits
the self-cast path for the caster.

### Startup check for quest room text

A new check runs beside `ValidateAllFlags` in `questengine/loader.go` and
panics at startup the same way. For every quest action's `room_text`:

- it must contain `{source}`;
- it must not use `{target}` or `{target_plain}`: a quest has no target, so
  they would render empty;
- it must not use `{source_plain}`: an untagged name cannot be anonymized, and
  the leak held for M5 must not spread to quests;
- it must use no unknown token.

This is stricter than buffs and spells, which only warn. That is deliberate: a
new rule with zero shipped violations can fail at boot at no cost, while
upgrading buffs and spells would change what is allowed to boot, so that is
filed rather than done.

## Tests

Each is written before the change it guards, and each is proven capable of
failing by a deliberate break that still compiles.

| Covers | Where | Asserts |
|---|---|---|
| D1 | hooks, dark cave room (`rooms.SeedBiomesForTest`) | an unsighted observer receives no buff room line; one with night vision receives the named line. Start, trigger and end; player and mob holders |
| D1 | hooks | a mob holder's name uses the mob tag |
| D4 | hooks | a mob holding a trigger-text buff sends its room line on the mob tick |
| D3 | hooks, replacing the assertion-free `PurgeSelf`, `HealSelf` and `BuffSelf` | the self-caster receives exactly one line; the room receives the new wording |
| D3 | hooks | an area heal and an area purge give the caster one line, not two |
| D2, D5, D6 | hooks, quest `RoomText` | `{source}` becomes the tagged name; an unsighted observer receives nothing; an infrared-only observer receives "a figure" |
| D6 | data test over the shipped quest files | every quest `room_text` contains `{source}` |
| startup check | questengine | each of the four rules rejects a broken fixture, and the shipped data passes |

## Playtest gate

Required by the content SOP, because D6 and D3 put new player-facing text into
the game. Two actors, one run.

| Lane | Where | Shows |
|---|---|---|
| Quest, lit | room 5343 (The Reliquary), `look disc` | the witness reads `Alice works something free of the cracked floor and pockets it.` (D6) |
| Quest, dark | room 6397 (zone Crash Site Interior, cave biome), `look records` | an unsighted witness receives nothing (D2); after drinking a Cat's Eye Draught (item `30047`, night vision) the witness reads the named line with no literal `{source}` (D5) |
| Buff, dark | cave room 3101, reusing the M2 darkness scenario | `m2-actor` self-casts Conviction Surge; `m2-witness`, who has no night vision by design, receives no buff room line (D1) |
| Self-cast, lit | any lit room | `m2-actor` self-casts Heal and Conviction Surge; `specialist-caster` casts Mass Mend; the caster reads one line and the room reads the new wording (D3) |
| Wording | throughout | both actors judge the rewritten quest lines and the new self-cast lines as prose |

**Limits of the run:**

- Each quest lane fires **once per character**. The disc trigger needs
  `missing_item: 40167` and the records trigger needs `missing: ["77-end"]`,
  so the goals file must stage them in order.
- **D4 is covered by the unit test only.** Of the 7 trigger-text buffs, Cold
  Discharge (94) and Hull Discharge (96) are applied by room mutators under
  `playerbuffids`, so to players only (`mutators/hull_discharge.yaml:5`,
  `hull_discharge_deep.yaml:5`), and Meditating (0) by the quit command
  (`usercommands/quit.go:16`). Venom (39) and Arc Trap (97) are applied by lock
  traps to whoever fails the lockpick (`gamelock/gamelock.go:16`); no behaviour
  tree uses `try_defuse`, so mobs do not reach the trap route. **Correction
  found in review, 2026-09-11:** mobs CAN hold Venom (39) and Spore Toxin (40)
  in ordinary play. The serpent, arachnid and carnivorous plant species list 39
  and the fungal colony lists 40 under `critbuffids`, and the combat hook
  applies crit buffs to whoever was hit (`hooks/NewRound_DoCombat_unified.go:570`),
  so one mob critting another gives it the buff. Staging a mob-on-mob crit on
  demand was not attempted, and no source was found for Searing Backlash (106),
  so D4 is still covered by the unit test only.

## Deliberately not in this slice

| Item | Where it belongs |
|---|---|
| Moving buffs, spells and quests onto the narration core | 5b |
| The merged "who plus what" line for spell buffs, which needs a caster in the buff event, a `{caster}` token, and rewriting start text on 17 buffs | M6 |
| The doubled narration for spell buffs cast on another player; only the self-cast wording is fixed here | M6, with the merged line |
| The `{source_plain}` anonymizer leak | M5, owner hold |
| Buffs and spells failing at boot on an unknown token instead of warning | filed |
| An area heal producing one room line per target | not examined; filed |
| Reworking how quests are reached and tied to rooms | the future quest arc, not yet scoped |

## Risks

- **The visual channel silences some observers.** That is the intended effect,
  and M2's darkness playtest established it for spell lines. It can still look
  like a bug in a playtest, so the gate states it up front.
- **21 authored lines change.** The rewrite is mechanical, but a player reads
  every one, which is why the gate includes a wording lane.
- **The startup check can refuse to boot** on a future authoring slip. That is
  the point: a quest line that fails silently in play is the defect this slice
  exists to remove.

## How we know it worked

- `go test ./...` is green, and every new test was seen to fail first.
- The existing goldens are byte-identical. 5a changes no message store's
  content, so any golden diff means something unintended moved.
- The server boots, and the startup check passes on the shipped quest data.
- The playtest shows each lane above, apart from D4, whose limit is stated.
