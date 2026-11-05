# #375 phase 7, tier 5: New Plymouth

**Status:** approved 2026-11-04. Plan: `docs/superpowers/plans/2026-11-04-p7-tier5-new-plymouth.md`, in
build.
**Arc:** #375 quest and dialogue rework, phase 7. Roadmap: `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`;
target page: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`. Earlier tiers:
`docs/superpowers/specs/completed/2026-11-02-quest-arc-phase7-tier1-design.md`, `...-tier2-design.md`,
`...-tier3-design.md`. Tier 4 was done by the disc line, `docs/superpowers/specs/2026-11-03-disc-line-design.md`.
**Quests:** 63, 65, 66, 67, 68, 69, 70, 71; plus quest 21 and the sample quest 1000000; plus two new
quests that give Bloom a source in the world (section 7).

Four PRs from one plan: **5a** docks and Common (63, 65, 66, 67), **5b** Crafting, Temple, Merchant and
Old Quarter (68, 69, 70, 71), **5c** the Bloom chain (section 7), **5d** quest 21, the sample quest and
the backlog guard.

## Facts verified against source (master `8e4ab0e27`, 2026-11-04)

| Fact | Source |
|---|---|
| `loop-backlog.txt` holds 64 lines: 63 (7), 65 (7), 66 (9), 67 (11), 68 (5), 69 (4), 70 (4), 71 (5), 21 (4), 1000000 (8) | `quests/loop-backlog.txt` |
| Typed-noun triggers (`room_interact`) remain in 63 (6 lines mention it), 65 (6), 68 (10), 69 (3), 70 (3), 71 (3); 66 and 67 have none | `quests/*.yaml` |
| Dead steps: `report` in 63, 65 and 67; `escort` and `detox` in 66. No quest file or dialogue grants them | `quests/63-*:33`, `65-*:38`, `66-*:32,41`, `67-*:108` |
| `66-escort` is only read, never granted: by Ysolde's opener variant, her `wrapperhandoff` exclusion and `escort_close`, and by Teels's variant and offer exclusion | `dialogue/new_plymouth_common/9323.yaml:46,86,105`; `dialogue/new_plymouth_docks/9392.yaml:35,89` |
| `escort_close` is Ysolde's hand-in for a save on `66-escort`, and sets `66-addict-fate` | `9323.yaml:95-110` |
| Rooms 6036 and 6037 are biome `dungeon` with no lamp set. A search with no sight counts for no quest (ruling 2026-11-02) | `rooms/new_plymouth_old_quarter/6036.yaml:23`, `6037.yaml:24`; `dogmud-authoring-quests` |
| Quest search pattern: `event: command`, `command: search`, `room: N` | `quests/73-*.yaml`, `quests/76-the_disc.yaml` |
| Step names leave through the manifest's `# retired` section | `quests/step-ids.txt:5,372` |
| Hint cap is 200 characters, enforced for quest 76's hints by test | `internal/questengine/quest76_disc_steps_test.go` |
| Quest 21 is `secret: true`, has `triggers: []`, and `grantNewcomerMarker` grants `21-start` on the newbie route | `quests/21-newcomers_path.yaml`; `internal/usercommands/start.go:267-272` |
| Quest 21 gates two topics: 9100 `newcomer_mutations` (`30-end`, `21-start`, not `21-end`) and 9106 `newcomer_players`, which grants `21-end` | `dialogue/*/9100.yaml:117-121`, `dialogue/*/9106.yaml:283-287` |
| A flag used in a topic must be declared by a quest, or validation refuses it | `internal/dialogue/world.go:329-342` |
| Quest 1000000 is the reserved template: id allocation stays below it and the editor hides it | `internal/quests/save.go:75-77`, `modules/gmcp/gmcp.Quest.go:25` |
| Factions on file: no Merchant Quarter faction exists. New Plymouth has `np_commonfolk`, `np_dockfolk`, `temple_np`, `cooperage_circle`, `bloom_trade` | `factions/` |
| Quest 71 pays `rep_faction: np_dockfolk`, and adds `cooperage_circle` +5 by trigger only with `69-end` | `quests/71-*.yaml:73-74,145-151` |
| Bloom Wafer (40108) has no source: outside its own file the id appears only in `internal/actions/drink.go`, in no mob, shop, recipe, quest or dialogue. The grep could succeed: it found `drink.go` | `grep -rE "\b40108\b" _datafiles internal modules` |
| The Bloom mechanic is built: communion, addiction tiers, withdrawal, mutation advance and seeding | `internal/actions/drink.go:411`, `internal/characters/bloom.go`, `bloom_mutation.go` |
| Bloom's knobs are not in `config.yaml` (0 lines), so they run on Go defaults: for example advance chance 0.50, addiction decay 300 rounds | `internal/configs/config.balance.combat.go:582-594` |
| Wafer: `not_salable: true`, `value: 25`, `toxicity: 20`. Ysolde's Purge (40109) exists, and she already gives it through her `detox` topic | `items/materials-40000/40108-*`, `40109-*`; `dialogue/new_plymouth_common/9323.yaml:188-189` |
| Marn's `bloom_undercover` line takes the player for a customer and nods at a wax-wrapped square; it sets `67-entry`, which nothing reads | `dialogue/new_plymouth_docks/9305.yaml:29-48` |
| Marn stands in Marn's Fabric-Remnants Shop, 5523, exits south, north, down | `rooms/new_plymouth_docks/5523.yaml` |
| `room_gate` refuses a move on conditions and says why: the pattern for a door only the knock opens | `quests/14-the_undertow.yaml:84-93` |
| A branching quest pattern: flag `resolution`, values per ending | `quests/18-bettas_silence.yaml:8-11` |
| Quest ids 64 and 102 are free: no quest file, and no line in the step manifest. 72 is taken by The Water Dispute; 78 to 87 and 29 are retired | `quests/`, `quests/step-ids.txt` |
| Novel: Cade's supply house sells fabric remnants in front, with a cupboard door and a knock he changes; Vane overhears "three months at minimum before the new one's producing at volume, if she survives the first month"; Horst of the bloodline's domestic office runs contractors and hires her to deliver a girl | `novel/what_the_moons_keep.md:746-764,1986`; outline line 145 |

Reported by the 2026-11-04 survey and to be re-checked in the plan's own facts table, because the
plan relies on line numbers: NPC schedules and light in section 3, hand-in gates (`:prev` lines), and
the story contradictions in section 2.

## 1. Owner rulings this design rests on

Standing: hand-holding ends at the Coulee, so no new bridges. A quest has to make sense as a story. A
step completes on something the player does, so typed nouns become `search`. An item comes from its
keeper, never by drop and pick up.

New (2026-11-04):
1. **Depth is words, not world.** Lines and hints are rewritten so nothing contradicts itself or lies.
   Dialogue variants may say what a character knows after an ending. NPCs and rooms do not change with
   an ending.
2. **Quest 66 is deliver, return, heal.** The player carries Ysolde's purge dose back to Teels.
3. **Quest 70 stands alone.** Orin remembers the 68 ending, and Gritta and Orin nod quietly toward 68
   and 69. Stay subtle.
4. **Bloom gets a source, modelled on the novel.** A first quest has the player track down a "fugitive"
   for a Bloom sample and some gold. It chains into a buy-your-way-in quest that unlocks a dealer who
   cannot otherwise be reached.
5. **The player can let the fugitive go,** and it costs them: no pay, and the chain closes.

## 2. Quest by quest

Every quest also gets: typed nouns as `search`, hand-ins that require the step just before them, hints
under 200 characters, a map destination on every step, and dead steps retired in the manifest.

**63 Dock Rat.** `search` in 5526 finds Hewd's books (item 40100). The `report` step goes. The constable
says he is waiting on a warrant for Hewd, so Hewd staying at his desk is true. Both the constable and
Jesset still take the books.

**65 The Street Sweeper's Secret.** `search` in 5624 at the seam (item 40101). The `report` step goes.
Coll's account is the true one: he finds the bits in the gutters and does not know the source. Gritta's
lines change to say fragments wash down into the gutters, not that she sends them to him.

**66 The Addict's Plight.** New steps: `start` (Teels gives wrapper 40110), `dose` (Ysolde takes the
wrapper and gives her purge), `end` (Teels takes the purge). `escort` and `detox` are retired. The purge
is the existing Ysolde's Purge (40109); Ysolde gives another if it is lost, through the `detox` topic she
already has. Quest 67 stays gated on `66-end` at Ysolde. `escort_close`, its variants and the `66-addict-fate` flag go once a
save check confirms no prod save holds `66-escort` or `66-detox`; if any does, the plan adds a migration
that moves that save to `dose`. The info topic `detox` on Ysolde is unrelated and stays.

**67 The Bloom Trail.** The steps stay, and `report` goes. Item 40111 becomes Quill's own observations,
with no wrapper and no delivery note, and his lines agree with it. Deren's lines stop treating the
operation as already finished while the quest runs. Hints stop carrying schedules and full routes. The
hint for the Old Quarter says to bring a light. The `67-entry` flag stays, because the Bloom chain reads
it (section 7).

**68 The Cooperage Circle.** `search` in 5720 finds the survey (item 40113). The search has to be fair
while Toby sleeps there, which the plan's playtest checks. Orin's lines read Toby's rubbing (40112), so
it stops being decoration; nothing takes it from the player. A line covers the paint: the mark was
re-cut somewhere the player cannot see. The old-save catch-up triggers stay only if a save can still
need them.

**69 The Gallery Cipher.** `search` in 5901 at the reliefs gives the rubbing (40115). Dross's `give_40116`
takes the rubbing as well as the reading, so the reward text is true, and the recovery paths still work.
Guide Ferrol's "get past" line is reworded if it implies a mechanic.

**70 The Pre-Founding Web.** `search` in 6037 gives the rubbing (40118), and the hint says to bring a
light. Gritta's lines and the reward make sense to a first-timer. Players holding `68-end` or `69-end`
hear a quiet nod. After the bloodline ending, Orin's `orinhandoff` takes the rubbing coldly and pays no
circle reputation; "part of the quiet" goes.

**71 The Tribute.** `search` in 5815 finds the page (item 40119). Reputation moves from `np_dockfolk` to
`np_commonfolk`, because no Merchant Quarter faction exists. The hand-in's unconditional circle +5 goes;
the `69-end` trigger keeps its bonus.

## 3. Light and hours

A `search` step refuses a player who sees nothing, so a dark search room needs a "bring a light" hint on
the step that sends the player there, and it must pass `guards/quest_search_light_guard_test.go`, which
requires sight with a torch at every sampled hour. A hint that names an NPC's hours stays out; the
quest log's destination already follows a scheduled NPC.

## 4. Quest 21 and the sample quest

**Quest 21 retires.** `grantNewcomerMarker` sets a flag instead of a token. The flag is declared by quest
30, the rite both topics already relate to (`30-newcomer`, values `new` and `told`). Hadwen's and Toke's
topics read `questFlagRequired`, and Toke's sets `told`, replacing the `21-end` grant. A saved `21-*`
token drops at login through the existing retired-quest path, so a player mid-tutorial on deploy day
loses those two teaching lines. That cost is accepted.

**Quest 1000000 is deleted.** It refers to a condition and a skill that do not exist, and no player can
reach it. `ReservedTemplateIdFloor` stays, because it keeps editor-made ids in range. If a test or the
editor needs the file, the plan says so and fixes the file's references instead.

## 5. Not in this tier

- The closing step: validators as hard boot failures, deleting `room_interact`, and the dash guards.
  It gets its own plan once the backlog is empty.
- Endings that change rooms or NPCs.
- New pointers into New Plymouth from outside it.
- Dashes in item descriptions and schedule text, which stay with #383. Dashes in the YAML comments of
  these quest files do get cleared.

## 6. Testing

- `questloop` and the walkthrough catch rule breaks. Each PR removes its own backlog lines, and the file
  rejects a stale line.
- A tier 5 backlog guard, with a fixture proving it can fail, as tiers 1 to 4 have. 5c ends with no
  backlog lines for these quests.
- Quest step tests in the style of `quest76_disc_steps_test.go` for 66's new shape and for 70's two
  Orin hand-ins.
- `dialoguecheck` on the New Plymouth folders: the `escort_close` UNREACHABLE TOPIC is gone.
- Quest step tests for both endings of the skip, and for the door: refused without the knock, open with
  it, and the runaway's hiding place closed once her quest is over.
- An adversarial in-game playtest closes the tier, covering 66's return walk, a search in Old Quarter
  darkness with and without a light, the floor search while Toby sleeps, both 68 endings before 70, and
  the Bloom chain from Marn's nod through a first dose, withdrawal and Ysolde's cure.

## 7. The Bloom chain

Follows the novel's Vane: a bounty hunter, a contractor from the bloodline's domestic office, and a fabric
remnants shop with a cupboard door. Marn the draper is the game's Cade; the name Cade is taken by
Greenford's enchanter, so it stays out. The fugitive gets a new name, not used elsewhere and not Junie.

**The breadcrumb.** Marn's `bloom_undercover` line, on the undercover route only (`67-entry`), lets slip
that the supply end pays well for people who can find things. It stays subtle, and its hint names Horst.
A player who took the honest route with Marn never sees this chain.

**Quest 64, the skip.** Offered by Horst (9344) at the gate of the watched house, gated on `67-entry`
being undercover. He hires the player to find a runaway "thief" from a noble household.
- `start`: Horst's offer. His opener does not say what she really is.
- `found`: `topic_heard` on the runaway, who hides in a room a `room_gate` opens only while this quest is
  running, with an `area` or `zone` destination that does not give the room away. Because the room closes
  afterwards, nobody meets her again after her story is over, which keeps depth A true.
- Two endings, by flag `64-outcome`, following quest 18's pattern:
  - `reported`: the player tells Horst where she is, at his hand-in. He pays gold and one Bloom Wafer.
  - `ran`: the player tells her to run, at her own topic, which ends the quest. No pay, and quest 102 never
    opens. It cannot be retried, so the choice stands. Nothing thanks the player later.

**Quest 102, the knock.** Offered by Marn, requiring `64-outcome: reported`, with a large stake through
`chargesGold` on the offer. He teaches the knock.
- `start`: paying Marn.
- `behind`: going through the cupboard door out of 5523 into a new back room. A `room_gate` refuses anyone
  without the knock. Going in the first time plays the overheard line from the novel: "three months at
  minimum before the new one's producing at volume, if she survives the first month." The player has
  already been paid when they work out what they did.
- `end`: talking to the dealer, a new NPC in the back room, who also runs the shop that sells wafers.

**Holding it open.** The Bloom Trade's description says its people never meet by design, so this cell
survives quest 67's bust, and the dealer stays open afterwards. A line covers it, in Marn's or the
dealer's voice. Quest 67's `bloom_undercover` hint, not Marn's stock, is the only route in.

**Numbers, for the plan to propose with evidence and the owner to set:** Horst's gold, the stake and the
wafer's price. The wafer stays `not_salable`, so buying from the dealer cannot be resold into a gold loop.
Bloom's knobs move into `config.yaml` at their current values, per `dogmud-balance-config`, with no
behaviour change. Nobody on prod has played the mutation boost, so the playtest reports how fast a player
can advance on it, and the owner reviews that before deploy.

**The hard lines.** The player can reach Bloom only through a deliberate choice at each of three points:
the shady line with Marn, reporting the runaway, and paying the stake. The cure stays open to everyone, and
the addiction text assumes no MUD experience, because this system has never met a real player.

**What was built differs from the above in these ways, and the owner has seen each.**
- **The knock is taught by a trigger.** Marn's offer takes the gold; a `quest_granted 102-start` trigger then speaks
  the knock, so it is never in the pitch, and a resume short of the room hears it again.
- **`102-stake`.** A flag set by the offer that charges. A shelf keeps it, so a shelved payer gets `q102_resume` and is
  never charged twice. A resume asks for no gold.
- **Odile tells; the overheard line confirms** (owner ruling, memory `feedback-bloom-chain-moral-choice`). She tells the
  player what she is running from before they choose, because the Skip is an informed choice of power against doing
  right. The line in Marn's back room confirms what the player already knows. It is no longer the first reveal, and the
  sentence above saying the player works it out after being paid no longer holds.

**Owner rulings of 2026-11-04.**
- **A. Wafers remain the way past the mutation wall.** They are not slowed or limited on load, and
  `RestockCadenceTier10Days` stays. A natural mutation costs `30 x 1.5^load`, which crosses the wafer near load 12, so
  "not faster than fighting" holds only at low load. The plan's numbers section carries the figures.
- **B. A relog must not stop withdrawal.** `BloomLastDoseRound` and `BloomHadCommunion` are saved. Time away counts as
  abstinence, because rounds run while a player is offline. A save from before this has its clock started at the first
  round online.
- **C. A real drug market is accepted, with a limit on buying.** A wafer may be given and auctioned, with no new trade
  block. One character may buy at most `BloomDealerWafersPerDay` (3) from any seller of item 40108 within a rolling
  `BloomDealerWindowHours` (24 real hours), enforced at the purchase, since a player could stand inside and buy each
  refill. The refusal is the seller's own, with no numbers. Sefton warns of it. This replaces "the wafer stays
  `not_salable`, so buying from the dealer cannot be resold" as the only check on supply: the wafer still cannot be sold
  to a shop, and at 3 a day one player gets at most about 1.65 ranks a day from purchase, before the auction market.
