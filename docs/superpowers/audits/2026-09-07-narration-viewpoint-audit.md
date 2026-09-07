# Narration viewpoint audit

**Date:** 2026-09-07
**Stage:** M1 of the messaging unification arc
**Scope:** every hand-rolled narration site the scanner flagged as missing a viewpoint

> Point-in-time. True as of the date above and **not** kept current. Verify
> against the code before acting on any row.

## What this is

DOGMud narrates a player-visible event from up to three viewpoints:

| Role | Who | How it is sent |
|------|-----|----------------|
| **actor** | the one doing it | `user.SendText(...)`, "You kick the goblin" |
| **actee** | the one it is done to | `targetUser.SendText(...)`, "Bob kicks you" |
| **observer** | everyone else present | `room.SendTextVisual(...)`, "Bob kicks the goblin" |

`tools/narration_viewpoint_scan.py` finds sites carrying only one or two of the
three. It found **247**. Most are correct: the actee is a mob and has no client
to send to, the target is a door rather than a person, or the line is a refusal
that concerns nobody but the actor.

This audit rules each of the 247 individually, so M2 knows which sites are
already right and must be preserved, and which are defects to be fixed.

## Result

| Verdict | Sites |
|---------|-------|
| `correct` (the missing viewpoint should be missing) | **240** |
| `gap` (a genuine defect) | **7** |
| `unsure` | **0** |

The seven gap sites are **five distinct defects** (two of them span two sites
each).

## The headline: every gap is a copied code path

This is the finding that matters for the arc.

Not one of the five defects came from an author deciding a viewpoint should not
exist. Every single one is a **duplicated code path that copied the mechanical
effect and dropped the narration**:

| Defect | The duplication |
|--------|-----------------|
| salvage | an `else if actor.IsPlayer()` split put the room broadcast on the mob branch only |
| buff spells | `case "buff":` did not copy the room send its sibling `case "heal":` has |
| Resonant Larynx | a second fan-out loop copied `AddCondition`/`AddBuff` and dropped the `SendText` |
| arm-slot equip | the arm-slot path duplicates the shared equip path minus the room line |
| admin zap | the engaged-target path duplicates the explicit-target path minus the victim line |

At every one of these sites the effect and its narration are two separate
hand-written things sitting next to each other. Copy the effect, lose the
narration, and nothing complains: no test, no compiler error, no runtime
warning. The player simply stops being told something.

**That is the case for M2's central seam.** If applying an effect and narrating
it were one call, none of these five could have happened. This audit is the
evidence that the arc fixes a real failure mode rather than tidying style.

## The pattern that is already right

Against those five, roughly forty sites across eight files already implement the
full trio correctly and identically, the special-move verbs:

`bash` · `drain` · `kick` · `maul` · `throttle` · `pounce` · `rake` · `trip` · `grapple`

Every one has the same shape:

```go
// actee looked up once, nil when the target is a mob
var targetUser *users.UserRecord
if target.UserId > 0 {
    targetUser = users.GetByUserId(target.UserId)
}

user.SendText(...)                                    // actor    (unconditional)
if targetUser != nil {
    targetUser.SendText(...)                          // actee    (skipped for mobs)
}
room.SendTextVisual(..., user.UserId, target.UserId)  // observer (unconditional)
```

Three separate rulers on three separate slices ruled this family identically
without conferring, which is a good sign the model holds.

**M2 is extracting this pattern, not inventing one.** The only drift is the
variable name: `bash.go` calls it `targetUser`, the rest call it `targetChar`.
The picker should settle on one.

## Confirmed defects

Each was verified against source by the reviewer, independently of the agent
that first reported it.

### 1. A player salvaging a corpse is invisible to the room

`internal/actions/salvage.go:202` and `:207`

```go
if actor.IsPlayer() {
    if len(recovered) > 0 { actor.SendText(...) } else { actor.SendText(...) }
} else if room.PlayerCt() > 0 {
    room.SendTextVisual(messaging.CategoryMobIdle,
        "%s kneels by the carcass and cuts strips of hide from it.", actor.GetName())
}
```

The room broadcast sits on the `else` branch, so it fires only for **mobs**. A
mob butchering a corpse is narrated to everyone present; a player doing exactly
the same thing produces no room text at all. Backwards from what you would
expect.

### 2. Buff spells are invisible to the room

`internal/hooks/spell_resolution.go:1075`

The `heal` arm of the effect switch ends with `sendVisualRoomText(room, ...)`
("envelops X in healing light"). The `buff` arm immediately below sends to the
caster and the target, then falls straight through to `case "shield":` with no
room send. Healing an ally is public; buffing one is not. Same switch, same
function.

### 3. Resonant Larynx buffs the party twice and tells them once, in both directions

`internal/usercommands/rally.go:72` and `internal/usercommands/warcry.go:76`

The `shout-stacking` mutation folds a rally into a war cry and a war cry into a
rally. Both main loops notify each party member. Neither fold loop does:

```go
// main loop: notifies
memberUser.Character.AddBuff(79, false)
memberUser.SendText(..., "'s warcry stirs your blood!")

// Resonant Larynx fold loop: silent
memberUser.Character.AddCondition(characters.ConditionRally, rd, rb, "rally")
memberUser.Character.AddBuff(80, false)
applyRallyToCompanions(memberUser, room, rb, rd)
```

The same omission is mirrored in both files, so party members receive the second
buff with no indication anything happened. Two rulers on different slices found
the two halves independently.

### 4. Swapping an item out of an arm slot is invisible to the room

`internal/usercommands/equip.go:218`

The shared equip path at `:272-282` sends the displaced-item line to the actor
**and** broadcasts "X removes their Y and stores it away" to the room. The
arm-slot path at `:216-221` sends only the actor line.

### 5. An admin zapping their engaged target never tells them

`internal/usercommands/admin.zap.go:82`

The explicit-target path sends all three, including `u.SendText(...)`, "X zaps
you with a bolt of lightning!". The engaged-target path sends actor and room
only. The victim drops to 1 health with no message explaining why.

## Conventions M2 has to settle

Ruled `correct`, but each names a decision the arc should make explicitly rather
than leave to each author.

**Detail lines riding on a parent event.** `throttle.go:77` tells the actor and
actee that a spell collapsed, while the room sees only the physical hit at
`:84`. `grapple.go:108` and `:131` do the same with prone/exposed flavor. The
parent event has a full trio and the supplementary line has fewer viewpoints.
Defensible, but currently a per-author choice. Decide whether a detail line
inherits its parent's viewpoint set.

**The quest bridge has no actee seam at all.** `internal/questengine/bridge.go`
exposes `SendText` (actor) and `RoomText` (observer) and nothing else, so a
quest script cannot message a third party. Not a defect in any existing quest,
but it means quest-authored narration cannot express the trio, and M2's seam
should close the hole.

**Two names for the actee.** `targetUser` in `bash.go`, `targetChar` everywhere
else, for the identical concept.

## The scanner is a candidate generator, not an oracle

**64 of the 247 labels (26%) were wrong** and were corrected by reading the
code. The scanner is three regexes:

```python
ACTOR    = r'\b(?:user|actor)\.SendText\('
OBSERVER = r'\broom\.SendText(?:Visual)?(?:ToUser)?\('
ACTEE    = r'\b(target\w*|victim\w*|defender\w*|recipient\w*|other\w*|receiver\w*)\.SendText\('
```

They fail in three distinct ways, and knowing which one is in play tells you
what a wrong label means:

- **Wrapper-blind.** `OBSERVER` matches only a literal `room.SendText*`.
  Everything in `internal/hooks/` broadcasts through
  `sendVisualRoomText(room, ...)`, a thin delegate defined at
  `NewRound_DoCombat_helpers.go:401`, so those sites look observer-less when
  they are not. This produced four false `gap` candidates in the spell
  resolution switch alone (`spell_resolution.go:983`, `:1004`, `:1034`,
  `spell_purgeaffliction.go:22`, all actually full trios).
- **The actee is matched by variable name.** A site that sends to `u` or
  `memberUser` rather than `target*`/`victim*`/`defender*` is invisible as an
  actee. `admin.zap.go:44` was labelled `actor+observer`; its actee is `u` at
  `:46`. `look.go:112`, `attack.go:338`, `combat_counter.go:83` and `:232`,
  `warcry.go:42` and `target.go:198` all failed this way.
- **False pairings from the 16-line forward window.** An error-branch line gets
  paired with a success-path message further down the same function that the
  error branch can never reach. This is the largest family: 21 actor-only
  refusals in slice 1 alone were mislabelled `actor+observer`. `ban.go:84` was
  labelled `actor+actee` because of a message at `:90` unreachable from it.

Rerun it to regenerate candidates; never treat its labels as findings. It
deliberately ignores `messaging.Category`, which cannot classify these sites, because
`give.go` sends one event across two categories.

## Method

Four read-only agents ruled balanced slices of the 247 (62/62/62/61)
independently, returning rulings rather than writing to shared files. The
reviewer pre-ruled two sites as a calibration baseline, verified every `gap`
claim against source before accepting it, resolved both `unsure` rulings, and
spot-checked high-risk `correct` rulings (`look.go:112`, `skill.cast.go:387`) to
confirm the rulers were not rubber-stamping.

The special-move verbs appear in three different slices and were ruled
identically by three different agents, which is the cross-slice consistency
check.

## The rulings

Columns: whether each viewpoint is actually sent for that event (not what the
scanner guessed), the verdict, the number of distinct `messaging.Category`
values the event spans, and the reason. Sorted by path. **A** = actor,
**E** = actee, **O** = observer.

| Site | Event | A | E | O | Verdict | Cats | Reason |
|---|---|---|---|---|---|---|---|
| `actions/combat_counter.go:83` | counter-swing lands after a defended special move | Y | Y | Y | correct | 1 | full trio: actee at :78, observer at :87 |
| `actions/combat_counter.go:232` | counter-taunt after a defy-crit | Y | Y | Y | correct | 1 | full trio: actee above, observer at :235 |
| `actions/defuse.go:161` | trap fires after a failed defuse | Y | N | Y | correct | 2 | target is the trap, not a character |
| `actions/defuse.go:242` | no exit or container matches the target | Y | N | N | correct | 1 | actor-only refusal |
| `actions/defuse.go:254` | container trap disarmed | Y | N | Y | correct | 2 | target is the lock |
| `actions/defuse.go:269` | exit trap disarmed | Y | N | Y | correct | 2 | target is the lock |
| `actions/forage.go:78` | player forages the ground | Y | N | Y | correct | 2 | solo action, no target person |
| `actions/mutation_cocoon.go:42` | cocoon mutation self-buff, drops room aggro | Y | N | Y | correct | 1 | self-buff, no actee |
| `actions/mutation_venom_coat.go:23` | venom-coat self-buff | Y | N | Y | correct | 1 | self-targeted buff |
| `actions/plant.go:202` | failed plant on a mob, caught | Y | N | Y | correct | 2 | actee is a mob |
| `actions/plant.go:431` | failed plant in a container, spotted | Y | N | Y | correct | 2 | container has no owner; the spotter is an observer |
| **`actions/salvage.go:202`** | **salvage recovers materials** | **Y** | **N** | **N** | **gap** | **2** | **room broadcast is on the `else if` mob branch, so a player salvaging is invisible** |
| **`actions/salvage.go:207`** | **salvage recovers nothing** | **Y** | **N** | **N** | **gap** | **2** | **same asymmetry as :202** |
| `actions/search.go:102` | search refused, on cooldown | Y | N | N | correct | 1 | actor-only refusal |
| `actions/search.go:112` | player begins searching the room | Y | N | Y | correct | 2 | solo action |
| `actions/sleep.go:60` | sleep refused, in combat | Y | N | N | correct | 1 | actor-only refusal |
| `actions/steal.go:286` | failed steal from a mob, caught | Y | N | Y | correct | 2 | actee is a mob |
| `actions/steal.go:554` | failed steal from a container, spotted | Y | N | Y | correct | 2 | container has no owner |
| `behaviortree/actions_dialogue.go:37` | NPC dialogue delivered to the asking player | Y | N | Y | correct | 2 | actee is a mob |
| `behaviortree/actions_dialogue.go:99` | private hint/flavor to the triggering player | Y | N | N | correct | 1 | generic private-text utility; authors pair it with a separate room action |
| `hooks/pinnacle_tick.go:524` | hunger-item fallback flavor | Y | N | N | correct | 1 | caller passes `room=nil`; a deliberately private sensation |
| `hooks/pinnacle_tick.go:527` | sentient item chatter | Y | N | Y | correct | 1 | item speech has no actee |
| `hooks/spell_foldanchor.go:18` | Chrysalis fold anchor set | Y | N | Y | correct | 1 | self-targeted spell |
| `hooks/spell_purgeaffliction.go:22` | purge cast on another player | Y | Y | Y | correct | 1 | full trio; observer via `sendVisualRoomText`, which the scanner cannot see |
| `hooks/spell_resolution.go:983` | damage spell hits a player | Y | Y | Y | correct | 1 | full trio via the wrapper at :988 |
| `hooks/spell_resolution.go:1004` | purge spell on a player | Y | Y | Y | correct | 1 | full trio via the wrapper at :1014 |
| `hooks/spell_resolution.go:1034` | heal spell on a player | Y | Y | Y | correct | 1 | full trio via the wrapper at :1044 |
| **`hooks/spell_resolution.go:1075`** | **buff spell on a player** | **Y** | **Y** | **N** | **gap** | **1** | **sibling `case "heal"` calls `sendVisualRoomText` at ~:1044; `case "buff"` never does** |
| `questengine/bridge.go:201` | generic quest `SendText` passthrough | Y | N | N | correct | 1 | plumbing, not a narration site; viewpoint is the quest author's call. **Note: the bridge exposes no actee seam at all** |
| `usercommands/admin.item.go:127` | admin conjures an item | Y | N | Y | correct | 2 | no player target |
| `usercommands/admin.mob.go:203` | admin spawns a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/admin.paz.go:27` | paz target not found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/admin.paz.go:33` | admin paz-illuminates a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/admin.paz.go:43` | admin paz-illuminates a player | Y | Y | Y | correct | 2 | full trio |
| `usercommands/admin.paz.go:58` | admin paz-illuminates self | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/admin.redescribe.go:50` | admin redescribes an item | Y | N | Y | correct | 2 | target is an item |
| `usercommands/admin.rename.go:52` | admin renames a backpack item | Y | N | Y | correct | 2 | target is an item |
| `usercommands/admin.skillset.go:54` | skillset usage help | Y | N | N | correct | 1 | admin/system output |
| `usercommands/admin.skillset.go:56` | skillset usage help | Y | N | N | correct | 1 | admin/system output |
| `usercommands/admin.skillset.go:58` | skillset usage help | Y | N | N | correct | 1 | admin/system output |
| `usercommands/admin.skillset.go:73` | confirmation, all skills set | Y | Y | N | correct | 1 | invisible stat edit, no public component |
| `usercommands/admin.skillset.go:89` | confirmation, one skill set | Y | Y | N | correct | 1 | invisible stat edit, no public component |
| `usercommands/admin.spawn.go:50` | admin conjures a container | Y | N | Y | correct | 2 | target is a container |
| `usercommands/admin.spawn.go:76` | admin conjures gold | Y | N | Y | correct | 2 | gold is not a person |
| `usercommands/admin.spawn.go:89` | spawn fails, hand-wave flourish | Y | N | Y | correct | 2 | no target |
| `usercommands/admin.zap.go:28` | zap target not found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/admin.zap.go:34` | admin zaps a named mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/admin.zap.go:44` | admin zaps a named player | Y | Y | Y | correct | 2 | full trio; actee `u` at :46, outside the window and not name-matched |
| `usercommands/admin.zap.go:60` | zap with no target, not in combat | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/admin.zap.go:67` | engaged mob target vanished | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/admin.zap.go:70` | admin zaps engaged mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/admin.zap.go:79` | engaged player target vanished | Y | N | N | correct | 1 | actor-only refusal |
| **`usercommands/admin.zap.go:82`** | **admin zaps engaged player** | **Y** | **N** | **Y** | **gap** | **2** | **the parallel explicit-target path at :46 tells the victim; this one drops them to 1 HP silently** |
| `usercommands/afk.go:28` | player returns from AFK | Y | N | Y | correct | 2 | self state change |
| `usercommands/afk.go:43` | goes AFK with a message | Y | N | Y | correct | 2 | self state change |
| `usercommands/afk.go:48` | goes AFK with no message | Y | N | Y | correct | 2 | self state change |
| `usercommands/appraise.go:75` | player pays a merchant to appraise | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/attack.go:251` | player commits to attacking a mob | Y | N | Y | correct | 1 | actee is a mob |
| `usercommands/attack.go:338` | player commits to attacking a player | Y | Y | Y | correct | 1 | full trio when not sneaking: actee :346, observer :350 |
| `usercommands/ban.go:84` | ban fails to persist | Y | N | N | correct | 1 | actor-only refusal; the scanner's "actee" at :90 is on the unreachable success path |
| `usercommands/bash.go:63` | shield bash knocks target down | Y | Y | Y | correct | 2 | full trio, `targetUser` nil-guarded for mobs |
| `usercommands/bash.go:72` | shield bash strikes | Y | Y | Y | correct | 2 | full trio |
| `usercommands/bash.go:84` | shield bash partially lands | Y | Y | Y | correct | 2 | full trio |
| `usercommands/bash.go:96` | shield bash misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/boot.go:32` | admin boots a player | Y | Y | N | correct | 1 | boot is server-wide, not room-scoped; no room to observe |
| `usercommands/break.go:17` | player breaks off combat | Y | N | Y | correct | 2 | no single target; opponents are covered by the room line |
| `usercommands/character.go:292` | player swaps to an alt | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/character.go:437` | player hires an alt as a companion | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/deletecharacter.go:40` | delete confirmation typed wrong | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/dismiss.go:104` | companion dismissed peacefully | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/dismiss.go:137` | charmed companion turns hostile | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/drain.go:73` | drain lands fully | Y | Y | Y | correct | 2 | full trio |
| `usercommands/drain.go:102` | drain lands partially | Y | Y | Y | correct | 2 | full trio |
| `usercommands/drain.go:135` | drain misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/drink.go:169` | drinks a spoiled potion, retches | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/drink.go:171` | same event as :169 | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/drink.go:227` | drinks a potion normally | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/drop.go:91` | drop gold, floor-drop errors | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/drop.go:100` | drop gold succeeds | Y | N | Y | correct | 2 | gold is not a person |
| `usercommands/drop.go:125` | "drop all", nothing dropped | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/drop.go:127` | "drop all" succeeds | Y | N | Y | correct | 2 | items are not a person |
| `usercommands/drop.go:141` | drop item, not found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/drop.go:147` | drop item succeeds | Y | N | Y | correct | 2 | item is not a person |
| `usercommands/eat.go:60` | eat, food spoiled | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/eat.go:67` | eat succeeds | Y | N | Y | correct | 2 | item is not a person |
| `usercommands/emote.go:17` | emote with no text | Y | N | Y | correct | 1 | free-form emote has no target |
| `usercommands/emote.go:29` | emote via alias | Y | N | Y | correct | 1 | free-form emote has no target |
| **`usercommands/equip.go:218`** | **displaced item returned (arm-slot path)** | **Y** | **N** | **N** | **gap** | **2** | **the shared path at :274/:277 broadcasts the same event to the room; this one does not** |
| `usercommands/equip.go:229` | equips an offhand item into an arm slot | Y | N | Y | correct | 2 | solo self-action |
| `usercommands/equip.go:231` | wields a weapon into an arm slot | Y | N | Y | correct | 2 | solo self-action |
| `usercommands/equip.go:274` | displaced item returned (shared path) | Y | N | Y | correct | 2 | solo self-action |
| `usercommands/equip.go:285` | wears a wearable | Y | N | Y | correct | 2 | solo self-action |
| `usercommands/equip.go:293` | wields a non-wearable | Y | N | Y | correct | 2 | solo self-action |
| `usercommands/get.go:54` | floor sweep hits overload | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/get.go:61` | floor sweep, nothing found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/get.go:64` | floor sweep succeeds | Y | N | Y | correct | 2 | items are not a person |
| `usercommands/get.go:386` | corpse gold, none present | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/get.go:394` | corpse gold taken | Y | N | Y | correct | 2 | corpse is not a person |
| `usercommands/get.go:430` | corpse item taken | Y | N | Y | correct | 2 | corpse is not a person |
| `usercommands/get.go:458` | take from pet, overload | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/get.go:469` | take item from pet | Y | N | Y | correct | 2 | pet is a mob |
| `usercommands/get.go:520` | container gold taken | Y | N | Y | correct | 2 | container is not a person |
| `usercommands/get.go:553` | container item taken | Y | N | Y | correct | 2 | container is not a person |
| `usercommands/get.go:580` | room gold, none present | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/get.go:592` | room gold taken | Y | N | Y | correct | 2 | gold is not a person |
| `usercommands/get.go:664` | stashed item retrieved | Y | N | Y | correct | 2 | item is not a person |
| `usercommands/get.go:672` | floor item retrieved | Y | N | Y | correct | 2 | item is not a person |
| `usercommands/get.go:693` | "get" a room noun, refused | Y | N | Y | correct | 2 | fixture is not a person; failure flavor still broadcasts |
| `usercommands/give.go:86` | item transfer to a player failed | Y | N | N | correct | 1 | actor-only error |
| `usercommands/give.go:90` | gives an item to a player | Y | Y | Y | correct | 2 | full trio |
| `usercommands/give.go:105` | gives gold to self | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/give.go:129` | gives gold to a player | Y | Y | Y | correct | 2 | full trio |
| `usercommands/give.go:167` | gold transfer to a mob failed | Y | N | N | correct | 1 | actor-only error |
| `usercommands/give.go:176` | gives gold to a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/give.go:200` | quest engine consumes the item first | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/give.go:221` | item transfer to a mob failed | Y | N | N | correct | 1 | actor-only error |
| `usercommands/give.go:227` | gives an item to a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/give.go:290` | pet owner not found | Y | N | N | correct | 1 | actor-only error |
| `usercommands/give.go:299` | gives an item to a pet | Y | N | Y | correct | 2 | pet has no client |
| `usercommands/go.go:98` | player unlocks a door | Y | N | Y | correct | 2 | acts on an exit, not a character |
| `usercommands/go.go:405` | player sneaks toward an exit | Y | N | N | correct | 1 | deliberately private; sneaking is meant to go unnoticed |
| `usercommands/go.go:410` | player walks through an exit | Y | N | Y | correct | 1 | movement has no target character |
| `usercommands/go.go:897` | player bumps into a wall | Y | N | Y | correct | 2 | acts on no one |
| `usercommands/gore.go:73` | gore hits, knocks down | Y | Y | Y | correct | 2 | full trio, actee nil-guarded for mobs |
| `usercommands/gore.go:95` | gore hits, no knockdown | Y | Y | Y | correct | 2 | full trio |
| `usercommands/gore.go:114` | gore partial hit | Y | Y | Y | correct | 2 | full trio |
| `usercommands/gore.go:135` | gore misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/grapple.go:97` | grapple succeeds | Y | Y | Y | correct | 2 | full trio |
| `usercommands/grapple.go:108` | flavor: target was already prone | Y | N | N | correct | 2 | supplementary detail; the parent event's trio fired at :97/:99/:101 |
| `usercommands/grapple.go:113` | grapple disarms the target | Y | Y | Y | correct | 2 | full trio |
| `usercommands/grapple.go:120` | grapple attempt fails | Y | Y | Y | correct | 2 | full trio |
| `usercommands/grapple.go:131` | flavor: failed grapple leaves actor exposed | Y | N | N | correct | 2 | supplementary detail; parent trio at :120/:122/:124 |
| `usercommands/grapple.go:136` | grapple failure critical counter | Y | Y | Y | correct | 2 | full trio |
| `usercommands/guild.go:281` | guild invite self | Y | N | N | correct | 1 | actor-only refusal, nothing happened |
| `usercommands/guild.go:285` | invite target already guilded | Y | N | N | correct | 1 | actor-only refusal before any invite is sent |
| `usercommands/guild.go:289` | invite target has a pending invite | Y | N | N | correct | 1 | actor-only refusal before any invite is sent |
| `usercommands/inventory.go:94` | own grenades destabilize in the backpack | Y | N | Y | correct | 2 | self-directed accident |
| `usercommands/kick.go:230` | kick knocks target down | Y | Y | Y | correct | 2 | full trio, actee nil-guarded for mobs |
| `usercommands/kick.go:236` | kick lands | Y | Y | Y | correct | 2 | full trio |
| `usercommands/kick.go:245` | kick partially defended | Y | Y | Y | correct | 2 | full trio |
| `usercommands/kick.go:254` | kick missed | Y | Y | Y | correct | 2 | full trio |
| `usercommands/lock.go:63` | relocks a container with a key | Y | N | Y | correct | 2 | target is an object |
| `usercommands/lock.go:85` | locks a container, keys it | Y | N | Y | correct | 2 | target is an object |
| `usercommands/lock.go:119` | relocks an exit with a key | Y | N | Y | correct | 2 | target is an object |
| `usercommands/lock.go:141` | locks an exit, keys it | Y | N | Y | correct | 2 | target is an object |
| `usercommands/look.go:112` | looks at another player | Y | Y | Y | correct | 2 | full trio at :89/:93, correctly gated on `!isSneaking` |
| `usercommands/look.go:263` | look-toward-exit refused, too dark | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/look.go:272` | look-toward-exit refused, locked | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/look.go:276` | peers toward a direction | Y | N | Y | correct | 2 | target is a direction |
| `usercommands/look.go:301` | looks at a backpack item | Y | N | Y | correct | 2 | target is an item; observer at :310 |
| `usercommands/look.go:303` | same event as :301 | Y | N | Y | correct | 2 | target is an item |
| `usercommands/look.go:307` | same event as :301 | Y | N | Y | correct | 2 | target is an item |
| `usercommands/look.go:407` | looks at a pet | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/look.go:431` | looks at a corpse | Y | N | Y | correct | 2 | corpse is not a live recipient |
| `usercommands/loot.go:117` | loot-all takes corpse gold | Y | N | Y | correct | 2 | corpse is not a person; observer deferred to :125 |
| `usercommands/maul.go:74` | maul bite lands | Y | Y | Y | correct | 2 | full trio, actee nil-guarded for mobs |
| `usercommands/maul.go:92` | maul bite partially defended | Y | Y | Y | correct | 2 | full trio |
| `usercommands/maul.go:113` | maul bite missed | Y | Y | Y | correct | 2 | full trio |
| `usercommands/pet.go:38` | pet name, no pet | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/pet.go:46` | pet name invalid | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/pet.go:58` | pet target not found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/pet.go:64` | pet target not found (second check) | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/pet.go:68` | pets a pet | Y | N | Y | correct | 2 | pet is a mob; the owner is another room observer |
| `usercommands/picklock.go:151` | lock opened instantly via keyring | Y | N | Y | correct | 2 | target is a lock |
| `usercommands/picklock.go:226` | lockpick breaks | Y | N | Y | correct | 2 | target is a lock |
| `usercommands/picklock.go:227` | same event as :226 | Y | N | Y | correct | 2 | target is a lock |
| `usercommands/picklock.go:228` | same event as :226 | Y | N | Y | correct | 2 | target is a lock |
| `usercommands/picklock.go:234` | lock trap triggers on a broken pick | Y | N | Y | correct | 2 | target is a trap |
| `usercommands/picklock.go:235` | same event as :234 | Y | N | Y | correct | 2 | target is a trap |
| `usercommands/picklock.go:267` | lock picked via minigame | Y | N | Y | correct | 2 | target is a lock |
| `usercommands/pounce.go:80` | pounce knocks target down | Y | Y | Y | correct | 2 | full trio, actee conditional on a player target |
| `usercommands/pounce.go:102` | pounce hits, target stays up | Y | Y | Y | correct | 2 | full trio |
| `usercommands/pounce.go:121` | pounce partially connects | Y | Y | Y | correct | 2 | full trio |
| `usercommands/pounce.go:142` | pounce misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/put.go:99` | put refused, not enough gold | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/put.go:112` | places gold into a container | Y | N | Y | correct | 2 | target is a container |
| `usercommands/put.go:127` | places an item into a container | Y | N | Y | correct | 2 | target is a container |
| `usercommands/rake.go:74` | rake hits | Y | Y | Y | correct | 2 | full trio, conditional actee |
| `usercommands/rake.go:92` | rake partially connects | Y | Y | Y | correct | 2 | full trio |
| `usercommands/rake.go:113` | rake misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/rally.go:29` | rally already active | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/rally.go:33` | rally on cooldown | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/rally.go:40` | rally succeeds | Y | N | Y | correct | 2 | AoE buff, no single actee; members notified in the loop at :58 |
| **`usercommands/rally.go:72`** | **Resonant Larynx war-cry fold** | **Y** | **N** | **Y** | **gap** | **2** | **the fold loop applies `AddCondition`/`AddBuff(79)` with no `SendText`, while the main loop at ~:56-60 notifies each member** |
| `usercommands/read.go:46` | reads an item they don't have | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/read.go:48` | reads a held item | Y | N | Y | correct | 2 | self-targeted |
| `usercommands/remove.go:65` | cursed removal permitted via enchant skill | Y | N | N | correct | 1 | internal curse-check with no public component |
| `usercommands/remove.go:74` | removes an equipped item | Y | N | Y | correct | 2 | target is an item |
| `usercommands/renameself.go:77` | rename fails | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/renameself.go:89` | rename succeeds | Y | N | Y | correct | 2 | self-directed |
| `usercommands/reply.go:22` | reply refused, nothing to reply to | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/reply.go:28` | reply refused, whisperer offline | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/report.go:62` | report target vanished | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/report.go:66` | report target not found | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/report.go:72` | report targeting self | Y | N | N | correct | 1 | self-target special case |
| `usercommands/report.go:79` | whisper-reports vitals to one player | Y | Y | N | correct | 1 | deliberately private; no room broadcast in this branch |
| `usercommands/report.go:86` | reports vitals to the room | Y | N | Y | correct | 2 | broadcast with no specific target |
| `usercommands/sell.go:89` | sell, no merchant | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/sell.go:100` | sells a single item | Y | N | Y | correct | 2 | merchant is a mob |
| `usercommands/sell.go:105` | sells multiple items | Y | N | Y | correct | 2 | merchant is a mob |
| `usercommands/show.go:52` | show resolved to a zero item id | Y | N | N | correct | 1 | defensive guard, unreachable in normal play |
| `usercommands/show.go:61` | shows an item to a player | Y | Y | Y | correct | 2 | full trio |
| `usercommands/show.go:84` | shows an item to a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/skill.cast.go:387` | spell cast begins | Y | N | Y | correct | 1 | fires before any target is affected; the target is reached by the room line |
| `usercommands/skill_move_defence.go:63` | room line of a defended special move | Y | Y | Y | correct | 2 | full trio by design; actor/actee suppressed only in `roomOnly` mode |
| `usercommands/stand.go:90` | stand-up transition fails | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/stand.go:102` | stands up from prone | Y | N | Y | correct | 2 | self-directed |
| `usercommands/stash.go:35` | stashes an item in the room | Y | N | Y | correct | 2 | self-directed, own item |
| `usercommands/suicide.go:46` | revive-on-death buff fires | Y | N | Y | correct | 1 | self-directed |
| `usercommands/talk.go:45` | talk target not found or is a player | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/target.go:191` | uncontested target switch onto a mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/target.go:198` | contested target switch onto a player | Y | Y | Y | correct | 2 | full trio |
| `usercommands/taunt.go:169` | taunt pulls mob aggro | Y | N | Y | correct | 2 | `AggroPulled` is only set when `target.MobInstanceId > 0`, so the actee is always a mob |
| `usercommands/taunt.go:207` | taunt hit/miss/crit/fumble | Y | Y | Y | correct | 2 | full trio, `targetPlayer` guarded for mobs |
| `usercommands/throttle.go:71` | throttle bite lands | Y | Y | Y | correct | 2 | full trio, actee nil-guarded for mobs |
| `usercommands/throttle.go:77` | throttle interrupts a spellcast | Y | Y | N | correct | 2 | supplementary detail; the room sees the physical hit at :84 |
| `usercommands/throttle.go:99` | throttle partially defended | Y | Y | Y | correct | 2 | full trio |
| `usercommands/throttle.go:122` | throttle missed | Y | Y | Y | correct | 2 | full trio |
| `usercommands/throw.go:284` | hurls a grenade into a room of mobs | Y | N | Y | correct | 2 | AoE against mobs, no single actee |
| `usercommands/throw.go:320` | grenade fumbles and backfires | Y | N | Y | correct | 2 | self-directed backfire |
| `usercommands/throw.go:352` | grenade interrupts a mob's cast | Y | N | Y | correct | 1 | actee is a mob |
| `usercommands/throw.go:398` | splash catches a defended mob | Y | N | Y | correct | 2 | actee is a mob |
| `usercommands/throw.go:402` | grenade fully defended by a mob | Y | N | Y | correct | 1 | actee is a mob |
| `usercommands/trip.go:72` | tailsweep knocks target down | Y | Y | Y | correct | 2 | full trio, conditional actee |
| `usercommands/trip.go:81` | tailsweep hits, target keeps footing | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:92` | trip knocks target down | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:101` | trip hits, target keeps footing | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:120` | tailsweep defended-partial | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:131` | trip defended-partial | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:153` | tailsweep misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/trip.go:162` | trip misses | Y | Y | Y | correct | 2 | full trio |
| `usercommands/unlock.go:58` | unlocks a container with a key | Y | N | Y | correct | 2 | target is an object |
| `usercommands/unlock.go:80` | unlocks a container, keys it | Y | N | Y | correct | 2 | target is an object |
| `usercommands/unlock.go:114` | unlocks an exit with a key | Y | N | Y | correct | 2 | target is an object |
| `usercommands/unlock.go:136` | unlocks an exit, keys it | Y | N | Y | correct | 2 | target is an object |
| `usercommands/use.go:62` | crafting container produces its item | Y | N | Y | correct | 2 | target is a container |
| `usercommands/use.go:63` | same event as :62 | Y | N | Y | correct | 2 | target is a container |
| `usercommands/use.go:64` | same event as :62 | Y | N | Y | correct | 2 | target is a container |
| `usercommands/use.go:78` | usable item not in backpack | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/use.go:84` | item is not a Usable subtype | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/use.go:91` | uses a consumable | Y | N | Y | correct | 2 | target is the item |
| `usercommands/use.go:102` | YAML-driven on-use flavor | Y | N | Y | correct | 2 | room text is a separate optional YAML field |
| `usercommands/usercommands.go:430` | loses concentration while fleeing a cast | Y | N | Y | correct | 2 | self-event |
| `usercommands/warcry.go:30` | warcry already active | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/warcry.go:35` | warcry on cooldown | Y | N | N | correct | 1 | actor-only refusal |
| `usercommands/warcry.go:42` | warcry buffs the party | Y | N | Y | correct | 2 | each member gets their own `SendText` at :58-59 |
| **`usercommands/warcry.go:76`** | **Resonant Larynx rally fold** | **Y** | **N** | **Y** | **gap** | **2** | **mirror of `rally.go:72`: the fold loop at :81-96 applies the buff with no `SendText`, while the main loop at :54-59 notifies each member** |
