# #375 phase 7 tier 5, New Plymouth: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clear all 64 backlog findings on quests 63, 65 to 71, 21 and 1000000, fix the story problems in
those quests, and give the Bloom a source through two new quests, 64 and 102, ending at a dealer behind
Marn's cupboard door.

**Architecture:** Four PRs from one plan, each mergeable alone, in order. **5a** docks and Common (63, 65, 66,
67). **5b** Crafting, Temple, Merchant, Old Quarter (68, 69, 70, 71). **5c** the Bloom chain: config knobs, two
rooms, two NPCs, quests 64 and 102, one small `Eligible` fix. **5d** quest 21 retired, quest 1000000 deleted, the
closing playtest.

**Tech stack:** Go, YAML world data, testify. Design (owner approved, its rulings are final):
`docs/superpowers/specs/2026-11-04-quest-arc-phase7-tier5-design.md`.

**Worktrees:** 5a: `C:/tmp/dogmud-375-p7-tier5a`, branch `feat/375-p7-tier5a`, cut from `feat/375-tier5`. 5b, 5c, 5d:
`C:/tmp/dogmud-375-p7-tier5b`, `...5c`, `...5d`, branches `feat/375-p7-tier5b`, `...5c`, `...5d`, each cut from master once
the PR before it has merged. Paths below are relative to the worktree; data paths are under `_datafiles/world/dogmud/`.

**Brief for every subagent:**
- Edit files with Edit and Write only. Never rewrite a repo file from the shell. Deleting a file with `git rm` is fine.
- `git add` named paths only. Never `--no-verify`. Commit messages through a quoted heredoc, ending with a blank
  line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- No em or en dashes anywhere, YAML comments included, and no `--` in player copy. Hints at most 200 characters.
  No `;` in spoken text. An unquoted `: ` in a YAML plain scalar breaks the loader.
- Do not start a server except the isolated boot, and do not push. Test logs go in the session scratchpad, never
  `C:/tmp` or `$TMP`.
- The sweep needs `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1` from the worktree root; it silently skips
  without the variable. Do not set it on `./internal/rooms`.
- `loop-backlog.txt`: delete a line when its finding is fixed. Never add a line.
- `HasQuest` counts later steps (`internal/characters/quests.go:67-89`).
- Do not weaken a test to pass. A test pinning the old behaviour is updated to follow the change, and reported.
- Report any fact below that does not match the source, rather than working around it.
- After any content task: `go test ./internal/narration/ -run TestSnapshotStores -count=1`. A diff that is only this
  task's own text is regenerated with `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`. Read
  the diff first.

## Design corrections (owner: one minute)

The source shows these statements in the design cannot be built as written. Each has the smallest fix.

| # | Design says | Source says | Fix in this plan |
|---|---|---|---|
| C1 | "Quest 67 stays gated on `66-end` at Ysolde" | `ruleChain` (`internal/questloop/rules_more.go:15-55`) refuses an offer gated on quest N's final token on an NPC with no hand-in for N. 66 now ends at Teels, so `chain 66 mob 9323 topic q67_grant` would be a fresh finding and refuse boot. | **Teels offers 67** at her hand-in, in place of Ysolde. The hand-in pitches it (requirements rule 5). `9323.q67_grant -> 9392.q67_grant` under `# renamed`. Teels already knows the draper (`9392.yaml:70-71`). Alternative: gate Ysolde's offer on a flag, which dodges the rule and costs a third walk. |
| C2 | "going through the cupboard door out of 5523 into a new back room" | The cupboard door already has a room. `5523.yaml:21` "a stair leads down behind the cupboard door", exit `down` to 5524 "Marn's Back Room", `5524.yaml:5` "The cupboard door swings inward", open to everyone. Nothing else names 5524 (grep of `_datafiles/world/dogmud`, `internal`, `modules`: only the two rooms and the light golden). | **The cupboard door becomes a new `east` exit to new room 5530.** The stair stays open and leads to the stock room 5524, reworded so it is no longer behind the door. (-7,-81,0) is free in the flat scan. 5524's "thick wall" with voices behind it now has a reason. |
| C3 | Dead steps are "retired in the manifest" | A save on a `# retired` step is stranded: `LoadUser` only logs it (`internal/users/users.go:579-583`), `HasQuest("66-start")` reads false, the offer returns, and `GiveQuestToken` refuses because `GetQuest` is nil on an unknown step (`internal/quests/quests.go:384-388`). `DoQuestStepMigration` would reset it to `start`, but runs only when the user index is missing (`main.go:423-427`). `git log -S"63-report"` shows old dialogue did name these steps. | All five dead steps leave through `# renamed`, which `LoadUser` applies to progress and shelf: `63-report -> 63-evidence`, `65-report -> 65-discover`, `67-report -> 67-witness`, `66-escort -> 66-dose`, `66-detox -> 66-dose`. Safe whatever Task 0 reports, so 5a needs no answer first. |
| C4 | Two endings "following quest 18's pattern" | Rewards apply at `-end` for every ending. Quest 18 pays nothing. The working pattern for two pays is quest 68: empty `rewards`, and `givesGold` on each hand-in (`9332.yaml:192`, `9349.yaml:50`). | Quest 64 uses 18's flag and 68's pay. |
| C5 | "stays `not_salable`, so buying from the dealer cannot be resold" | `not_salable` only skips a validator (`internal/items/validation.go:31`). Resale is refused because the wafer has no `vendor_categories` (`internal/shops/buyrules.go:66-68`), checked before stock. | No change to the item's fields. A guard pins the real reason. |
| C6 | "a large stake through `chargesGold` on the offer" | Works, but the validator forces `goldRequired` equal to it (`internal/dialogue/world.go:245`, guard `quest_charge_gold_gate_guard_test.go`). So the offer hides until the player has the gold, and an abandoned 102 cannot be resumed by a player who is short, though a resume charges nothing (`internal/dialogue/topics.go:250-252`). | A Marn opener variant names the price while the offer is hidden. `Eligible` skips `goldRequired` on a resume (Task 13). No shipped offer charges today, so nothing else moves. |
| C7 | 69: Dross "takes the rubbing as well as the reading" | A topic takes one item (`RequiresItem`). | `requiresItem: 40116`, `carriesItem: 40115`, and a `quest_granted 69-end` trigger with `consume_item: 40115`. |

## Numbers for the owner to set

| Number | Proposed | Evidence | Where it lives |
|---|---|---|---|
| Horst's gold | **150** | Vell pays 100 for the survey (`9349.yaml:50`), Orin 60. Top quest gold in the game is 150 (quests 19, 60) bar two `givesGold: 350` in Stillwater. New Plymouth pays 25 to 110. This is the zone's dirtiest job, so it is its best pay. | `9344.yaml` `q64_report` `givesGold` |
| The stake | **500** | All eight quests of this tier together pay 515 at most (35+30+25+75+100+70+70+110). The disc line's minimum stake is 400. Veyra's cheapest secret is 12,500 (`9584.yaml`). The `mid` playtest profile holds 500. Large, reachable, and a real choice. | `9305.yaml` `q102_offer` `goldRequired`, `chargesGold`, `text`, `objective`, and the opener variant |
| Wafer price | **value 120** (now 25), so **130 to 169 gold** at the shelf | Same-class potions: Mutagen Brew value 80, toxicity 45, Chrysalis Catalyst value 100, toxicity 50, both only speed natural progress. A wafer gives a whole rank 55 percent of the time. Against that, Elixir of Renewal is 30 and Phial of Second Birth is 30,000. | `items/materials-40000/40108-bloom_wafer.yaml` `value` |

If the owner changes the stake, change all five places and the test in Task 15. Prices come from
`shops.CalcSellPrice`: `ceil(value x multiplier)`, floor 0.75, ceiling 1.5, threshold 3.0 (`config.yaml`).

**The buy limit (owner, 2026-11-04).** A real drug market is accepted, so a wafer may be given and auctioned, and
there is no new trade block. What is limited is how often one character can buy: at most
`BloomDealerWafersPerDay: 3` from any seller of item 40108 within a rolling `BloomDealerWindowHours: 24` real hours,
counted per character and saved across relog and restart, with a companion's purchase counted against its owner. It is enforced at the purchase, because a limit on the door
fails: a player could stand inside and buy each refill. Past it the seller refuses in his own voice, nothing is charged
and nothing moves. **With 3 a day, one player gets at most about 1.65 ranks a day from purchase** (`3 x 0.55`), before
the auction market. The shelf refills about 24 wafers a day (`5 per 4 h 56 m`), so eight players at their limit are
what empties it. `internal/actions/buy_limit.go`; guarded by `TestBloomDealerLimitIsOnTheShippedConfig`.

### How fast a player can advance on wafers (shipped config, Go defaults for Bloom)

**Per dose:** new mutation 0.10, else advance `0.90 x 0.50 = 0.45`. **0.55 ranks per wafer, 1.82 wafers per rank**
(`internal/actions/drink.go`; `config.balance.combat.go:591-596`).

**Toxicity gate:** a dose is refused when `toxicity + 20 > max`, and `max = 0 + alchemy/2.5 + vitality/3`
(`internal/characters/resources.go:475-481`, `config.yaml:2331-2333`). AutoHeal ticks every 3 rounds, 12 seconds
(`hooks/NewRound_AutoHeal.go:35`), and sheds 1 point, or 0.5 at or above 75 percent of max (`:371-374`).
- Vitality 100, no alchemy, max 33.3: second dose after 7 ticks (84 s). From then on each dose lands at 33.3, takes 17
  ticks down to 25 and 12 more to 13.3: **29 ticks, 5.8 minutes a dose, about 10 doses an hour.** The crash's vitality
  -8 lowers max to 30.7 and gives the same 29 ticks. Vitality 122 gets two doses at once, then the same pace.
- Toxicity alone would allow about 5.7 ranks an hour.

**Shelf cap (the real limit):** stock caps at **5** and refills **5 every 4,440 rounds, 4 h 56 m**, shared by the whole
server (below). Emptying the shelf costs 130+139+148+158+169 = **744 gold**, mean 149, before barter (at most -15
percent), about 20 minutes for one player, and yields an expected **2.75 ranks**, about **271 gold a rank**. Sustained
upper bound for the whole server: **1.01 wafers an hour, 0.56 ranks an hour.**

**For comparison:** natural progress runs in combat only, threshold `30 x 1.5^load`, gain `2 x eye` a round
(`hooks/NewRound_UserRoundTick.go:417-440`). At an Eye multiplier of 1.0 (it runs 0.5 to 1.5) that is a cost per
mutation of `30 x 1.5^load`, so it grows steeply: load 4 is 76 rounds of combat, 5 minutes; load 8, 26 minutes; load
10, 58 minutes; load 12, 130 minutes. **"Not faster than fighting" is true only at low load.** The shelf's sustained
pace, 107 minutes a rank, crosses that cost near **load 12**, and past it the wafer is the faster road.

**The owner chose this on 2026-11-04: wafers remain the way past the mutation wall.** They are not slowed or limited
on load, and `RestockCadenceTier10Days` stays as it is. The buy limit above caps one character's purchases and does
not read load: one player buying alone gets a rank about every 14.5 hours, and the auction market is on top of that.

What the wafer bypasses is the point:
- It skips the affinity depth gate: `BloomSeedNewMutation` asks only `mutations.CanGain` and weights by rarity, so a
  rare apex can seed on a character who never drifted there.
- It ignored `MutationMaxCount: 8` until Task 11b, which the owner chose on 2026-11-04; only the combat path read it (`CanGain`, `eligibility.go:18-38`, does not).
- `BloomLastDoseRound` was `yaml:"-"`, so a relog cleared the withdrawal clock until the next dose. The owner ruled on
  2026-11-04 that a relog must not stop withdrawal, and that **only time online counts**: a player must not be able to
  log out and wait off a craving or a withdrawal. So the field is gone. `BloomAbstinentRounds` counts the rounds spent
  online since the last dose, ticked only by `BloomTick`, reset by a dose, less one period per point decayed, and saved,
  as is `BloomHadCommunion`. So onset at 60 rounds is 4 online minutes, and a point of decay at 300 rounds is 20 online
  minutes. A save with addiction and no count reads 0 and starts at the first round online: no false withdrawal, never
  stuck. Ysolde's Purge works at any time. The buy limit's real-hours window is a rate limit on the shop, not on the
  craving, so it runs by real time whether the player is online or not.

### Config-only ways to slow or stop it

| Knob (`config.yaml`, `Balance`) | Effect |
|---|---|
| `RestockCadenceTier10Days: 5` | **The dealer's own clock.** No shipped item has `rarity_tier: 10` (a grep for it finds none; the same grep shape finds tier 40 on item 40523), so after this plan it moves only the wafer. `30` is 29.6 hours. `3650` is off once the shelf is bought out. `0` restores 5 (`config.balance.shops.go:88`). |
| `BloomMutationAdvanceChance: 0.05`, `BloomNewMutationChance: 0.01` | 0.06 ranks per wafer, 17 wafers a rank. The dealer still sells the high and the crash. **Never `0`:** the validators are `<= 0` and restore 0.50 and 0.10. Use `0.001` for effectively none. |

The owner wipes shop saves on every deploy, so each deploy reseeds the shelf to 5. To remove the shop outright is one
data edit, the `shop:` block in `mobs/new_plymouth_docks/9802-sefton.yaml`, not config.

### Why the stock cap is safe (Malk's 19 to 80)

Malk's torn page is `rarity_tier: 40`: `EffectiveMaxStock = 40 x 1.0 x ShopMaxStockMultiplier 2.0 = 80`, refilling 5
every 2 game hours (`config.yaml:1600,1746`). An untiered item would cap at the fallback 20 and restock at tier 50
hourly (`shopinventory.go:381-400`). So the wafer is `rarity_tier: 10` and Sefton is `stock_multiplier: 0.25`:
`10 x 0.25 x 2.0 = 5`. Seed `min(5, ceil(5 x 3.0)) = 5`. Overstock decay needs `Current > RestockQty` and caravan pickup
needs the same (`caravan/visit.go`), and 5 is never above 5. Docks is not in `CaravanServedZones`, so
`TickMobShopRestock` runs. Forager, ferry and caravan deliveries need an item in a supply bucket or a wagon; nothing
carries the wafer. The ceiling price at stock 1 is 169.

## Facts verified against source (`83c7edc9b`, 2026-11-04)

| Fact | Source |
|---|---|
| Backlog: 64 lines. 5a removes 34 (63: 7, 65: 7, 66: 9, 67: 11), 5b 18 (68: 5, 69: 4, 70: 4, 71: 5), 5d 12 (21: 4, 1000000: 8) | `quests/loop-backlog.txt:7-70` |
| Typed nouns: 63 (6 triggers), 65 (6), 68 (10), 69 (3), 70 (3), 71 (3). 66, 67: none | `quests/*.yaml` |
| Search pattern `event: command, command: search, room: N`, raised by `questengine.NotifyCommand(user, room.RoomId, "search")` unless on cooldown; a dark search moves nothing | `internal/usercommands/skill.search.go:24-90` |
| Same-command rule: a recovery trigger with `missing_item` of the item the grant trigger gives passes. Put recovery first, judged before the grant | `internal/quests/samecommand.go:21,92` |
| A failed `give_item` abandons the trigger's later actions, so `give_item` goes before `grant` | `internal/questengine/actions.go:18-21` |
| `ruleOneAction`: a topic needs the previous token in `questRequired`; a trigger may not carry item, flag or gold conditions if it grants a step | `internal/questloop/rule_oneaction.go:37-50` |
| `talk-on-quest`: a mid step is a `topic_heard` trigger, never a topic grant | `internal/questloop/context.md:36` |
| `chargesGold` on a non-sale needs `goldRequired` equal to it. No shipped offer charges today (grep of `dialogue` outside 9584 finds none) | `internal/dialogue/world.go:245`, `internal/guards/quest_charge_gold_gate_guard_test.go` |
| Resume charges nothing and pays nothing | `internal/dialogue/topics.go:250-256`; `resume_test.go` |
| `offerCard` shows objective and reward lines, no price; `objective` replaces the first line | `internal/conversation/card.go:34-44` |
| `resumes`: an offer excluding another quest's token cannot resume. Flag **required** gates are fine | `internal/dialogue/offers.go:44-60` |
| A flag gate reads `questFlagRequired`; shelf keeps flags | `internal/dialogue/engine.go:62-73`; skill abandon rule 4 |
| `room_gate`: runs for `go`, and flee asks it through `questGateBlocksRoomId`; a teleport or system move is exempt; conditions read on the near side; destination matched by template id | `internal/actions/quest_gate.go:10-14`, `flee.go:218-224`, `usercommands/go.go:207` |
| Party follower re-runs `go`, so a follower without the token reads the refusal and stays. Charmed mobs and wanderers ignore gates | `usercommands/go.go:366-382`, `:388-399`, `flee.go:218` |
| A secret exit breaks the path finder until visited, so the quest log's route fails (`mapper.nearest.go:46`). New exits are plain | `internal/mapper/mapper.nearest.go`, `internal/rooms/roomdetails.go:359` |
| Wanderers: only `maxwander` above 0 in these zones are 9308, 9313, 9317, 9318 (1) and 9328 (2). Two steps from home at most (`mobcommands/wander.go`). 9308 at 5519 reaches 5523, never 5530. 9328 at 5602, 5604 does not reach 5614 | `mobs/new_plymouth_*` |
| No mob in Docks, Common or Merchant is `hostile: true`, and no file there names `steal` or `thief` beyond the maxwander list above | `grep -lE` over `mobs/new_plymouth_{docks,common,merchant}` |
| Schedules: Horst gate 8-19, parlor 19-22, gate 22-2, asleep 2-8. Marn in shop 8-21, asleep 21-8. Toby at front 6-22, asleep on 5720 22-6 | `schedules/new_plymouth_*/np_*.yaml` |
| Talk refuses a sleeping NPC. `TestEveryQuestGiverIsSeenByFace` skips sleep and needs faces light; interior reads 50 to 59, and Marn passes in 5523 with no light, so 5530 and 5625 pass | `guards/quest_giver_light_guard_test.go`; `testdata/lighting_daycycle.golden` |
| Search light: 5526, 5624, 5720, 5815, 5901 interior 50 to 59. 6037 light 0 at all 12 samples, passes with a torch (`TestQuestSearchLightGuard_CanFail`) | same golden; `guards/quest_search_light_guard_test.go` |
| `TestEveryQuestSearchRoomIsSeenByATorch` needs no edit: it reads every search trigger | same file |
| Free ids: quests 64, 102. Rooms 5530, 5625 (no file, no `roomid:` line). Mobs 9801, 9802 (`id_inventory.py --alloc mobs 2`). Flat coordinates (-7,-81,0) and (-5,-80,1) free over 1306 rooms | `tools/id_inventory.py`; scan |
| Names: `Odile`, `Sefton` have no hit in `git grep` over `_datafiles`, `novel`, `docs`, and no near match in the roster | grep |
| `RegisterMobShop`: `shop:` entries get `RestockQty 5`, `MaxStock` from `EffectiveMaxStock` else 20. Gold floor 500 | `internal/mobs/crafter.go:44-92` |
| A shop mob needs `craft_support`, unless in `BaubleFenceGroups` | `internal/shops/validation.go:39` |
| Resale: no `vendor_categories` refuses every shop, stocked or not, and the dealer's too. Salvage: no recipe and no `salvage_returns`, nothing back. Auctions: NPC buyers match categories; player auctions only move gold between players | `buyrules.go:66`, `actions/salvage.go`, `modules/auctions/npc_buyers.go:187` |
| Bloom faction: "quiet intermediaries who never meet each other by design" | `factions/bloom_trade.yaml` |
| Save fields `questprogress`, `shelvedquests`; alts in `users/<id>.alts.yaml` | `characters/character.go:320,323`; `characters/alts.go:14` |
| Quest 21: `grantNewcomerMarker` at `start.go:267-272`; topics `9100.yaml:117-121`, `9106.yaml:283-287`; `coulee_spine_smoke_test.go:439,445,467` | as named |
| Quest 1000000 is read by no code: `save.go`, `gmcp.Quest.go` name the floor, `save_test.go:120` seeds its own, and `narration/testdata/stores/quests.golden:426` holds its text | grep for `1000000-`, `Generic Quest` |
| `RetiredQuests`: every step under `# retired` and none live drops the quest at load and at `SwapToAlt` | `internal/quests/manifest.go:77-102`, `users/users.go:573`, `userrecord.go:948` |
| Tier guard shape: `tier4_backlog_guard_test.go`. Light golden flag `-update-lighting-daycycle` | `internal/guards/` |

### Story problems re-checked (design section 2)

All confirmed in the source: Hewd at his desk against "I'll have Hewd in by the next bell" (`9316.yaml:55-56`). Gritta
"sent fragments to Coll" against Coll finding them himself (`9381.yaml:102,107-109`; `9320.yaml:106-108`). Item 40111 and
the constable claiming a wrapper and delivery note (`40111`; `9316.yaml:68-69`; `67-*:99-101`). Deren's past tense
(`9379.yaml:5,46-50,57`). Toby's rubbing unread. The re-cut mark against room 5719's unchanging paint (`9332.yaml:40`,
`9338.yaml:28`). Dross laying down a rubbing he never took (`69-*:77`, `9360.yaml:139`). "Part of the quiet" (`70-*:58-59`).
71's reward on `np_dockfolk` and the unconditional +5 (`71-*:73-74`, `9347.yaml:111-113`). Ferrol's "Get past him"
(`9360.yaml:154`). Item 40118 "Gritta took for you" while the player takes it.

### Save behaviour for removed things

- Step in `# renamed`: moved at `LoadUser`, progress and shelf. An alt is moved at its next login after a swap.
- `66-dose` reached by rename holds no purge. Ysolde's `66-dose` variant says so and `detox` gives one.
- `66-addict-fate` flag: left in saves, read by nothing.
- `21-*`: dropped at load. A player mid-tutorial on deploy loses two teaching lines (design: accepted).

---

# PR 5a: docks and Common

### Task 0: Owner save check (owner, read-only, does not block 5a)

**Done 2026-11-04, on the deployed droplet:** control 25, check printed nothing. No save holds `66-escort`,
`66-detox`, `63-report`, `65-report` or `67-report`. The five stay under `# renamed` anyway, for a save restored
from a backup. Skip the steps below.

- [ ] Ask the owner to run this on the droplet. It prints counts only, reads no secrets, changes nothing.

```bash
cd ~/DOGMud/_datafiles
# Control: must print a number above 0, or the path is wrong and the next line proves nothing.
grep -rlE '^\s+30: end\s*$' --include='*.yaml' world/dogmud/users users 2>/dev/null | wc -l
# The check.
grep -rhoE '^\s+(66: (escort|detox)|63: report|65: report|67: report)\s*$' --include='*.yaml' world/dogmud/users users 2>/dev/null | sort | uniq -c
```

- [ ] **Control is 0:** the path is wrong. Ask for `ls ~/DOGMud/_datafiles`, fix the path, rerun.
- [ ] **Control above 0, check prints nothing:** no save holds them. The owner may choose to have the five lines moved
  from `# renamed` to `# retired`, which changes no behaviour. Recommend keeping them renamed: the cost is one line.
- [ ] **Check prints any line:** the five stay under `# renamed` (required, C3). Report the counts in the PR.

Either way `escort_close`, its variants and `66-addict-fate` go, and 5a is safe built as written.

### Task 1: Quest 63 by `search`

**Files:** `quests/63-dock_rat.yaml`, `quests/step-ids.txt`, `quests/loop-backlog.txt`, `dialogue/new_plymouth_docks/9300.yaml`,
`9316.yaml`. Test: create `internal/questengine/tier5_new_plymouth_test.go`, `internal/dialogue/tier5_new_plymouth_test.go`.

- [ ] **Step 1: Failing tests.**

```go
package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func tier5Search(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }

// Rule 14: a step completes on something the player does, never a typed noun.
func TestTier5QuestsHaveNoTypedNouns(t *testing.T) {
	for file, id := range map[string]int{
		"63-dock_rat.yaml": 63,
	} {
		for _, tr := range loadShippedQuest(t, file, id).Triggers {
			require.NotEqual(t, "room_interact", tr.Event, "quest %d", id)
		}
	}
}

func TestQuest63TheBookIsFoundBySearch(t *testing.T) {
	const office, book = 5526, 40100
	q := loadShippedQuest(t, "63-dock_rat.yaml", 63)
	require.Equal(t, []string{"start", "evidence", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(office+1))
	require.Equal(t, "start", w.p.current)
	w.notify("command", tier5Search(office))
	require.Equal(t, "evidence", w.p.current)
	require.Equal(t, []int{book}, w.ctx.givenItems, "the search gives one book")

	lost := newQuestWalker(q).at("evidence")
	lost.notify("command", tier5Search(office))
	require.Equal(t, []int{book}, lost.ctx.givenItems, "a lost book has a spare")

	held := newQuestWalker(q).at("evidence")
	held.p.items[book] = true
	held.notify("command", tier5Search(office))
	require.Empty(t, held.ctx.givenItems, "no second book while one is held")
}

// C3: a save on a removed step must keep going, so each leaves by rename.
func TestTier5RemovedStepsAreRenamed(t *testing.T) {
	m, err := quests.ParseStepManifest(repoFile(t, "_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
	require.NoError(t, err)
	require.Equal(t, "63-evidence", m.Renamed["63-report"])
}
```

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func carrying(ps *PlayerState, ids ...int) *PlayerState {
	has := func(id int) bool {
		for _, i := range ids {
			if i == id {
				return true
			}
		}
		return false
	}
	ps.HasItem, ps.CarriesItem = has, has
	return ps
}

func withFlags(ps *PlayerState, flags map[string]string) *PlayerState {
	ps.GetQuestFlag = func(k string) string { return flags[k] }
	return ps
}

func TestTier5DockRatHandInsFollowTheBook(t *testing.T) {
	for _, file := range []string{"new_plymouth_docks/9300.yaml", "new_plymouth_docks/9316.yaml"} {
		df := loadShippedDialogue(t, file)
		require.NotContains(t, menuKeys(df, carrying(heldState("63-start"), 40100)), "tallyhandoff", file)
		require.Contains(t, menuKeys(df, carrying(heldState("63-start", "63-evidence"), 40100)), "tallyhandoff", file)
	}
}
```

Run `go test ./internal/questengine/ ./internal/dialogue/ -run 'Tier5|Quest63' -count=1`. See them fail on `room_interact`.
**Sabotage after green:** add a `room_interact` trigger to 63 (`TestTier5QuestsHaveNoTypedNouns` red). Drop `give_item`
(`TestQuest63...` red). Move `63-report` to `# retired` (`TestTier5RemovedStepsAreRenamed` red). Put `63-start` back in
`questRequired` (`TestTier5DockRat...` red). Restore each.

- [ ] **Step 2: Replace `quests/63-dock_rat.yaml` from `steps:` down.** Header and `rewards` are unchanged.

```yaml
steps:
  - id: start
    destination:
      room: 5526
    description: >-
      Jesset doesn't buy the theft charge against his hand Teo. He's asked
      you to find who's really been skimming cargo. The foreman, Hewd,
      keeps his books in the office above the constable's post, and Jesset
      told you to mind the floor there.
    hint: >-
      From the Long Quay, where Jesset keeps his board, go north five times
      to the Outer Dock Lane, then east into the Dock Constable's Post and up
      into the Foreman's Office. Type search.
  - id: evidence
    destination:
      mob: [9316, 9300]  # the Dock Constable or Jesset takes the tally-book (owner ruling 16)
    description: >-
      Beneath a loose floorboard in the foreman's office you found Hewd's
      hidden tally-book, with two sets of numbers: proof that he is the one
      skimming and that he hung the shortage on Teo.
    hint: >-
      Take the tally-book to the Dock Constable, just down the stair from the
      Foreman's Office. Jesset on the Long Quay will take it too. Type talk
      constable.
  - id: end
    description: >-
      Hewd's skimming is exposed, his tally-book handed over, and Teo's
      name is cleared. The docks know who really took the cargo.
```

Replace `triggers:` with:

```yaml
triggers:
  # LOST tally-book: Hewd kept a fair copy under the same board. Before the
  # evidence trigger, so it is judged first and one search gives one book.
  - event: command
    command: search
    room: 5526
    conditions:
      has: ["63-evidence"]
      missing: ["63-end"]
      missing_item: 40100
    actions:
      - give_item: 40100
      - actor: >-
          You lift the board again. Hewd kept a fair copy as well as his
          working book, and it is still there in the oilcloth. You take it.
      - observer: "{actor} lifts a loose floorboard and takes something wrapped in oilcloth."

  # EVIDENCE: a search of the foreman's office finds the proud board. The item
  # moves before the grant: a refused item moves no step.
  - event: command
    command: search
    room: 5526
    conditions:
      has: ["63-start"]
      missing: ["63-evidence"]
    actions:
      - give_item: 40100
      - grant: "63-evidence"
      - actor: >-
          One floorboard sits proud of its neighbors, and it lifts with a little
          persuasion. In the gap beneath, wrapped in oilcloth, lies a small
          tally-book, and a glance inside is enough. Two columns of numbers for
          every shipment: the manifest count on the left, the real one on the
          right. Foreman Hewd has been skimming the difference for a long time,
          and the shortage he reported was Teo's only by his choosing.
      - observer: "{actor} works a loose floorboard free and pockets something wrapped in oilcloth."
```

- [ ] **Step 3: Manifest.** In `quests/step-ids.txt` delete line `63-report` and add under `# renamed`: `63-report -> 63-evidence`.

- [ ] **Step 4: Jesset, `9300.yaml`.**

| Where | Old | New |
|---|---|---|
| `tallyhandoff` `questRequired` | `- 63-start` | `- 63-evidence` |
| Variant `63-evidence`, and `63-start` excluding it, hints; offer `dockrat` hints | `Type look floorboard.` / `There, type look floorboard.` | `Type search.` / `There, type search.` |
| Variant `63-end` text | "Teo's back on the board, and Hewd answers to the constable now. That is how it should have gone the first time." | "Teo's back on the board, and the constable has a warrant coming for Hewd. That is how it should have gone the first time." |

- [ ] **Step 5: The constable, `9316.yaml`.** The warrant makes Hewd at his desk true.

| Where | Old | New |
|---|---|---|
| `tallyhandoff` `questRequired` | `- 63-start` | `- 63-evidence` |
| `tallyhandoff` text | "I'll have Hewd in by the next bell, and the boy's name off the board." | "A foreman takes a warrant, and warrants take days. Until it comes back he sits at his desk where I can see him. The boy's name comes off the board today." |
| Variant `63-end` text | "Hewd is answering for his two columns now, and Teo's name is off the board. That's the docks put a little straighter." | "The warrant for Hewd is with the magistrate's clerk, and he is sitting up there pretending not to know it. Teo's name is off the board. That's the docks put a little straighter." |
| Variant `63-end` hints | "...what happens to Hewd now..." | unchanged |
| `hewd` text | "He answers for the difference between his two columns. Teo is back on the manifest, and I'm reading this season's other short counts again with the foreman's hand in mind. I expect I'll find it on more than one page." | "When the warrant comes back he answers for the difference between his two columns. Until then he stays at his desk, where he cannot run without my seeing it. Teo is back on the manifest, and I'm reading this season's other short counts with the foreman's hand in mind. I expect I'll find it on more than one page." |

- [ ] **Step 6: Backlog.** Delete these 7 lines:

```
one-action 63 mob 9300 topic tallyhandoff:prev
one-action 63 mob 9316 topic tallyhandoff:prev
one-action 63 step report:none
walk-dead-end 63 mob 9300 topic tallyhandoff
walk-dead-end 63 mob 9316 topic tallyhandoff
walk-dead-end 63 step report
walk-destination 63 step report
```

- [ ] **Step 7: Run** `go test ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/narration/ . ./internal/guards/ -count=1` and the sweep. Update the narration golden per the brief.
- [ ] **Step 8: Commit** with named paths, message `fix(quests): Dock Rat's book is found by search, and Hewd waits on a warrant`.

### Task 2: Quest 65 by `search`, and where the gray bits come from

**Files:** `quests/65-the_street_sweepers_secret.yaml`, `step-ids.txt`, `loop-backlog.txt`, `dialogue/new_plymouth_common/9320.yaml`,
`dialogue/new_plymouth_old_quarter/9381.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `"65-the_street_sweepers_secret.yaml": 65,` to `TestTier5QuestsHaveNoTypedNouns`, `require.Equal(t, "65-discover", m.Renamed["65-report"])` to the rename test, then append:

```go
func TestQuest65TheSeamIsFoundBySearch(t *testing.T) {
	const foundation, lump = 5624, 40101
	q := loadShippedQuest(t, "65-the_street_sweepers_secret.yaml", 65)
	require.Equal(t, []string{"start", "discover", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(foundation))
	require.Equal(t, "discover", w.p.current)
	require.Equal(t, []int{lump}, w.ctx.givenItems)
	lost := newQuestWalker(q).at("discover")
	lost.notify("command", tier5Search(foundation))
	require.Equal(t, []int{lump}, lost.ctx.givenItems)
}
```

```go
func TestTier5GrittaDoesNotSendColl(t *testing.T) {
	gritta := loadShippedDialogue(t, "new_plymouth_old_quarter/9381.yaml")
	require.NotContains(t, gritta.FindTopic("founding").Text, "Coll")
	require.Contains(t, gritta.FindTopic("finds").Text, "gutters")
	coll := loadShippedDialogue(t, "new_plymouth_common/9320.yaml")
	require.NotContains(t, menuKeys(coll, carrying(heldState("65-start"), 40101)), "sweeperhandoff")
	require.Contains(t, menuKeys(coll, carrying(heldState("65-start", "65-discover"), 40101)), "sweeperhandoff")
}
```

**Sabotage:** restore "to Coll and to Orin" (`TestTier5GrittaDoesNotSendColl` red). Drop the recovery trigger (`TestQuest65...` red on `lost`).

- [ ] **Step 2: Replace `quests/65-the_street_sweepers_secret.yaml` from `steps:` through `- id: end`'s description, then `triggers:`.** Header and `rewards` are unchanged.

```yaml
steps:
  - id: start
    destination:
      room: 5624
    description: >-
      Coll the sweeper has been finding fragments of a strange gray
      material in the gutters for two years. He's asked you to trace them
      to their source. The gutters run down from somewhere, and the oldest
      stone in the quarter lies below, down the Undercroft Stair past the
      Fighting-Pit Gate.
    hint: >-
      The gutters run down to the Old Foundation, below the Undercroft Stair,
      east of the Fighting-Pit Gate in the Common Quarter. Type search there.
  - id: discover
    destination:
      mob: 9320  # follow Coll the Sweeper (schedule np_common_coll)
      room: 5602  # fallback when Coll the Sweeper is not spawned
    description: >-
      Beneath the Common Quarter you found a pre-Founding undercroft, and the
      gray material runs in seams through stonework far older than New
      Plymouth itself. This is the source of Coll's gutter-fragments.
    hint: >-
      Take the lump back to Coll the sweeper in the Common Quarter. Type talk
      coll.
  - id: end
    description: >-
      Coll knows where his fragments come from: the buried city beneath
      the Common, and the gray material seamed through its bones. Some
      rumors, it turns out, are worth confirming quietly.
```

```yaml
triggers:
  # LOST lump: the seam gives up another piece. First, so one search gives one lump.
  - event: command
    command: search
    room: 5624
    conditions:
      has: ["65-discover"]
      missing: ["65-end"]
      missing_item: 40101
    actions:
      - give_item: 40101
      - actor: >-
          You work another lump of the gray material loose from the seam.
          It is just as cold, and just as heavy.
      - observer: "{actor} works a heavy gray lump loose from a seam in the old stonework."

  # DISCOVER: a search of the Old Foundation finds the seam.
  - event: command
    command: search
    room: 5624
    conditions:
      has: ["65-start"]
      missing: ["65-discover"]
    actions:
      - give_item: 40101
      - grant: "65-discover"
      - actor: >-
          You set a hand to the seam and the cold of it goes straight to
          the bone: not the cold of stone but something older and more
          patient. A lump comes loose in your fingers, fist-sized and far
          too heavy: the same gray material Coll finds in fragments, here
          in the mass, seamed through worked stone that was old before New
          Plymouth's first wall was laid. The rain loosens pieces of it and
          the gutters carry them up to the streets above. This is the
          source.
      - observer: "{actor} works a heavy gray lump loose from a seam in the old stonework."
```

- [ ] **Step 3: Manifest.** Delete `65-report`; add under `# renamed`: `65-report -> 65-discover`.

- [ ] **Step 4: Coll, `9320.yaml`.** `sweeperhandoff` `questRequired`: `65-start` becomes `65-discover`.
Variant `65-discover` hints become: "If you lost the lump, more of the gray material is in the seam below. The Old Foundation is under the Undercroft Stair, east of the Fighting-Pit Gate. Type search there."
Variant `65-start` hints become: "The Old Foundation is under the Undercroft Stair, east of the Fighting-Pit Gate. Type search there."
Variant `65-start` text: old "Look at what runs through those walls." stays. Offer `tracefragments` hints: old "There, type look seam." new "There, type search."

- [ ] **Step 5: Gritta, `9381.yaml`.**

| Where | Old | New |
|---|---|---|
| `founding` text, last sentences | "I have sent the best fragments to Coll and to Orin. They can read what I cannot." | "I have sent the best fragments up to Orin. He can read what I cannot." |
| `founding` hints | "She named Coll and Orin, contacts up in the city who receive her finds." | "She named Orin, up in the Crafting Quarter, who receives her finds." |
| `finds` text | whole field | "Orin keeps a bookstall in the Crafting Quarter. He has old survey records and maps from before the colony that the Archive does not officially hold. What I can carry, I send to him. My part is the finding. The reading is his. The small pieces I cannot keep. The flood takes them and the rain carries them up the old drains into the gutters of the Common. There is a sweeper up there, Coll, who picks them out without knowing what they are. I have never met him." |
| `finds` hints | whole field | "She named Orin in the Crafting Quarter, and a sweeper named Coll who has never met her." |

- [ ] **Step 6: Backlog.** Delete these 7 lines:

```
hint-length 65 step discover
hint-length 65 step start
one-action 65 mob 9320 topic sweeperhandoff:prev
one-action 65 step report:none
walk-dead-end 65 mob 9320 topic sweeperhandoff
walk-dead-end 65 step report
walk-destination 65 step report
```

- [ ] **Step 7:** Run as Task 1 Step 7. **Commit:** `fix(quests): The Street Sweeper's Secret by search, and Coll truly does not know the source`.

### Task 3: Quest 66 is deliver, return, heal

**Files:** `quests/66-the_addicts_plight.yaml`, `step-ids.txt`, `loop-backlog.txt`, `dialogue/topic-keys.txt`,
`dialogue/new_plymouth_common/9323.yaml`, `dialogue/new_plymouth_docks/9392.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `require.Equal(t, "66-dose", m.Renamed["66-escort"])` and `require.Equal(t, "66-dose", m.Renamed["66-detox"])` to the rename test. Append:

```go
// Quest 66 is deliver, return, heal (owner ruling 2): Ysolde takes the wrapper
// and gives her purge, and Teels takes the purge.
func TestQuest66DeliverReturnHeal(t *testing.T) {
	const ysolde = 9323
	q := loadShippedQuest(t, "66-the_addicts_plight.yaml", 66)
	require.Equal(t, []string{"start", "dose", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	require.Empty(t, q.Flags, "66-addict-fate went with escort_close")

	w := newQuestWalker(q).at("start")
	w.notify("topic_heard", EventDetails{MobId: ysolde, Topic: "bloom"})
	require.Equal(t, "start", w.p.current)
	w.notify("topic_heard", EventDetails{MobId: ysolde, Topic: "wrapperhandoff"})
	require.Equal(t, "dose", w.p.current)

	y := loadShippedDialogue(t, "new_plymouth_common", "9323.yaml")
	in := y.FindTopic("wrapperhandoff")
	require.Equal(t, 40110, in.RequiresItem)
	require.Equal(t, 40109, in.GivesItem)
	require.Empty(t, in.GrantsQuest, "a mid step is a topic_heard trigger")
	require.Nil(t, y.FindTopic("escort_close"))
	require.NotNil(t, y.FindTopic("detox"), "her info topic stays, and replaces a lost purge")

	teels := loadShippedDialogue(t, "new_plymouth_docks", "9392.yaml")
	out := teels.FindTopic("purge_handoff")
	require.Equal(t, 40109, out.RequiresItem)
	require.Equal(t, []string{"66-dose"}, out.QuestRequired)
	require.Equal(t, "66-end", out.GrantsQuest)
	require.Equal(t, []string{"66-end"}, teels.FindTopic("q67_grant").QuestRequired)
	require.Nil(t, y.FindTopic("q67_grant"), "C1: the offer moved to Teels")
}
```

```go
func TestTier5YsoldeReplacesALostPurge(t *testing.T) {
	y := loadShippedDialogue(t, "new_plymouth_common/9323.yaml")
	text, _ := y.OpenerFor(heldState("66-start", "66-dose"))
	require.Contains(t, text, "Where is the cup")
	require.Contains(t, menuKeys(y, heldState("66-start", "66-dose")), "detox")
	text, _ = y.OpenerFor(carrying(heldState("66-start", "66-dose"), 40109))
	require.Contains(t, text, "You have the cup")
}
```

**Sabotage:** put `requiresItem` on `purge_handoff` to 40110 (`TestQuest66...` red). Swap the two `66-dose` variants (`TestTier5Ysolde...` red). Remove the `topic_heard` trigger (`TestQuest66...` red on "dose").

- [ ] **Step 2: Replace `quests/66-the_addicts_plight.yaml` whole.**

```yaml
questid: 66
name: The Addict's Plight
description: >-
  A Bloom addict named Teels is barely holding on, collapsing on the Lower
  Wharf, clear only in flashes. They press a spent wax wrapper into your
  hands and beg you to find the healer Ysolde in the Common Quarter. Carry
  it to her, bring back what she gives you, and see Teels through it.
secret: false

steps:
  - id: start
    destination:
      mob: 9323  # follow Ysolde (schedule np_common_ysolde)
      room: 5620  # fallback when Ysolde is not spawned
    description: >-
      Teels, a Bloom addict on the Lower Wharf, begged you for help. They
      pressed a spent wax wrapper into your hands. The copper-flower smell of
      Bloom is on the cloth, and a healer who knows that smell may know what
      to do.
    hint: >-
      Ysolde's Room is off the Back-Alley in the Common Quarter, east of
      Sweeper's Corner. Show her the wax wrapper. Type talk ysolde.
  - id: dose
    destination:
      mob: 9392  # Teels stays on the Lower Wharf
    description: >-
      Ysolde took the wrapper and knew the copper-flower note at once. She
      gave you her purge in a sealed clay cup, for Teels and not for you. She
      made something plain: the purge holds only until the craving returns,
      and the craving returns every time while the source is still running.
    hint: >-
      Take the purge back to Teels on the Lower Wharf. Do not drink it. Type
      talk teels.
  - id: end
    description: >-
      Teels is through the worst of it and stable. You learned what the Bloom
      does to a person up close, that a healer in the Common knows the symptoms
      by heart, and that the trade has a source somewhere on the docks that is
      still running.

rewards:
  gold: 25
  rep_faction: np_commonfolk
  rep_amount: 10
  actor: >-
    Teels gets the cup down, and the purge does what Ysolde said it would, and
    it is ugly to watch. When it is over they are shaking, and clear, and still
    here. Ysolde told you the rest: the purge holds until the craving comes back,
    and the craving comes back while the source is still running. The wrapper is
    gone but the smell stays with you. The docks know where the Bloom comes from,
    or someone on them does.
  observer: >-
    The traveller looks up from a quiet exchange with the look
    of someone who has just learned something they cannot unlearn.

triggers:
  # dose: hearing Ysolde's "wrapperhandoff" topic, which takes the wrapper and
  # hands over the purge.
  - event: topic_heard
    mob: 9323
    topic: wrapperhandoff
    conditions:
      has: ["66-start"]
      missing: ["66-dose"]
    actions:
      - grant: "66-dose"
```

- [ ] **Step 3: Manifests.** `step-ids.txt`: delete `66-detox`, `66-escort`; add `66-dose` between `66-end`'s neighbours (sorted, above `66-end`); under `# renamed` add `66-escort -> 66-dose` and `66-detox -> 66-dose`. `topic-keys.txt`: delete `9323.escort_close` and `9323.q67_grant`; add `9392.purge_handoff`, `9392.q67_grant` beside `9392.plea`. Under `# retired`:

```
# Quest 66 ends at Teels (#375 phase 7 tier 5): a save at 66-escort is renamed to
# 66-dose, so Ysolde's hand-in for it can never be chosen.
9323.escort_close
```

and under `# renamed`: `9323.q67_grant -> 9392.q67_grant`.

- [ ] **Step 4: Ysolde, `9323.yaml`.**
  - Variant `66-end` hints: old "If you want to follow the Bloom to whoever sells it, choose The Bloom Trail below." new "If you want to follow the Bloom to whoever sells it, Teels on the Lower Wharf knows where it starts. Type talk teels."
  - Delete the `66-escort` variant (lines 45-51). In its place:

```yaml
  - questRequired:
    - 66-dose
    questExcluded:
    - 66-end
    carriesItem: 40109
    text: You have the cup. Why are you still here? Teels needs it more than you
      need to watch me work.
    hints: Take the purge back to Teels on the Lower Wharf. Do not drink it. Type
      talk teels.
  - questRequired:
    - 66-dose
    questExcluded:
    - 66-end
    text: Where is the cup? Lost, or did you drink it yourself? Ask, and I will give
      you another.
    hints: Choose the line about a cure below, and Ysolde gives you another purge.
```

  - The two `66-start` variants add `66-dose` to `questExcluded`.
  - Delete topics `escort_close` (with its comment) and `q67_grant`. Replace `wrapperhandoff`:

```yaml
- key: wrapperhandoff
  kind: info
  label: Teels sent me to you with this wrapper.
  text: This wrapper. I know this smell. Copper-flower, under the wax. I've been
    taking it off people's skin for the better part of two seasons. Teels sent
    you? Then they need the purge, not just sympathy. Here, a sealed cup. Take it
    back to the wharf and see that they drink it, because they will not reach me on
    their own feet. But hear me on this. The purge holds until the craving comes
    back, and the craving comes back every time while the source is still running.
    Someone is putting this on the docks. That is the problem I cannot fix.
  hints: Ysolde takes the wrapper and hands you her purge in a sealed clay cup. Take
    it back to Teels on the Lower Wharf. Do not drink it. Type talk teels.
  marked: true
  questRequired:
  - 66-start
  questExcluded:
  - 66-dose
  - 66-end
  requiresItem: 40110
  givesItem: 40109
```

  - `teels_q66` text, old first sentences "I've seen to Teels. The purge is a rough crossing." new "So you saw Teels through it. The purge is a rough crossing."
  - `detox` stays as it is.

- [ ] **Step 5: Teels, `9392.yaml`.** Replace `variants:` below the `67-end` variant, keeping that one first:

```yaml
  - questRequired:
    - 66-end
    questExcluded:
    - 67-start
    text: You came back. Most don't. The purge holds, for now. The source is still
      running, and I know it is. I'm not going back. I just know it is still there.
    hints: If you want to follow the Bloom to whoever sells it, choose The Bloom Trail
      below.
  - questRequired:
    - 66-end
    text: You came back. Most don't. The purge holds, for now. The source is still
      running, and I know it is. I'm not going back. I just know it is still there.
    hints: Teels seems steadier, but clear about what the purge cannot fix while the
      Bloom still flows from somewhere on the docks.
  - questRequired:
    - 66-dose
    questExcluded:
    - 66-end
    carriesItem: 40109
    text: Is that from her? Give it here. Please.
    hints: Choose The Addict's Plight below to give Teels the purge.
  - questRequired:
    - 66-dose
    questExcluded:
    - 66-end
    text: She gave you something for me? Where is it?
    hints: If you lost the purge, Ysolde has more, in her room off the Back-Alley in
      the Common Quarter. Type talk ysolde.
  - questRequired:
    - 66-start
    questExcluded:
    - 66-dose
    - 66-end
    carriesItem: 40110
    text: I'm... managing. Go. The alley, Common Quarter. Ysolde. She'll know the
      smell.
    hints: Ysolde's Room is off the Back-Alley in the Common Quarter, east of
      Sweeper's Corner. Type talk ysolde.
  - questRequired:
    - 66-start
    questExcluded:
    - 66-dose
    - 66-end
    text: Where's the wrapper? You need it. She has to smell it.
    hints: If you lost the wax wrapper, choose to tell Teels below.
```

`lost_wrapper` `questExcluded`: `66-escort` becomes `66-dose`. Topic `plea` keeps its text. Add after `plea`:

```yaml
- key: purge_handoff
  kind: handin
  quest: 66
  label: The Addict's Plight
  playerLine: Ysolde sent this back for you. Drink it.
  text: From her. I know what this does. I know. Give it here. Stay a minute, would
    you. I don't want to do this one alone.
  questRequired:
  - 66-dose
  questExcluded:
  - 66-end
  requiresItem: 40109
  grantsQuest: 66-end
- key: q67_grant
  kind: offer
  quest: 67
  label: The Bloom Trail
  playerLine: Do you want help finding where the Bloom comes from?
  text: She did what she could. So did you. But the craving comes back, every time,
    while the source keeps running. I can tell you where my wrappers came from.
    There is a draper named Marn on the docks, at the north end of the Outer Dock
    Lane. A clean shop, respectable-looking. I never saw what he moves or how it
    moves. I only know what I paid him for. He is where the trail starts.
  hints: Accept, then go to Marn's fabric shop on the docks, at the north end of the
    Outer Dock Lane.
  questRequired:
  - 66-end
  questExcluded:
  - 67-start
  - 67-end
  grantsQuest: 67-start
```

- [ ] **Step 6: Backlog.** Delete these 9 lines:

```
hint-length 66 step start
one-action 66 mob 9323 topic escort_close:prev
one-action 66 mob 9323 topic wrapperhandoff:prev
one-action 66 step detox:none
one-action 66 step escort:none
walk-dead-end 66 mob 9323 topic wrapperhandoff
walk-dead-end 66 step escort
walk-destination 66 step detox
walk-destination 66 step escort
```

- [ ] **Step 7:** Run as Task 1 Step 7. `TestWalk` must show no `walk-no-source`: the purge's source is `detox`, ungated. **Commit:** `fix(quests): The Addict's Plight is deliver, return, heal, and Teels offers The Bloom Trail`.

### Task 4: Quest 67, the steps stay and the story agrees

**Files:** `quests/67-the_bloom_trail.yaml`, `step-ids.txt`, `loop-backlog.txt`, `items/materials-40000/40111-case_file.yaml`,
`dialogue/new_plymouth_docks/9316.yaml`, `dialogue/new_plymouth_old_quarter/9379.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `require.Equal(t, "67-witness", m.Renamed["67-report"])`. Add `"66-the_addicts_plight.yaml": 66, "67-the_bloom_trail.yaml": 67,` to the no-typed-nouns map. Append:

```go
func TestQuest67HasNoReportStep(t *testing.T) {
	q := loadShippedQuest(t, "67-the_bloom_trail.yaml", 67)
	require.Equal(t, []string{"start", "front", "lintel", "delivery", "source", "witness", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())
	require.Contains(t, q.Steps[3].Hint, "bring a light", "the Old Quarter is dark")
}
```

```go
func TestTier5CaseFileHandInFollowsTheWitness(t *testing.T) {
	c := loadShippedDialogue(t, "new_plymouth_docks/9316.yaml")
	require.NotContains(t, menuKeys(c, carrying(heldState("67-start"), 40111)), "casefilehandoff")
	require.Contains(t, menuKeys(c, carrying(heldState("67-start", "67-witness"), 40111)), "casefilehandoff")
	require.NotContains(t, c.FindTopic("casefilehandoff").Text, "wrapper")
	d := loadShippedDialogue(t, "new_plymouth_old_quarter/9379.yaml")
	require.NotContains(t, d.FindTopic("operation").Text, "no longer runs")
	for _, g := range d.Greetings {
		require.NotContains(t, g.Text, "already closed")
	}
}
```

**Sabotage:** restore "The wrapper." in `casefilehandoff` (`TestTier5CaseFile...` red). Drop "bring a light" from `delivery` (`TestQuest67...` red).

- [ ] **Step 2: Replace `quests/67-the_bloom_trail.yaml` from `flags:` through the `end` step.** `rewards` and `triggers` are unchanged.

```yaml
flags:
  - key: entry
    values: [undercover, evidence]
    description: "How the player got past Marn: by posing as a user, or by evidence and leverage"

steps:
  - id: start
    destination:
      mob: 9305  # follow Marn the Draper (schedule np_docks_marn)
      room: 5523  # fallback when Marn the Draper is not spawned
    description: >-
      Trace the Bloom to its source. It starts with a clean little
      draper's shop on the docks.
    hint: >-
      Marn's fabric shop is at the north end of the Outer Dock Lane, on the
      docks. Type talk marn.
  - id: front
    destination:
      mob: 9345  # follow Falk the Auctioneer (schedule np_merchant_falk)
      room: 5808  # fallback when Falk the Auctioneer is not spawned
    description: >-
      You got past Marn's front. The supply comes up from an address in
      the Old Quarter, 215 Lintel Street, routed through people who
      never ask questions.
    hint: >-
      The auctioneer Falk, at his Auction House in the Merchant Quarter, knows
      who holds the Lintel Street property. Type talk falk.
  - id: lintel
    destination:
      mob: 9369  # follow Wenna the Servant (schedule np_noble_wenna)
      room: 6013  # fallback when Wenna the Servant is not spawned
    description: >-
      Falk knows the Lintel Street address as a quiet property held
      through nominal owners.
    hint: >-
      The servant Wenna, in the Noble Quarter, has seen the unmarked crates.
      She frightens easily. Type talk wenna.
  - id: delivery
    destination:
      mob: 9379  # follow Deren (schedule np_oldquarter_deren)
      room: 6029  # fallback when Deren is not spawned
    description: >-
      Wenna confirmed the delivery house: unmarked goods at odd hours,
      under the bloodline office's eye.
    hint: >-
      Deren keeps the house at 215 Lintel Street, down in the Old Quarter, past
      the Underdocks. It is dark down there, so bring a light. Type talk deren.
  - id: source
    destination:
      mob: 9380  # follow Quill the Lamplighter (schedule np_oldquarter_quill)
      room: 6031  # fallback when Quill the Lamplighter is not spawned
    description: >-
      You found 215 Lintel Street: the production room, the apparatus,
      and Deren, exposed but untouchable, the bloodline office at his
      back.
    hint: >-
      Find a witness. The lamplighter Quill walks the canal off Lower Lintel
      Street, and he has seen what passes the iron door. Type talk quill.
  - id: witness
    destination:
      room: 5525
    description: >-
      Quill gave you his account, eleven years of dates and times in his own
      hand. That is a case the watch cannot ignore.
    hint: >-
      Take the case file to the Dock Constable, in his post off the Outer Dock
      Lane. Type talk constable.
  - id: end
    description: >-
      The Bloom trade is disrupted and the truth is known. Some powers
      in this city you can reach, and some you cannot. Teels, at least,
      has a chance.
```

- [ ] **Step 3: Manifest.** Delete `67-report`; add under `# renamed`: `67-report -> 67-witness`.

- [ ] **Step 4: Item.** `items/materials-40000/40111-case_file.yaml` `description` becomes:

```yaml
description: >-
  A folded bundle in a steady, plain hand: eleven years of a lamplighter's dates
  and times, what he saw pass the iron door on Lintel Street and at what hour,
  signed at the foot. It is one honest man's account, and it is the kind the
  watch cannot wave off.
```

- [ ] **Step 5: Constable, `9316.yaml`.** `casefilehandoff` `questRequired`: `67-start` becomes `67-witness`. Text, old "Let me read this properly. The wrapper. The address. A delivery account and the lamplighter's word. This is enough to move on the trade." new "Let me read this properly. Eleven years of dates and times, in the lamplighter's own hand, and the address they all point at. This is enough to move on the trade."

- [ ] **Step 6: Deren, `9379.yaml`.** His lines must hold whether or not the quest is over.

| Where | Old | New |
|---|---|---|
| Greeting 1 | "You found the address. Whatever you came to close is already closed. But ask what you came to ask." | "You found the address. Ask what you came to ask." |
| Opener hints | "...the arrangement that ran from this address..." | "...the arrangement that runs through this address..." |
| `operation` label | "What arrangement ran from this address?" | "What runs through this address?" |
| `operation` text | whole field | "A supply arrangement, or it was the last time I was asked. I manage the handing out. The quantities, the timing, the contacts. I do not need to know more than my part, and I do not know more than my part. Whatever questions remain are for people with the authority to press them, and none of those people are you." |
| `marn` text | "Whether he carries on now that this end has gone quiet, I would not know." | "Whether he carries on when this end goes quiet, I would not know." |

- [ ] **Step 7: Backlog.** Delete these 11 lines:

```
hint-length 67 step delivery
hint-length 67 step front
hint-length 67 step lintel
hint-length 67 step source
hint-length 67 step start
hint-length 67 step witness
one-action 67 mob 9316 topic casefilehandoff:prev
one-action 67 step report:none
walk-dead-end 67 mob 9316 topic casefilehandoff
walk-dead-end 67 step report
walk-destination 67 step report
```

- [ ] **Step 8:** Run as Task 1 Step 7. **Commit:** `fix(quests): The Bloom Trail's case file is Quill's own, and Deren stops closing the case early`.

### Task 5: 5a guard, docs, gates, PR

**Files:** create `internal/guards/tier5_backlog_guard_test.go`; `docs/PATCH_NOTES.md`, `docs/README.md`.

- [ ] **Step 1: The guard.**

```go
package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
)

// The #375 phase 7 tier 5 pass (New Plymouth). A loop finding on one of its
// quests is fixed in content, never listed.
var tier5Quests = map[int]bool{63: true, 65: true, 66: true, 67: true}

func tier5BacklogLines(t *testing.T, data []byte) []string {
	t.Helper()
	b, err := questloop.ParseBacklog(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range b {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if id, err := strconv.Atoi(fields[1]); err == nil && tier5Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier5QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier5BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 5 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier5QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 66 step start\none-action 63 step report:none\nchain 76 mob 9553 topic x\n")
	if got := tier5BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 5 lines, want 2: %v", len(got), got)
	}
}
```

- [ ] **Step 2: Patch note,** at the top of `docs/PATCH_NOTES.md`:

```markdown
## 2026-11-05: Down on the docks

- Dock Rat and The Street Sweeper's Secret are now followed by searching: where the
  trail leads you, type search.
- The Addict's Plight is a longer walk. Ysolde sends you back to Teels with what they
  need, and Teels points you at the next thread.
- The Bloom Trail's case file is now the lamplighter's own record. The hints on that
  trail tell you who to find and let you find your own way.
- Foreman Hewd keeps his desk for a good reason now: the constable is waiting on a
  warrant.
```

- [ ] **Step 3: `docs/README.md`,** beside the tier 4 guard row and the tier 5 design row:

```markdown
| [`superpowers/plans/2026-11-04-p7-tier5-new-plymouth.md`](superpowers/plans/2026-11-04-p7-tier5-new-plymouth.md) | Plan for the tier 5 design, four PRs: 5a docks and Common, 5b Crafting, Temple, Merchant and Old Quarter, 5c the Bloom chain, 5d quest 21 and the closing playtest |
| [`../internal/guards/tier5_backlog_guard_test.go`](../internal/guards/tier5_backlog_guard_test.go) | Content guard for the #375 phase 7 tier 5 pass: no `loop-backlog.txt` line may name a tier 5 quest, with a fixture proving the guard can fail |
| [`../internal/questengine/tier5_new_plymouth_test.go`](../internal/questengine/tier5_new_plymouth_test.go) | Step tests for tier 5: each search, each recovery, 66's return walk, and every removed step leaving by rename |
| [`../internal/dialogue/tier5_new_plymouth_test.go`](../internal/dialogue/tier5_new_plymouth_test.go) | Shipped-dialogue tests for tier 5: hand-ins follow the last step, and the story fixes hold |
```

- [ ] **Step 4: Gates.** From the worktree root. `S` is the session scratchpad the system prompt names.

```bash
gofmt -l internal/ modules/                                   # prints nothing
go build ./...                                                # no output
go test . ./internal/guards/ ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/narration/ -count=1   # all ok
go test ./... -count=1                                        # all ok
DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1         # ok
~/go/bin/golangci-lint run --new-from-merge-base=origin/master # 0 issues
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones new_plymouth_common,new_plymouth_docks,new_plymouth_old_quarter > "$S/dc.txt"; echo $?   # 0
grep -c escort_close "$S/dc.txt"                              # 0, on its own line (exit 1 is the pass)
grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .   # 30
grep -nP '\x{2014}|\x{2013}| -- ' _datafiles/world/dogmud/quests/6[3567]-*.yaml   # nothing (exit 1 is the pass)
```

Isolated boot:

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1   # exit 124 is success
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"              # 0
grep -c "Server Ready" "$S/boot.log"                                                     # 1
```

- [ ] **Step 5: Commit** with named paths, `test(quests): tier 5 backlog guard, and notes for the docks`. Push, `gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-tier5a`, with the Task 0 answer if it has come. Do not write a closing keyword for #375. Wait for all CI jobs, merge with `--merge`, remove the worktree.

---

# PR 5b: Crafting, Temple, Merchant, Old Quarter

### Task 6: Quest 68 by `search`

**Files:** `quests/68-the_cooperage_circle.yaml`, `loop-backlog.txt`, `dialogue/new_plymouth_crafting/9332.yaml`, `9338.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `"68-the_cooperage_circle.yaml": 68,` to the no-typed-nouns map. Append:

```go
// A search on the Cooperage Floor finds the survey at any hour: Toby may be
// asleep on the cot beside it. Old saves need no catch-up: a save holding the
// survey at 68-evidence takes a harmless spare, and one at 68-choice without it
// takes the recovery.
func TestQuest68TheSurveyIsFoundBySearch(t *testing.T) {
	const floor, survey = 5720, 40113
	q := loadShippedQuest(t, "68-the_cooperage_circle.yaml", 68)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(floor))
	require.Equal(t, "start", w.p.current, "Toby's word comes first")
	require.Empty(t, w.ctx.givenItems)
	w = newQuestWalker(q).at("evidence")
	w.notify("command", tier5Search(floor))
	require.Equal(t, "survey", w.p.current)
	require.Equal(t, []int{survey}, w.ctx.givenItems)
	old := newQuestWalker(q).at("choice")
	old.notify("command", tier5Search(floor))
	require.Equal(t, "choice", old.p.current)
	require.Equal(t, []int{survey}, old.ctx.givenItems)
	done := newQuestWalker(q).at("end")
	done.notify("command", tier5Search(floor))
	require.Empty(t, done.ctx.givenItems, "no survey after the choice")
}
```

```go
func TestTier5TheMarkIsReCutWhereNobodyLooks(t *testing.T) {
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	require.Contains(t, orin.FindTopic("circle_choice").Text, "inner door")
	require.Contains(t, orin.FindTopic("q68_choice").Text, "rubbing")
	text, _ := orin.OpenerFor(withFlags(heldState("68-start", "68-evidence", "68-survey", "68-choice", "68-end"), map[string]string{"68-allegiance": "circle"}))
	require.Contains(t, text, "inner door")
	text, _ = orin.OpenerFor(carrying(heldState("68-start", "68-evidence", "68-survey"), 40112))
	require.Contains(t, text, "Asha")
}
```

**Sabotage:** give the search `has: ["68-start"]` (`TestQuest68...` red on "Toby's word"). Delete the recovery (red on `old`). Restore "beside the door" (`TestTier5TheMark...` red).

- [ ] **Step 2: `quests/68-the_cooperage_circle.yaml`.** Replace hints:

| Step | New hint |
|---|---|
| `start` | The Abandoned Cooperage is north of Craftwell Square, up the lane from the Inkwalk. Find Toby. Type talk toby. |
| `evidence` | Edvar's survey is under a loose floorboard by the cot, on the Cooperage Floor north of the front. Type search there. |
| `survey` | unchanged |
| `choice` | Orin will tell you what standing with the circle means. Clerk Vell, in the Permit Office, takes the survey for the bloodline. Neither can be undone. Type talk orin. |

Remove the long dash from each `# follow` comment. The comment on `survey` about `look floorboard` becomes "Granted by a search of the Cooperage Floor. Orin's choice needs it." Replace `triggers:` whole:

```yaml
triggers:
  # evidence: hearing Toby the Cooper's Lad's "q68_toby" topic, which gives
  # the bench-mark rubbing.
  - event: topic_heard
    mob: 9338
    topic: q68_toby
    conditions:
      has: ["68-start"]
      missing: ["68-evidence"]
    actions:
      - grant: "68-evidence"

  # choice: hearing Orin the Bookseller's "q68_choice" topic.
  - event: topic_heard
    mob: 9332
    topic: q68_choice
    conditions:
      has: ["68-survey"]
      missing: ["68-choice"]
    actions:
      - grant: "68-choice"

  # LOST survey, and old saves at 68-choice that never lifted the board. First,
  # so one search gives one survey.
  - event: command
    command: search
    room: 5720
    conditions:
      has: ["68-survey"]
      missing: ["68-end"]
      missing_item: 40113
    actions:
      - give_item: 40113
      - actor: >-
          You lift the loose floorboard by the cot, quietly. Edvar's survey is
          back in its oilcloth where someone returned it, or another quarter of
          it is. You ease it free.
      - observer: "{actor} works a loose floorboard free and lifts something wrapped in oilcloth."

  # SURVEY: a search of the Cooperage Floor finds it, Toby asleep beside it or not.
  - event: command
    command: search
    room: 5720
    conditions:
      has: ["68-evidence"]
      missing: ["68-survey"]
    actions:
      - give_item: 40113
      - grant: "68-survey"
      - actor: >-
          A floorboard by the cot sits proud of its neighbors, as Toby said. You
          keep your hands quiet. Beneath it, wrapped in oilcloth against the damp,
          is a torn quarter of a survey in an old, exact hand: Edvar's work. You
          ease it free and pocket it.
      - observer: "{actor} works a loose floorboard free and lifts something wrapped in oilcloth."
```

- [ ] **Step 3: Orin, `9332.yaml`.**

| Where | Old | New |
|---|---|---|
| Variant `68-end` (not bloodline) text | "Toby has re-cut the mark beside the cooperage door, and it reads as a working shop again." | "Toby has re-cut the mark on the inner door, where an inspector looks and the street does not, and it reads as a working shop again." |
| Variants `68-choice` no survey, `68-evidence` hints | `Type look floorboard.` | `Type search.` |
| Variant `68-start` hints | "Toby keeps it by day. Type talk toby." | "Type talk toby." |
| `q68_choice` text | "So it is true. The tools are kept, the mark is clean, the circle did not all leave, and Edvar's last survey survived them." | "So it is true. The tools are kept, the circle did not all leave, and Edvar's last survey survived them. Toby's rubbing says the same, if you still have it: Asha's name, and the line she never finished." |
| `q68_choice` text | "and Toby re-cuts the guild mark beside the door so it reads as a working shop." | "and Toby re-cuts the guild mark on the inner door so it reads as a working shop to whoever inspects it." |
| `circle_choice` text | "and Toby re-cuts the mark tonight so it reads as a working shop." | "and Toby re-cuts the mark on the inner door tonight, where an inspector looks and the street does not." |

Add before the `68-survey` variant:

```yaml
  # Toby's rubbing is read here, and nothing takes it from the player.
  - questRequired:
    - 68-survey
    questExcluded:
    - 68-choice
    - 68-end
    carriesItem: 40112
    text: You have Edvar's hand, and Toby's rubbing with it. Asha's name, and the line
      she never finished. He keeps that bench as if she were coming back.
    hints: Choose to tell Orin you found Edvar's survey.
```

- [ ] **Step 4: Toby, `9338.yaml`.** Hints `Type look floorboard.` (variants `68-choice` no survey, `68-evidence`; topic `q68_toby`) become `Type search there.` Variant `68-end` text: old "The mark beside the door is re-cut and reads true again. Let the clerk call that derelict. Thank you." new "The mark on the inner door is re-cut and reads true again. The old paint outside can fade. Let the clerk call that derelict. Thank you."

- [ ] **Step 5: Backlog.** Delete these 5 lines:

```
hint-length 68 step choice
one-action 68 quest 68 trigger room_interact room=5720 noun=floorboard grants=68-survey #2:cond
one-action 68 quest 68 trigger room_interact room=5720 noun=floorboard grants=68-survey:cond
one-action 68 quest 68 trigger room_interact room=5720 noun=loose floorboard grants=68-survey #2:cond
one-action 68 quest 68 trigger room_interact room=5720 noun=loose floorboard grants=68-survey:cond
```

- [ ] **Step 6:** Run as Task 1 Step 7. **Commit:** `fix(quests): The Cooperage Circle's survey by search, and Orin reads Toby's rubbing`.

### Task 7: Quest 69 by `search`, and Dross keeps the rubbing

**Files:** `quests/69-the_gallery_cipher.yaml`, `loop-backlog.txt`, `dialogue/new_plymouth_temple/9360.yaml`, `dialogue/new_plymouth_noble/9371.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `"69-the_gallery_cipher.yaml": 69,` to the map. Append:

```go
func TestQuest69DrossKeepsTheRubbing(t *testing.T) {
	const sanctuary, rubbing = 5901, 40115
	q := loadShippedQuest(t, "69-the_gallery_cipher.yaml", 69)
	require.NoError(t, q.Validate())
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(sanctuary))
	require.Equal(t, "rubbing", w.p.current)
	require.Equal(t, []int{rubbing}, w.ctx.givenItems)
	lost := newQuestWalker(q).at("gallery")
	lost.notify("command", tier5Search(sanctuary))
	require.Equal(t, []int{rubbing}, lost.ctx.givenItems)

	end := newQuestWalker(q).at("end")
	end.p.items[rubbing] = true
	end.notify("quest_granted", EventDetails{QuestToken: "69-end"})
	require.Contains(t, end.ctx.consumedItems, rubbing)
}
```

```go
func TestTier5DrossNeedsBothPapers(t *testing.T) {
	d := loadShippedDialogue(t, "new_plymouth_temple/9360.yaml")
	held := []string{"69-start", "69-rubbing", "69-gallery", "69-reading"}
	require.Contains(t, menuKeys(d, carrying(heldState(held...), 40115, 40116)), "give_40116")
	require.NotContains(t, menuKeys(d, carrying(heldState(held...), 40116)), "give_40116", "no rubbing")
	require.NotContains(t, menuKeys(d, carrying(heldState("69-start", "69-rubbing", "69-gallery"), 40115, 40116)), "give_40116", "the step before")
	text, _ := d.OpenerFor(carrying(heldState(held...), 40116))
	require.Contains(t, text, "where is the rubbing")
	require.NotContains(t, d.FindTopic("q69_gallery").Text, "Get past him")
}
```

**Sabotage:** delete the `quest_granted` trigger (`TestQuest69...` red). Drop `carriesItem: 40115` (`TestTier5DrossNeeds...` red on "no rubbing").

- [ ] **Step 2: `quests/69-the_gallery_cipher.yaml`.** Hints:

| Step | New hint |
|---|---|
| `start` | The processional reliefs run along the north wall of the Grand Temple Sanctuary, south twice from the Temple Courtyard. Type search there. |
| `rubbing` | Take the rubbing to Scholar Dross at the Grand Temple. Type talk dross. |
| `gallery` | Keeper Lysha keeps the Art Gallery in the Noble Quarter, north of the Administrative Office. Type talk lysha. |
| `reading` | Bring Lysha's reading and your rubbing back to Scholar Dross at the Grand Temple. Type talk dross. |

Remove the long dash from each comment. Replace `triggers:` whole:

```yaml
triggers:
  # Lost rubbing: Dross and Lysha both need to see it, and Dross keeps it at the
  # end. First, so one search gives one rubbing.
  - event: command
    command: search
    room: 5901
    conditions:
      has: ["69-rubbing"]
      missing: ["69-end"]
      missing_item: 40115
    actions:
      - give_item: 40115
      - actor: >-
          You press a fresh sheet to the processional relief and work the
          charcoal across it again. The hands come up in the same precise
          row, the same counted sequence.
      - observer: "{actor} takes a careful charcoal rubbing of the old processional reliefs."

  # A search of the sanctuary finds the sequence in the reliefs.
  - event: command
    command: search
    room: 5901
    conditions:
      has: ["69-start"]
      missing: ["69-rubbing"]
    actions:
      - give_item: 40115
      - grant: "69-rubbing"
      - actor: >-
          You press paper to the processional relief and work charcoal
          across it. The figures come up under your hand, and the longer
          you look, the less devotional they seem. Each hand is positioned
          too precisely to be art. Read along the row, they are a sequence:
          a number, set into stone by someone who wanted it kept where it
          could not be edited out.
      - observer: "{actor} takes a careful charcoal rubbing of the old processional reliefs."

  # gallery: hearing Scholar Dross's "q69_gallery" topic, shown the rubbing.
  - event: topic_heard
    mob: 9360
    topic: q69_gallery
    conditions:
      has: ["69-rubbing"]
      missing: ["69-gallery"]
    actions:
      - grant: "69-gallery"

  # reading: hearing Keeper Lysha's "q69_lysha" topic, which gives her reading.
  - event: topic_heard
    mob: 9371
    topic: q69_lysha
    conditions:
      has: ["69-gallery"]
      missing: ["69-reading"]
    actions:
      - grant: "69-reading"

  # Dross keeps the rubbing with the reading, as the reward line says. A topic
  # takes one item, so this takes the other. A rubbing in a bag stays with the
  # player, which harms nothing.
  - event: quest_granted
    quest_token: "69-end"
    conditions:
      has_item: 40115
    actions:
      - consume_item: 40115
```

- [ ] **Step 3: Dross, `9360.yaml`.**

| Where | Old | New |
|---|---|---|
| `give_40116` `questRequired` | `- 69-gallery` | `- 69-reading` |
| `give_40116` | (add) | `carriesItem: 40115` below `requiresItem: 40116` |
| Variants `69-rubbing`, `69-start`, offer `q69_start` hints | `Type look reliefs.` | `Type search there.` |
| `q69_gallery` text | "They are not. Get past him to the keeper. Her name is Lysha." | "They are not. Do not let him steer you. Keep walking to the keeper. Her name is Lysha." |

A variant has one `carriesItem`, so the two papers are told apart with `itemExcluded`. Replace the first variant (`69-gallery`, `carriesItem: 40116`) with these two:

```yaml
  - questRequired:
    - 69-reading
    questExcluded:
    - 69-end
    carriesItem: 40116
    itemExcluded: 40115
    text: Her reading, but where is the rubbing? I need them side by side.
    hints: If you lost the rubbing, take another. The processional reliefs are in the
      Grand Temple Sanctuary, south twice from the Temple Courtyard. Type search there.
  - questRequired:
    - 69-reading
    questExcluded:
    - 69-end
    carriesItem: 40116
    text: Is that her reading? And the rubbing. Give them here. Carefully.
    hints: Choose The Gallery Cipher below to give Dross the reading and the rubbing.
```

`give_40116` text is unchanged.

- [ ] **Step 4: Lysha, `9371.yaml`.** Variant `69-gallery` no rubbing hints: old "Type look reliefs there." new "Type search there."

- [ ] **Step 5: Backlog.** Delete these 4 lines:

```
hint-length 69 step gallery
hint-length 69 step reading
hint-length 69 step rubbing
one-action 69 mob 9360 topic give_40116:prev
```

- [ ] **Step 6:** Run as Task 1 Step 7. **Commit:** `fix(quests): The Gallery Cipher by search, and Dross keeps both papers`.

### Task 8: Quest 70 by `search`, and Orin remembers

**Files:** `quests/70-the_pre_founding_web.yaml`, `loop-backlog.txt`, `dialogue/topic-keys.txt`, `dialogue/new_plymouth_crafting/9332.yaml`, `dialogue/new_plymouth_old_quarter/9381.yaml`, `items/materials-40000/40118-lintel_rubbing.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `"70-the_pre_founding_web.yaml": 70,` to the map. Append:

```go
func TestQuest70TheLintelIsFoundBySearch(t *testing.T) {
	const lintel, rubbing = 6037, 40118
	q := loadShippedQuest(t, "70-the_pre_founding_web.yaml", 70)
	require.NoError(t, q.Validate())
	require.Contains(t, q.Steps[0].Hint, "bring a light")
	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(lintel))
	require.Equal(t, "witness", w.p.current)
	require.Equal(t, []int{rubbing}, w.ctx.givenItems)
	lost := newQuestWalker(q).at("witness")
	lost.notify("command", tier5Search(lintel))
	require.Equal(t, []int{rubbing}, lost.ctx.givenItems)
}
```

```go
func TestTier5OrinRemembersTheCooperage(t *testing.T) {
	orin := loadShippedDialogue(t, "new_plymouth_crafting/9332.yaml")
	held := func() *PlayerState { return carrying(heldState("70-start", "70-witness"), 40118) }
	cold := map[string]string{"68-allegiance": "bloodline"}
	warm := map[string]string{"68-allegiance": "circle"}

	require.Contains(t, menuKeys(orin, held()), "orinhandoff", "a first timer gets the warm line")
	require.NotContains(t, menuKeys(orin, held()), "orinhandoff_cold")
	require.Contains(t, menuKeys(orin, withFlags(held(), warm)), "orinhandoff")
	bl := menuKeys(orin, withFlags(held(), cold))
	require.Contains(t, bl, "orinhandoff_cold")
	require.NotContains(t, bl, "orinhandoff")
	require.Empty(t, orin.FindTopic("orinhandoff_cold").BumpsRep, "no circle reputation")
	require.NotEmpty(t, orin.FindTopic("orinhandoff").BumpsRep)

	gritta := loadShippedDialogue(t, "new_plymouth_old_quarter/9381.yaml")
	text, _ := gritta.OpenerFor(heldState("69-end"))
	require.Contains(t, text, "temple wall")
	text, _ = gritta.OpenerFor(withFlags(heldState("68-end"), warm))
	require.Contains(t, text, "cooperage")
	root, _ := gritta.OpenerFor(heldState())
	bloody, _ := gritta.OpenerFor(withFlags(heldState("68-end"), cold))
	require.Equal(t, root, bloody, "no nod to the one who sold them out")
}
```

**Sabotage:** give `orinhandoff_cold` the `bumpsRep` (red). Remove `questFlagExcluded` from `orinhandoff` (red on `bl`). Reorder Gritta's variants so `68-end` loses its flag (red).

- [ ] **Step 2: Replace `quests/70-the_pre_founding_web.yaml` from `steps:` down.** Header unchanged.

```yaml
steps:
  - id: start
    destination:
      room: 6037  # the Buried Lintel
    description: >-
      Gritta asked you to go deeper, to see the Buried Lintel and the
      sealed Deep Canal with your own eyes, take a rubbing of the lintel
      symbol, and carry it up to Orin in the Crafting Quarter.
    hint: >-
      The Buried Lintel lies west twice from Gritta's Flooded Cellar, past the
      old stonework. It is black dark down there, so bring a light. Mind the
      canal to the north. Type search there.
  - id: witness
    destination:
      mob: 9332  # follow Orin the Bookseller (schedule np_crafting_orin)
      room: 5706  # fallback when Orin the Bookseller is not spawned
    description: >-
      You stood beneath the Buried Lintel and took a rubbing of the
      eight-pointed symbol, carved before the colony arrived.
    hint: >-
      Carry the lintel rubbing up to Orin the bookseller, in his stall off the
      Inkwalk in the Crafting Quarter. Type talk orin.
  - id: end
    description: >-
      Gritta's lintel is on record now, carved before the colony arrived. The
      colony found this and wrote itself into the record as first. The stone
      simply goes on being older.

rewards:
  gold: 70
  rep_faction: np_commonfolk
  rep_amount: 10
  actor: >-
    The rubbing is out of your hands now. Whatever the city teaches about its
    founding, someone up here has seen what is carved beneath it, and the stone
    simply goes on being older.
  observer: >-
    The old bookseller files a sheet of charcoal rubbing away and says nothing
    more about it.

triggers:
  # Lost rubbing: the lintel has not gone anywhere. First, so one search gives one.
  - event: command
    command: search
    room: 6037
    conditions:
      has: ["70-witness"]
      missing: ["70-end"]
      missing_item: 40118
    actions:
      - give_item: 40118
      - actor: >-
          You press a fresh sheet to the underside of the lintel and bring the
          rings and the eight points up in charcoal again.
      - observer: "{actor} takes a careful rubbing of the symbol carved beneath the buried lintel."

  # WITNESS: a search beneath the lintel, with enough light to see it.
  - event: command
    command: search
    room: 6037
    conditions:
      has: ["70-start"]
      missing: ["70-witness"]
    actions:
      - give_item: 40118
      - grant: "70-witness"
      - actor: >-
          You press paper to the underside of the lintel and bring the
          symbol up in charcoal: four concentric rings, eight points at
          equal intervals, cut by someone who drew it before they carved
          it. The colony arrived here, found this, and wrote itself into
          the founding record as first. The rubbing in your hands says
          otherwise, quietly, the way the stone does.
      - observer: "{actor} takes a careful rubbing of the symbol carved beneath the buried lintel."
```

- [ ] **Step 3: Item.** `40118-lintel_rubbing.yaml` `description`:

```yaml
description: >-
  A large rubbing you took from the underside of the Buried Lintel: four
  concentric rings and eight points at equal intervals, carved with a precision
  that required prior drawing, set at head height for people meant to look up
  and read it as they passed beneath.
```

- [ ] **Step 4: Orin, `9332.yaml`.** `orinhandoff`: `questRequired` `70-start` becomes `70-witness`; add `questFlagExcluded: {68-allegiance: bloodline}` in the file's block form. Add after it:

```yaml
- key: orinhandoff_cold
  kind: handin
  quest: 70
  label: The Pre-Founding Web
  playerLine: Gritta sent me up with a rubbing of the Buried Lintel.
  text: Leave it on the counter. I will see it reaches people who can read it, and
    you will not be one of them. That is all we have to say to each other.
  questRequired:
  - 70-witness
  questExcluded:
  - 70-end
  questFlagRequired:
    68-allegiance: bloodline
  requiresItem: 40118
  grantsQuest: 70-end
```

Replace the first opener variant with:

```yaml
  - questRequired:
    - 70-start
    questExcluded:
    - 70-end
    carriesItem: 40118
    questFlagRequired:
      68-allegiance: bloodline
    text: Whatever that is on your fingers, put it on the counter and go.
    hints: Choose The Pre-Founding Web below to give Orin the lintel rubbing.
  - questRequired:
    - 70-start
    - 69-end
    questExcluded:
    - 70-end
    carriesItem: 40118
    text: That is charcoal on your fingers. You have been counting the temple's hands
      as well, I hear. What have you brought me up from below?
    hints: Choose The Pre-Founding Web below to give Orin the lintel rubbing.
  - questRequired:
    - 70-start
    questExcluded:
    - 70-end
    carriesItem: 40118
    text: That is charcoal on your fingers. What have you brought me up from below?
    hints: Choose The Pre-Founding Web below to give Orin the lintel rubbing.
```

Add `9332.orinhandoff_cold` to `topic-keys.txt` beside `9332.orinhandoff`.

- [ ] **Step 5: Gritta, `9381.yaml`.** Hints `Type look symbol there.` and `Type look symbol.` (variants `70-witness` lost, `70-start`; offer `q70_start`) become `Type search there.` Add to `q70_start` hints: "Bring a light." Add at the end of `variants:`, after `70-start`:

```yaml
  # A quiet nod, for a player who has come at this from another side.
  - questRequired:
    - 69-end
    questExcluded:
    - 70-start
    text: You have the look of someone who has counted hands on a temple wall. Then
      you know the mark I mean. I work the flooded chambers below, and I have found
      where it comes from.
    hints: Choose a line below to learn about the gray material in the deep stone,
      about the old symbol on the buried lintels, or about who receives her finds.
  - questRequired:
    - 68-end
    questExcluded:
    - 70-start
    questFlagRequired:
      68-allegiance: circle
    text: Orin says a stranger stood by the cooperage when it counted. I do not ask
      who. I work the flooded chambers below, and what the silt has covered, I find.
    hints: Choose a line below to learn about the gray material in the deep stone,
      about the old symbol on the buried lintels, or about who receives her finds.
```

- [ ] **Step 6: Backlog.** Delete these 4 lines:

```
hint-length 70 step start
hint-length 70 step witness
one-action 70 mob 9332 topic orinhandoff:prev
one-action 70 quest 70 trigger room_interact room=6037 noun=symbol grants=70-witness:cond
```

- [ ] **Step 7:** Run as Task 1 Step 7. `TestEveryQuestSearchRoomIsSeenByATorch` now covers 6037 and must pass. **Commit:** `fix(quests): The Pre-Founding Web by search, and Orin remembers the cooperage`.

### Task 9: Quest 71 by `search`

**Files:** `quests/71-the_tribute.yaml`, `loop-backlog.txt`, `dialogue/new_plymouth_merchant/9347.yaml`, `9344.yaml`. Tests: the two Task 1 files.

- [ ] **Step 1: Failing tests.** Add `"71-the_tribute.yaml": 71,` to the map. Append:

```go
func TestQuest71ThePageIsFoundBySearch(t *testing.T) {
	const parlor, page = 5815, 40119
	q := loadShippedQuest(t, "71-the_tribute.yaml", 71)
	require.NoError(t, q.Validate())
	require.Equal(t, "np_commonfolk", q.Rewards.RepFaction)
	w := newQuestWalker(q).at("horst")
	w.notify("command", tier5Search(parlor))
	require.Equal(t, "ledger", w.p.current)
	require.Equal(t, []int{page}, w.ctx.givenItems)
	lost := newQuestWalker(q).at("ledger")
	lost.notify("command", tier5Search(parlor))
	require.Equal(t, []int{page}, lost.ctx.givenItems)
}
```

```go
func TestTier5OstryPaysNoCircleRep(t *testing.T) {
	o := loadShippedDialogue(t, "new_plymouth_merchant/9347.yaml")
	in := o.FindTopic("ledgerhandoff")
	require.Empty(t, in.BumpsRep, "the 69-end trigger keeps its bonus, the hand-in pays none")
	require.Equal(t, []string{"71-ledger"}, in.QuestRequired)
}
```

**Sabotage:** restore `bumpsRep` (red). Set `rep_faction: np_dockfolk` (red).

- [ ] **Step 2: `quests/71-the_tribute.yaml`.** `rewards.rep_faction: np_commonfolk`. Hints:

| Step | New hint |
|---|---|
| `start` | Clerk Vell keeps the Permit Office, north of Coin Alley from Dame Ostry's Armoury. Type talk vell. |
| `vell` | The tribute runs upward, past the clerk, to Horst at the Watched House Gate, north of the Watched Lane. Type talk horst. |
| `horst` | The papers are in the private parlor of the Gilt Threshold, through the curtain to the east of its taproom. Type search there. |
| `ledger` | Take the ledger page back to Dame Ostry in her Armoury, south of Coin Alley. Type talk ostry. |

Remove the long dash from each comment. Replace the three `room_interact` triggers. Keep `topic_heard` for `vell`, `horst`, and the `quest_granted` bonus as they are:

```yaml
  # Lost ledger page: another of the same column has worked loose. First, so one
  # search gives one page.
  - event: command
    command: search
    room: 5815
    conditions:
      has: ["71-ledger"]
      missing: ["71-end"]
      missing_item: 40119
    actions:
      - give_item: 40119
      - actor: >-
          The ledger-board is still on the table among the cup rings, and
          another page of the same column has worked loose beneath it. You
          take it: the same quarterly sums, routed up to the same unnamed
          account.
      - observer: "{actor} palms a page from the table among the cup rings."

  # LEDGER: a search of the parlor finds what the torn scrap came from.
  - event: command
    command: search
    room: 5815
    conditions:
      has: ["71-horst"]
      missing: ["71-ledger"]
    actions:
      - give_item: 40119
      - grant: "71-ledger"
      - actor: >-
          The torn scrap on the floor reads "before the permit clears",
          and the page it tore from is still on the table among the
          cup rings, slid half under a ledger-board. You take it: a column
          of quarterly sums against vendor names, every figure routed up to
          one unnamed account. It does not prove a crime. It proves exactly
          where the money goes, and who waits on whom.
      - observer: "{actor} palms a page from the table among the cup rings."
```

- [ ] **Step 3: Ostry, `9347.yaml`.** `ledgerhandoff`: `questRequired` `71-horst` becomes `71-ledger`; delete its `bumpsRep` block. Variants `71-ledger`, `71-horst` hints: `Type look paper scrap.` becomes `Type search there.` Variant `71-horst` carrying: `questRequired` becomes `71-ledger`.

- [ ] **Step 4: Horst, `9344.yaml`.** `q71_horst` hints: old "Type look paper scrap there." new "Type search there."

- [ ] **Step 5: Backlog.** Delete these 5 lines:

```
hint-length 71 step horst
hint-length 71 step ledger
hint-length 71 step start
hint-length 71 step vell
one-action 71 mob 9347 topic ledgerhandoff:prev
```

- [ ] **Step 6:** Run as Task 1 Step 7. `grep -rn "Type look" _datafiles/world/dogmud/dialogue/new_plymouth_* _datafiles/world/dogmud/quests/6[3-9]-* _datafiles/world/dogmud/quests/7[01]-*` should find nothing (exit 1 is the pass). **Commit:** `fix(quests): The Tribute by search, on the commonfolk's reputation`.

### Task 10: 5b guard, docs, gates, PR

- [ ] **Step 1:** In `tier5_backlog_guard_test.go`, `tier5Quests` adds `68: true, 69: true, 70: true, 71: true`.
- [ ] **Step 2: Patch note:**

```markdown
## 2026-11-05: Older than the city

- The Cooperage Circle, The Gallery Cipher, The Pre-Founding Web and The Tribute are
  now followed by searching: where the trail leads you, type search. The Buried
  Lintel is black dark, so bring a light.
- Scholar Dross keeps your rubbing as well as Lysha's reading when you bring them
  back.
- Orin remembers how the cooperage ended, and says so when you bring him the lintel
  rubbing.
- The Tribute now earns standing with the common folk.
```

- [ ] **Step 3: Gates.** From the worktree root:

```bash
gofmt -l internal/ modules/                                   # prints nothing
go build ./...                                                # no output
go test . ./internal/guards/ ./internal/questengine/ ./internal/dialogue/ ./internal/quests/ ./internal/questloop/ ./internal/narration/ -count=1   # all ok
go test ./... -count=1                                        # all ok
DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1         # ok
~/go/bin/golangci-lint run --new-from-merge-base=origin/master # 0 issues
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones new_plymouth_crafting,new_plymouth_merchant,new_plymouth_noble,new_plymouth_old_quarter,new_plymouth_temple; echo $?   # 0
grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .   # 12
grep -nP '\x{2014}|\x{2013}| -- ' _datafiles/world/dogmud/quests/6[89]-*.yaml _datafiles/world/dogmud/quests/7[01]-*.yaml   # nothing (exit 1 is the pass)
```

Isolated boot, as Task 5 Step 4's second block.

- [ ] **Step 4: Commit,** push, `gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-tier5b`. Wait for CI, merge with `--merge`, remove the worktree.

---

# PR 5c: the Bloom chain

### Task 11: Bloom's knobs move into `config.yaml`, no change in behaviour

**Files:** `_datafiles/config.yaml`; create `internal/configs/config.balance_bloom_test.go`.

- [ ] **Step 1: Failing test.**

```go
package configs

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// The shipped config names every Bloom knob, at its Go default, so the live
// value is visible in config.yaml rather than silently the default. Moving them
// changed no behaviour: this compares each shipped value to validateCombat's.
func TestBloomKnobsShipAtTheirDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	shipped := cfg.Balance
	if shipped.BloomMutationAdvanceChance == 0 {
		t.Fatal("BloomMutationAdvanceChance is absent from config.yaml")
	}
	def := &Balance{}
	def.validateCombat()
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"BloomAddictionPerDose", float64(shipped.BloomAddictionPerDose), float64(def.BloomAddictionPerDose)},
		{"BloomAddictionDecayRounds", float64(shipped.BloomAddictionDecayRounds), float64(def.BloomAddictionDecayRounds)},
		{"BloomWithdrawalOnsetRounds", float64(shipped.BloomWithdrawalOnsetRounds), float64(def.BloomWithdrawalOnsetRounds)},
		{"BloomMutationAdvanceChance", float64(shipped.BloomMutationAdvanceChance), float64(def.BloomMutationAdvanceChance)},
		{"BloomNewMutationChance", float64(shipped.BloomNewMutationChance), float64(def.BloomNewMutationChance)},
		{"BloomCommunionRounds", float64(shipped.BloomCommunionRounds), float64(def.BloomCommunionRounds)},
		{"BloomCrashRoundsMult", float64(shipped.BloomCrashRoundsMult), float64(def.BloomCrashRoundsMult)},
	} {
		if c.got != c.want {
			t.Errorf("shipped %s = %v, default %v", c.name, c.got, c.want)
		}
	}
}
```

The Bloom block is inside `validateCombat` (`config.balance.combat.go:107`, block at `:582`). **Sabotage:** ship `BloomCommunionRounds: 31` (red), then delete the whole block (red on "absent").

- [ ] **Step 2: Config.** Insert after `ToxicityHighDecaySlowMult: 0.5` (`config.yaml:2336`):

```yaml
  # Bloom Wafer (item 40108): the drug the Bloom Trade sells. A dose gives Communion
  # (a high), then a crash, adds addiction and may change a mutation. Moved here
  # from Go defaults at their same values (#375 tier 5). Rounds are 4 seconds.
  # Every knob here treats 0 as absent and restores its default, so to slow the
  # mutation roll use a small number (0.001), never 0.
  BloomAddictionPerDose: 1          # addiction gained per dose
  BloomAddictionDecayRounds: 300    # rounds of abstinence per point of addiction lost
  BloomWithdrawalOnsetRounds: 60    # rounds after the last dose before withdrawal
  BloomMutationAdvanceChance: 0.50  # chance a dose deepens the strongest mutation
  BloomNewMutationChance: 0.10      # chance, rolled first, of a whole new mutation
  BloomCommunionRounds: 30          # how long the high lasts
  BloomCrashRoundsMult: 2.5         # crash length as a multiple of condition 91's baseline
```

- [ ] **Step 3:** `go test ./internal/configs/ . ./internal/guards/ -count=1`. **Commit:** `chore(config): Bloom's knobs ship in config.yaml at their current values`.

### Task 11b: A wafer cannot grant a mutation past `MutationMaxCount` (owner, 2026-11-04)

**Files:** `internal/characters/bloom_mutation.go`. Test: `internal/characters/bloom_mutation_cap_test.go`.

A wafer can still deepen a mutation the player already has, as ranks stay limited by each mutation's own cap. It
cannot add a new one once the character holds `Balance.MutationMaxCount`, the same count the combat path reads
(`internal/actions/mutation_earn.go:29`, `len(c.Mutations) < MutationMaxCount`). Read the file's existing tests
for how it seeds mutations and config, and follow them.

- [ ] **Step 1: Failing test.** Seed at least one gainable mutation the character does not own. Set
  `MutationMaxCount` to 2 with `configs.SetConfigForTest`. A character holding 2 other mutations must get `("", 0)`
  from `BloomSeedNewMutation`. A character holding 1 must get a mutation. Run it and confirm the first case fails.
- [ ] **Step 2: Implement,** as the first lines of `BloomSeedNewMutation`:

```go
	// The count every other way of gaining a mutation obeys (owner, 2026-11-04).
	if len(c.Mutations) >= int(configs.GetBalanceConfig().MutationMaxCount) {
		return "", 0
	}
```

- [ ] **Step 3:** Run `go test ./internal/characters/ ./internal/actions/ -count=1`. Expected: ok.
- [ ] **Step 4: Commit** `internal/characters/bloom_mutation.go` and the test, message
  `fix(bloom): a wafer stops at the mutation count`.

### Task 12: The rooms, the NPCs, the wafer

**Files:** create `rooms/new_plymouth_docks/5530.yaml`, `rooms/new_plymouth_common/5625.yaml`, `mobs/new_plymouth_docks/9802-sefton.yaml`, `mobs/new_plymouth_common/9801-odile.yaml`, `internal/guards/tier5_bloom_shop_test.go`. Edit `rooms/new_plymouth_docks/5523.yaml`, `5524.yaml`, `rooms/new_plymouth_common/5614.yaml`, `items/materials-40000/40108-bloom_wafer.yaml`, `internal/guards/testdata/lighting_daycycle.golden`.

Run `python tools/id_inventory.py --type rooms` and `--alloc mobs 2` first. Report if 5530, 5625, 9801 or 9802 has gone.

- [ ] **Step 1: Failing guard.**

```go
package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"gopkg.in/yaml.v2"
)

const bloomWaferId = 40108

// The dealer's shelf is capped on purpose. Malk's torn page (tier 40) rose from
// 19 to 80 in an hour through EffectiveMaxStock, so the wafer's cap is set here,
// and it is the only tier 10 item, so RestockCadenceTier10Days is its own clock.
func TestBloomWaferShelfIsCapped(t *testing.T) {
	setupSearchLightWorld(t)
	cfg := configs.GetConfig()
	cfg.Balance.ShopMaxStockMultiplier = 2.0 // config.yaml:1600
	configs.SetConfigForTest(t, cfg)

	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "mobs", "new_plymouth_docks", "9802-sefton.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var mob struct {
		StockMultiplier float64 `yaml:"stock_multiplier"`
	}
	if err := yaml.Unmarshal(data, &mob); err != nil {
		t.Fatal(err)
	}
	if got := shops.EffectiveMaxStock(bloomWaferId, mob.StockMultiplier); got != 5 {
		t.Errorf("the wafer's shelf cap is %d, want 5", got)
	}

	tier10 := regexp.MustCompile(`(?m)^rarity_tier:\s*10\s*$`)
	found := map[string]bool{}
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "items")
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && tier10.Match(b) {
			found[filepath.Base(p)] = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || !found["40108-bloom_wafer.yaml"] {
		t.Errorf("tier 10 items %v: RestockCadenceTier10Days must stay the wafer's own clock", found)
	}
}

// What stops a resale is having no vendor_categories, not not_salable, which
// only skips a validator. It holds at the dealer who stocks it.
func TestBloomWaferCannotBeSoldBack(t *testing.T) {
	setupSearchLightWorld(t)
	spec := items.GetItemSpec(bloomWaferId)
	if spec == nil || len(spec.VendorCategories) != 0 || !spec.NotSalable {
		t.Fatalf("wafer spec %+v", spec)
	}
	inv := &shops.ShopInventory{Gold: 5000, StartingGold: 5000, CraftSupport: shops.CraftSupportAlchemy,
		Stock: []shops.StockEntry{{ItemId: bloomWaferId, RestockQty: 5, MaxStock: 5, Current: 1}}}
	offer := shops.EvaluateBuyRules(items.New(bloomWaferId), inv, "", false, shops.PricingConfigFromBalance(), nil)
	if offer.Price != 0 || offer.Refused != shops.BuyRefusedNoTrade {
		t.Errorf("offer %+v, want refused with no trade", offer)
	}
}
```

**Sabotage:** `stock_multiplier: 1.0` (cap 20, red). `rarity_tier: 40` on the wafer (red both ways). Add `vendor_categories: [alchemy]` to the wafer (`CannotBeSoldBack` red). Add `rarity_tier: 10` to another item (red).

- [ ] **Step 2: The wafer.** Replace `items/materials-40000/40108-bloom_wafer.yaml`:

```yaml
itemid: 40108
name: Bloom Wafer
namesimple: wafer
description: A thin disc of pressed botanical matter, iridescent at the edges
  and nearly odorless, the kind of thing you could mistake for a breath mint
  if you didn't know better. It sits light in the palm. It changes hands quickly
  in dim corners and is never sold from a proper stall. To take it, type quaff
  wafer. Nobody who sells it will tell you what that costs later.
type: potion
subtype: drinkable
uses: 1
not_salable: true
toxicity: 20
weight: 0.1
value: 120
rarity_tier: 10
```

- [ ] **Step 3: The door.** `rooms/new_plymouth_docks/5523.yaml`:

| Where | Old | New |
|---|---|---|
| description | "At the back wall, past the shelving, a `<ansi ...>cupboard door</ansi>` stands half in shadow." | "At the back wall, east past the shelving, a `<ansi ...>cupboard door</ansi>` stands half in shadow." |
| description | "South returns to the outer lane; a stair leads down behind the cupboard door." | "South returns to the outer lane; a stair goes down behind the last shelf to the stock room." |
| `exits` | (add) | `east:` / `  roomid: 5530` |
| noun `cupboard door`, end | "by more people than a bolt-count would warrant." | "by more people than a bolt-count would warrant. It is barred from inside. It takes a knock, and you do not know it." |

Keep the `<ansi fg="itemname">` tag exactly as it is.

`5524.yaml` description, old first line: "The cupboard door swings inward on a space that is smaller than expected:" new: "The stair from the shop comes down into a space that is smaller than expected:".

Create `rooms/new_plymouth_docks/5530.yaml`:

```yaml
roomid: 5530
zone: New Plymouth Docks
title: Behind the Cupboard
description: >
  Behind the cupboard door the building is a different building. The room is
  narrow and warm and smells of wax and copper-flower, and there is no window in
  it. A plain <ansi fg="itemname">table</ansi> stands against the far wall under
  a shaded lamp, squared to the wall, with a chair on each side. Along the wall
  are <ansi fg="itemname">shallow shelves</ansi>, kept nearly empty. Nothing here
  is for show. The only way out is west, back through the cupboard door to the
  shop.
biome: interior
exits:
  west:
    roomid: 5523
nouns:
  table: A plain deal table, clean, with a small ledger closed on it and a
    square of waxed cloth folded to one side. The chair facing the door has worn
    its feet into the floor.
  shallow shelves: Shelves one hand deep, built to hold very little. A few
    wax-wrapped squares sit on the lowest, spaced apart so that a glance tells
    how many are left.
idlemessages:
- Someone in the shop beyond the door moves a bolt of cloth and sets it down.
- The shaded lamp does not flicker. Someone trims it on a schedule.
- The copper-flower smell comes and goes, sweet and slightly wrong at the back of
  the throat.
- A voice comes through the wall from the stock room side, low, and stops.
spawninfo:
  - mobid: 9802
    respawnrate: "10 real minutes"
x: -7
y: -81
```

- [ ] **Step 4: The stair.** `rooms/new_plymouth_common/5614.yaml`: description, old end "Tenants feel looked-after. Visitors feel assessed." new "Tenants feel looked-after. Visitors feel assessed. Inside the door a <ansi fg="itemname">stair</ansi> climbs past the first landing toward the condemned floors, a chair stood across its foot." Add noun and exit:

```yaml
exits:
  up:
    roomid: 5625
nouns:
  stair: A narrow stair running up past the first landing, its treads worn at the
    edges and clean at the middle, which is the wrong way round for a stair nobody
    uses. A kitchen chair stands across the foot of it. The chair is not heavy.
```

(merge into the existing `exits:` and `nouns:`). Create `rooms/new_plymouth_common/5625.yaml`:

```yaml
roomid: 5625
zone: New Plymouth Common
title: The Condemned Floor
description: >
  The upper floor has been condemned for eleven years, and the writ is still
  pinned to the first door you pass. The joists are sound. The rooms have been
  stripped of anything worth carrying, and the plaster has come down in sheets
  along one wall. At the far end someone has made a <ansi fg="itemname">bed</ansi>
  of sacking behind a door taken off its hinges and leaned across the corner. It
  is a place to sleep with your back to a wall and your boots on. The stair goes
  back down.
biome: interior
exits:
  down:
    roomid: 5614
nouns:
  bed: Sacking over old floorboards, folded with care, and a canvas bag beside it
    packed and tied. Whoever sleeps here can leave in the time it takes to stand
    up.
idlemessages:
- The building settles, which is what old buildings do.
- A voice on the first landing says something short, and nobody answers it.
- Dust comes down from the ceiling in a thin line, and stops.
spawninfo:
  - mobid: 9801
    respawnrate: "10 real minutes"
x: -5
y: -80
z: 1
```

- [ ] **Step 5: The NPCs.** A shop mob must carry `craft_support`, so Sefton is `alchemy` and buys alchemy goods like any alchemy vendor; the group `fence` would make him buy stolen baubles, which nobody asked for. `mobs/new_plymouth_docks/9802-sefton.yaml`:

```yaml
mobid: 9802
zone: New Plymouth Docks
behavior_archetype: noncombat_shopkeeper
non_combatant: true
charm_immune: true
hostile: false
statpool: 38
maxwander: 0
craft_support: alchemy
stock_multiplier: 0.25
groups:
  - humanoid
  - bloom_trade
idlecommands:
  - 'emote turns a wax-wrapped square a quarter turn on the table, and leaves it there.'
  - ''
  - 'emote opens the small ledger, writes one figure, and closes it.'
  - ''
  - 'emote looks at the door, then at you, and does not ask you anything.'
activitylevel: 10
character:
  name: Sefton
  description: |
    He is a tidy man at a tidy table. His sleeves are clean to the wrist and he
    keeps his hands flat on the wood when they have nothing to do. He looks at
    you the way a clerk looks at a figure he has already added up. He does not
    offer you the other chair.
  speciesid: 1
  level: 1
  gold: 500
  stats:
    charisma:
      base: 108
    perception:
      base: 110
    willpower:
      base: 104
  shop:
    - itemid: 40108
```

`mobs/new_plymouth_common/9801-odile.yaml`:

```yaml
mobid: 9801
zone: New Plymouth Common
behavior_archetype: noncombat_questgiver
non_combatant: true
charm_immune: true
hostile: false
statpool: 30
maxwander: 0
groups:
  - humanoid
idlecommands:
  - 'emote listens to the stair for a moment, and goes back to sitting very still.'
  - ''
  - 'emote checks the knot on her canvas bag, which is already tied.'
activitylevel: 10
character:
  name: Odile
  description: |
    A young woman in a servant's plain dress with the household mark cut off the
    collar. She sits with her back to the wall and her boots on. In a city where
    the Bloom touches nearly everyone who stays long enough, there is no change
    on her at all: skin plain, hands unmarked, and nothing about her you would
    look at twice. She looks like she has noticed you noticing.
  speciesid: 1
  level: 1
  gold: 0
  stats:
    perception:
      base: 112
    dexterity:
      base: 110
```

- [ ] **Step 6: Light golden.** Predict: exactly 24 new lines, 12 samples for each of 5530 and 5625, `biome=interior`, values as 5524's at the same sample. `go test ./internal/guards/ -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -count=1`, then `git diff --stat` on the golden: `24 insertions`, 0 deletions, or stop and report.

- [ ] **Step 7:** `go test . ./internal/guards/ ./internal/items/ ./internal/mobs/ ./internal/shops/ ./internal/rooms/ -count=1`, the sweep, narration update. A route golden move in `TestShippedRoutesSurviveZoneBoundedMaps` is unexpected: stop and report. **Commit:** `feat(world): the room behind Marn's cupboard, the condemned floor, Sefton and Odile`.

### Task 13: A resume asks for no gold

**Files:** `internal/dialogue/topics.go`, `internal/dialogue/resume_test.go`.

- [ ] **Step 1: Failing test,** appended:

```go
// C6: a resume charges nothing, so it asks for no gold. A player who paid, set
// the quest aside and spent their gold can still take it back up. A first
// accept still needs the price in hand.
func TestEligible_ResumeAsksForNoGold(t *testing.T) {
	df := &DialogueFile{MobId: 1}
	paid := &Topic{Key: "o", Kind: KindOffer, Quest: 10, Effects: Effects{GrantsQuest: "10-start", ChargesGold: 5},
		Gates: Gates{GoldRequired: 5}}
	state := func(shelved bool) *PlayerState {
		return &PlayerState{
			QuestOpen: func(int) bool { return true },
			HasQuest:  func(string) bool { return false },
			HasGold:   func(int) bool { return false },
			Shelved:   func(int) bool { return shelved },
		}
	}
	require.False(t, df.Eligible(paid, state(false)), "a first accept needs the price")
	require.True(t, df.Eligible(paid, state(true)), "a resume does not")
}
```

**Sabotage:** remove the `resuming` branch below (red).

- [ ] **Step 2: The fix.** In `Eligible` (`topics.go:91`):

```go
func (df *DialogueFile) Eligible(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil {
		return false
	}
	gates := t.Gates
	if resuming(t, ps) {
		// A resume charges nothing (Apply), so it asks for no gold either.
		gates.GoldRequired = 0
	}
	if !gates.pass(ps) {
		return false
	}
	if ps == nil {
		return true
	}
```

keeping the rest. In `internal/dialogue/context.md`, in its notes on `Eligible` or resume, add one line: "A resume skips `goldRequired`, as `Apply` skips the charge." Read the file first and put it where it fits.

- [ ] **Step 3:** `go test ./internal/dialogue/ ./internal/conversation/ . ./internal/guards/ -count=1`. **Commit:** `fix(dialogue): a resume charges nothing, so it asks for no gold`.

### Task 14: Quest 64, The Skip

**Files:** create `quests/64-the_skip.yaml`, `dialogue/new_plymouth_common/9801.yaml`, `internal/questengine/tier5_bloom_chain_test.go`, `internal/dialogue/tier5_bloom_chain_test.go`. Edit `quests/step-ids.txt`, `dialogue/topic-keys.txt`, `dialogue/new_plymouth_merchant/9344.yaml`, `dialogue/new_plymouth_docks/9305.yaml`, `dialogue/new_plymouth_common/9325.yaml`.

Mechanism, chosen against the source: **Offer** on Horst, `questFlagRequired {67-entry: undercover}`, no breadcrumb, because a breadcrumb would speak to every player. **Two endings:** two `handin` topics on two mobs, as 68 has (`9332.circle_choice`, `9349.vellhandoff`). Pay on the topic, because `rewards` pay every ending (C4). **Destination:** bare `zone`, because `mob` would point straight at her in a lit room (skill: "Seen means seen where it stands"). **Gate:** two `room_gate` triggers, before and after the quest; the shelf reads as not held, so abandon closes it too, and a player inside can always walk out, because the gate guards the destination only. **Teleports** are exempt (owner ruling 2026-10-26). A player who fold-anchored there meets her closed opener, and that is the only gap. **Abandon** with `67-entry` set: the flag stays on the shelf, Horst's offer still shows, and that player chose the shady line. A player who never took 66 cannot reach `67-start` (Teels's offer needs `66-end`) and never holds the flag.

- [ ] **Step 1: Failing tests.**

```go
package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuest64TheSkip(t *testing.T) {
	const odile, floor = 9801, 5625
	q := loadShippedQuest(t, "64-the_skip.yaml", 64)
	require.Equal(t, []string{"start", "found", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	open := func(step string) bool {
		w := newQuestWalker(q).at(step)
		return !w.e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: floor}, w.p)
	}
	require.False(t, open(""), "no quest, no stair")
	require.True(t, open("start"))
	require.True(t, open("found"))
	require.False(t, open("end"), "her hiding place closes with her story")

	w := newQuestWalker(q).at("start")
	w.notify("topic_heard", EventDetails{MobId: odile, Topic: "who"})
	require.Equal(t, "start", w.p.current)
	w.notify("topic_heard", EventDetails{MobId: odile, Topic: "q64_found"})
	require.Equal(t, "found", w.p.current)

	require.NotZero(t, q.Steps[0].Destination)
	require.Empty(t, q.Rewards.Gold, "each ending pays on its own hand-in")
}
```

```go
package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTier5HorstHiresTheUndercoverOnly(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	require.NotContains(t, menuKeys(h, heldState()), "q64_offer")
	require.NotContains(t, menuKeys(h, withFlags(heldState("67-start"), map[string]string{"67-entry": "evidence"})), "q64_offer")
	require.Contains(t, menuKeys(h, withFlags(heldState("67-start"), map[string]string{"67-entry": "undercover"})), "q64_offer")
	require.Empty(t, h.Breadcrumbs(heldState()), "an honest player hears nothing of this chain")
	shelved := withFlags(heldState(), map[string]string{"67-entry": "undercover"})
	require.Contains(t, menuKeys(h, shelved), "q64_offer", "a player who shelved 67 keeps the shady line")
}

func TestTier5TheSkipHasTwoEndings(t *testing.T) {
	h := loadShippedDialogue(t, "new_plymouth_merchant/9344.yaml")
	o := loadShippedDialogue(t, "new_plymouth_common/9801.yaml")
	report, run := h.FindTopic("q64_report"), o.FindTopic("q64_run")
	for _, tp := range []*Topic{report, run} {
		require.Equal(t, KindHandIn, tp.Kind)
		require.Equal(t, "64-end", tp.GrantsQuest)
		require.Equal(t, []string{"64-found"}, tp.QuestRequired)
	}
	require.Equal(t, "reported", report.SetsQuestFlag.Value)
	require.Equal(t, 150, report.GivesGold)
	require.Equal(t, 40108, report.GivesItem)
	require.Equal(t, "ran", run.SetsQuestFlag.Value)
	require.Zero(t, run.GivesGold)
	require.Zero(t, run.GivesItem)

	found := heldState("64-start", "64-found")
	require.NotContains(t, menuKeys(h, found), "q64_report", "hear the terms first")
	require.NotContains(t, menuKeys(o, found), "q64_run", "hear her first")
	heard := heldState("64-start", "64-found")
	heard.HasHeard = func(string) bool { return true }
	require.Contains(t, menuKeys(h, heard), "q64_report")
	require.Contains(t, menuKeys(o, heard), "q64_run")
	ended := heldState("64-start", "64-found", "64-end")
	ended.HasHeard = func(string) bool { return true }
	require.NotContains(t, menuKeys(o, ended), "q64_run")
}

func TestTier5MarnLetsItSlipOnlyToACustomer(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	require.Contains(t, m.FindTopic("bloom_undercover").Hints, "Horst")
	require.NotContains(t, m.FindTopic("bloom_evidence").Hints, "Horst")
}
```

**Sabotage:** drop the `has: ["64-end"]` gate (`open("end")` red). Add a breadcrumb to `q64_offer` (red). Put `givesGold` on `q64_run` (red). Set `gold:` in the quest's `rewards` (red). Drop `heardRequired` on `q64_run` (red).

- [ ] **Step 2: `quests/64-the_skip.yaml`.**

```yaml
questid: 64
name: The Skip
description: >-
  Horst, at the gate of the watched house, pays people who can find things. A
  thief has run from a noble household and gone to ground in the Common Quarter.
  He wants to know where she sleeps.
secret: false

flags:
  - key: outcome
    values: [reported, ran]
    description: "Whether the player told Horst where the runaway hides, or told her to run"

steps:
  - id: start
    destination:
      zone: New Plymouth Common
    description: >-
      Horst wants a runaway found: a young woman named Odile, a thief from a
      noble household, he says. She went to ground in the Common Quarter, among
      people who do not ask for papers.
    hint: >-
      She is somewhere in the Common Quarter. Mother Brisk keeps tenants she will
      not name, at her block off Tenement Row. Type talk brisk.
  - id: found
    destination:
      mob: 9344  # follow Horst (schedule np_merchant_horst)
      room: 5820  # fallback when Horst is not spawned
    description: >-
      You found Odile on the condemned floor above Mother Brisk's block. She is
      not what Horst said she was. You can tell her to run, or you can tell Horst
      where she is.
    hint: >-
      Tell her to run, and you are paid nothing. Tell Horst, at the Watched House
      Gate, and he pays. Neither can be undone. Type talk odile or talk horst.
  - id: end
    description: >-
      The matter of the runaway is settled, one way or the other.

rewards:
  actor: ""
  observer: ""

triggers:
  # The stair above Mother Brisk's block opens only while this quest runs. Sight
  # neutral, and it names no one, so it reads the same to anyone who tries it.
  - event: room_gate
    room: 5625
    conditions:
      missing: ["64-start"]
    actions:
      - actor: >-
          A gray-haired woman on the first landing looks at you and does not move
          out of the way. The floors above are condemned, she says, and the chair
          across the stair stays where it is.
  - event: room_gate
    room: 5625
    conditions:
      has: ["64-end"]
    actions:
      - actor: >-
          A gray-haired woman on the first landing looks at you and does not move
          out of the way. The floors above are condemned, she says, and the chair
          across the stair stays where it is.

  # found: hearing Odile's "q64_found" topic.
  - event: topic_heard
    mob: 9801
    topic: q64_found
    conditions:
      has: ["64-start"]
      missing: ["64-found"]
    actions:
      - grant: "64-found"
```

- [ ] **Step 3: `dialogue/new_plymouth_common/9801.yaml`.**

```yaml
mobid: 9801
zone: New Plymouth Common
defaultMood: cautious
greetings:
- text: Quietly, if you have to say anything at all.
  moods:
  - cautious
opener:
  text: You came up past the chair, so she let you, or you moved it. Say what you
    came to say, and say it quietly.
  hints: A young woman sits against the wall with her boots on and her bag tied.
  variants:
  - questRequired:
    - 64-end
    questFlagRequired:
      64-outcome: ran
    text: Go on, then. I will be gone before the bell.
    hints: She does not look at you again.
  - questRequired:
    - 64-end
    text: Someone was on the stair this morning, asking which floor. That was you,
      wasn't it?
    hints: She does not move. There is nowhere for her to move to.
  - questRequired:
    - 64-found
    text: You know what I am now. Tell me to run, or go and tell him. I cannot stop
      you doing either.
    hints: Telling her to run ends this job unpaid, and that door stays shut. Telling
      Horst pays. Neither can be undone.
  - questRequired:
    - 64-start
    text: Did Horst send you? He sends people who are good at finding things. Are you
      going to listen, or only look?
    hints: Choose a line below.
topics:
- key: q64_found
  kind: info
  label: Why does Horst want you?
  text: I took nothing. Ask him what I took, and watch him not say. Look at me. No
    change on me anywhere, not a mark, in a house where every servant has something.
    That is what they want. There was a woman called Hartley before me, in the
    rooms under that house. Then one morning there was not, and they started looking
    at me the way they looked at her. Nobody comes back from those rooms, and
    somebody up here buys what comes out of them. So I ran.
  hints: Telling her to run ends this job with no pay and shuts that door for good.
    Telling Horst pays. Neither can be undone.
  marked: true
  questRequired:
  - 64-start
  questExcluded:
  - 64-end
  moodChange: cautious
- key: q64_run
  kind: handin
  quest: 64
  label: The Skip
  playerLine: Run. Tonight. I never found you.
  text: You know you get nothing for this. Then I was never here, and neither were
    you.
  hints: You told her to run. Horst pays you nothing, and he has no more work for you.
    It cannot be undone.
  heardRequired:
  - q64_found
  questRequired:
  - 64-found
  questExcluded:
  - 64-end
  grantsQuest: 64-end
  setsQuestFlag:
    key: 64-outcome
    value: ran
- key: who
  kind: info
  label: Who keeps you up here?
  text: Mother Brisk does not ask, and I do not tell her. She has a rule about the
    upper floors. I am the reason for the rule this season.
```

- [ ] **Step 4: Horst, `9344.yaml`.** Add under `opener:`:

```yaml
  variants:
  - questRequired:
    - 64-end
    questFlagRequired:
      64-outcome: ran
    text: The girl was gone when my people reached the floor. You were slow, or you
      were something else. I have no more work for you.
    hints: Horst has nothing more for you. He is polite about it.
  - questRequired:
    - 64-end
    text: She was where you said. The office is satisfied, and so am I.
    hints: Marn the draper, on the docks, has something to sell to someone who has done
      this kind of work. Type talk marn.
  - questRequired:
    - 64-found
    text: Well? Is she found?
    hints: Hear him out first. If you are sure, choose The Skip below. It cannot be
      undone.
  - questRequired:
    - 64-start
    text: She will not find herself. The Common Quarter, where no one asks for papers.
    hints: Mother Brisk keeps tenants she will not name, at her block off Tenement Row.
      Type talk brisk.
```

Add to `topics:`, first:

```yaml
- key: q64_offer
  kind: offer
  quest: 64
  label: The Skip
  playerLine: Marn says the supply end pays for finding things.
  text: Marn talks more than he should. Yes. A household I serve has lost a girl,
    Odile by name, and some things of theirs with her. A thief, then. She has gone to
    ground in the Common Quarter, among people who do not ask for papers. I do not
    need her brought in. I need to know where she sleeps. Find her, tell me, and I pay
    in gold and in kind.
  hints: Accept, then go to the Common Quarter and ask who is not on the books.
  questFlagRequired:
    67-entry: undercover
  questExcluded:
  - 64-start
  - 64-end
  grantsQuest: 64-start
  moodChange: cautious
- key: q64_terms
  kind: info
  label: What happens to her if I tell you?
  text: She goes back to the household that lost her. I do not decide what happens past
    the gate, and I would not be good at my work if I asked. You will be paid either
    way.
  hints: He did not answer the question. Telling him pays you, and it cannot be undone.
    If you are sure, choose The Skip below.
  marked: true
  questRequired:
  - 64-found
  questExcluded:
  - 64-end
- key: q64_report
  kind: handin
  quest: 64
  label: The Skip
  playerLine: She is on the condemned floor above Brisk's block.
  text: The condemned floor. Yes, that would do it. Here. Gold, and one of these, which
    some people would tell you is worth more than the gold. You have done well, and you
    will not hear from me about it again.
  hints: Horst pays you in gold and a Bloom Wafer. A wafer is not a potion, and it is
    hard to put down. Marn the draper has more to sell. Type talk marn.
  heardRequired:
  - q64_terms
  questRequired:
  - 64-found
  questExcluded:
  - 64-end
  grantsQuest: 64-end
  setsQuestFlag:
    key: 64-outcome
    value: reported
  givesGold: 150
  givesItem: 40108
```

- [ ] **Step 5: Marn, `9305.yaml`.** `bloom_undercover`, text, old end "That's where it starts. Who holds that house, I couldn't tell you." new "That's where it starts. Who holds that house, I couldn't tell you. The people at that end pay well for anyone who can find things, if that is a trade you have." Hints replace whole: "Marn nods at a wax-wrapped square under the remnants, and takes you for a customer. Horst at the Watched House Gate hires people who can find things. For the address, ask Falk. Type talk falk."

- [ ] **Step 6: Mother Brisk, `9325.yaml`.** Add first in `topics:`:

```yaml
- key: q64_brisk
  kind: info
  label: Have you taken in a runaway?
  text: I have fourteen tenants and I have told you I will not name two of them. Nobody
    goes past the first landing after last bell, and nobody goes up to the condemned
    floors at all. There is a chair across that stair. If you are who I think you are
    I cannot stop you moving it, and I will remember that you did.
  hints: She looked up at the stair past the first landing a moment too long. The
    condemned floor is up from her block. Type up.
  marked: true
  questRequired:
  - 64-start
  questExcluded:
  - 64-found
  - 64-end
  moodChange: cautious
```

- [ ] **Step 7: Manifests.** `step-ids.txt`, sorted among the keys: `64-end`, `64-found`, `64-start`. `topic-keys.txt`: `9344.q64_offer`, `9344.q64_report`, `9344.q64_terms`, `9325.q64_brisk`, `9801.q64_found`, `9801.q64_run`, `9801.who`.

- [ ] **Step 8:** `go test ./internal/questengine/ ./internal/dialogue/ . ./internal/guards/ -count=1`, the sweep, narration update. Expect `TestEveryQuestGiverIsSeenByFace` to now cover Odile. **Commit:** `feat(quests): The Skip, Horst's runaway, with two endings`.

### Task 15: Quest 102, The Knock

**Files:** create `quests/102-the_knock.yaml`, `dialogue/new_plymouth_docks/9802.yaml`. Edit `step-ids.txt`, `topic-keys.txt`, `dialogue/new_plymouth_docks/9305.yaml`. Tests: the two Task 14 files.

Mechanism: **Charge** on the offer, `goldRequired` = `chargesGold` (guard). **Chain rule** stays silent because the gate is a flag, not a token. **Gate** is `missing: ["102-start"]`, which stays false through `102-end` (`HasQuest` counts later steps), so the door stays open for good and the shop needs no second gate. A shelved 102 shuts the door; the player inside walks out; a resume charges nothing and asks for no gold (Task 13). **Reveal** is `room_enter` with `missing: ["102-behind"]`, so it fires once. **After `67-end`** Marn's offer and Sefton's shop are gated on nothing from 67, so both stay. The constable's -15 `bloom_trade` gates no shop (`internal/actions/buy.go` reads no faction). A 67 hand-in after buying wafers reads as it should: the player line accuses the trade, which by then the player knows from inside, and no line calls them innocent. **Marn's stock** is unchanged: quest 67's `bloom_undercover` line is the only route in.

- [ ] **Step 1: Failing tests,** appended:

```go
func TestQuest102TheKnock(t *testing.T) {
	const door = 5530
	q := loadShippedQuest(t, "102-the_knock.yaml", 102)
	require.Equal(t, []string{"start", "behind", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	open := func(step string) bool {
		w := newQuestWalker(q).at(step)
		return !w.e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: door}, w.p)
	}
	require.False(t, open(""), "no knock, no door")
	require.True(t, open("start"))
	require.True(t, open("end"), "the dealer stays open")

	w := newQuestWalker(q).at("start")
	w.notify("room_enter", EventDetails{RoomId: door})
	require.Equal(t, "behind", w.p.current)
	line := "three months at minimum before the new one's producing at volume, if she survives the first month"
	require.Contains(t, w.ctx.sentTexts[0], line)
	n := len(w.ctx.sentTexts)
	w.notify("room_enter", EventDetails{RoomId: door})
	require.Len(t, w.ctx.sentTexts, n, "the overheard line plays once")
}
```

```go
func TestTier5TheKnockCosts(t *testing.T) {
	m := loadShippedDialogue(t, "new_plymouth_docks/9305.yaml")
	offer := m.FindTopic("q102_offer")
	require.Equal(t, 500, offer.ChargesGold)
	require.Equal(t, 500, offer.GoldRequired)
	require.Contains(t, offer.Text, "Five hundred gold")
	require.Contains(t, offer.Objective, "five hundred gold")

	state := func(flag string, gold bool, held ...string) *PlayerState {
		ps := withFlags(heldState(held...), map[string]string{"64-outcome": flag})
		ps.HasGold = func(int) bool { return gold }
		return ps
	}
	require.NotContains(t, menuKeys(m, state("", true)), "q102_offer", "no skip, no knock")
	require.NotContains(t, menuKeys(m, state("ran", true)), "q102_offer", "she ran, the chain closes")
	require.NotContains(t, menuKeys(m, state("reported", false)), "q102_offer", "the price in hand")
	require.Contains(t, menuKeys(m, state("reported", true)), "q102_offer")
	require.Contains(t, menuKeys(m, state("reported", true, "67-start", "67-end")), "q102_offer", "survives the bust")
	require.NotContains(t, menuKeys(m, state("reported", true, "102-start")), "q102_offer")

	text, _ := m.OpenerFor(state("reported", false))
	require.Contains(t, text, "Five hundred gold", "the price is named while the offer is hidden")
	plain, _ := m.OpenerFor(state("", true))
	require.NotContains(t, plain, "Five hundred")
	ran, _ := m.OpenerFor(state("ran", true))
	require.NotContains(t, ran, "Five hundred")

	s := loadShippedDialogue(t, "new_plymouth_docks/9802.yaml")
	in := s.FindTopic("q102_in")
	require.Equal(t, []string{"102-behind"}, in.QuestRequired)
	require.Contains(t, in.Text, "never on that paper", "this cell survives quest 67")
	require.Contains(t, s.FindTopic("what_it_does").Hints, "Ysolde")
}
```

**Sabotage:** change `missing` on the door gate to `102-behind` (`open("start")` red). Drop `missing: ["102-behind"]` from the `room_enter` (red on `Len`). Set `goldRequired: 400` (red, and the charge guard red). Add `questExcluded: 67-end` to the offer (red on "survives"). Remove the Ysolde pointer (red).

- [ ] **Step 2: `quests/102-the_knock.yaml`.**

```yaml
questid: 102
name: The Knock
description: >-
  Marn the draper sells more than cloth ends, to the right people, for a price.
  Pay him and he teaches you the knock for the cupboard door at the back of his
  shop.
secret: false

steps:
  - id: start
    destination:
      room: 5530
    description: >-
      Marn took your gold and taught you the knock. The cupboard door is at the
      back of his shop, east past the shelving.
    hint: >-
      The cupboard door is in Marn's shop, east past the shelving. You know the
      knock now. Type east.
  - id: behind
    destination:
      mob: 9802  # Sefton stays at his table
    description: >-
      Behind the cupboard door you heard two men talking, and you know now what
      they meant. The man who sells is still at his table.
    hint: >-
      Sefton sits at the table in the room behind the cupboard door. Type talk
      sefton.
  - id: end
    description: >-
      You have met the man who sells the Bloom. The door stays open to you, and
      so does his shelf.

rewards:
  actor: ""
  observer: ""

triggers:
  # The cupboard door opens only to the knock. Stays open once learned.
  - event: room_gate
    room: 5530
    conditions:
      missing: ["102-start"]
    actions:
      - actor: >-
          You try the cupboard door. It is barred from the other side, and nobody
          answers your hand on the latch. It takes a knock you do not know.

  # behind: going through for the first time. The player has already been paid
  # when they work out what they did.
  - event: room_enter
    room: 5530
    conditions:
      has: ["102-start"]
      missing: ["102-behind"]
    actions:
      - grant: "102-behind"
      - actor: >-
          You knock the way Marn showed you, and a bar lifts. Two men at the table
          have not quite stopped talking. "...three months at minimum before the
          new one's producing at volume, if she survives the first month." One of
          them sees you, stands, and goes out past you without a word. The other
          squares his ledger and waits. You think about a condemned floor, and a
          woman with no change on her, and the gold you were paid.
```

- [ ] **Step 3: Marn, `9305.yaml`.** Add at the head of `variants:`:

```yaml
  - questRequired:
    - 102-end
    text: The door is the door. What you do behind it is between you and him.
    hints: The cupboard door is east past the shelving. Type east.
  - questRequired:
    - 102-start
    text: You know the knock. I do not teach it twice.
    hints: The cupboard door is east past the shelving. Type east.
  - questFlagRequired:
      64-outcome: reported
    text: Horst says you can find things. Then you have some idea what is behind a door
      like mine. Five hundred gold and I teach you the knock. I do not haggle and I do
      not give credit.
    hints: When you hold five hundred gold, The Knock is on his list. It is a choice,
      and what is sold back there is hard to put down.
```

Add first in `topics:`:

```yaml
- key: q102_offer
  kind: offer
  quest: 102
  label: The Knock
  playerLine: What would the knock cost me?
  text: Five hundred gold. It changes every few months, and what is behind the door
    does not. Three short, then one, low on the frame. You have already been paid once
    for what you did, so do not tell me about your conscience. You go through on your
    own feet.
  hints: Accepting takes the gold now. It is a choice, and what is sold back there is
    hard to put down.
  objective: Pay Marn five hundred gold for the knock, then go through the cupboard door.
  questFlagRequired:
    64-outcome: reported
  goldRequired: 500
  chargesGold: 500
  questExcluded:
  - 102-start
  - 102-end
  grantsQuest: 102-start
  moodChange: cautious
```

`backroom` stays: "Storage..." is Marn lying, and the stair is now truly storage.

- [ ] **Step 4: `dialogue/new_plymouth_docks/9802.yaml`.**

```yaml
mobid: 9802
zone: New Plymouth Docks
defaultMood: neutral
greetings:
- text: Close the door.
  moods:
  - neutral
opener:
  text: You have the knock, so you have the price. I sell one thing. Do not ask me for
    credit.
  hints: To see what he sells, type list. Ask him what it does first.
  variants:
  - questRequired:
    - 102-behind
    questExcluded:
    - 102-end
    text: Marn said someone was coming. You heard something on your way in. You will
      keep it to yourself.
    hints: Choose The Knock below to finish your business with him.
topics:
- key: q102_in
  kind: handin
  quest: 102
  label: The Knock
  playerLine: Marn sent me. I paid.
  text: Then you are a customer. I am Sefton. I sell one thing, and I keep a small
    stock, because a large stock is how people get noticed. Whatever the watch breaks
    up on Lintel Street is a house and a ledger. This room was never on that paper. None
    of us has ever met the others. That is what the arrangement is for.
  hints: To see what he sells, type list. To take a wafer, type quaff wafer. Ask him
    what it does to you first.
  questRequired:
  - 102-behind
  questExcluded:
  - 102-end
  grantsQuest: 102-end
- key: what_it_does
  kind: info
  label: What does a wafer do to me?
  text: For a little while you feel like the best day you ever had. Then you pay for
    it, and you feel worse than you ever have, for a good deal longer. Something in you
    changes, sometimes a lot, and you do not get to choose what. Take a few and you will
    need them. Stop, and the need comes for you. That is the business.
  hints: Each wafer poisons you a little, so your body will refuse one too many. The
    craving fades slowly on its own. Ysolde the healer has a harsh cure. Type talk ysolde.
  marked: true
- key: where_from
  kind: info
  label: Where does this come from?
  text: You heard where it comes from, on your way in. That is as much as you will hear
    from me.
```

- [ ] **Step 5: Manifests.** `step-ids.txt`: `102-behind`, `102-end`, `102-start`. `topic-keys.txt`: `9305.q102_offer`, `9802.q102_in`, `9802.what_it_does`, `9802.where_from`.

- [ ] **Step 6:** `go test ./internal/questengine/ ./internal/dialogue/ ./internal/conversation/ . ./internal/guards/ -count=1`, the sweep, narration update. Confirm `TestDialogueGoldPurchasesAreAtomic` is green with 102's charge. Confirm no `offered but not abandonable` line names 64 or 102 in the boot log. **Commit:** `feat(quests): The Knock, and Sefton behind Marn's cupboard`.

### Task 16: 5c guard, docs, gates, PR

- [ ] **Step 1:** `tier5Quests` adds `64: true, 102: true`.
- [ ] **Step 2: Patch note.** The chain is a thing to discover, so this says little:

```markdown
## 2026-11-05: Something under the remnants

- The Bloom Wafer is now out there in the world, for those who go looking for it and
  will pay what it costs. It is not like a potion. It changes you, it is hard to put
  down, and when you stop, the need comes for you. The healer Ysolde in the Common
  Quarter has a harsh cure.
```

- [ ] **Step 3: `docs/README.md` rows:**

```markdown
| [`../_datafiles/world/dogmud/quests/64-the_skip.yaml`](../_datafiles/world/dogmud/quests/64-the_skip.yaml) | Quest 64 The Skip (#375 tier 5): Horst's runaway, two endings by flag `64-outcome`, a `room_gate` that opens her floor only while the quest runs |
| [`../_datafiles/world/dogmud/quests/102-the_knock.yaml`](../_datafiles/world/dogmud/quests/102-the_knock.yaml) | Quest 102 The Knock (#375 tier 5): Marn's stake, the `room_gate` on the cupboard door, the overheard line |
| [`../_datafiles/world/dogmud/rooms/new_plymouth_docks/5530.yaml`](../_datafiles/world/dogmud/rooms/new_plymouth_docks/5530.yaml) | Behind the Cupboard, Sefton's room, east of Marn's shop |
| [`../_datafiles/world/dogmud/rooms/new_plymouth_common/5625.yaml`](../_datafiles/world/dogmud/rooms/new_plymouth_common/5625.yaml) | The Condemned Floor, Odile's hiding place, up from Mother Brisk's block |
| [`../_datafiles/world/dogmud/mobs/new_plymouth_docks/9802-sefton.yaml`](../_datafiles/world/dogmud/mobs/new_plymouth_docks/9802-sefton.yaml) | Sefton, who sells the Bloom Wafer; stock capped at 5 by `stock_multiplier` and the wafer's `rarity_tier` |
| [`../_datafiles/world/dogmud/mobs/new_plymouth_common/9801-odile.yaml`](../_datafiles/world/dogmud/mobs/new_plymouth_common/9801-odile.yaml) | Odile, the runaway of quest 64 |
| [`../_datafiles/world/dogmud/dialogue/new_plymouth_docks/9802.yaml`](../_datafiles/world/dogmud/dialogue/new_plymouth_docks/9802.yaml) | Sefton's topics |
| [`../_datafiles/world/dogmud/dialogue/new_plymouth_common/9801.yaml`](../_datafiles/world/dogmud/dialogue/new_plymouth_common/9801.yaml) | Odile's topics, including the `ran` hand-in |
| [`../internal/guards/tier5_bloom_shop_test.go`](../internal/guards/tier5_bloom_shop_test.go) | Pins the wafer's shelf cap, its own restock clock, and why it cannot be sold back |
| [`../internal/questengine/tier5_bloom_chain_test.go`](../internal/questengine/tier5_bloom_chain_test.go) | Step tests for quests 64 and 102: both gates, the found step, the reveal firing once |
| [`../internal/dialogue/tier5_bloom_chain_test.go`](../internal/dialogue/tier5_bloom_chain_test.go) | Who sees the chain, both endings of The Skip, Marn's stake, Sefton's cure pointer |
| [`../internal/configs/config.balance_bloom_test.go`](../internal/configs/config.balance_bloom_test.go) | The shipped config names every Bloom knob at its default |
```

Add to `internal/items/context.md`, `internal/shops/context.md` one line each if either lists Bloom or stock caps: read first. Update `.claude/skills/dogmud-balance-config/SKILL.md` only if its knob count is cited as live: the count is not changed by this task.

- [ ] **Step 4: Gates.** From the worktree root:

```bash
gofmt -l internal/ modules/                                   # prints nothing
go build ./...                                                # no output
go test . ./internal/guards/ ./internal/questengine/ ./internal/dialogue/ ./internal/conversation/ ./internal/quests/ ./internal/questloop/ ./internal/configs/ ./internal/items/ ./internal/mobs/ ./internal/shops/ ./internal/rooms/ ./internal/narration/ -count=1   # all ok
go test ./... -count=1                                        # all ok
DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1         # ok
~/go/bin/golangci-lint run --new-from-merge-base=origin/master # 0 issues
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones new_plymouth_common,new_plymouth_docks,new_plymouth_merchant; echo $?   # 0, and no UNSTARTABLE QUEST for 64 or 102
grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .   # 12
```

Isolated boot, as Task 5 Step 4's second block, with `GamePlay.MapConsistencyEnforce: panic` as shipped: the two new rooms' crawl positions are checked here for the first time. Report `Server Ready`.

- [ ] **Step 5: Commit,** push, `gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-tier5c`. The PR body lists the three owner numbers and says they are proposals. Wait for CI, merge with `--merge`, remove the worktree. The owner sets numbers after the playtest, before deploy.

---

# PR 5d: quest 21, the sample quest, the close

### Task 17: Quest 21 retires

**Files:** `git rm quests/21-newcomers_path.yaml`. Edit `quests/30-the_awakening.yaml`, `step-ids.txt`, `loop-backlog.txt`, `internal/usercommands/start.go`, `dialogue/pothole_coulee/9100.yaml`, `9106.yaml`, `coulee_spine_smoke_test.go`. Test: create `internal/usercommands/start_newcomer_test.go`.

- [ ] **Step 1: Failing tests.**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The newcomer marker is a flag on The Awakening now, read by Hadwen's and
// Toke's teaching topics. Quest 21 is retired.
func TestGrantNewcomerMarkerSetsTheFlag(t *testing.T) {
	u := users.NewTestUser(9731, "newcomer", "Newcomer", 0)
	grantNewcomerMarker(u)
	require.Equal(t, "new", u.Character.GetQuestFlag("30-newcomer"))
	require.Empty(t, u.Character.QuestProgress, "no quest token any more")
}
```

Append to `internal/dialogue/tier5_new_plymouth_test.go`:

```go
func TestTier5NewcomerTopicsReadTheFlag(t *testing.T) {
	newFlag := map[string]string{"30-newcomer": "new"}
	told := map[string]string{"30-newcomer": "told"}

	toke := loadShippedDialogue(t, "pothole_coulee/9106.yaml")
	require.Contains(t, menuKeys(toke, withFlags(heldState(), newFlag)), "newcomer_players")
	require.NotContains(t, menuKeys(toke, withFlags(heldState(), told)), "newcomer_players")
	require.NotContains(t, menuKeys(toke, heldState()), "newcomer_players")
	require.Equal(t, "told", toke.FindTopic("newcomer_players").SetsQuestFlag.Value)
	require.Empty(t, toke.FindTopic("newcomer_players").GrantsQuest)

	hadwen := loadShippedDialogue(t, "pothole_coulee/9100.yaml")
	require.Contains(t, menuKeys(hadwen, withFlags(heldState("30-end"), newFlag)), "newcomer_mutations")
	require.NotContains(t, menuKeys(hadwen, withFlags(heldState("30-end"), told)), "newcomer_mutations")
	require.NotContains(t, menuKeys(hadwen, withFlags(heldState(), newFlag)), "newcomer_mutations")
}
```

**Sabotage:** set the flag to `told` in `grantNewcomerMarker` (red). Drop `questFlagRequired` from `newcomer_players` (red).

- [ ] **Step 2: `quests/30-the_awakening.yaml`.** Add after `secret: false`:

```yaml

flags:
  # Set when a total newbie starts (usercommands.grantNewcomerMarker). Hadwen and
  # Toke teach the basics to a holder of "new"; Toke's talk sets "told".
  - key: newcomer
    values: [new, told]
    description: "Whether a new-to-MUDs character has had the guides' teaching"
```

- [ ] **Step 3: `internal/usercommands/start.go:264-272`.**

```go
// grantNewcomerMarker flags a total-newbie character with The Awakening's
// newcomer flag, which gates the newbie-only teaching beats (Cleric Hadwen's
// mutations/progression talk; Crier Toke's player-interaction talk).
func grantNewcomerMarker(user *users.UserRecord) {
	user.Character.SetQuestFlag("30-newcomer", "new")
}
```

Remove the `events` import only if nothing else in the file uses it. It is used elsewhere; let the compiler say.

- [ ] **Step 4: Topics.** `9100.yaml` `newcomer_mutations`: replace the gates with:

```yaml
  questRequired:
  - 30-end
  questFlagRequired:
    30-newcomer: new
```

`9106.yaml` `newcomer_players`: replace `questRequired`, `questExcluded`, `grantsQuest` with:

```yaml
  questFlagRequired:
    30-newcomer: new
  setsQuestFlag:
    key: 30-newcomer
    value: told
```

- [ ] **Step 5: `coulee_spine_smoke_test.go`.**

| Line | Old | New |
|---|---|---|
| 433 comment | "except 21-start, queued the way the new-to-MUDs route does." | "and the newcomer flag is set the way the new-to-MUDs route sets it." |
| 439 | `events.AddToQueue(events.Quest{UserId: w.user.UserId, QuestToken: "21-start"})` | `c.SetQuestFlag("30-newcomer", "new")` |
| 444-445 | `if len(c.QuestProgress) != 1 {` / `"a new character holds only the newcomer marker, not %v"` | `if len(c.QuestProgress) != 0 {` / `"a new character holds no quest, not %v"` |
| 467 | `want := map[int]string{21: "start", 28: "end"}` | `want := map[int]string{28: "end"}` |

Read the test past 467 for other maps holding key 21 and change them the same way. Remove the `w.pump()` at 440 only if it fails to compile or changes behaviour. `internal/characters/questfocus_test.go:93` stubs its own secret quest and needs no change.

- [ ] **Step 6: Manifests, backlog.** `step-ids.txt`: delete `21-end`, `21-start`. Under `# retired`:

```
# Quest 21 Newcomer's Path (#375 phase 7 tier 5): a flag on quest 30 now, so
# users.LoadUser drops it from saves (quests.RetiredQuests).
21-end
21-start
```

Delete these 4 backlog lines:

```
end 21 mob 9106 topic newcomer_players
end 21 step end
hint 21 step start
start 21 step start
```

- [ ] **Step 7:** `go test ./internal/usercommands/ ./internal/dialogue/ ./internal/quests/ ./internal/characters/ . ./internal/guards/ -count=1`, `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke' -count=1`, narration update. **Commit:** `refactor(quests): the newcomer marker is a flag on The Awakening, and quest 21 retires`.

### Task 18: Quest 1000000 is deleted

**Files:** `git rm quests/1000000-generic_quest.yaml`. Edit `step-ids.txt`, `loop-backlog.txt`, `internal/quests/save.go:74-76`, `modules/gmcp/gmcp.Quest.go:25-28`, `internal/quests/save_test.go:117` comment, `internal/narration/testdata/stores/quests.golden`.

- [ ] **Step 1:** The grep could find a reader: `grep -rn "1000000-\|generic_quest\|Generic Quest" internal modules cmd *.go` finds `save.go`, `save_test.go`, `gmcp.Quest.go`, and the golden. `save_test.go:120` seeds its own quest 1000000, so it still proves the floor. `genericQuestIdFloor` stays. **Sabotage for the kept test:** make `CreateNewQuestFile` not skip the floor; `TestCreateNewQuestFile_SkeletonIsBootSafe` goes red.

- [ ] **Step 2: Comments.** `save.go`: "ReservedTemplateIdFloor marks the top of the id range. Id allocation stays below it and the editor list hides anything at or above it. The sample quest that once sat there is gone (#375 tier 5)." `gmcp.Quest.go`: "The reserved range at and above quests.ReservedTemplateIdFloor is hidden from the editor list." `save_test.go`: "A quest in the reserved range sits in the same cache; id allocation must IGNORE that range or every created quest lands above it, invisible to the editor list."

- [ ] **Step 3: Manifests, backlog.** `step-ids.txt`: delete `1000000-end`, `1000000-givegold`, `1000000-start`. Under `# retired`:

```
# Quest 1000000, the sample quest (#375 phase 7 tier 5): deleted. It named a
# condition and a skill that do not exist, and no player could reach it.
1000000-end
1000000-givegold
1000000-start
```

Delete these 8 backlog lines:

```
end 1000000 step end
one-action 1000000 step end:none
one-action 1000000 step givegold:none
refs 1000000 quest 1000000 refs: rewards conditionid: condition 4 does not exist
refs 1000000 quest 1000000 refs: rewards skillinfo: skill "map" does not exist
start 1000000 step start
walk-destination 1000000 step givegold
walk-destination 1000000 step start
```

- [ ] **Step 4:** `go test ./internal/quests/ ./modules/gmcp/ ./internal/narration/ . ./internal/guards/ -count=1`, the sweep. The narration diff must be only lines naming quest 1000000 and 21. **Commit:** `chore(quests): delete the unreachable sample quest`.

### Task 19: Guard, roadmap, docs

- [ ] **Step 1:** `tier5Quests` adds `21: true, 1000000: true`. Backlog now holds comments only: `grep -v '^#' _datafiles/world/dogmud/quests/loop-backlog.txt | grep -c .` prints 0 (exit 1 is the pass). `questloop.ParseBacklog` and the sweep must accept an empty list (`context.md`: "an empty list means every rule is hard").

- [ ] **Step 2: Roadmap.** `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, old "Tier 5 (New\nPlymouth, quests 63 and 65 to 71) is not started." new:

```markdown
Tier 5 (New Plymouth, quests 63 and 65 to 71) is built, and `loop-backlog.txt`
is empty (`internal/guards/tier5_backlog_guard_test.go`). Quest 21 is a flag on
quest 30, the sample quest is deleted, and quests 64 and 102 give the Bloom a source
behind Marn's cupboard door. The closing step comes next, in its own plan:
validators as hard boot failures, deleting `room_interact`, and the dash guards.
```

In `QUEST_DIALOGUE_REQUIREMENTS.md` change nothing: its boxes wait for the closing step, and a line there is the owner's.

- [ ] **Step 3:** `internal/questloop/context.md`: no rule changed, nothing to do. Update `.claude/skills/dogmud-authoring-quests/SKILL.md`'s two mentions of quest 68's `loose-floorboard` trap only if the fact is now false: it is, so reword to past tense ("Quest 68's trigger noun was...", kept as the example), and leave the rule.

- [ ] **Step 4: Patch note:**

```markdown
## 2026-11-05: Tidying up

- Cleric Hadwen and Crier Toke still teach new arrivals the basics. Anyone caught
  part way through those lessons today may miss the two talks.
```

- [ ] **Step 5: Commit** with named paths, `docs(#375): tier 5 closes the backlog, roadmap catches up`.

### Task 20: The closing playtest (controller)

Fixture lessons this run uses: distinct profiles, or seats read each other as themselves (tier 2-3). Tally claims from raw `events.jsonl`, since a seat's own grep hides lines (667). The sit-and-meditate line is a quit, not a death. Search has a cooldown, so a repeat search waits. A flag must be declared by a quest, or the env dies at boot: `64-outcome` exists from 5c. `set_quest_tokens` with a chain plus `start_room` starts a seat late. A seat is not naive: it holds commit subjects, so weigh early interest. `playtestrun stop` needs `--run-id`, the container needs `docker rm -f`, and the network needs removing. Only time online counts for withdrawal, and the buy limit stops at 3 (both owner, 2026-11-04), so the shipped goals add two seats, `bloom-relog` and `bloom-load`, and `bloom.yaml` buys until refused; the files under `tools/playtest/goals/scenarios/375-p7-tier5/` are the source of truth for the goals below. The Old Quarter needs a real light in the goal grant.

**Files:** create `tools/playtest/scenarios/375-p7-tier5.yaml` and `tools/playtest/goals/scenarios/375-p7-tier5/{docks,teels,bloomtrail,circle,bloodline,cipher,tribute,skip-ran,bloom,newcomer}.yaml`. Rows in `docs/README.md`.

- [ ] **Step 1: Scenario.**

```yaml
# #375 phase 7 tier 5: New Plymouth. Each quest by search, 66's return walk, both
# 68 endings before 70, both endings of The Skip, and the Bloom chain through a
# first dose, withdrawal and Ysolde's cure.
name: 375-p7-tier5
mode: party
summary: >-
  The New Plymouth quests are followed by search and their hints, the stories agree
  with themselves, Orin remembers the cooperage, and the Bloom can be reached only by
  three deliberate choices, then survived.
on_actor_stop: continue
budgets:
  wall_clock: 90m
requires:
  max_connections: 20
roster:
  - {id: docks, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/docks.yaml}
  - {id: teels, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/teels.yaml}
  - {id: bloomtrail, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/bloomtrail.yaml}
  - {id: circle, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/circle.yaml}
  - {id: bloodline, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/bloodline.yaml}
  - {id: cipher, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/cipher.yaml}
  - {id: tribute, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/tribute.yaml}
  - {id: skip-ran, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/skip-ran.yaml}
  - {id: bloom, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/bloom.yaml}
  - {id: newcomer, personality: bug-finder, goals: goals/scenarios/375-p7-tier5/newcomer.yaml}

group_goals:
  - id: no-dead-ends
    do: Each seat follows the game's own hints and map, with no route help.
    verify: Every hint names something the player can do where they stand, and no seat gets stuck.
  - id: makes-sense
    do: Every seat reads NPC lines and hints as a player new to MUDs would.
    verify: No line promises an item, place or command that is not there, and no line contradicts another.
```

- [ ] **Step 2: Goals.** Every file opens with its `ephemeral:` block, then `goals:`. Each goal ends "Quote game lines VERBATIM and in order."

`docks.yaml` (63, 65):

```yaml
ephemeral:
  profile: mid
  start_room: 5503
  budgets:
    wall_clock: 50m
goals:
  - >-
    Type `talk jesset` and take Dock Rat. Follow only `hint` and the `quests` line. In the
    office, `look floorboard`, then `search`, and quote every line. Search again after the
    cooldown and say what happens.
  - >-
    Type `drop tally-book` in another room and leave it. Go back, `search`, and quote what
    you get. Finish with the constable. Then `talk hewd`, `talk constable`, `talk jesset`,
    and say whether Hewd at his desk makes sense from what they tell you.
  - >-
    Type `talk coll`, take The Street Sweeper's Secret, and follow the hint. Say whether you
    could find the Old Foundation from it, and how many moves it cost. `search`, then finish
    with Coll. Then find Gritta in the Old Quarter, ask who receives her finds, and say
    whether her account fits what Coll told you.
```

`teels.yaml` (66's return walk):

```yaml
ephemeral:
  profile: crafter
  start_room: 5501
  budgets:
    wall_clock: 40m
goals:
  - >-
    Type `talk teels` and take The Addict's Plight. Follow `hint`. Say whether you ever had to
    guess what the wrapper was for. Give it to Ysolde and quote her lines.
  - >-
    Before going back, type `quaff purge` and quote every line, then `status`. Go on to Teels and
    say what they tell you. Go back to Ysolde, ask for another purge, and quote it.
  - >-
    Walk back to Teels and hand it over. Quote the reward line. Say whether The Bloom Trail is
    pitched to you there, and whether it makes sense coming from Teels. Then `talk ysolde` and say
    whether what she says fits.
```

`bloomtrail.yaml` (67, evidence route):

```yaml
ephemeral:
  profile: charmer
  start_room: 5523
  overlays:
    set_quest_tokens: [66-start, 66-dose, 66-end, 67-start]
    grant_items: [20096]
  budgets:
    wall_clock: 55m
goals:
  - >-
    Type `talk marn` and choose to press him about the wax wrapper. Follow only `hint` and `quests`
    from there to the end of The Bloom Trail. Say at each step whether the hint told you who to find,
    and how many moves it took without a route.
  - >-
    Talk to Deren and read every line he offers. Say whether anything he says treats the case as
    already over. Read the case file with `look` when Quill gives it to you, and say whether the
    constable's hand-in matches what the file holds.
  - >-
    After the hand-in, type `talk horst`. You took the honest route, so say whether anything there or
    at Marn's hints at a job or a door.
```

`circle.yaml` (68 circle, then 70, and the floor search while Toby sleeps):

```yaml
ephemeral:
  profile: specialist-caster
  start_room: 5706
  overlays:
    grant_items: [20096]
  budgets:
    wall_clock: 60m
goals:
  - >-
    Type `talk orin` and take The Cooperage Circle. Find Toby and take his rubbing. Wait until Toby
    sleeps on the Cooperage Floor at night: `look` until he is there. Equip your torch and type
    `search` beside him. Quote every line. Say whether it feels fair that you could do this with him
    asleep, and whether anything woke.
  - >-
    Show Orin the survey with the rubbing in hand and quote him. Hear what standing with the circle
    costs, choose it, and quote it. Then `look old guild mark` at the cooperage front and `talk toby`,
    and say whether what they tell you fits what you see.
  - >-
    Go to Gritta's Flooded Cellar. Quote her opener. Take The Pre-Founding Web. First, unequip your
    light, go to the Buried Lintel, and type `search`. Quote it. Then equip it and search again after
    the cooldown. Quote it. Finish with Orin and quote the reward.
```

`bloodline.yaml` (68 bloodline, then 70):

```yaml
ephemeral:
  profile: m2-actor
  start_room: 5706
  overlays:
    set_quest_tokens: [68-start, 68-evidence, 68-survey, 68-choice]
    grant_items: [40113, 20096]
  budgets:
    wall_clock: 45m
goals:
  - >-
    Go to Clerk Vell, hear what filing the survey means, and hand it over. Quote it. Type `talk orin`
    and `talk toby` and quote both.
  - >-
    Go to Gritta. Say whether she nods at what you did, as she should not. Take The Pre-Founding Web,
    take the rubbing with a light, and bring it to Orin. Quote the hand-in and the reward. Say whether
    anything treats you as a friend of the circle.
```

`cipher.yaml` (69):

```yaml
ephemeral:
  profile: ranged-range
  start_room: 5916
  budgets:
    wall_clock: 45m
goals:
  - >-
    Type `talk dross` and take The Gallery Cipher. Follow `hint`. `search` at the reliefs. At the
    gallery, read what Dross told you about the guide against what the guide does, and say whether
    anything sounds like a door or a fight you have to win.
  - >-
    Back at Dross with the reading, drop the rubbing first, and quote what he says. Take another with
    `search`, then hand in. Quote the reward. Type `inventory` and say whether the rubbing is gone and
    whether the reward line was true.
```

`tribute.yaml` (71):

```yaml
ephemeral:
  profile: m2-witness
  start_room: 5810
  budgets:
    wall_clock: 35m
goals:
  - >-
    Type `talk ostry` and take The Tribute. Follow `hint` to the end. In the parlor, `search`, and quote
    it. Say whether Horst being there or not changes anything. Finish with Ostry. Type `reputation` or
    the nearest command and quote which standing moved.
```

`skip-ran.yaml` (the `ran` ending):

```yaml
ephemeral:
  profile: early
  start_room: 5820
  overlays:
    set_quest_tokens: [66-start, 66-dose, 66-end, 67-start, 67-front]
    set_quest_flags: {67-entry: undercover}
    set_gold: 2000
  budgets:
    wall_clock: 40m
goals:
  - >-
    Type `talk horst` and take The Skip. Follow `hint`. Try `up` from Mother Brisk's block before you
    talk to her, and quote it. Say whether the quest log gave her floor away.
  - >-
    Find Odile. Hear why Horst wants her. Tell her to run. Quote it. Go down, then try `up` again and
    quote it. Type `talk horst` and quote him.
  - >-
    Go to Marn with your 2000 gold and `talk marn`. Say whether anything offers you the knock. Type
    `east` at the cupboard door and quote it. Abandon nothing. Report whether this ending felt final.
```

`bloom.yaml` (the whole chain, withdrawal, cure. **Do not log out.**):

```yaml
ephemeral:
  profile: counters
  start_room: 5523
  overlays:
    set_quest_tokens: [66-start, 66-dose, 66-end, 67-start]
    set_gold: 2000
  budgets:
    wall_clock: 70m
goals:
  - >-
    Type `east` at the cupboard door and quote it. `down` and look at the stock room. Back up. Type
    `talk marn`, choose to pose as a buyer, and quote his line and hint. Follow the hint to Horst and
    take The Skip. Find Odile. Tell Horst, after hearing him. Quote what he pays you. `inventory`.
  - >-
    Go to Marn. Read his opener. Accept The Knock and quote the card. Type `score` or the nearest and
    say what it cost. Type `east` and quote the whole arrival. Say what you understood at that moment.
    Finish with Sefton. Type `list` and quote prices. Try to `sell wafer` to him and to one shop
    outside. Quote both.
  - >-
    Hear what a wafer does. Type `quaff wafer` and quote every line, in order. Note the time. Type
    `status` every few rounds. Stay logged in. Buy and quaff another, and another, until your body
    refuses, and quote the refusal. Say how many, how long between, and any change that arrived.
    Report each `list` price as the shelf empties. Count to the last wafer.
  - >-
    Stop. Wait for the crash and the craving and quote each. Say whether a new player would know what
    is happening and where to go. Follow only what the game has told you, to Ysolde. Ask for the cure,
    `quaff purge`, and quote it. Type `status` afterwards. Say whether the craving came back.
  - >-
    Finish The Bloom Trail if budget remains, and quote the constable's hand-in. Say whether it reads
    sensibly from someone who has just bought from the trade. Go back through the cupboard door after
    it and say whether Sefton's shop is still open and what he says about the watch.
```

`newcomer.yaml` (quest 21):

```yaml
ephemeral:
  profile: fresh
  start_room: 5203
  overlays:
    set_quest_tokens: [30-start, 30-rite, 30-woken, 30-end]
    set_quest_flags: {30-newcomer: new}
  budgets:
    wall_clock: 20m
goals:
  - >-
    Type `talk toke` and find the line about getting on with people. Quote it. Type `talk toke` again and
    say whether it is still marked. Go to Hadwen and say whether his line about mutations is still open.
```

- [ ] **Step 3: Run** `/playtest-scenario --checkout <abs worktree> tools/playtest/scenarios/375-p7-tier5.yaml`. Tally seat claims from raw `events.jsonl`. **From the `bloom` seat report to the owner:** doses before refusal, time between doses, ranks gained, gold spent, and shelf prices, against the arithmetic above. Fix what belongs to this pass; file the rest on `pruuk/DOGMud`, exploit-class privately.

- [ ] **Step 4:** Clean up by run id and PID, `docker rm -f`, remove the network. Put measured numbers in the PR body. The owner reviews them before deploy (design section 7).

### Task 21: 5d gates, PR

- [ ] **Step 1: `docs/README.md` rows:**

```markdown
| [`../tools/playtest/scenarios/375-p7-tier5.yaml`](../tools/playtest/scenarios/375-p7-tier5.yaml) | Closing playtest for #375 phase 7 tier 5: ten seats, goals under `tools/playtest/goals/scenarios/375-p7-tier5/` (docks, teels, bloomtrail, circle, bloodline, cipher, tribute, skip-ran, bloom, newcomer) |
| [`../internal/usercommands/start_newcomer_test.go`](../internal/usercommands/start_newcomer_test.go) | The newcomer marker is a flag on quest 30 |
```

- [ ] **Step 2: Gates.** From the worktree root:

```bash
gofmt -l internal/ modules/                                   # prints nothing
go build ./...                                                # no output
go test . ./internal/guards/ ./internal/questengine/ ./internal/dialogue/ ./internal/usercommands/ ./internal/characters/ ./internal/quests/ ./internal/questloop/ ./internal/narration/ ./modules/gmcp/ -count=1   # all ok
go test ./... -count=1                                        # all ok
DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1         # ok
~/go/bin/golangci-lint run --new-from-merge-base=origin/master # 0 issues
go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue _datafiles/world/dogmud/dialogue -zones pothole_coulee,new_plymouth_common,new_plymouth_crafting,new_plymouth_docks,new_plymouth_merchant,new_plymouth_noble,new_plymouth_old_quarter,new_plymouth_temple; echo $?   # 0
```

Isolated boot, as Task 5 Step 4's second block. Grep the log for `dropped retired quest` only if you seeded a save: not expected on a fresh checkout.

- [ ] **Step 3: Commit,** push, `gh pr create --repo pruuk/DOGMud --base master --head feat/375-p7-tier5d`, with the playtest numbers. Do not close #375. Wait for CI, merge with `--merge`, remove the worktree. The owner deploys.

## Self-review against the design

| Design section | Where |
|---|---|
| 1 rulings 1 to 5 | Depth in words: Tasks 1, 2, 6, 8. 66: Task 3. 70 stands alone: Task 8. Chain: Tasks 14, 15. Ran costs: Task 14 |
| 2 per quest | Tasks 1 to 4, 6 to 9 |
| 3 light and hours | Hints in 67, 70 name a light; hours removed from every step hint; torch guard |
| 4 quest 21, sample | Tasks 17, 18 |
| 5 not in tier | No `room_interact` deletion, no dash guard; 40100 and others keep their dashes (#383) unless a fix touches them |
| 6 testing | Guards, step tests, `dialoguecheck`, closing playtest, all named |
| 7 chain | Tasks 11 to 16, numbers table above |
