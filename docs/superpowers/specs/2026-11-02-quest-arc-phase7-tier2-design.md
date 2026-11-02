# #375 phase 7, tier 2: the road north and Stillwater

**Status:** design approved in brainstorm 2026-11-02; awaiting owner review of this file. No plan
is written until the owner approves it.
**Arc:** #375 quest and dialogue rework, phase 7. Roadmap:
`docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`; target page:
`docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`. Tier 1:
`docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier1-design.md`.
**Zones:** Ashwick, Labyrinth of Low Tunnels, North Road, North Road North, Stillwater.
**Quests:** 2, 16, 17, 18, 19, 20, 60, 61.

## Owner rulings this design rests on

From tier 1 (2026-11-02), and they still hold:
1. Hand-holding ends at the Coulee: no new bridges or hubs. A zone pass fixes the quests.
2. A quest handles an item the player already holds as a general case.
3. A quest has to make sense as a story. A zone pass audits where each quest's items come from.

New for this tier (2026-11-02):
4. **Quest 20 moves to `search`.** Each site is found with a real `search` in that room in place of
   `look <noun>`. The discovery text stays.
5. **Quest 61 has a fixed order.** The hedgerow bandit first, then the Reach-Touched.

## Facts verified against source (master `d6fcb2b1b`, 2026-11-02)

| Fact | Source |
|---|---|
| These quests carry 28 backlog lines: quest 2 (1), 16 (4), 17 (1), 18 (1), 19 (6), 20 (7), 60 (3), 61 (5) | `quests/loop-backlog.txt` |
| 16 of the 28 are `hint-length`: 2 `start`; 16 `start`, `forager`; 17 `start`; 18 `start`; 19 `start`, `hunter`, `signed`; 20 `garden_marker`, `wardstone_altar`, `bone_shoals_cache`, `brindle_confession`, `kingfisher_found`; 60 `start`, `waybill`; 61 `start` | as above |
| `search` raises a `command` quest event unless refused for cooldown. `event: command, command: search, room: N` is the shipped pattern | `internal/usercommands/skill.search.go:24-33,51`; `quests/49-the_old_shrine.yaml:103-110`; quests 58, 76 |
| Quest 20's triggers on typed nouns: rooms 4138, 4122, 4124, 4144, 4145, 4130, 4140, 17 `room_interact` triggers in all, many only spelling variants | `quests/20-ullas_silence.yaml` |
| Quest 18 has one `room_interact` on `bedroll` in room 4050, gated on `missing: 18-start`, which sets the `18-barn` flag Betta's topic reads | `quests/18-bettas_silence.yaml`; `dialogue/north_road/277.yaml:42,134` |
| Quest 20's "Try look" lines: room 4138 line 34, 4124 line 77, 4130 line 71, 4140 line 92 | `rooms/stillwater/` |
| NPC lines saying `look <noun>` for quest 20: `dialogue/stillwater/337.yaml:52`, `344.yaml:26,44`, `347.yaml:74,113`, `355.yaml:39,48` | grep |
| Rooms 4124, 4130, 4140 hide their noun behind `hidden_nouns`; the others are plain nouns | `rooms/stillwater/` |
| Hand-ins off the last step: `guardhandoff` needs `60-start`; `ullahandoff` needs `20-kingfisher_found`; `vellahandoff` needs nothing past `20-brindle_confession` in its trigger | `dialogue/new_plymouth_outskirts/9208.yaml:43`; `dialogue/stillwater/347.yaml:90`; `quests/20-*.yaml` |
| Quest 19's hunter kill needs `19-start`; the signing needs both `19-shrimp` and `19-hunter`; step `signed` has no destination | `quests/19-*.yaml`; backlog |
| Quest 61's either-order logic: three `mob_death` triggers and the `61-reach-first` flag, read by Pemberton's opener | `quests/61-*.yaml`; `dialogue/north_road_north/9171.yaml:25-46` |
| `HasQuest` counts later steps, so a save at `61-reach` holds `61-bandit` | `internal/characters/quests.go:67-89` |
| The offers are real NPC topics with end tokens excluded; no quest here is trigger-started | `dialogue/<zone>/<mob>.yaml` |
| Item sources: salve 30036 sold by Ilsa; pin 54 on Soren; tooth 40054 from the sump dweller; kingfisher 40060 from the grave; page 40061 from Vella; packets 40073 and 40074 handed along the relay | `quests/2-*.yaml`, `17-*.yaml`, `19-*.yaml`, `20-*.yaml`, `60-*.yaml`; dialogue |
| Plain `item_gain` is not used by these quests, so Hook 1 has nothing to add here | parse of the quest files |
| The veteran profile has quests 2 and 17 finished; `mid` and `early` have none of these | `tools/playtest/profiles/` |

## What the player sees

- Quest 20 is the same mystery. At each site the player types `search`, and the same discovery text
  plays. `look carving` still describes the carving and no longer moves the quest.
- Pemberton asks for the bandit first.
- Quests name what to do next in a short hint, and the map carries the route.
- Nothing else changes. No new quests, NPCs or rewards, and no change to who gives what.

## Item audit (ruling 3)

Nothing lies loose with a keeper standing beside it. The salve is bought, and the pin and tooth drop
from what the player kills. Delia's herbs lie in the Hidden Grove, and they are no longer part of
quest 16: the owner removed that hand-in because it let a player finish without meeting the forager.
No quest here has a findable item that an NPC should notice, so none gets a second offer.

## Quest fixes

### Quest 20: Ulla's Silence

Each group of `room_interact` triggers for a site becomes one trigger. The `actor` text, the `has`
and `missing`, and the grant stay as they are, and step ids do not change. The `give_item` on the
grave stays.

```yaml
  - event: command
    command: search
    room: 4138
    conditions:
      has: ["20-start"]
      missing: ["20-workshop_carving"]
    actions:
      - grant: "20-workshop_carving"
      - actor: "..."   # unchanged
```

| Site | Room | Grants |
|---|---|---|
| Elgar's bench-vise | 4138 | `20-workshop_carving` |
| Temple Approach pillars | 4122 | `20-temple_pillars` |
| Garden marker | 4124 | `20-garden_marker` |
| Chapel altar | 4144 | `20-chapel_altar` |
| Wardstone slab | 4145 | `20-wardstone_altar` |
| Elgar's cache | 4130 | `20-bone_shoals_cache` |
| Elgar's grave | 4140 | `20-kingfisher_found`, gives 40060 |

- The priest's `acknowledge_spiral` topic stays as the other way past the pillars.
- **A lost kingfisher:** the replacement trigger becomes `command: search`, room 4140, `has`
  `20-kingfisher_found`, `missing` `20-vella_journal`, `missing_item` 40060, with its text. It grants
  nothing, so it is not a finding.
- **Hints and NPC lines:** every "Type look carving" and similar becomes "Type search". This covers
  the step hints, the plain noun text in room 4138, and the NPC lines in the facts table. Room nouns
  stay, so `look` still describes each thing.
- **Hidden descriptions keep "Try look".** A `hidden_description` (rooms 4124, 4130, 4140) shows only
  after a search has found the noun, and then on every look at the room, so "Try search" would be false
  there and "Try look <noun>" is true. Each of those search triggers also carries `discover:` for its
  hidden noun, so `look` works on what the quest just found.
- `vellahandoff`'s trigger `has` becomes `20-kingfisher_found`, and the topic's `questRequired`
  follows. `ullahandoff`'s `questRequired` becomes `20-vella_journal`.

A `search` refused for cooldown raises nothing, so a player who searches twice in a row may have to
wait. That is the search command's shipped behaviour. The step's hint already says to search there.

### Quest 18: Betta's Silence

The `bedroll` trigger becomes `command: search`, room 4050, with the same conditions, flag and text.
Betta's topics read the flag as before. Room 4050's text already mentions the bedroll.

### Quest 61: Hedgerow Toll

Fixed order: `start`, then `bandit` on `mob_death 9173`, then `reach` on `mob_death 9178`.

- The two extra triggers and the `reach-first` flag declaration go, along with `missing_flag` on the
  first trigger and Pemberton's flagged opener variant (`9171.yaml`). `9178` needs `61-bandit`.
- Killing the Reach-Touched first grants nothing and says so:
  > The Reach-Touched shudders, goes still, and is just a body on the stones. It will not stay down
  > long. Pemberton wanted the bandit in the reeds dealt with first.
- Pemberton's offer text and hint name the order: "Put the brute down first, then the thing in the
  Reach." and "Pemberton wants the hedgerow bandit at the Mire Cutoff dead first, and then the
  Reach-Touched in the Empty Reach."
- **Old saves:** one on `61-start` holding the `reach-first` flag kills the bandit, then meets the
  Reach-Touched again. It respawns and the hint says where. The stale flag does nothing.

### Quests 16, 19, 60: hand-ins and order

- **Owner decision, 2026-11-02:** Delia's `quest_complete_herbs` hand-in is deleted, because it let a
  player finish quest 16 without meeting the forager. Its key moves to the retired section of
  `dialogue/topic-keys.txt`. `quest_complete_forager` is the only hand-in. A player who gives her the
  herbs gets them handed back by her archetype's `player_give` branch.
- Quest 19: the `mob_death 331` trigger's `has` becomes `19-shrimp`. `sign_bounty`'s trigger needs
  only `19-hunter`, which holds `19-shrimp` by `HasQuest`. Step `signed` gains `destination: {mob: 335,
  room: 4110}`. A player who kills a hunter before a shrimp swarm kills another; both respawn.
- `guardhandoff`: `questRequired` becomes `60-manifest`.

### Hints

Replaced exactly, all at most 200 characters.

| Step | Hint |
|---|---|
| 2 `start` | Bring a healing salve to the Tunnel Shaman in the Low Junction, below the Narrow Descent. Apothecary Ilsa in Stillwater sells them. Type talk shaman. |
| 16 `start` | From the East Edge of Ashwick, follow the forest path north three times to a hidden camp. Whoever lives there gathers just west of it in the mornings. Type talk forager. |
| 16 `forager` | Delia keeps her cottage north of Ashwick's Central Green. In the afternoon she is in the garden behind it, and at dusk on the green. Type talk delia. |
| 17 `start` | The bandits camp west of the north road: from the village square, go north, then west. Kill their leader, Soren, take his iron pin, and bring it back. Type talk caravan. |
| 18 `start` | A bloodline agent in a dark coat walks the road around the Road Fork. When you meet the agent, you must choose what to say, and it cannot be taken back. Type talk agent. |
| 19 `start` | Bring a light: the caves are dark. The Cave Mouth is past the boat pier, north of the Fishing Docks. Go down into the Dripping Antechamber. Type attack swarm. |
| 19 `hunter` | Report to Constable Drunn at the Constabulary, east of North Square. Type talk drunn. |
| 19 `signed` | For the full reward, kill the sump dweller in the Hollow Sump, north of the Pool Chamber, and bring back its tooth. To report, with or without it, type talk drunn. |
| 20 `start` | From Ulla's Parlor, go up the stair to the workshop. Something is carved into the handle of the vise on Elgar's bench. Type search. |
| 20 `workshop_carving` | From the workshop, go down, then east to North Square. Go south twice to Lakefront Square, then east three times to the Temple Approach. Type search. |
| 20 `temple_pillars` | From the Temple Approach, go east into the temple, then south into the Temple Garden. Something is buried at the foot of the crab-apple tree. Type search. |
| 20 `garden_marker` | The Wardstone Circle is northwest of town, and the Old Chapel Ruin is just west of it. In the ruin, type search. |
| 20 `chapel_altar` | Go back east into the Wardstone Circle. A carved altar slab lies at its centre. Type search. |
| 20 `wardstone_altar` | Bring a light: the caves are dark. A drowned hunter hides in the Bone Shoals, west of the Dripping Antechamber, below the Cave Mouth. A cairn stands there. Type search. |
| 20 `bone_shoals_cache` | Smith Brindle should know what you found. By day he is in his smithy, and in the evening at the Pike & Lantern, west of Lakefront Square. He sleeps at night. Type talk brindle. |
| 20 `brindle_confession` | Go to Stillwater Cemetery, three rooms west of North Square, through Ulla's Parlor and the Healer's Cottage. At the foot of Elgar's grave, type search. |
| 20 `kingfisher_found` | Take the carved kingfisher to Vella Thorne at the Healer's Cottage. In the morning she is at the temple's Healer's Alcove, and in the evening in Ulla's Parlor. Type talk vella. |
| 60 `start` | Follow the road north from the Tollgate through the Empty Reach to Hartcharn's South Gate, then north twice and west to the Ferry and Coach Agent. Type talk pell. |
| 60 `waybill` | From the Market Square, follow the road north over the Greywater ford to Kingsbarrow Vale. At the Tithe Road, go west into the Great Tithe Barn. Type talk verrold. |
| 61 `start` | Start with the hedgerow bandit, hidden in the reeds at the Mire Cutoff: from the Lake & Ladle's yard, go southwest. Then Pemberton wants the Reach-Touched dead too. |

The hint-length rule checks each of these. Only the names and directions already in the old hints
are used.

## Breadcrumbs and markers

No changes. Quest 19's `signed` keeps its flag and its two hand-ins, to Drunn and to Dock Master Arn.

## Testing

- **The ratchet:** all 28 backlog lines deleted with the sweep green, and no line added. A guard like
  tier 1's holds quests 2, 16, 17, 18, 19, 20, 60 and 61 at no backlog lines, with a fixture proving
  it can fail.
- **Quest 20 and 18 on shipped content:** no `room_interact` remains in either, and every site's
  trigger is `command: search` in the right room with the right grant.
- **Quest 61:** killing the Reach-Touched first grants nothing and prints the line. Bandit then
  Reach-Touched reaches the hand-in. The flagged opener is gone, and the `reach-first` flag is
  referenced nowhere.
- **Quest 19:** a hunter killed before a shrimp swarm grants nothing. `sign_bounty` needs the hunter.
- **Hand-ins:** each of the four changed gates hides the topic on the previous step and shows it on
  the last.
- **Gates before push:** `gofmt`, `go build`, `go test ./...`, `DOGMUD_BOOT_SMOKE=1` root package,
  `golangci-lint`, isolated boot.
- Anything the plan proves only by a test or a seat is said so in the PR.

## Playtest

Three seats after the plan's last task, using `mid` (the veteran has finished 2 and 17):

| Seat | Must show |
|---|---|
| Mystery | Quest 20 from Ulla to the hook-spear. Every site by `search`, and `look <noun>` at each saying nothing false. Loses the kingfisher and searches again. Notes where the hints alone were not enough, since the routes are shorter now |
| Bounties | Quest 19 with shrimp and hunter in the wrong order, then the tooth; quest 61 with the Reach-Touched first, then in order |
| Roads | Quests 2, 16, 17, 18 (with the bedroll found first by `search`), and quest 60's first leg. An old save holding `20-garden_marker` carries on, and one flagged `61-reach-first` finishes |

## Out of scope

- A real lookout muster (#765). Soren's camp stays as #764 left it, so quest 17's fight is in the
  camp. Note any trouble in the Roads seat.
- Quest 21 and sample quest 1000000.
- Splitting quest 17 into smaller steps. It has no finding, and its hint fits. A later pass can split
  it if the playtest shows a need.
- Any new bridge or hub.
- Tiers 3 to 5.

## Done means

All 28 lines gone, no `room_interact` in quests 18 and 20, the playtest ran, and the roadmap names
tier 2 as done. One PR. `docs/PATCH_NOTES.md` carries a dated entry. Findings outside this pass go to
issues.
