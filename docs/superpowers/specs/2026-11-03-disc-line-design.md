# The Disc Line: a hard, fair mystery (#667, #625)

**Status:** draft for owner review. **Arc:** #375 phase 7, needed before tier 4.

**Owner decisions, 2026-11-03:**
- Follow the man east (shape A).
- Add novel passages as readable items, scattered so the story is not linear.
- All three pointers to the quest, plus gossip.
- Two crash site achievements, one for the Core Guardian and one for the truth.
- Minimum crash site entry about 400 gold.
- Pages on booksellers in bulk, the rest found in taverns, the bathhouse and church officials' rooms.
- Pages that state or come close to the big reveals are damaged fragments that occlude or misdirect.
- Small conflicts with the novel are fine.

## What a player experiences

A player hears a rumour in a tavern, buys a torn page from a bookseller, or follows the ring-and-line mark out of the Confluence's archives. Any of these can lead to Aldith the Bookseller, and from there to Brennan. Brennan and Reth set them on a thread that runs north to a dead man's cottage, then east up his trail through the Cascade Pass into the Highlands, past fights that a lone player will not win. At the end of it is his camp, his bones and his disc. The player finishes his work at his own hearthstone. The woken disc rouses the Sentinel. Past it, the Threshold Keeper opens the door for at least 400 gold. Inside, the Core Guardian and the oracle-stones wait.

## Facts verified against source (master `307042aa8`)

| Fact | Where |
|---|---|
| 75 ends on `room_interact` noun `field-notes`, granting two tokens at once | `quests/75-the_surveyors_report.yaml` |
| Reth's `testimony` topic already gives the route, and ends by pointing at the table | `dialogue/greenford/9524.yaml` |
| 76 today: search and `cast glow` in 4023, hand-in at Brennan (`disc_in`) | `quests/76-the_disc.yaml`, `dialogue/greenford/9516.yaml` |
| 76 steps today: `start`, `journal`, `attune`, `end` | `quests/76-the_disc.yaml` |
| 77 is one step, started and ended by `look records` in 6397 | `quests/77-the_truth.yaml` |
| Keeper gates `open_crash` on carrying 40168 | `dialogue/eastern_highlands/9553.yaml` |
| `min_gold: 200` plus "two hundred" guidance bands | `behaviors/eastern_highlands/9553-the_threshold_keeper.yaml:34,44-47` |
| "She takes no less than 200 gold" and "for the smallest stake, three moderately well armed" | `templates/help/instances.template:27` onward |
| Trail rooms: 6339 Survivor's Shelter (mob 9535), 6349 Lightning-Split Cairn, 6351 Abandoned Survey Camp, 6353 Vegetation Line (Reach-Alpha 9547), 6365 Cleared Section, 6371 Sentinel's Watch (Sentinel 9552), 6372 Disc-Door | room and mob files |
| 6365 already describes one man's abandoned camp: lean-to, cold stones, tools, "whoever cleared this earth, alone" | `rooms/eastern_highlands/6365.yaml` |
| 9535 already has the topic `others`, "Did others go up before me?" | `dialogue/cascade_pass_road/9535.yaml` |
| Statpools: Reach-Alpha 560, Sentinel 2000, Core Guardian 2800 (room 6395) | mob files |
| `mob_death` quest triggers and kill stats credit every damager plus party members in the room | `internal/hooks/Death_MobKillCredit.go` `killCreditUserIds` |
| Achievement triggers are a fixed vocabulary; `quest_completed` takes a token | `internal/achievements/evaluate.go`, `loader.go` |
| `read` raises no quest event for a carried item, and it reads `description` when no blob is set | `internal/usercommands/read.go` |
| Commands raise quest events through `questengine.NotifyCommand` with a target | `internal/actions/buy.go:862` and others |
| Booksellers: Aldith 9509 (Greenford), Drunn 9444 (Confluence), Orin 9332 (New Plymouth Crafting) | mob files |
| Aldith's marked topic `surveyors` already sends players to Brennan | `dialogue/greenford/9509.yaml` |
| Gossip picks from an NPC's known facts or heard events every `GossipIntervalRounds` (75); 32 mobs are gossipers | `internal/hooks/MobIdle_HandleIdleMobs.go`, `_datafiles/config.yaml:2130` |
| Facts are in `facts.yaml`; each NPC's knowledge is in `facts.awareness/` | `internal/facts/context.md` |
| The only bathing room is 5609 The Bathhouse; no room mentions a privy | search of `rooms/` |
| The mark is a ring struck through with a line in the game; four nested arcs in the novel | quests 49, 73, 76; novel L1740 |

The novel's canon and 30 candidate passages, with line numbers: `2026-11-03-disc-line-novel-canon-and-passages.md`, beside this file. In the novel the father's disc stays home and no one wakes it. By the owner's ruling these stand as legend and memory disagreeing.

## Quest 75, The Surveyor's Report

Steps: `start`, `survey`, `intro`, `notes`, `end`.

1. `start`: Brennan's offer. Unchanged.
2. `survey`: `look filed survey` in the Archive. Unchanged.
3. `intro`: Brennan's `turnin` topic. Unchanged.
4. `notes`: new. Reth's `testimony` topic, with `questRequired: 75-intro`, gives the marked map (40160) and bumps margin rep 20. It replaces the `look field notes` trigger. The topic's last lines stop pointing at the table.
5. `end`: new hand-in at Brennan, a short walk in town. Brennan then offers 76 on the spot.

*Small change from the route as discussed:* the quest ends back at Brennan, not at Reth. The next offer is Brennan's, and the arc rule is that a chain runs through the hand-in. This clears backlog lines `chain 75` and `end 75`, and the one-action lines.

Retired token: `75-testimony`. Anyone holding it also holds `75-end`.

## Quest 76, The Disc

Steps: `start`, `journal`, `shelter`, `cairn`, `camp`, `alpha`, `remains`, `attune`, `sentinel`, `end`.

| Step | Where | Action | What it gives the player |
|---|---|---|---|
| `start` | Brennan, 6305 | offer | A man from Ashwick studied this mark and went east. Read what he left. |
| `journal` | Cottage 4023 | `search` | His method, and that he took the disc and followed the surveyor's marks. No disc here. |
| `shelter` | 6339 | `topic_heard` 9535 `others`, plus one new line for a player on this step | Someone who went up with a grey thing in his hand and a mattock on his back. Toward the split stone. |
| `cairn` | 6349 | `search` | His mark scratched fresh on a fallen stone, pointing to the old camp. |
| `camp` | 6351 | `search` | He stayed here, then went on, past where the green stops. |
| `alpha` | 6353 | `mob_death` 9547 | The Reach-Alpha holds the ground. A fight for a party. |
| `remains` | 6365 | `search` | Remains under the lean-to: a strap, a coat, few bones, a satchel. The grey disc (40167). His own cut hearthstone. |
| `attune` | 6365 | `cast chrysalis-glow` at self | His method, finished at his own hearthstone. Consumes 40167, gives 40168. **Attune achievement.** |
| `sentinel` | 6371 | `mob_death` 9552 | The woken disc rouses the watcher. The readiness test. |
| `end` | Keeper, 6372 | hand-in, carries 40168 | She has watched a hundred come. She tells them about staking gold. |

**Rules:**
- Hints are under 200 characters and say where to think, not how to walk. The minimap has a target on every step.
- Clues are the world's own text, so a careful reader can reason forward. For example: Reth's route names the cairn and the camp, and the journal names Reth's marks.
- A lone player can walk the road, but `alpha` and `sentinel` should stop them. Tune by playtest: Reach-Alpha's 560 stat points are likely too low for "a fight for a party", and the fix is a tuning pass on the mob, not a new one. A player who can already kill it alone has earned the step. Credit goes to all party members in the room.
- 6365's text keeps its feeling but must stop saying no one is there. It gains nouns: `lean-to`, `remains`, `satchel`, `hearthstone`.
- 6372's text and the Keeper stay. The "marked hearth" in 4023 stays as his first attempt: the journal says it never answered there, and that he would cut another where the thing could be seen.
- The disc pulling `alpha` and `sentinel` into the story is said in their steps' text. Neither mob is changed for players not on this quest.

**Removed:** Brennan's `disc_in` and `disc_quest`, the cottage's disc and attune triggers. `disc_tale` stays as the offer. Maren's letter behind the stone stays.

**Recovery:** a player with `76-attune` and no 40168 searches 6365 for another grey disc (his last, in the satchel lining) and casts glow there. Grants nothing. The Keeper's `crash` hint moves there. Same one-disc logic as today's triggers.

**Saves:**

| Holds | What happens |
|---|---|
| `76-end` | Done. Attune achievement from `76-attune`. |
| `76-attune`, no `76-end` | Carries on to the `sentinel` step, then the Keeper. |
| `76-journal`, no `76-attune`, with or without 40167 | Goes to `shelter` and walks the trail. A held 40167 stays in the pack and the `remains` search gives no second one. |
| `76-start` only | Searches the cottage as normal. |
| 40167 or 40168 in hand, no quest | Keeps them (#625). A 40168 still opens the door. |

The plan must confirm how the quest log treats a saved later token against steps inserted before it, and test this table.

## Quest 77, The Truth

Steps: `start`, `read`, `end`.

- `start`: offered by the Keeper when carrying 40168, after `76-end`.
- `read`: `search` at the stones in 6397 grants it, gives the Fragment (40176) and plays today's full text.
- `end`: hand-in at the Keeper. Her existing `77-end` variant is the payoff line. **Truth achievement.**

`look records` becomes a description of the stones, no longer an instruction to look at them. Anyone holding `77-end` is done. Clears the backlog lines `start 77` and `end 77`.

*Open for the plan:* a player inside the instance who has never taken 77 should find a line pointing to the Keeper, not a dead room.

## Crash site entry

- `min_gold: 400`, and the matching "no less than" line.
- Guidance bands lose the lowest: 400 to 800 for three of you very well armed and armoured, 800 and up for a great many friends.
- `help instances` gets the same change.
- The stake's scaling is untouched: 400 was already the second band.

## The pages

**The set:** about 30 readable items, "a torn page", named from what is legible, not numbered. The source is the 30 candidates plus the file's alternates. None is a clean novel quote that states a reveal.

| Kind | Handling |
|---|---|
| Subtle or atmospheric (the rhyme, Reth's scrub line) | Close to the novel, trimmed to play |
| Comes close to a reveal (hill is made, deathbed moons, the remains) | Damaged: holes in the text, so it occludes the truth |
| States a reveal (P02 moons are ships, P29 the Chrysalis) | Damaged so it misleads until the oracle-stones settle it, or swapped for an alternate |

Novel-only names can stay where a player can follow them. Text follows `dogmud-player-copy`: 80 columns, no dashes.

**Owner gate:** all page text goes in one review document before any is built, the same as the dialogue batches.

**Where:** a shuffle, so no one seller's set reads in order.
- About 22 on booksellers and other sellers: roughly 7 each for Orin, Drunn and Aldith, and a few with Peddler Malk or another seller off the path.
- About 8 found in rooms: taverns (for example 472, 6116, 5378, 5512), 5609 The Bathhouse, church officials' rooms (for example 5912, 5907, 6190, 4022).

**Found pages:** room text mentions something tucked or left behind, and `search` gives the page through a `command: search` trigger, gated `missing_item` per player. Triggers live on a new secret quest with no steps a player sees. All pages are `not_salable`, so finding one cannot be a gold loop. A player can find a page again after dropping it, which harms nothing.

## Achievements

| Achievement | Trigger | Needs |
|---|---|---|
| Key in the Dark: woke the disc | `quest_completed` `76-attune` | Nothing new |
| The Torn Record: read every page | new type `items_read` with an item list | Engine change |
| The Last Warden: killed the Core Guardian | new type `mob_killed` with a mob id | Engine change |
| A Swallowed Stone: learned the truth | `quest_completed` `77-end` | Nothing new |

Names are suggestions. Points get matched to existing tiers in the plan.

**Engine changes:**
- `read.go` records carried reads in a saved per-character set, and evaluates achievements. Each page counts once. Reading then selling or dropping still counts.
- `items_read` reports numeric progress, "12 of 30". The loader refuses an unknown item id.
- `mob_killed` reads the per-mob kill record `KD.AddMobKill` already keeps, so earlier kills count, and party credit is the same `killCreditUserIds`. The loader refuses an unknown mob id.
- Neither type is specific to this quest. `dogmud-persistence` rules apply to the new character field.

## Pointers

1. **Pages:** several mention the mark, the eastern country or a surveyor who would not say what he saw.
2. **Booksellers:** Drunn and Orin each get one topic that names Aldith in Greenford as the one who hears those questions.
3. **The mark:** Quist or Brother Cael get one line after 73, that someone at the university has chased this mark across a lot of margins. 74's "north and older" thread stays.
4. **Gossip:** five or six vague new entries in `facts.yaml`, such as lights on the eastern hills, a surveyor who quit without saying why, and a man from Ashwick who never came home. Each is known to a few gossipers in `facts.awareness/`, with taverns favoured. Rarity comes from how few NPCs know each one, alongside what those NPCs already know, on the existing 75-round interval. **Measure it in the plan:** a player sitting in a tavern should hear one occasionally, not each visit. Check `facts.yaml` and `facts.awareness/` are safe to edit, given which of them the server rewrites.

## Testing

- `quest76_disc_steps_test.go` is replaced by tests of the new walk, recovery and the saves table.
- Tests for 75 and 77, the two achievement types, and `read` recording once.
- Guards: every page has exactly one home, in a shop or a found-room trigger. `items_read`'s list equals the page set, so a page cannot be added and left out.
- `dialoguecheck` and the loop validators: lines for 75 and 77 clear.
- `min_gold` and the help text agree.
- Required final task: an adversarial playtest of the whole line with a real party, plus a naive player finding the start from a rumour or page. Fight tuning comes from this.

## Delivery

Four PRs, in order, each working alone:
1. Engine: `items_read`, `mob_killed`, `read` recording, the entry cost.
2. Quests 75, 76, 77: rooms, dialogue, saves, attune, Guardian and truth achievements.
3. Pages: owner review of the text, then items, shops, found rooms, the read-all achievement.
4. Pointers: bookseller topics, the Confluence line, gossip. Then the playtest.

## Out of scope

- The moons, and anything past the crash site.
- Rebalancing the crash site interior.
- Rotating bookseller stock.
- Editing the novel. `novel/` is read only here.
- Other tier 4 quests. They follow this.
