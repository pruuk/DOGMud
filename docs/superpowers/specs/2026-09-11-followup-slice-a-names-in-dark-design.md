# Follow-up Slice A: Names in the Dark

The first follow-up slice from messaging M3 item 5a
([`2026-09-11-messaging-m3-item5a-narration-defects-design.md`](2026-09-11-messaging-m3-item5a-narration-defects-design.md),
merged as PR #119). The owner ordered the follow-ups A, F, B, C, D+E. This
slice covers two of the 5a findings:

- **Casting at a player you cannot see works**, and the target is told the
  caster's name. Found in the 5a dark buff playtest lane.
- **SWEEP names both combatants to a player who cannot see.** The crit
  counter's room line goes out on the audio channel with plain names.

Reading the source widened both. Every "happens to you" line that names the
other party ignores sight, and hidden creatures can be named by every command
that names a creature, so `look <hidden creature>` gives them away. This slice
is player-facing only. Making mobs perceive darkness is slice F, which comes
next.

## Facts verified against source, 2026-09-11

Read from master `62b326bfe`. Nothing here is recalled.

### The narration seam

| Fact | Evidence |
|---|---|
| `SendTrio` delivers finished strings; it does not render or tokenise | `internal/messaging/trio.go:75-97` |
| `Recipient` has only `SendText`; `Broadcaster` has only `SendTextVisual` | `trio.go:44-52` |
| `Audience` has `Actor`, `ActorId`, `Actee`, `ActeeId`, `Room`, and no names | `trio.go:67-73` |
| 132 `messaging.SendTrio` calls in 26 files: 13 in mobcommands, 12 in usercommands, 1 in `actions/salvage.go` | grep, non-test |
| `*rooms.Room` is the only production `Broadcaster`; `fakeBroadcaster` is the only fake | `trio_test.go:15`; grep for `SendTextVisual(cat` |
| Root guard `TestEveryAudienceLiteralPairsIdsWithRecipients` requires `ActorId` beside `Actor` and `ActeeId` beside `Actee` | `m2_routing_guard_test.go:432` |
| `users.UserRecord.SendText` renders on the audio channel, which skips the sight gate and the anonymizer | `internal/messaging/pipeline.go:17-22`, `:59-67` |
| `Anonymize` swaps tagged `username`, `mobname` (with suffixes) and `petname` names for `<ansi fg="combat-anon">a figure</ansi>`; bare names leak | `internal/messaging/anonymize.go:13-30` |
| A rendered mob name is title-cased and carries ` #N` for duplicates | `internal/characters/formattedname.go:50-72` |
| `Room.SendTextVisual` judges each recipient: clear sight gets the text, shapes only gets it anonymized, neither gets nothing | `internal/rooms/rooms.go:307`, `:333-365` |

### Sight

| Fact | Evidence |
|---|---|
| `CanSeeClearly`: false when blinded or sleeping, otherwise true when the room is lit or the observer has night vision | `internal/messaging/predicates.go:24` |
| `CanSeeSightImpairedOnly` is the same without the sleep gate, kept because combat must not treat sleep as darkness | `predicates.go:72` |
| `CanSeeShapes`: clear sight, or infrared when not blinded and not sleeping | `predicates.go:92` |
| The melee darkness rewrite judges players with `CanSeeSightImpairedOnly`, so a sleeper struck in a lit room is still told what hit them | `internal/hooks/NewRound_DoCombat_unified.go:523-549` |
| Existing dark wording includes "Something strikes you in the dark!" and "Something hits you hard in the dark!" | `internal/hooks/NewRound_DoCombat_helpers.go:437` onward |
| Special moves hand-write a "Something" line using `canSeeInDark` (lit, or night vision; infrared ignored): 24 calls in 18 files, two copies of the function | `internal/mobcommands/darkness.go:44`, `internal/usercommands/skill_move_defence.go:93` |
| Flags: `hidden`, `nightvision`, `infraredvision`, `see-hidden` | `internal/buffs/buffspec.go:58`, `:62`, `:63`, `:76` |
| Nothing in dogmud content grants buff 85, InfraredVision | `buffs/85-infraredvision.yaml`; no spell, item, mutation or species references it |

### Lines that name an unseen party today

| Fact | Evidence |
|---|---|
| Cross-cast lines in `applyPlayerEffect` name both parties with no sight check: damage, purge, heal, buff, shield, default | `internal/hooks/spell_resolution.go:986-1000`, `:1008-1016`, `:1047-1055`, `:1106-1114`, `:1145-1152`, `:1164-1169` |
| Most spell lines wrap a plain name in an identity tag, for example `<ansi fg="username">%s</ansi>'s %s envelops you in healing energy.` | `spell_resolution.go:1050-1052` |
| Mob casts on players name the mob to the target | `resolveMobSpellAgainstPlayer`: `spell_resolution.go:1580`, `:1615`, `:1654`, `:1664`, `:1695`, `:1708` |
| The spell defence triad sends both personal lines with `SendTextVisualToUser`, so a defender who cannot see is told nothing | `spell_resolution.go:475-509` |
| Purge Affliction's cross-cast lines name both parties | `internal/hooks/spell_purgeaffliction.go:21-32` |
| RIPOSTE, SWEEP and SHIELD SLAM build all three lines from plain `defender.Name` and `attacker.Name` | `internal/hooks/combat_shared_helpers.go:282-440` |
| **The SWEEP leak:** the crit personal lines are sent ungated, and the room line goes out with `atkRoom.SendText`, the audio channel | `NewRound_DoCombat_unified.go:555-563` |
| The crit block runs after the darkness rewrite, so the rewrite never sees it | `NewRound_DoCombat_unified.go:547-552` |
| Skill-move counters build their lines from `attacker.Name` and `defender.Name` tokens | `internal/combat/counter.go:187`, `:199-201` |
| `DispatchCounterMessages` sends both personal lines ungated; it has 23 callers | `internal/actions/combat_counter.go:71-91` |
| Spell counters: personal lines ungated, room line visual but with plain names | `internal/hooks/counter_tier.go:33-60` |

### Casting

| Fact | Evidence |
|---|---|
| `InitiateCast` resolves typed names with `room.FindByName`, with no sight check | `internal/actions/cast.go:78`, `:107`, `:186`, `:213`, `:232` |
| With no name, a harmful cast aims at the caster's foe, then the party leader's foe in the same room | `resolvePlayerAggroTarget`, `cast.go:407` |
| A help spell with no name, or the caster's own name, is a self-cast | `cast.go:212`, `:227` |
| Help-multi, harm-area and help-area casts take no typed target. Neutral casts pass the text on: identify names an item, raise spells a corpse, conjure nothing | `cast.go:243-286`; `hooks/spell_resolution.go:89`; `hooks/companion_summon.go:29` |
| dogmud spells by type: 21 helpsingle, 15 harmsingle, 12 neutral, 7 harmarea, 4 helparea, none harmmulti or helpmulti | grep `^type:` in `spells/` |
| Veil Sight is a helpsingle spell | `spells/veil-sight.yaml:5` |
| The cast command refuses on `NoTarget` before any cost; `RefusalExplained` suppresses the generic line | `internal/usercommands/skill.cast.go:234-259` |
| Mobs reach `InitiateCast` through `mobcommands/cast.go:43` | grep |
| The cast help file has no section on targets | `templates/help/cast.template` |

### Naming a creature

Every name lookup in the game goes through `Room.FindByName`, directly or
through `actions.ResolveTargetActor`.

| Fact | Evidence |
|---|---|
| `FindByName` searches `GetMobs` and `GetPlayers`; neither filters hidden creatures | `internal/rooms/rooms.go:1890-1990` |
| `N.item`, `all.item` and `item#N` are parsed once, by `util.GetMatchNumber`, inside `FindMatchIn` | `internal/util/util.go:321` |
| 20 non-test `FindByName` calls in 13 files | grep |
| `attack`: the named branch uses `FindByName`; `*`, `*mob` and `*user` pick from unfiltered lists | `internal/actions/combat_attack.go:30-109`; callers `usercommands/attack.go:95`, `mobcommands/attack.go:42` |
| 11 melee special moves (bash, drain, gore, grapple, kick, maul, pounce, rake, taunt, throttle, trip) resolve a name out of combat through `StageMeleeTarget`, then `ResolveTargetActor` | `usercommands/bash.go:15`; `actions/melee_target.go:158-181`; `actions/target_resolution.go:73-84` |
| `shoot` resolves its target twice: once for the pre-fire guards, once in `ExecuteFire` | `usercommands/shoot.go:52`, `:628-634`; `actions/combat_fire.go:182-187` |
| Player commands through `ResolveTargetActor`: look, consider, give, show, talk, ask, party invite, report, plant, steal, shadow, target | `usercommands/look.go:81`, `consider.go:28`, `give.go:73`, `show.go:43`, `talk.go:43`, `ask.go:94`, `party.go:182`, `report.go:60`, `skill.skullduggery.plant.go:96`, `skill.skullduggery.steal.go:84`, `skill.skullduggery.shadow.go:57`, `target.go:62` |
| Other player lookups: give checks the recipient phrase, sell strips a leading merchant name, buy resolves `from <merchant>`, follow resolves its target | `give.go:376`; `sell.go:127`; `actions/buy.go:322`; `modules/follow/follow.go:410` (player), `:536` (mob) |
| Staff lookups: six admin commands, and moderation, which falls back to a global name lookup | `admin.ai.go:30`, `admin.buff.go:81`, `admin.command.go:43`, `admin.paz.go:25`, `admin.skillset.go:46`, `admin.zap.go:26`; `moderation_target.go:12-16` |
| Self checks use `FindByName` only to tell "that's you" from "not here" | `target.go:67`, `skill.skullduggery.shadow.go:62`, `actions/buy.go:333`, `actions/melee_target.go:175` |
| The parser's creature adapters are reached by no player command; `get` uses the parser for containers and corpses only | `internal/parser/adapters.go:92-111`; `usercommands/get.go:320-332` |

### Who sees a hidden creature

| Fact | Evidence |
|---|---|
| "Also here" skips a hidden player or mob unless the viewer has a pet AND see-hidden. Three copies | `internal/rooms/roomdetails.go:254`, `:299`, `:317` |
| These are the only see-hidden checks in the code | grep `SeeHidden` |
| **No player can own a pet in dogmud.** No code assigns `Character.Pet` (the same search finds `c.Charmed = `, so it could match); no dogmud data has `pettype:`; `GetPetCopy` is called only by the shop listing. So "Also here" has never shown a hidden creature to anyone | grep; `usercommands/list.go:373`; `characters/shop.go:22` |
| The pet check is an upstream leftover: before GoMud commit `463a76727` (#130, 2024-10-10) see-hidden was a pet power, `Pet.HasPower(pets.SeeHidden)`. #130 moved it to a buff flag and kept the `Pet.Exists()` guard | `git show 463a76727 -- rooms/rooms.go` |
| See-hidden sources: buff 53 Veil Sight ("revealing hidden creatures"), buff 65 Cat's Eye Draught, and the mutations chameleon-skin, compound-eyes, discorporation, second-sight and tremorsense. Species 32 wraith and 33 spectre carry buff 53 | grep `see-hidden` |
| Charmed mobs are listed after the other mobs | `roomdetails.go:287`, `:367` |
| Players hide with `sneak`; entering the hidden state adds buff 9 | `usercommands/usercommands.go:209`; `hooks/Awareness_Cascades.go:57` |
| Buff 9 has no duration; it ends on combat (`cancel-on-combat`) or when an action cancels it | `buffs/9-hidden.yaml` |
| Five mobs re-hide through an idle `sneak`: Thornwall Highwayman (90), Smuggler Runner (245), Torvan Cresk (249), Chrysalis Skulker (270), Chrysalis Phantom (272) | `mobs/thornwall_outskirts/90-thornwall_highwayman.yaml:17`; `mobs/thornwall_city/245-*.yaml:14`, `249-*.yaml:15`, `270-*.yaml:15`, `272-*.yaml:16` |
| **Torvan Cresk is not hostile** (`hostile: false`, `behavior_archetype: noncombat_questgiver`), spawns in room 498, and quest 14 tells players to "Fight him, take the key". His dialogue grants and requires no quest tokens | `mobs/thornwall_city/249-torvan_cresk.yaml:4-15`; `rooms/thornwall_city/498.yaml`; `quests/14-the_undertow.yaml:31`; `dialogue/thornwall_city/249.yaml` |
| `search` spots hiders by an opposed contest and shows the searcher their names marked "(hiding)", but does not end their hiding | `internal/actions/search.go:39`, `:231-275` |
| A sneaking player who moves into a room can be spotted by those already there, which ends their hiding for everyone | `usercommands/go.go:555-600` |

## Owner rulings, 2026-09-11

1. A cast aimed at someone the caster cannot see is **refused**. Nothing is
   spent and the target is told nothing.
2. **Every targeted cast needs sight**, whether by typed name or at your foe
   with no name. Only self and area casts work in the dark.
3. **"Something" is the one word for every unseen actor**, spells included:
   "Something's Cleansing Wave purges the toxins from your body."
4. **Infrared sees shapes, not names.** It may cast at the foe it is fighting
   and at `shape`, `2.shape` or `shape#2`. A typed name is refused.
5. Names are hidden **on the shared narration seam**, not by writing a dark copy
   of each line. The owner accepted that this pulls part of 5b into this slice.
6. **Callers hand the seam the names** they put in the sentences.
7. Mobs perceiving darkness is **slice F**, right after this one.
8. **One rule decides who you perceive**, for the room listing and for
   targeting alike, and the pet check is dropped.
9. **The shared finder covers every player command that names a creature**,
   so `look <hidden creature>` stops giving them away. Pets are not in the
   game, so dropping the pet check costs nothing.
10. **A player's successful `search` ends a hider's hiding for everyone**, the
    same way walking in and spotting them already does. Torvan Cresk keeps his
    idle `sneak`.

## Design

### 1. Participant sight

A new predicate in `internal/messaging/predicates.go`:

```go
// ParticipantSight is what a party to an event makes out of the other party.
func ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision
```

- `SightFull` when `CanSeeSightImpairedOnly` is true.
- `SightShapes` when not full, not blinded, and the observer has
  `infraredvision` from any source.
- `SightNone` otherwise.

**Sleep is deliberately not a factor**, for the reason the melee darkness
rewrite already records: a sleeper struck in a lit room must be told what hit
them. Observers who are not a party to the event keep today's judgement
(`CanSeeClearly` / `CanSeeShapes`), so a sleeper still receives no room lines.

It has two users: cast admission (section 4) and the seam's personal lines
(section 2).

### 2. The seam hides names per viewer

`messaging.Audience` gains two fields:

```go
ActorName string // exactly as it appears in the sentences, plain or tagged
ActeeName string
```

A new routine does the swapping:

```go
func HideNames(text string, names []string, d SightDecision) string
```

- `SightFull` returns the text unchanged.
- `SightShapes` replaces each name with "a figure"; `SightNone` with
  "something". The replacement is wrapped in `<ansi fg="combat-anon">`, as
  `Anonymize` does.
- A name matches as an **exact substring**, longest name first. When a name
  begins or ends with a letter or digit, the neighbouring character must not be
  one, so "Kesh" does not match inside "Keshara". A following `'s` stays:
  "Kesh's" becomes "something's".
- When the match is the whole content of an identity tag (`username`,
  `mobname` with any suffix, `petname`), the tag goes with it, so the output
  never nests a `combat-anon` tag inside a name tag.
- The replacement is **capitalized at a sentence start**: the start of the
  text, or after `.`, `!` or `?` and a space, looking through ANSI tags. So
  "⚡ SWEEP! Kesh dodges" becomes "⚡ SWEEP! Something dodges".
- Empty names are ignored.

`messaging.Broadcaster` changes:

- `SendTextVisual` is replaced in the interface by
  `SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int)`.
  Per recipient: clear sight gets the text; shapes only gets `HideNames` with
  `SightShapes` and then `Anonymize`; neither gets nothing. `*rooms.Room`
  implements it on the existing `sendTextVisualJudgedBy` path, and
  `SendTextVisual` itself stays as it is for every other caller.
- A new method, `ParticipantSight(userId int) SightDecision`, looks the user up
  and applies section 1 to this room. An unknown user returns `SightFull`.

`SendTrio` then renders each role for its reader:

| Line | Hides | Judged by |
|---|---|---|
| Actor | `ActeeName` | `Room.ParticipantSight(ActorId)` |
| Actee | `ActorName` | `Room.ParticipantSight(ActeeId)` |
| Observer | `ActorName` and `ActeeName` | each observer, through `SendTextVisualHidingNames` |

A nil `Room` hides nothing, because there is no room to judge light by.

**Every existing call passes names.** All 132 `SendTrio` calls gain
`ActorName` and `ActeeName`; an event with no actee (salvage) writes
`ActeeName: ""` explicitly. The Audience guard is extended to require both
keys on every `messaging.Audience` literal, so a silent omission cannot slip
back in; an explicit empty string is the considered absence, the same idea as
`NoLine`.

**In a lit room nothing changes**: clear sight returns the text unchanged, so
every existing line is byte-identical there.

The hand-written `canSeeInDark` branches in the special-move files stay. Their
dark text names no one, so the swap finds nothing to replace.

### 3. Lines that move onto the seam

| Site | Actor (reads) | Actee (reads) | What players notice |
|---|---|---|---|
| `applyPlayerEffect` cross-casts: damage, purge, heal, buff, shield, default | caster | target | a target who cannot see reads "Something's Heal envelops you in healing energy." A caster whose target went dark mid-cast reads "something" |
| `resolveMobSpellAgainstPlayer` | the mob (no line) | target player | the target reads "Something" instead of the mob's name |
| `sendSpellChannelDefenceMessages` | attacker | defender | a defender who cannot see is now told they defended, with the attacker hidden, instead of nothing |
| `resolvePurgeAffliction` cross-cast | caster | target | as for purge above |
| Crit counters in `dispatchCritAndMessaging` (RIPOSTE, SWEEP, SHIELD SLAM) | defender, the counterer | attacker | **the room line becomes visual**: observers who cannot see get nothing, infrared observers read "a figure". Each combatant reads the other hidden when they cannot see |
| `DispatchCounterMessages` (23 callers) | counterer | countered | as for crit counters |
| `fireSpellCounterTier` | defender | caster | as for crit counters |

Self-cast lines are untouched: there is no other party to hide.

For the counters, `combat.CounterResult` gains `CountererName` and
`CounteredName`, set in `ExecuteCounter` from the same names it tokenizes, so
the dispatchers can hand them to the seam. The crit dispatch already holds both
characters.

### 4. Targeted casts need sight

Applies to **player casters only**; mob casts are unchanged until slice F. The
check runs inside `InitiateCast`, before target resolution, for harmsingle,
harmmulti and helpsingle spells. Help-multi, area and neutral casts are
unchanged.

| Caster's sight | No name typed | Own name (help) | `shape`, `N.shape`, `shape#N` | Any other name |
|---|---|---|---|---|
| Clear | harm: your foe, then your party leader's foe. Help: yourself | yourself | the Nth figure | a creature you perceive (section 5) |
| Shapes only | harm: **your own foe only**. Help: yourself | yourself | the Nth figure | refused, with a hint |
| None | harm: refused. Help: yourself | yourself | refused | refused |

**Figures** are the creatures the caster perceives (section 5), excluding the
caster: other players in room order, then mobs in room order. A helpsingle
spell on a mob figure keeps today's rule: only the caster's own companion.

**Your own foe only, under shapes.** Ruling 4 allows "the foe it is fighting";
the party leader's foe needs clear sight.

Refusal lines, each sent with `RefusalExplained` so the generic line does not
follow. Nothing is spent, because the cast command returns before any cost:

| Case | Line |
|---|---|
| Shapes only, a name typed | `You can only make out shapes here. Try cast <spell> shape or cast <spell> 2.shape.` |
| No sight, no name (harm) or `shape` typed | `You can't see anything to aim at.` |
| No sight, a name typed | `You don't see them here.` |

**Sight is judged when the cast starts.** A target who goes dark mid-cast still
receives the spell, and the lines hide names as in section 3.

### 5. One rule for who you perceive

```go
// Perceives reports whether c can make out other in the same room.
func (c *Character) Perceives(other *Character) bool
```

True when `other` is `c`, when `other` is not hidden, or when `c` has
`see-hidden` from any source. **No pet is required.**

The rule reaches every lookup through one finder:

- **The three "Also here" checks** in `roomdetails.go` call `Perceives`,
  replacing the pet condition.
- **`(r *Room) FindByNameSeenBy(viewer *characters.Character, searchName string, findTypes ...FindFlag)`**
  skips creatures the viewer does not perceive **before** matching, so
  `2.guard` counts only the guards you perceive. A nil viewer behaves exactly
  like `FindByName`.
- **`ResolveTargetOptions.Viewer`**, a new field, passes the viewer through.

**Every player command that names a creature passes the player as viewer:**

| Group | Commands |
|---|---|
| Combat | `cast` (typed names and the figure list), `attack` (named, and the `*`, `*mob`, `*user` pools), the 11 melee special moves through `StageMeleeTarget`, `target`, `shoot` (both resolutions) |
| Looking and talking | `look`, `consider`, `show`, `talk`, `ask`, `report` |
| Trade and social | `give` (both lookups), `buy ... from`, `sell` (the leading merchant name), `party` invite, `follow` |
| Skullduggery | `plant`, `steal`, `shadow` |

Each command keeps its existing "not here" wording, so a hidden creature reads
exactly as an absent one.

**These pass no viewer, on purpose:**

| Caller | Why |
|---|---|
| The six admin commands and moderation | staff tools must reach everyone; moderation already falls back to a global name lookup |
| Self checks (`target`, `shadow`, `buy`, `StageMeleeTarget`) | they only ask "is that name me?", and you always perceive yourself |
| Mob callers, including the mob side of `follow`, `shoot` and `attack` | mobs perceiving anything is slice F |

**A root guard keeps it that way.** It lists every non-test call to
`FindByName`, `FindByNameSeenBy` and `ResolveTargetActor` with its
classification: passes a viewer, or exempt with one of the reasons above. A new
call that is not in the registry fails the guard, which is the mistake this
change makes easy: adding a command that names a creature and forgetting the
viewer.

### 6. Help

`templates/help/cast.template` gains a section after Usage:

```
━━━ Targets ━━━

  cast <spell> <target>   Aim at someone by name.
  cast <spell>            A harmful spell aims at your foe; a helpful one at you.

You must be able to see your target. In the dark you can cast only on
yourself or on the whole room. If you can make out shapes but not faces,
aim at your foe, or at a shape: cast <spell> shape, cast <spell> 2.shape.
```

### 7. A successful search ends hiding (owner ruling 10, option A)

**The problem.** Torvan Cresk re-hides through his idle `sneak` and is not
hostile, so combat never ends his hiding. Under section 5, `attack torvan` and
`talk torvan` read "You don't see them here." to anyone without see-hidden,
while quest 14 tells players to fight him.

**What already exists.** A player walking into a room rolls against each hidden
occupant, and a win ends that occupant's hiding for everyone
(`usercommands/go.go:616-700`). So a player who wins the entry roll reaches
Torvan today. One who loses it has no second try short of leaving and coming
back, because `search` rolls the same contest (`spotsHider`) but only reports
the find.

**The change.** When a **player** searches and wins against a hidden creature,
`actions.Search` ends its hiding exactly as the entry roll does:

- A hidden player: `Awareness.TransitionToRevealing` with
  `awareness.TriggerObserverSearch`, then `SetMiscData("sneaking", nil)`, then
  the hider is told "`<searcher>` searches the room and spots you!", or
  "Someone searches the room and spots you!" when the hider cannot see clearly.
  The same shape as `go.go:630-647`.
- A hidden mob: `Awareness.TransitionToRevealing` with
  `awareness.TriggerObserverSearch`, as `go.go:686-687`.
- The searcher's "(hiding)" report is unchanged.

The Awareness cascade cancels buff 9 (`hooks/Awareness_Cascades.go`), so the
creature is listed and nameable for the whole room until it sneaks again.
**Torvan keeps his idle `sneak`.** A mob's search is unchanged: mobs perceiving
hidden creatures is slice F.

## Tests

Each is written before the change it guards and proven capable of failing by a
deliberate break that still compiles. Every task runs `go test .` in the repo
root as well as its own packages, because the root guards read these files.

| Covers | Where | Asserts |
|---|---|---|
| Participant sight | messaging | lit, dark, night vision, infrared, blinded; sleeping in a lit room is full sight, sleeping in the dark with infrared is shapes |
| `HideNames` | messaging | full leaves text unchanged; shapes and none swap; tagged and bare names; a whole identity tag is replaced, never nested; possessive; capital after `! ` and at the start through a tag; "Kesh" not inside "Keshara"; longest name first; empty name ignored |
| Seam | messaging, `trio_test.go` | each role hides the other party by its own reader's sight; the observer line hides both; nil room hides nothing; no names given is byte-identical |
| Audience guard | root | a literal missing `ActorName` or `ActeeName` fails, shown by removing one from a real call |
| `Perceives` | characters | self; not hidden; hidden with see-hidden from a buff and from a mutation; hidden without it; **no pet needed** |
| Listing and lookup agree | rooms | the three "Also here" checks call `Perceives` itself, so the listing cannot drift from it (no room test can build "Also here": `GetDetails` needs a full user and world). The lookup test pins `FindByNameSeenBy` to `Perceives` for hidden, see-hidden and neither; `2.name` skips an unperceived first match; a nil viewer matches `FindByName` |
| Search ends hiding | actions | a player's find ends a hidden mob's and a hidden player's hiding and tells the player hider; a mob's find ends nothing |
| Lookup registry guard | root | an unregistered `FindByName` or `ResolveTargetActor` call fails, shown by adding one; a registered viewer call that drops its viewer fails |
| Cast admission | actions, `cast_test.go` | one case per cell of the section 4 table; conviction and cooldown untouched on refusal; a mob caster in the dark is unaffected |
| Hidden targets | actions, usercommands | one case per command group in section 5: a hidden creature cannot be named without see-hidden and can with it; each keeps its "not here" wording; an admin command still reaches a hidden player |
| Moved lines | hooks, dark room via the existing `darken` helper | for each site in section 3: a reader who cannot see reads "Something" or "something", an infrared reader reads "a figure", a lit reader reads the name |
| SWEEP | hooks | the crit room line reaches no observer who cannot see; an infrared observer reads "a figure"; each combatant's line hides the other in the dark |
| Goldens | existing | byte-identical, since every existing golden renders in the light |

## Playtest gate

Two lanes, reusing the 5a staging, plus a third once the open decision is made.
Every command below is typed by a tester.

**Lane 1, dark cave**, room 3101 (Cave Mouth, cave biome, unlit). Actors:

- the caster, on `m2-actor`, which knows Heal and Conviction Surge, plus one
  harmsingle spell granted by the goals file's `grant_spells` overlay;
- the witness, on `m2-witness`: no special senses, carries a Cat's Eye Draught
  (item `30047`);
- an infrared tester on a new profile that knows Heal and carries buff 85. The
  plan proves that profile loads with the buff active before the lane relies
  on it.

| Step | Shows |
|---|---|
| Caster types `cast heal <witness>` and the granted harmful spell at the witness | both refused with "You don't see them here."; conviction unchanged |
| Caster types `cast heal` with no target | the self-cast still works in the dark |
| Infrared tester types `cast heal <witness>` | refused, with the shape hint |
| Infrared tester types `cast heal shape`, then `cast heal 2.shape` | the two casts land on the two other players, one each, in the room's player order; the witness, who cannot see, reads "Something's Heal envelops you in healing energy." |
| Caster drinks the draught and types `cast heal <witness>` | lands; the witness, still unable to see, reads "Something's Heal", not the caster's name |
| Infrared tester watches that heal | reads "a figure" for both parties |

**Lane 2, sneaking**, any lit room. Actors: a sneaker on the `mid` profile
(skullduggery 8), and the witness with Veil Sight granted by overlay and **no
pet**.

| Step | Shows |
|---|---|
| Sneaker types `sneak`; witness types `look` | the sneaker is not listed |
| Witness types `look <sneaker>`, `consider <sneaker>`, `attack <sneaker>` and `cast veil-sight <sneaker>` | each reads as if nobody by that name is there |
| Witness types `cast veil-sight`, then `look` | the sneaker is listed, with no pet |
| Witness types `look <sneaker>` and `consider <sneaker>` | both resolve |

**Lane 3, quest 14**, room 498. One actor on `m2-witness` (no see-hidden),
placed in the room on step `confront`, so no entry roll happens.

| Step | Shows |
|---|---|
| Actor types `look` until Torvan Cresk is not listed | he has sneaked |
| Actor types `attack torvan` | reads as if nobody by that name is there |
| Actor types `search` until the find lists "Torvan Cresk (hiding)" | the find |
| Actor types `look`, then `attack torvan` | he is listed, and the attack starts |

**SWEEP is covered by unit tests only**: a defensive crit cannot be staged on
demand.

## Deliberately not in this slice

| Item | Where it belongs |
|---|---|
| Mobs perceiving darkness or hidden creatures, restoring innate night vision for feline and arachnid mobs | slice F |
| The hand-written `canSeeInDark` branches ignore infrared, so an infrared player on the receiving end of a special move reads "Something" where the seam would say "a figure" | 5b, when special moves move onto the narration core |
| Melee in the dark is otherwise unchanged: blind swings at your foe still work | owner ruling |
| GMCP bypassing darkness | its own slice |
| The pet system itself, which no player can reach | not examined; filed |
| Red names, the buff expiry notice, Purge Affliction targeting and the buff flag guard | slices B, C and D+E |
| Neutral spells, which aim at items and corpses rather than creatures | not a creature target |

## Risks

- **Grammar around a swapped name.** Lines were written around names, so some
  read awkwardly: "stomps on the downed something". The dark render test for
  the moved lines lists each rendering, and the awkward ones go to the owner.
  They are not rewritten in this slice.
- **Players will notice behaviour changes.** No more blind casting. A hidden
  creature cannot be looked at, talked to, traded with or attacked without
  see-hidden. A healer cannot heal a sneaking party member without it. Veil
  Sight, the draught and the five mutations now reveal sneakers, so sneaking
  is weaker against them.
- **A hidden creature that never fights needs finding first.** Torvan Cresk is
  the one found. Walking in and winning the spotting roll, or a successful
  `search` (section 7), ends his hiding; a player who fails both can still
  leave and come back.
- **An exact-string swap misses a name the caller changed** between the sentence
  and the Audience, such as a different case. The per-site dark tests catch it.
- **The interface change and 132 edited calls touch frozen M2 guards.** Running
  `go test .` in every task surfaces that immediately.

## How we know it worked

- `go test ./...` is green, and every new test was seen to fail first.
- The existing goldens are byte-identical.
- The server boots.
- All three playtest lanes show every step above; SWEEP's limit is stated.
