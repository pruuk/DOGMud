# Loose Issues Sweep 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix fourteen more small open issues (#217, #244, #262, #273, #277, #308, #287, #346, #266, #286, #293, #339, #362, #253) in one branch.

**Spec:** `docs/superpowers/specs/2026-10-10-loose-issues-sweep-2-design.md` (owner approved 2026-10-10).

**Architecture:** One task per issue, each touching its own files, then a gate task. No new packages. Behavioural changes get a test that fails first; copy, YAML and dead-code changes do not, but each is checked by a build, a guard or a boot.

**Tech Stack:** Go, testify, Go `text/template` world templates, YAML content.

**Worktree:** `C:\Users\Calabe Davis\workspace\DOGMud\.claude\worktrees\loose-issues-sweep`, branch `fix/loose-issues-sweep-2` (from master ab027c43d). Run every command from there. Never `cd` to, or run git against, the main checkout `C:\Users\Calabe Davis\workspace\DOGMud`.

---

## Ground rules for every task

- **Do not touch these files** (the Messaging M6 slice 1 session is editing them): anything in `internal/conditions/`; in `internal/hooks/`: `Condition_ApplyConditions.go`, `spell_effects.go`, `spell_help_effects.go`, `spell_resolution.go`, `light_spell.go`, `NewRound_MobRoundTick.go`, `NewRound_UserRoundTick.go`; in `internal/actions/`: `combat_drain.go`, `combat_hamstring.go`, `combat_maul.go`, `combat_rake.go`, `combat_throttle.go`, `actor_ref.go`; `internal/behaviortree/actions_item_proc.go`, `action_cast_best_in_category.go`; `internal/combat/ai.go`; `internal/characters/combat.go`, `conditions.go`; `internal/events/eventtypes.go`; `internal/users/userrecord.go`; `internal/mobs/mobs.go`; `keywords.yaml`; root `condition_apply_path_guard_test.go`, `shipped_narration_data_guard_test.go`; the `context.md` of conditions, characters, events, users, mobs, actions, behaviortree, combat and hooks. If a task seems to need one of these, stop and report.
- The two line-keyed root guards key these files only, and no task here edits any of them: `condition_apply_path_guard_test.go` keys `file|line` rows in `internal/actions/{actor_mob,actor_user,combat_*,drink,sleep}.go`, `internal/behaviortree/actions_item_proc.go`, `internal/combat/{grapple_move,submission_outcome}.go`, `internal/hooks/{Awareness_Cascades,Condition_ApplyConditions,Life_Cascades,light_spell,manifester_companions,pinnacle_tick,spell_effects,spell_help_effects}.go`, `internal/justice/arrest.go`, `internal/mobcommands/consume.go`, `internal/usercommands/{admin.setcondition,character,rally,skill.disenchant,warcry}.go`; `shipped_narration_data_guard_test.go` keys `tips.yaml`, `gossip_templates.yaml`, `messaging/position_control.yaml`, `messaging/grapple_outcomes.yaml`. `spell_condition_data_guard_test.go` does not exist on this branch.
- Stage named paths only. Never `git add -A` or `git add .`.
- Never edit a file with a Python read-modify-write. Use the Edit tool. Templates and YAML are CRLF in the working tree and LF in the index (`core.autocrlf=true`); an Edit that leaves some lines LF is normalised on `git add`, so that is fine.
- `_datafiles/config.yaml` is touched by Task 12 only, as that task says.
- No em dashes or en dashes in player text, comments or commit messages.
- Player text: 80 columns, no raw numbers for damage, healing, armor, chances or durations.
- New tests restore every piece of shared state they change (user 1's items, gold, party, role, storage, stats; room corpses, lamp, sky light, storage flag; mob items; config; round count) with `t.Cleanup`. The usercommands package already has a shuffle-order problem (#440); do not add to it.
- Every commit message ends with a blank line, then:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- Test command form: `go test ./internal/<pkg>/ -run '<Regex>' -count=1`.

## Facts verified against source (worktree at 728f709c0, master ab027c43d)

| Issue | Fact | Where |
|---|---|---|
| #217 | `get all <corpse>` resolves the corpse with `room.FindCorpseIndex(args[len(args)-1], user.Character)`, the last word only | `internal/usercommands/get.go:194-196` |
| #217 | The single-item path already splits the trailing container or corpse span with `parser.SplitTrailingContainer(scope, rest)`; it is at 372-389, not ~330-347 as the brief said | `internal/usercommands/get.go:372-389` |
| #217 | `SplitTrailingContainer(s Scope, input string) (itemPart string, cm Match, ok bool)`: with `from`, only the span after it is tried; without, it tries the longest trailing span first. `Match.Kind == parser.KindCorpse`, `Match.CorpseIdx` | `internal/parser/helpers.go:26-44` |
| #217 | `FindCorpseIndex` names a mob corpse `Character.Name + " corpse"` and lists newest first, so a bare `corpse` means the newest | `internal/rooms/rooms.go:1591-1662` |
| #244 | `cmdPartyList` appends every member's `GetCharmIds()` and lists them as `♥friend` (342-361), then lists every `Character.Companions` entry as `♦companion` (363-396). The charm loop uses `mobs.GetInstance` and `rooms.LoadRoom` unchecked; the member loop (300) and invite loop (398) use `users.GetByUserId` unchecked | `internal/usercommands/party.go:290-426` |
| #244 | `Character.GetCharmIds()` copies `CharmedMobs`; `CompanionInfo{MobId, InstanceId, SourceType, Name, ...}`, `characters.CompanionCharmed` | `internal/characters/charminfo.go:42`, `companions.go:62` |
| #262 | `BuildCounterTauntMessages(counterer, countered *characters.Character, crit bool, damage, counteredMaxCP int) (countererMsg, taunterMsg, roomMsg string)` appends one `dmgTag` to both personal lines (284-292); the fallback `buildGenericCounterTauntMessages` does the same (296-307) | `internal/combat/counter.go:274-307` |
| #262 | `GetConvictionDamageDescription` bands: "a feeble jab at their resolve", "a stinging insult", "a rattling verbal assault", "a crushing blow to their confidence", "a devastating attack on their will", "a soul-shattering tirade"; "a mild rebuke" when max is 0 | `internal/combat/damage_pipeline.go:171-192` |
| #262 | Sibling, same bug: a taunt's `{damage}` is rendered once for all three audiences (`GetTauntTriad`, taunt_messages.go:146, `{damage}` at 171), so the taunted player reads "their resolve" too; `TauntResult.DmgDesc` (combat_taunt.go:57-58, set 282 and 355) feeds `sendTauntMessages` (usercommands/taunt.go:197), `sendMobTauntTriad` (mobcommands/taunt.go:129), and the target lines at `mobcommands/howl.go:54` and `mobcommands/taunt.go:71`. Shipped `rhetoric.yaml` actee lines carry `{damage}` (line 29 on) | as listed |
| #262 | `combat/context.md` documents neither `GetConvictionDamageDescription` nor `GetTauntTriad`; it is off-limits anyway | grep |
| #273 | Mob `Emote` sends `rest` (or the alias text) unchanged; the player strips a leading `@` with `if rest[0] == '@' && len(rest) > 1 { rest = rest[1:] }` | `internal/mobcommands/emote.go:15-40`, `internal/usercommands/emote.go:55-57` |
| #273 | Test room 1 holds users 1 and 2 (`room1.AddPlayer(1)`, `(2)`); mob instance 100 is the test mob; `mobSpeechHeard(uid)` drains a user's lines tag-stripped | `internal/mobcommands/mobcommands_test.go:217-218,280-287`, `speech_sight_test.go:26` |
| #277 | Mob `Eat` has no spoilage check; the player check is inline at 50-62 using `items.CalcEffectiveAgingSpeed(1.0, matchItem.CraftSkill)` and `items.GetAgingPhase` | `internal/mobcommands/eat.go:13-39`, `internal/usercommands/eat.go:50-62` |
| #277 | `actions.Drink` is the shared drink body (mob and player); there is no `actions.Eat`. No shipped food has `aging:` data today, so the check is dormant in play | `internal/actions/drink.go`, grep |
| #277 | `func (i *Item) GetSpec() ItemSpec`; `Item.CraftedRound uint64`, `Item.CraftSkill int`; `items.Food` type, `items.Edible` subtype | `internal/items/items.go:49-50,329`, `itemspec.go:135,154` |
| #308 | `give.template:3` and `show.template:3` open "The `look` command"; `sell.template:8` documents only `sell <item>`; `storage.template` lists `storage add [item/all]` and `storage remove [item/all]` only; `bank.template` has no See also | `_datafiles/world/dogmud/templates/help/{give,show,sell,storage,bank}.template` |
| #308 | `sell` forms: `sell <item>`, `sell N <item>`, `sell all <item>`, `sell all.<item>`; bare `sell all` is refused | `internal/usercommands/sell.go:28-73` |
| #308 | `storage add all` takes the backpack AND component bag; `add N <item>`, `add all <item>`; `remove all`, `remove all <item>`, `remove all.<item>`, `remove N <item>`; `unstore` is an alias of `storage remove` | `internal/usercommands/storage.go:77-89,98-128,214-299`, `keywords.yaml:354` |
| #308 | **Bug found:** `storage remove 3` never reaches the slot branch (257-266): the quantity parse (82) takes `3` as qty and leaves `itemName` empty, so the player reads `You don't have a  in storage.` | `internal/usercommands/storage.go:77-89,257` |
| #308 | `give` and `show` strip prepositions, so `give sword to sam` works | `internal/usercommands/give.go:22`, `show.go:19`, `internal/util/util.go:46-60` |
| #287 | Dashes: `flee.go:18` "You're locked in — there's nowhere to flee to.", `flee.go:20` "You can't break off to flee right now — you can only fight.", `main.go:1017` "Command dropped — AI rate limit (%d/round)...". `go.go` and `usercommands.go` literals are already clean. The guard `noDashCopyFiles` does not list `flee.go` or `main.go` | `internal/usercommands/flee.go:18,20`, `main.go:1017`, `copy_no_dash_test.go:17-35` |
| #346 | `EvaluateBuyRules` reads `configs.GetBalanceConfig().ShopGoldReserveRatio` (89-94); `PricingConfig` has `BuyRatio, PriceFloor, PriceCeiling, AbundanceThreshold, DefaultBaselineQty`; sibling reads at `sell_bauble.go:154-157` and `npc_buyers.go:175-181` (`reserveRatio()`, used at 246) | `internal/shops/buyrules.go:89-94`, `pricing.go:10-50`, `internal/actions/sell_bauble.go:153-160`, `modules/auctions/npc_buyers.go:174-181` |
| #346 | `pricing_balance_guard_test.go` fails any non-test file outside `pricing.go` that contains `DefaultPricingConfig(`, so `buyrules.go` cannot call it for a fallback | `internal/shops/pricing_balance_guard_test.go` |
| #346 | `configs.validate` already floors `ShopGoldReserveRatio` at 0.50 when <= 0; `shops/context.md:142-145` says it is read directly, not through `PricingConfig` | `internal/configs/config.balance.shops.go:28-30` |
| #266 | Halix's patrol lists `room: 507` (line 14); the forager profile repeats 507 in `VendorRooms` (only `VendorRooms[0]` is read, by `actions_forager.go:282`) | `patrols/ironwind_steppe/steppe_forager_delivery.yaml:14`, `internal/forager/territory.go:49` |
| #286 | `consider` resolves the target and calls `actions.Consider` with no harm check; `attack` refuses through `mobs.CheckPlayerHarm(m)`: companion "X is someone's companion!", non-combatant or attack-immune "You can't attack X." (`HideNames` at the attacker's sight) | `internal/usercommands/consider.go:15-56`, `attack.go:206-218`, `internal/mobs/harm_authorization.go:41-55` |
| #286 | `considerTestMob(instId, name, roomId)` fixture; `Mob.NonCombatant`, `Mob.PlayerAttackImmune` | `internal/usercommands/consider_multiword_test.go:15`, `internal/mobs/mobs.go:137,193` |
| #293 | Mob 284 has `itemdropchance: 25`, carries 20068 and 10026 under `character.items` (43-45) and equips the same two (46-50); carried items drop at 100%, equipped at `ItemDropChance` | `_datafiles/world/dogmud/mobs/north_road/284-bandit_fighter.yaml:7,43-50`, `internal/hooks/Death_MobLoot.go:37-58` |
| #339 | `GetBaseCastSuccessChance` (16-75, comment included) has no callers; its imports `math`, `configs`, `skills`, `spells` have no other use in the file | `internal/characters/spells.go:1-75` |
| #339 | Knob `SpellProficiencyCastsPerPoint`: field `config.balance.go:630`, default `config.balance.spells.go:48-50`, shipped `config.yaml:1935-1939` (two `#` lines, two comment lines, the key). No test counts or lists balance knobs (`lighting_config_surface_test.go` reads `Light*` keys only) | as listed |
| #339 | `_datafiles/config.yaml` in this worktree is the committed blob (`git ls-files -v` shows `H`, not `S`) and `git diff HEAD` is empty | git |
| #362 | `TestHelpFileCompleteness_Commands` ranges over `userCommands` only (82). Module commands: 13 `AddUserCommand` calls (aicompanion, auction, trash, follow, mudletmap, mudletui, checkclient, discord, leaderboard, ai-flag, ai-list, time, weather) and 7 direct `usercommands.RegisterCommand` calls in `aicompanion.go:585-591`. Help lookup reads module filesystems before the world root | `internal/usercommands/helpfile_completeness_test.go:76-117`, `modules/*/*.go`, `internal/templates/u8_help_test.go:289-317` |
| #362 | Missing help today: `companion-court`, `companion-boundary`, `companion-ask` (handlers `modules/aicompanion/commands.go:537,554,614`; texts `romance.go:394-460`). Module help lives in `modules/<m>/files/datafiles/templates/help/`, embedded by `//go:embed files/datafiles/*` | `modules/aicompanion/aicompanion.go:33` |
| #253 | `unicodeToAscii` covers light, double and mixed box drawing, blocks, weather and map glyphs, NBSP; nothing for heavy rules, ♥ ♦ ☠ ⚔ or dashes. `ConvertToAscii` runs per recipient at `connectiondetails.go:316` | `internal/util/util.go:1127-1193`, `internal/util/util_test.go:1187` |
| #253 | Heavy rules actually used in server text: only `━` (2614 uses: help section rules, `banner.go:39`). `┃ ┏` and the rest of the heavy family appear only in the vendored `xterm.4.19.0.js`. ♥ in `formattedname.go:26` and `party.go`, ♦ in `party.go`, ☠ in `formattedname.go:27,32`, ⚔ in `counter.go:189,258` and `hooks/combat_shared_helpers.go:341-345` | scan of `internal`, `modules`, `_datafiles`, root `*.go` |

---

### Task 1: #217 `get all <corpse>` takes from the corpse you named

**Model: sonnet**

**Files:**
- Modify: `internal/usercommands/get.go:187-215`
- Create: `internal/usercommands/get_all_named_corpse_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTwoCorpses lays an older Lookout corpse (holding an Iron Sword) and a
// newer Skeleton corpse (holding Chain Mail) in user 1's room at midsummer
// noon, so nothing is refused as blind.
func seedTwoCorpses(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)

	user, room := getTestUserAndRoom(t)
	origItems, origGold, origCorpses := user.Character.Items, user.Character.Gold, room.Corpses
	t.Cleanup(func() {
		user.Character.Items, user.Character.Gold, room.Corpses = origItems, origGold, origCorpses
	})
	user.Character.Items = nil

	lookout := rooms.Corpse{MobId: 1, Loot: rooms.Container{Items: []items.Item{items.New(10001)}}}
	lookout.Character.Name = "Lookout"
	skeleton := rooms.Corpse{MobId: 1, Loot: rooms.Container{Items: []items.Item{items.New(20001)}}}
	skeleton.Character.Name = "Skeleton"
	room.Corpses = []rooms.Corpse{lookout, skeleton} // appended on death: the skeleton is newest

	// The bug needs a bare "corpse" to mean the skeleton and the full name
	// to mean the lookout; without both the tests below prove nothing.
	require.Equal(t, 1, room.FindCorpseIndex("corpse", user.Character), "precondition: corpse means the newest")
	require.Equal(t, 0, room.FindCorpseIndex("lookout corpse", user.Character), "precondition: the name reaches the lookout")
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

// #217: `get all <corpse>` resolved the corpse from its last word, so
// `get all lookout corpse` swept whichever corpse was newest.
func TestGetAll_NamedCorpseTakesFromThatCorpse(t *testing.T) {
	user, room := seedTwoCorpses(t)
	handled, err := Get("all lookout corpse", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Empty(t, room.Corpses[0].Loot.Items, "the lookout is emptied")
	assert.Len(t, room.Corpses[1].Loot.Items, 1, "the skeleton is untouched")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, 10001, user.Character.Items[0].ItemId)
}

// The form from the issue: `get all from <corpse>`.
func TestGetAllFrom_NamedCorpseTakesFromThatCorpse(t *testing.T) {
	user, room := seedTwoCorpses(t)
	Get("all from lookout corpse", user, room, 0)
	assert.Empty(t, room.Corpses[0].Loot.Items, "the lookout is emptied")
	assert.Len(t, room.Corpses[1].Loot.Items, 1, "the skeleton is untouched")
}

// A bare `get all corpse` still means the newest corpse.
func TestGetAll_BareCorpseStillMeansTheNewest(t *testing.T) {
	user, room := seedTwoCorpses(t)
	Get("all corpse", user, room, 0)
	assert.Len(t, room.Corpses[0].Loot.Items, 1, "the lookout is untouched")
	assert.Empty(t, room.Corpses[1].Loot.Items, "the skeleton is emptied")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, 20001, user.Character.Items[0].ItemId)
}
```

- [ ] **Step 2: Run them and confirm the first two fail**

Run: `go test ./internal/usercommands/ -run 'TestGetAll(From)?_.*Corpse' -count=1`
Expected: `TestGetAll_NamedCorpseTakesFromThatCorpse` and `TestGetAllFrom_NamedCorpseTakesFromThatCorpse` FAIL (the skeleton is emptied); `TestGetAll_BareCorpseStillMeansTheNewest` PASSES. If a precondition `require` fails, the fixture is wrong, not the fix: read `FindCorpseIndex` and adjust the fixture names before going on.

- [ ] **Step 3: Resolve the corpse from the whole span**

In `get.go`, in the `// get all <corpse>` block, replace:

```go
		if len(args) >= 2 {
			if cIdx := room.FindCorpseIndex(args[len(args)-1], user.Character); cIdx >= 0 {
				corpse := &room.Corpses[cIdx]
```

with:

```go
		// The corpse is the whole trailing span, as `get <item> <corpse>`
		// resolves it (the parser split below), not the last word: that made
		// `get all lookout corpse` sweep whichever corpse was newest (#217).
		if len(args) >= 2 {
			if _, cm, ok := parser.SplitTrailingContainer(parser.Scope{User: user, Room: room}, rest); ok && cm.Kind == parser.KindCorpse {
				corpse := &room.Corpses[cm.CorpseIdx]
```

Leave the rest of the block as it is. `parser` is already imported. Also update the block's leading comment from `resolve via FindCorpseIndex` to `resolve via parser.SplitTrailingContainer (FindCorpseIndex underneath)`.

Note for the reviewer: with no `from`, a named corpse that is not there falls back to a shorter trailing span (`corpse` alone), exactly as the single-item path does. With `from`, only the span after it is tried.

- [ ] **Step 4: Run the tests and the get suites**

Run: `go test ./internal/usercommands/ -run 'TestGet|Corpse' -count=1`
Expected: PASS (including `TestGet_CorpseLoot` and the `get_corpse_sight_test.go` shapes tests, which use a bare `corpse`).

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/get.go internal/usercommands/get_all_named_corpse_test.go
git commit -m "fix(get): get all <corpse> takes from the corpse named, not the newest (#217)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: #244 `party list` shows a charmed companion once

**Model: sonnet**

**Files:**
- Modify: `internal/usercommands/party.go:290-426`
- Create: `internal/usercommands/party_list_companion_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #244: a charmed companion is in GetCharmIds AND in Companions, so party
// list showed it twice, once as ♥friend and once as ♦companion.
func TestPartyList_CharmedCompanionListedOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)

	const ownerId, compId, friendId = 9244, 9245, 9246
	u := users.NewTestUser(ownerId, "keeper", "Keeper", uint64(ownerId))
	u.Character.HealthMax.Value = 100
	u.Character.Companions = []characters.CompanionInfo{{
		MobId: 1, InstanceId: compId, Name: "Bandit Scout", SourceType: characters.CompanionCharmed,
	}}
	u.Character.CharmedMobs = []int{compId, friendId}

	mk := func(id int, name string) *mobs.Mob {
		m := &mobs.Mob{MobId: 1, InstanceId: id}
		m.Character.Name = name
		m.Character.RoomId = 1
		m.Character.Health = 50
		m.Character.HealthMax.Value = 100
		m.Character.Charm(ownerId, 100, "")
		return m
	}
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{
		compId:   mk(compId, "Bandit Scout"),
		friendId: mk(friendId, "Stray Hound"),
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{ownerId: u}))

	party := parties.New(ownerId)
	require.NotNil(t, party, "no party left over for this id")
	t.Cleanup(party.Disband)
	events.DrainQueuedMessagesForTest(ownerId)

	cmdPartyList(u, party)

	out := strings.Join(events.DrainQueuedMessagesForTest(ownerId), "\n")
	assert.Equal(t, 1, strings.Count(out, "Bandit Scout"), "a charmed companion is one row:\n%s", out)
	assert.Contains(t, out, "companion")
	assert.Equal(t, 1, strings.Count(out, "Stray Hound"), "a charmed creature that is no companion still shows:\n%s", out)
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestPartyList_CharmedCompanionListedOnce -count=1`
Expected: FAIL, `Bandit Scout` counted 2.

- [ ] **Step 3: Skip companions in the charm loop and nil-check every lookup**

In `cmdPartyList`:

1. Replace `charmedMobInstanceIds := []int{}` with:

```go
		charmedMobInstanceIds := []int{}
		// A charmed companion is in GetCharmIds too; it is listed once, with
		// the companions below, not also as a charmed friend (#244).
		companionInstanceIds := map[int]bool{}
```

2. In the member loop, directly after `u := users.GetByUserId(uid)`, add:

```go
			if u == nil {
				continue
			}
```

and directly after `uRoom := rooms.LoadRoom(u.Character.RoomId)` add:

```go
			if uRoom == nil {
				continue
			}
```

3. Replace `charmedMobInstanceIds = append(charmedMobInstanceIds, u.Character.GetCharmIds()...)` with:

```go
			charmedMobInstanceIds = append(charmedMobInstanceIds, u.Character.GetCharmIds()...)
			for _, comp := range u.Character.Companions {
				if comp.InstanceId != 0 {
					companionInstanceIds[comp.InstanceId] = true
				}
			}
```

4. In the charm loop, replace:

```go
		for _, mobInstanceId := range charmedMobInstanceIds {
			m := mobs.GetInstance(mobInstanceId)
			mRoom := rooms.LoadRoom(m.Character.RoomId)
```

with:

```go
		for _, mobInstanceId := range charmedMobInstanceIds {
			if companionInstanceIds[mobInstanceId] {
				continue
			}
			m := mobs.GetInstance(mobInstanceId)
			if m == nil {
				continue
			}
			mRoom := rooms.LoadRoom(m.Character.RoomId)
			if mRoom == nil {
				continue
			}
```

5. In the invite loop (`for _, uid := range currentParty.InviteUserIds {` inside `cmdPartyList`, not the one at line 550), directly after `u := users.GetByUserId(uid)`, add the same `if u == nil { continue }`.

- [ ] **Step 4: Run the party tests**

Run: `go test ./internal/usercommands/ -run 'Party' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/party.go internal/usercommands/party_list_companion_test.go
git commit -m "fix(party): party list shows a charmed companion once and skips missing lookups (#244)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: #262 The one who took conviction damage reads it about their own will

**Model: sonnet**

Design: one band function, two readers. `GetConvictionDamageDescription` keeps "their" for the one dealing the words and for onlookers; a new `GetConvictionDamageDescriptionToTarget` gives the same band with "your" for the one who took it. The counterer's line is unchanged. The spec names the retort; Part B fixes the same defect in ordinary taunts and howls (found while verifying: `GetTauntTriad` renders one `{damage}` into all three audiences), per "a flaw found mid-task widens scope". Part B can be dropped without touching Part A.

**Files:**
- Modify: `internal/combat/damage_pipeline.go:171-192`
- Modify: `internal/combat/counter.go:284-307`
- Modify: `internal/combat/counter_narration_test.go:92-94` (one assertion)
- Modify: `internal/combat/taunt_messages.go` (imports; add `WithDefenderDamage` after `GetTauntTriad`)
- Modify: `internal/actions/combat_taunt.go:57-58,282,348-359`
- Modify: `internal/usercommands/taunt.go:84-126,197-204`
- Modify: `internal/mobcommands/taunt.go:52,68-71,92,129-138`
- Modify: `internal/mobcommands/howl.go:54`
- Modify: `internal/mobcommands/taunt_store_test.go:62,115,156` (add one argument)
- Create: `internal/combat/conviction_damage_person_test.go`
- Create: `internal/mobcommands/taunt_target_person_test.go`

**Part A: the retort (the spec's fix)**

- [ ] **Step 1: Write the failing tests**

`internal/combat/conviction_damage_person_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #262: the bands say "their resolve" etc. The one who took the damage
// reads the same band about their own: "your resolve".
func TestGetConvictionDamageDescriptionToTarget_SecondPerson(t *testing.T) {
	for _, tc := range []struct {
		damage, max int
		want        string
	}{
		{2, 100, "a feeble jab at your resolve"},
		{10, 100, "a stinging insult"},
		{20, 100, "a rattling verbal assault"},
		{40, 100, "a crushing blow to your confidence"},
		{60, 100, "a devastating attack on your will"},
		{90, 100, "a soul-shattering tirade"},
		{10, 0, "a mild rebuke"},
	} {
		assert.Equal(t, tc.want, GetConvictionDamageDescriptionToTarget(tc.damage, tc.max), "%d of %d", tc.damage, tc.max)
	}
	// The onlooker form is unchanged.
	assert.Equal(t, "a devastating attack on their will", GetConvictionDamageDescription(60, 100))
}

// The retort: the counterer reads "their will", the countered "your will".
func TestBuildCounterTauntMessages_CounteredReadsYourWill(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(map[items.DefencePool]*items.DefenseMessageGroup{
		items.CounterPoolDefy: counterDefyMessageFixture(),
	})
	defer restore()
	counterer, countered := counterTauntFixtureChars("Selka", "Rurik")

	countererMsg, taunterMsg, _ := BuildCounterTauntMessages(counterer, countered, false, 120, 200)
	require.Contains(t, countererMsg, "a devastating attack on their will")
	require.Contains(t, taunterMsg, "a devastating attack on your will")
	require.NotContains(t, taunterMsg, "their will")
}

// The fallback lines when the counter-defy pool is not loaded.
func TestBuildCounterTauntMessages_FallbackCounteredReadsYourWill(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(nil)
	defer restore()
	counterer, countered := counterTauntFixtureChars("Selka", "Rurik")

	countererMsg, taunterMsg, _ := BuildCounterTauntMessages(counterer, countered, false, 120, 200)
	require.Contains(t, countererMsg, "a devastating attack on their will")
	require.Contains(t, taunterMsg, "a devastating attack on your will")
	require.NotContains(t, taunterMsg, "their will")
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/combat/ -run 'SecondPerson|CounteredReadsYourWill' -count=1`
Expected: FAIL to compile, `undefined: GetConvictionDamageDescriptionToTarget`.

- [ ] **Step 3: Split the band function**

In `damage_pipeline.go`, replace the whole `GetConvictionDamageDescription` function (comment line 171 to its closing brace) with:

```go
// GetConvictionDamageDescription converts conviction damage to descriptive
// text, as the one dealing it and any onlooker read it ("their resolve").
func GetConvictionDamageDescription(damageAmount int, targetMaxConviction int) string {
	return convictionDamageDescription(damageAmount, targetMaxConviction, "their")
}

// GetConvictionDamageDescriptionToTarget is the same band as the one who
// took the damage reads it on their own line: "your resolve", not "their
// resolve" (#262).
func GetConvictionDamageDescriptionToTarget(damageAmount int, targetMaxConviction int) string {
	return convictionDamageDescription(damageAmount, targetMaxConviction, "your")
}

// convictionDamageDescription is the one band table; whose is the
// possessive the reader sees ("their" or "your").
func convictionDamageDescription(damageAmount int, targetMaxConviction int, whose string) string {
	if targetMaxConviction <= 0 {
		return "a mild rebuke"
	}

	pct := float64(damageAmount) / float64(targetMaxConviction) * 100

	switch {
	case pct < 5:
		return "a feeble jab at " + whose + " resolve"
	case pct < 15:
		return "a stinging insult"
	case pct < 30:
		return "a rattling verbal assault"
	case pct < 50:
		return "a crushing blow to " + whose + " confidence"
	case pct < 75:
		return "a devastating attack on " + whose + " will"
	default:
		return "a soul-shattering tirade"
	}
}
```

- [ ] **Step 4: Give the countered player the second-person tag**

In `counter.go`, in `BuildCounterTauntMessages`, replace:

```go
	dmgTag := ""
	if damage > 0 {
		dmgTag = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetConvictionDamageDescription(damage, counteredMaxCP))
	}
	return retortPrefix + string(triad.ToDefender) + dmgTag,
		retortPrefix + string(triad.ToAttacker) + dmgTag,
		retortPrefix + string(triad.ToRoom)
```

with:

```go
	// The counterer reads the damage about the countered ("their will");
	// the countered reads it about their own ("your will", #262).
	dmgTag, dmgTagToCountered := "", ""
	if damage > 0 {
		dmgTag = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetConvictionDamageDescription(damage, counteredMaxCP))
		dmgTagToCountered = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetConvictionDamageDescriptionToTarget(damage, counteredMaxCP))
	}
	return retortPrefix + string(triad.ToDefender) + dmgTag,
		retortPrefix + string(triad.ToAttacker) + dmgTagToCountered,
		retortPrefix + string(triad.ToRoom)
```

In `buildGenericCounterTauntMessages`, replace:

```go
		dmgDesc := GetConvictionDamageDescription(damage, counteredMaxCP)
		return fmt.Sprintf(retortPrefix+`You throw %s's words right back in their face! (<ansi fg="damage">%s</ansi>)`, counteredName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws your words right back in your face! (<ansi fg="damage">%s</ansi>)`, countererName, dmgDesc),
```

with:

```go
		dmgDesc := GetConvictionDamageDescription(damage, counteredMaxCP)
		dmgDescToCountered := GetConvictionDamageDescriptionToTarget(damage, counteredMaxCP)
		return fmt.Sprintf(retortPrefix+`You throw %s's words right back in their face! (<ansi fg="damage">%s</ansi>)`, counteredName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws your words right back in your face! (<ansi fg="damage">%s</ansi>)`, countererName, dmgDescToCountered),
```

In `counter_narration_test.go`, in `TestBuildCounterTauntMessagesRendersFromCounterDefyPool`, replace `require.Contains(t, taunterMsg, dmgDesc)` with `require.Contains(t, taunterMsg, GetConvictionDamageDescriptionToTarget(tc.damage, 200))`.

- [ ] **Step 5: Run the combat tests**

Run: `go test ./internal/combat/ -count=1`
Expected: PASS

**Part B: taunts and howls (the same defect, widened)**

- [ ] **Step 6: Write the failing tests**

Append to `internal/combat/conviction_damage_person_test.go`:

```go
// A taunt renders {damage} once for all three audiences; the defender's
// line swaps in the second-person band (#262).
func TestTauntTriad_WithDefenderDamage(t *testing.T) {
	triad := TauntTriad{
		ToAttacker: "You sneer at Bob! (a feeble jab at their resolve)",
		ToDefender: "Ann sneers at you! (a feeble jab at their resolve)",
		ToRoom:     "Ann sneers at Bob!",
	}
	got := triad.WithDefenderDamage("a feeble jab at their resolve", "a feeble jab at your resolve")
	assert.Equal(t, "You sneer at Bob! (a feeble jab at their resolve)", got.ToAttacker)
	assert.Equal(t, "Ann sneers at you! (a feeble jab at your resolve)", got.ToDefender)
	assert.Equal(t, "Ann sneers at Bob!", got.ToRoom)
	assert.Equal(t, triad, triad.WithDefenderDamage("", ""), "no damage, no change")
}
```

`internal/mobcommands/taunt_target_person_test.go`:

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #262: a mob's taunt told its player target "a feeble jab at their
// resolve" about the player's own resolve.
func TestMobTauntTriad_TargetReadsYourResolve(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(combat.SeedTauntMessagesForTest(map[combat.TauntIntensity]*combat.TauntMessages{
		combat.TauntHit: {
			ToAttacker: []string{`You sneer at <ansi fg="{acteetype}">{actee}</ansi>! ({damage})`},
			ToDefender: []string{`<ansi fg="{actortype}">{actor}</ansi> sneers at you! ({damage})`},
			ToRoom:     []string{`<ansi fg="{actortype}">{actor}</ansi> sneers at <ansi fg="{acteetype}">{actee}</ansi>!`},
		},
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	target := users.GetByUserId(1)
	require.NotNil(t, target)
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	oldLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = oldLamp })
	events.DrainQueuedMessagesForTest(target.UserId)

	said := sendMobTauntTriad(combat.TauntHit,
		"a feeble jab at their resolve", "a feeble jab at your resolve",
		messaging.CategoryTauntSuccess, mob, target.Character.Name, target, room)
	require.True(t, said)

	lines := events.DrainQueuedMessagesForTest(target.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], "a feeble jab at your resolve")
	require.NotContains(t, lines[0], "their resolve")
}
```

- [ ] **Step 7: Run them and confirm they fail**

Run: `go test ./internal/combat/ -run TestTauntTriad_WithDefenderDamage -count=1` and `go test ./internal/mobcommands/ -run TestMobTauntTriad_TargetReadsYourResolve -count=1`
Expected: both FAIL to compile (`WithDefenderDamage` undefined; too many arguments to `sendMobTauntTriad`).

- [ ] **Step 8: Add `WithDefenderDamage`**

In `taunt_messages.go`, add `"strings"` to the imports, and add directly after `GetTauntTriad`:

```go
// WithDefenderDamage gives the defender's line the second-person damage
// description. A triad renders one {damage} for all three audiences, so the
// one taunted read "a feeble jab at their resolve" about their own (#262).
// rendered is what the triad was rendered with; toDefender replaces it in
// ToDefender only. Either one empty leaves the triad as it is.
func (t TauntTriad) WithDefenderDamage(rendered, toDefender string) TauntTriad {
	if rendered != "" && toDefender != "" {
		t.ToDefender = strings.Replace(t.ToDefender, rendered, toDefender, 1)
	}
	return t
}
```

- [ ] **Step 9: Carry the second-person band on the taunt result**

In `internal/actions/combat_taunt.go`:

After the `DmgDesc string` field (57-58), add:

```go

	// DmgDescToTarget is DmgDesc as the target reads it, about their own
	// resolve rather than "their" resolve (#262).
	DmgDescToTarget string
```

Replace `dmgDesc := combat.GetConvictionDamageDescription(dmg, convMaxRef)` with:

```go
	dmgDesc := combat.GetConvictionDamageDescription(dmg, convMaxRef)
	dmgDescToTarget := combat.GetConvictionDamageDescriptionToTarget(dmg, convMaxRef)
```

In the final `return TauntResult{...}` (348), add `DmgDescToTarget: dmgDescToTarget,` after `DmgDesc:     dmgDesc,` and run `gofmt -w internal/actions/combat_taunt.go`.

- [ ] **Step 10: Use it at every target line**

`internal/usercommands/taunt.go`:
- Change the signature to `func sendTauntMessages(intensity combat.TauntIntensity, dmgDesc, targetDmgDesc, source, target, sourceType, targetType string,` (second line unchanged).
- Replace `triad := combat.GetTauntTriad(intensity, source, target, sourceType, targetType, dmgDesc)` with:

```go
	triad := combat.GetTauntTriad(intensity, source, target, sourceType, targetType, dmgDesc).
		WithDefenderDamage(dmgDesc, targetDmgDesc)
```

- At the five call sites: the fumble (84) and miss (126) calls change `""` to `"", ""`; the three hit calls (88, 97, 117) change `result.DmgDesc,` to `result.DmgDesc, result.DmgDescToTarget,`.

`internal/mobcommands/taunt.go`:
- Change the signature to `func sendMobTauntTriad(intensity combat.TauntIntensity, dmgDesc, targetDmgDesc string, cat messaging.Category,` (second line unchanged).
- Replace the `GetTauntTriad` call with:

```go
	triad := combat.GetTauntTriad(intensity, mob.Character.Name, targetName,
		"mobname", targetType, dmgDesc).WithDefenderDamage(dmgDesc, targetDmgDesc)
```

- Call sites: fumble (52) and miss (92) change `""` to `"", ""`; the hit call (68) changes `result.DmgDesc,` to `result.DmgDesc, result.DmgDescToTarget,`. In the fallback personal line (71), change `result.DmgDesc` to `result.DmgDescToTarget`.

`internal/mobcommands/howl.go:54`: change `result.DmgDesc` to `result.DmgDescToTarget` (the personal line to the howl's target).

`internal/mobcommands/taunt_store_test.go`: at the three `sendMobTauntTriad(` calls (62, 115, 156), add `"",` after the second argument.

- [ ] **Step 11: Build and run every affected package**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/combat/ ./internal/actions/ ./internal/mobcommands/ -count=1 && go test ./internal/usercommands/ -run 'Taunt|Howl' -count=1 && go test ./internal/narration/ -count=1`
Expected: no gofmt output; PASS

- [ ] **Step 12: Commit**

```bash
git add internal/combat/damage_pipeline.go internal/combat/counter.go internal/combat/counter_narration_test.go internal/combat/taunt_messages.go internal/combat/conviction_damage_person_test.go internal/actions/combat_taunt.go internal/usercommands/taunt.go internal/mobcommands/taunt.go internal/mobcommands/howl.go internal/mobcommands/taunt_store_test.go internal/mobcommands/taunt_target_person_test.go
git commit -m "fix(combat): the one who takes conviction damage reads it about their own will (#262)" -m "The retort's countered line, and the taunted or howled-at player's line, now read \"your resolve\" where the dealer and onlookers keep \"their\"." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

`combat/context.md` and `actions/context.md` are off-limits on this branch and document neither function nor field today; note in the PR that they gain a line when M6 lands.

---

### Task 4: #273 Mob `emote` strips a leading `@`

**Model: haiku**

**Files:**
- Modify: `internal/mobcommands/emote.go:31-35`
- Create: `internal/mobcommands/emote_at_test.go`

- [ ] **Step 1: Write the failing test**

```go
package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #273: the player's `emote @<text>` strips the @; a mob's emote passed it
// through, so watchers read "Skeleton @shrugs."
func TestMobEmote_LeadingAtIsStripped(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	oldSky, oldLamp := room.SkyLight, room.Lamp
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	t.Cleanup(func() { room.SkyLight, room.Lamp = oldSky, oldLamp })
	require.GreaterOrEqual(t, room.PlayerCt(), 1, "Emote sends nothing to an empty room")
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)

	handled, err := Emote("@shrugs.", mob, room)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(mobSpeechHeard(1), "\n")
	require.Contains(t, out, "shrugs.", "the watcher reads the emote")
	assert.NotContains(t, out, "@")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/mobcommands/ -run TestMobEmote_LeadingAtIsStripped -count=1`
Expected: FAIL, output contains `@`.

- [ ] **Step 3: Strip it**

In `emote.go`, replace:

```go
	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	}
```

with:

```go
	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	} else if rest[0] == '@' && len(rest) > 1 {
		// The @ form, stripped as the player command strips it (#273).
		emoteText = rest[1:]
	}
```

- [ ] **Step 4: Run the mob tests**

Run: `go test ./internal/mobcommands/ -run 'Emote|Speech' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mobcommands/emote.go internal/mobcommands/emote_at_test.go
git commit -m "fix(mobs): mob emote strips a leading @ like the player command (#273)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: #277 One spoilage check, and a mob leaves spoiled food

**Model: sonnet**

There is no `actions.Eat` to extend, and building one is out of proportion: the shared piece is the spoilage question, so it moves onto `items.Item` next to the aging code it reads.

**Files:**
- Modify: `internal/items/aging.go` (add `IsSpoiledFood`)
- Modify: `internal/items/context.md` (Aging section)
- Modify: `internal/usercommands/eat.go:50-62`
- Modify: `internal/mobcommands/eat.go` (imports; after the Edible check)
- Create: `internal/items/spoiled_food_test.go`
- Create: `internal/mobcommands/eat_spoiled_test.go`
- Create: `internal/usercommands/eat_spoiled_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/items/spoiled_food_test.go`:

```go
package items

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #277: one spoilage check, shared by player and mob eat.
func TestIsSpoiledFood(t *testing.T) {
	aging := AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}
	food := Item{ItemId: 999951, Spec: &ItemSpec{ItemId: 999951, Type: Food, Subtype: Edible, Aging: aging}, CraftedRound: 100}

	assert.False(t, food.IsSpoiledFood(120), "inside its life")
	assert.True(t, food.IsSpoiledFood(140), "at its spoil point")

	unknownAge := food
	unknownAge.CraftedRound = 0
	assert.False(t, unknownAge.IsSpoiledFood(100000), "no craft round: never spoils")

	plain := Item{ItemId: 999952, Spec: &ItemSpec{ItemId: 999952, Type: Food, Subtype: Edible}, CraftedRound: 1}
	assert.False(t, plain.IsSpoiledFood(100000), "no aging data: never spoils")
}
```

`internal/mobcommands/eat_spoiled_test.go`:

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mobLoafItemId = 999961

// seedMobLoaf gives mob 100 one loaf made at craftedRound, at round 1000.
func seedMobLoaf(t *testing.T, craftedRound uint64) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		mobLoafItemId: {ItemId: mobLoafItemId, Name: "Stale Loaf", NameSimple: "loaf", Type: items.Food,
			Subtype: items.Edible, Uses: 1,
			Aging: items.AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}},
	}))
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	mob, room := getTestMobAndRoom(t)
	orig := mob.Character.Items
	t.Cleanup(func() { mob.Character.Items = orig })
	loaf := items.New(mobLoafItemId)
	loaf.CraftedRound = craftedRound
	mob.Character.Items = []items.Item{loaf}
	return mob, room
}

// #277: a mob ate spoiled food and took its conditions; a player is refused.
func TestMobEat_SpoiledFoodIsLeft(t *testing.T) {
	mob, room := seedMobLoaf(t, 1)
	handled, err := Eat("loaf", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Len(t, mob.Character.Items, 1, "a mob does not eat food that has gone bad")
}

func TestMobEat_FreshFoodIsEaten(t *testing.T) {
	mob, room := seedMobLoaf(t, 995)
	Eat("loaf", mob, room)
	assert.Empty(t, mob.Character.Items, "fresh food is eaten")
}
```

`internal/usercommands/eat_spoiled_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

const playerLoafItemId = 999962

// #277: the player's refusal keeps its words now the check is shared.
func TestEat_SpoiledFoodIsRefused(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		playerLoafItemId: {ItemId: playerLoafItemId, Name: "Stale Loaf", NameSimple: "loaf", Type: items.Food,
			Subtype: items.Edible, Uses: 1,
			Aging: items.AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}},
	}))
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	user, room := getTestUserAndRoom(t)
	orig := user.Character.Items
	t.Cleanup(func() { user.Character.Items = orig })
	loaf := items.New(playerLoafItemId)
	loaf.CraftedRound = 1
	user.Character.Items = []items.Item{loaf}
	events.DrainQueuedMessagesForTest(user.UserId)

	Eat("loaf", user, room, 0)

	assert.Contains(t, sentTo(user), "The food has gone bad!")
	assert.Len(t, user.Character.Items, 1, "the loaf is not eaten")
}
```

- [ ] **Step 2: Run them and confirm the new ones fail**

Run: `go test ./internal/items/ -run TestIsSpoiledFood -count=1`, `go test ./internal/mobcommands/ -run 'TestMobEat_' -count=1`, `go test ./internal/usercommands/ -run TestEat_SpoiledFoodIsRefused -count=1`
Expected: items FAILS to compile (`IsSpoiledFood` undefined); `TestMobEat_SpoiledFoodIsLeft` FAILS (the loaf is eaten), `TestMobEat_FreshFoodIsEaten` PASSES; `TestEat_SpoiledFoodIsRefused` PASSES (it locks the player wording across the move).

- [ ] **Step 3: Add the shared check**

At the end of `internal/items/aging.go`:

```go
// IsSpoiledFood reports whether this food has gone bad at round now: its
// spec has aging thresholds, it was made at a known round, and it is past
// its spoil point. Food has no bottle, so only the cook's skill slows it.
// The one check player and mob eat both ask (#277).
func (i *Item) IsSpoiledFood(now uint64) bool {
	spec := i.GetSpec()
	if !spec.Aging.HasAging() || i.CraftedRound == 0 {
		return false
	}
	var elapsed uint64
	if now >= i.CraftedRound {
		elapsed = now - i.CraftedRound
	}
	phase, _ := GetAgingPhase(elapsed, spec.Aging, CalcEffectiveAgingSpeed(1.0, i.CraftSkill))
	return phase == PhaseSpoiled
}
```

- [ ] **Step 4: Use it on both sides**

In `internal/usercommands/eat.go`, replace the block from `// Check if food has spoiled` through its closing `}` (the `if itemSpec.Aging.HasAging() && matchItem.CraftedRound > 0 { ... }` block) with:

```go
		// Food that has gone bad is refused; mob eat asks the same (#277).
		if matchItem.IsSpoiledFood(util.GetRoundCount()) {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi>`)
			return true, nil
		}
```

In `internal/mobcommands/eat.go`, add `"github.com/GoMudEngine/GoMud/internal/util"` to the imports, and directly after the `if itemSpec.Subtype != items.Edible { return true, nil }` block add:

```go

		// A mob leaves food that has gone bad, as a player is refused it.
		// No message: nobody asked it why (#277).
		if matchItem.IsSpoiledFood(util.GetRoundCount()) {
			return true, nil
		}
```

- [ ] **Step 5: Document it**

In `internal/items/context.md`, in the `### Aging` section, add a line inside the code block after `func CalcEffectiveAgingSpeed(...)`:

```go
func (i *Item) IsSpoiledFood(now uint64) bool
```

and after the paragraph that starts "`CalcEffectiveAgingSpeed` =" add:

```
`IsSpoiledFood` is the eat-side question: an item whose spec ages, made at a
known `CraftedRound`, past `PhaseSpoiled` at bottle speed 1.0. Player and mob
`eat` both ask it; no shipped food carries `aging:` yet.
```

Run `python tools/context_md_audit.py` and confirm no finding for `internal/items`.

- [ ] **Step 6: Run the three packages**

Run: `go build ./... && go test ./internal/items/ ./internal/mobcommands/ -count=1 && go test ./internal/usercommands/ -run 'Eat' -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/items/aging.go internal/items/context.md internal/items/spoiled_food_test.go internal/usercommands/eat.go internal/usercommands/eat_spoiled_test.go internal/mobcommands/eat.go internal/mobcommands/eat_spoiled_test.go
git commit -m "fix(eat): one spoilage check; a mob leaves food that has gone bad (#277)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: #308 give, show, sell and storage help, and `storage remove <slot>`

**Model: sonnet**

The storage help documents `storage remove <slot#>`, which does not work today (see the facts table): it is fixed here so the help does not promise a form that fails.

**Files:**
- Modify: `internal/usercommands/storage.go:82`
- Create: `internal/usercommands/storage_remove_slot_test.go`
- Modify: `_datafiles/world/dogmud/templates/help/give.template`
- Modify: `_datafiles/world/dogmud/templates/help/show.template`
- Modify: `_datafiles/world/dogmud/templates/help/sell.template`
- Modify: `_datafiles/world/dogmud/templates/help/storage.template`
- Modify: `_datafiles/world/dogmud/templates/help/bank.template`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #308: `storage remove 2` read the 2 as a quantity with no item name and
// answered "You don't have a  in storage." It takes slot 2.
func TestStorageRemove_BySlotNumber(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	oldIsStorage := room.IsStorage
	room.IsStorage = true
	origStorage, origItems := user.ItemStorage, user.Character.Items
	origStrength := user.Character.Stats.Strength.ValueAdj
	t.Cleanup(func() {
		room.IsStorage = oldIsStorage
		user.ItemStorage, user.Character.Items = origStorage, origItems
		user.Character.Stats.Strength.ValueAdj = origStrength
	})
	user.ItemStorage = users.Storage{}
	user.Character.Items = nil
	user.Character.Stats.Strength.ValueAdj = 50
	require.True(t, user.ItemStorage.AddItem(items.New(10001)))
	require.True(t, user.ItemStorage.AddItem(items.New(20001)))
	want := user.ItemStorage.GetSlots()[1].Item.ItemId
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Storage("remove 2", user, room, 0)
	require.NoError(t, err)

	assert.NotContains(t, sentTo(user), "You don't have a")
	require.Len(t, user.Character.Items, 1, "slot 2 comes out")
	assert.Equal(t, want, user.Character.Items[0].ItemId)
	assert.Equal(t, 1, user.ItemStorage.SlotCount())
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestStorageRemove_BySlotNumber -count=1`
Expected: FAIL, output contains `You don't have a`.

- [ ] **Step 3: A lone number is a slot, not a quantity**

In `storage.go`, replace:

```go
		} else if n, err := strconv.Atoi(remaining[0]); err == nil && n > 0 {
```

with:

```go
		} else if n, err := strconv.Atoi(remaining[0]); err == nil && n > 0 && len(remaining) > 1 {
			// A number is a quantity only when a name follows. A lone
			// number is a slot: "storage remove 3" (#308).
```

Run: `go test ./internal/usercommands/ -run 'Storage|Stolen' -count=1`
Expected: PASS (the stolen-bauble storage test covers `add 2 thimble`).

- [ ] **Step 4: `give.template`**

Replace the whole file with:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">give</ansi>

The <ansi fg="command">give</ansi> command gives an object to another player or mob.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">give sword sam</ansi>
  This gives your sword to sam. <ansi fg="command">give sword to sam</ansi> works too.

  <ansi fg="command">give 30 gold sam</ansi>
  This gives 30 gold to sam.

Giving a stolen trinket back to whoever it was taken from
returns it. Its owner will be glad of it, and a thief who
has been caught stealing wins back a little of the goodwill
that cost them. See <ansi fg="command">help steal</ansi>.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help show</ansi>, <ansi fg="command">help sell</ansi>, <ansi fg="command">help storage</ansi>
```

- [ ] **Step 5: `show.template`**

Replace the whole file with:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">show</ansi>

The <ansi fg="command">show</ansi> command shows an object to another player or mob.
They get a look at it, and you keep it.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">show sword sam</ansi>
  This shows your sword to sam.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help give</ansi>
```

- [ ] **Step 6: `sell.template`**

Replace:

```
  <ansi fg="command">sell <item></ansi>     Sell an item to a merchant.
```

with:

```
  <ansi fg="command">sell <item></ansi>          Sell one item to a merchant.
  <ansi fg="command">sell 5 <item></ansi>        Sell up to five of that item.
  <ansi fg="command">sell all <item></ansi>      Sell every one of that item you carry.
  <ansi fg="command">sell all.<item></ansi>      The same as sell all <item>.

If the merchant runs short of gold partway through, you are
told how many sold.
```

and replace the last line:

```
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help buy</ansi>, <ansi fg="command">help appraise</ansi>, <ansi fg="command">help list</ansi>
```

with:

```
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help buy</ansi>, <ansi fg="command">help appraise</ansi>, <ansi fg="command">help list</ansi>,
<ansi fg="command">help give</ansi>, <ansi fg="command">help storage</ansi>
```

- [ ] **Step 7: `storage.template`**

Replace the block from `<ansi fg="yellow">Usage: </ansi>` up to (not including) `<ansi fg="yellow">Locations: </ansi>` with:

```
<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">storage</ansi>
  See what you have in storage. Each slot is numbered.

  <ansi fg="command">storage add <item></ansi>
  Put one item into storage.

  <ansi fg="command">storage add 5 <item></ansi>
  Put up to five of that item into storage.

  <ansi fg="command">storage add all <item></ansi>
  Put every one of that item you carry into storage.

  <ansi fg="command">storage add all</ansi>
  Put everything in your backpack and your component bag into
  storage.

  <ansi fg="command">storage remove <item></ansi>
  Take one item out of storage. <ansi fg="command">unstore <item></ansi> does the same.

  <ansi fg="command">storage remove 5 <item></ansi>
  Take up to five of that item out of storage.

  <ansi fg="command">storage remove all <item></ansi>
  Take every one of that item out of storage. You can also type
  <ansi fg="command">storage remove all.<item></ansi>.

  <ansi fg="command">storage remove 3</ansi>
  Take out whatever is in slot 3 of your storage list.

  <ansi fg="command">storage remove all</ansi>
  Take everything out of storage.

When you add an item by name, your backpack is searched first,
then your component bag.

```

and after the last line (`the last three days. See <ansi fg="command">help steal</ansi>.`) add:

```

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help stow</ansi>, <ansi fg="command">help stash</ansi>, <ansi fg="command">help sort</ansi>, <ansi fg="command">help component-bag</ansi>,
<ansi fg="command">help bank</ansi>, <ansi fg="command">help give</ansi>, <ansi fg="command">help sell</ansi>
```

- [ ] **Step 8: `bank.template`**

After the last line (`cover your storage costs.`) add:

```

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help storage</ansi>, <ansi fg="command">help stow</ansi>
```

- [ ] **Step 9: Check width, dashes and the help tests**

Run: `for f in give show sell storage bank; do sed 's/<[^>]*>//g' _datafiles/world/dogmud/templates/help/$f.template | tr -d '\r' | awk -v f=$f 'length($0) > 80 { print f": "length($0)": "$0 }'; done`
Expected: no output.

Run: `grep -n "[—–]" _datafiles/world/dogmud/templates/help/{give,show,sell,storage,bank}.template`
Expected: no output (exit 1 is the pass; run it on its own, not in an `&&` chain).

Run: `go test ./internal/usercommands/ -run 'Help' -count=1 && go test ./internal/templates/ -count=1`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/usercommands/storage.go internal/usercommands/storage_remove_slot_test.go _datafiles/world/dogmud/templates/help/give.template _datafiles/world/dogmud/templates/help/show.template _datafiles/world/dogmud/templates/help/sell.template _datafiles/world/dogmud/templates/help/storage.template _datafiles/world/dogmud/templates/help/bank.template
git commit -m "fix(help): give, show, sell and storage help; storage remove <slot> works (#308)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: #287 Flee and AI rate-limit lines lose their dashes

**Model: haiku**

**Files:**
- Modify: `copy_no_dash_test.go:17-35`
- Modify: `internal/usercommands/flee.go:18,20`
- Modify: `main.go:1017`

- [ ] **Step 1: Widen the guard first**

In `copy_no_dash_test.go`, after `"internal/combat/combat_helpers.go",` add:

```go
	// #287, the flee refusals and the AI rate-limit notice.
	"internal/usercommands/flee.go",
	"main.go",
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test . -run TestCopyFilesHaveNoDashesInStringLiterals -count=1`
Expected: FAIL naming `flee.go:18`, `flee.go:20` and `main.go:1017`, and nothing else. If it names any other literal, fix that one the same way and list it in the commit message.

- [ ] **Step 3: Replace the dashes**

`flee.go:18`: `` `You're locked in — there's nowhere to flee to.` `` becomes `` `You're locked in, with nowhere to flee to.` ``

`flee.go:20`: `` `You can't break off to flee right now — you can only fight.` `` becomes `` `You can't break off to flee right now. You can only fight.` ``

`main.go:1017`: `"Command dropped — AI rate limit (%d/round). Wait for the next round.\r\n"` becomes `"Command dropped: AI rate limit (%d/round). Wait for the next round.\r\n"`

- [ ] **Step 4: Run the guard and the flee tests**

Run: `go test . -run TestCopyFilesHaveNoDashesInStringLiterals -count=1 && go test ./internal/usercommands/ -run 'Flee' -count=1`
Expected: PASS. A flee test asserting the old text is updated to the new text.

- [ ] **Step 5: Commit**

```bash
git add copy_no_dash_test.go internal/usercommands/flee.go main.go
git commit -m "fix(copy): flee refusals and the AI rate-limit notice lose their dashes (#287)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(Add any flee test file Step 4 updated.)

---

### Task 8: #346 The gold reserve rides on `PricingConfig`

**Model: sonnet**

**Files:**
- Modify: `internal/shops/pricing.go:9-50`
- Modify: `internal/shops/buyrules.go:3-9,17-34,88-94`
- Modify: `internal/actions/sell_bauble.go:153-157`
- Modify: `modules/auctions/npc_buyers.go:174-181`
- Modify: `internal/shops/context.md:142-145`
- Create: `internal/shops/buyrules_reserve_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package shops

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #346: the reserve gate read the global config, so a test could not pin it
// through cfg. It now reads cfg.GoldReserveRatio.
func TestBuyRules_GoldReserveComesFromPricingConfig(t *testing.T) {
	item := makeItem(items.ItemSpec{
		ItemId:           100,
		Value:            200,
		Type:             items.Object,
		VendorCategories: []string{"alchemy"},
	})
	shop := baseShop() // 1000 gold, started with 1000
	shop.CraftSupport = CraftSupportAlchemy
	cfg := DefaultPricingConfig()

	// Unstocked, so the flat price: 200 x 0.5 = 100. Keeping half back
	// leaves 500 to spend.
	assert.Equal(t, 100, EvaluateBuyRules(item, shop, "", false, cfg, nil).Price)

	// Keeping 95% back leaves 50 to spend: the same offer is refused.
	cfg.GoldReserveRatio = 0.95
	assert.Equal(t, 0, EvaluateBuyRules(item, shop, "", false, cfg, nil).Price)
}

func TestPricingConfigFromBalance_CarriesGoldReserveRatio(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.ShopGoldReserveRatio = 0.8
	configs.SetConfigForTest(t, c)
	assert.Equal(t, 0.8, PricingConfigFromBalance().GoldReserveRatio)
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/shops/ -run 'GoldReserve' -count=1`
Expected: FAIL to compile, `cfg.GoldReserveRatio undefined`.

- [ ] **Step 3: Add the field**

In `pricing.go`:

1. Add `GoldReserveRatio   float64 // Share of a shop's starting gold kept back before it buys (default 0.50)` as the last field of `PricingConfig`.
2. Above `// PricingConfigFromBalance creates ...` add:

```go
// defaultGoldReserveRatio is DefaultPricingConfig's reserve, named so
// EvaluateBuyRules can fall back to it for a hand-built cfg without calling
// DefaultPricingConfig (pricing_balance_guard_test.go forbids that).
const defaultGoldReserveRatio = 0.50
```

3. In `PricingConfigFromBalance`, before `return cfg`, add:

```go
	if float64(b.ShopGoldReserveRatio) > 0 {
		cfg.GoldReserveRatio = float64(b.ShopGoldReserveRatio)
	}
```

4. In `DefaultPricingConfig`, add `GoldReserveRatio:   defaultGoldReserveRatio,` as the last field.

Run `gofmt -w internal/shops/pricing.go`.

- [ ] **Step 4: Read it in `EvaluateBuyRules`**

In `buyrules.go`, replace:

```go
	// Gold-reserve gate.
	b := configs.GetBalanceConfig()
	reserveRatio := float64(b.ShopGoldReserveRatio)
	if reserveRatio <= 0 {
		reserveRatio = 0.50 // fallback default
	}
```

with:

```go
	// Gold-reserve gate. The ratio comes in on cfg like every other pricing
	// knob (#346); a hand-built cfg that leaves it unset gets the default.
	reserveRatio := cfg.GoldReserveRatio
	if reserveRatio <= 0 {
		reserveRatio = defaultGoldReserveRatio
	}
```

Remove `"github.com/GoMudEngine/GoMud/internal/configs"` from the imports (it was the only use). In the doc comment, replace the three lines of item 6:

```go
//  6. Vendor can't afford the buy price without dropping below
//     shopInv.GoldReserve(BalanceConfig.ShopGoldReserveRatio) — defaults
//     to 0.50 when the config knob is unset.
```

with:

```go
//  6. Vendor can't afford the buy price without dropping below
//     shopInv.GoldReserve(cfg.GoldReserveRatio), the ShopGoldReserveRatio
//     knob carried on PricingConfig (0.50 when a cfg leaves it unset).
```

- [ ] **Step 5: Route the two sibling reads through the same value**

`internal/actions/sell_bauble.go`, replace:

```go
		ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
		if ratio <= 0 {
			ratio = 0.50
		}
		if !shopInv.CanAfford(price, shopInv.GoldReserve(ratio)) {
```

with:

```go
		// The reserve EvaluateBuyRules reads, from the same PricingConfig (#346).
		ratio := shops.PricingConfigFromBalance().GoldReserveRatio
		if !shopInv.CanAfford(price, shopInv.GoldReserve(ratio)) {
```

`modules/auctions/npc_buyers.go`, replace:

```go
// reserveRatio returns the shop gold-reserve fraction from config (0.50 fallback).
func reserveRatio() float64 {
	r := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
	if r <= 0 {
		r = 0.50
	}
	return r
}
```

with:

```go
// reserveRatio returns the shop gold-reserve fraction, the PricingConfig
// value shops.EvaluateBuyRules reads (#346).
func reserveRatio() float64 {
	return shops.PricingConfigFromBalance().GoldReserveRatio
}
```

Both files keep their `configs` import (other uses remain); `go build` will say if not.

- [ ] **Step 6: Update the shops context**

In `internal/shops/context.md`, replace the `ShopGoldReserveRatio` bullet (142-145) with:

```
- **`ShopGoldReserveRatio`**: shipped `0.50`, Go default `0.50`. Carried as
  `PricingConfig.GoldReserveRatio` (filled by `PricingConfigFromBalance`) and
  read from there by `EvaluateBuyRules`, by the bauble offer in
  `internal/actions/sell_bauble.go` and by `modules/auctions/npc_buyers.go`.
  Fraction of a shop's starting gold held back before it will buy from a
  seller.
```

Run `python tools/context_md_audit.py` and confirm no finding for `internal/shops`.

- [ ] **Step 7: Build and test every reader**

Run: `go build ./... && go test ./internal/shops/ ./modules/auctions/ -count=1 && go test ./internal/actions/ -run 'Sell|Bauble|Shop' -count=1`
Expected: PASS (including `TestDefaultPricingConfig_OnlyCalledByPricingConfigFromBalance`).

- [ ] **Step 8: Commit**

```bash
git add internal/shops/pricing.go internal/shops/buyrules.go internal/shops/buyrules_reserve_test.go internal/shops/context.md internal/actions/sell_bauble.go modules/auctions/npc_buyers.go
git commit -m "refactor(shops): the gold reserve ratio rides on PricingConfig (#346)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: #266 Halix stops trying to reach room 507

**Model: haiku**

Owner call 2026-10-10: drop the waypoint. The forager profile lists the same room as a vendor stop, so it goes there too, or the two lists disagree.

**Files:**
- Modify: `_datafiles/world/dogmud/patrols/ironwind_steppe/steppe_forager_delivery.yaml:14`
- Modify: `internal/forager/territory.go:49`

- [ ] **Step 1: Drop the waypoint**

In the patrol file, delete exactly this line:

```yaml
  - { room: 507, dwell_rounds: 3, arrival_event: forager_vendor }
```

- [ ] **Step 2: Drop it from the profile**

In `territory.go:49`, change `VendorRooms:      []int{464, 470, 471, 475, 480, 481, 482, 483, 507},` to `VendorRooms:      []int{464, 470, 471, 475, 480, 481, 482, 483},`.

- [ ] **Step 3: Test**

Run: `go build ./... && go test ./internal/forager/ -count=1 && go test ./internal/behaviortree/ -run 'Forager' -count=1`
Expected: PASS. The isolated boot in Task 15 confirms the patrol still loads.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/patrols/ironwind_steppe/steppe_forager_delivery.yaml internal/forager/territory.go
git commit -m "fix(content): Halix no longer patrols to the locked listening post (#266)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: #286 `consider` on a mob you may not fight says so

**Model: sonnet**

Owner call 2026-10-10: "You can't fight X." The check is `mobs.CheckPlayerHarm`, the one `attack` uses; any blocked reason (companion, non-combatant, attack-immune) gets the same line.

**Files:**
- Modify: `internal/usercommands/consider.go:3-13,44-45`
- Create: `internal/usercommands/consider_protected_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #286: consider rated Drillmaster Vorn "An even contest" while attack
// refused him. Owner call 2026-10-10: say "You can't fight X." instead.
func TestConsider_ProtectedMobSaysYouCantFightIt(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	oldLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = oldLamp })

	const vornId, championId, ruffianId = 786, 787, 788
	vorn := considerTestMob(vornId, "Drillmaster Vorn", room.RoomId)
	vorn.NonCombatant = true
	champion := considerTestMob(championId, "Arena Champion", room.RoomId)
	champion.PlayerAttackImmune = true
	ruffian := considerTestMob(ruffianId, "Dock Ruffian", room.RoomId)
	for id, m := range map[int]*mobs.Mob{vornId: vorn, championId: champion, ruffianId: ruffian} {
		mobs.SetInstanceForTest(id, m)
		room.AddMob(id)
		t.Cleanup(func() {
			room.RemoveMob(id)
			mobs.SetInstanceForTest(id, nil)
		})
	}

	for _, tc := range []struct{ arg, name string }{
		{"drillmaster vorn", "Drillmaster Vorn"},
		{"arena champion", "Arena Champion"},
	} {
		events.DrainQueuedMessagesForTest(user.UserId)
		handled, err := Consider(tc.arg, user, room, 0)
		require.True(t, handled)
		require.NoError(t, err)
		out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
		assert.Contains(t, out, "You can't fight", tc.arg)
		assert.Contains(t, out, tc.name)
		assert.NotContains(t, out, "Your instincts tell you", tc.arg)
	}

	// A mob you may fight is still weighed up.
	events.DrainQueuedMessagesForTest(user.UserId)
	Consider("dock ruffian", user, room, 0)
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), "Your instincts tell you")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestConsider_ProtectedMobSaysYouCantFightIt -count=1`
Expected: FAIL, the protected mobs get "Your instincts tell you".

- [ ] **Step 3: Refuse before rating**

In `consider.go`, add `"fmt"` and `"github.com/GoMudEngine/GoMud/internal/mobs"` to the imports, and replace:

```go
	actor := actions.UserActorAtSight(user, room)
	actions.Consider(actor, target)
```

with:

```go
	// A mob the player may not harm is not a fight to weigh up: attack
	// refuses it through the same check, so consider says so instead of
	// rating the odds (#286). The quest notification below still fires.
	if m := mobs.GetInstance(target.GetMobInstanceId()); mobs.CheckPlayerHarm(m).Blocked() {
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You can't fight <ansi fg="mobname">%s</ansi>.`, m.Character.Name),
			[]string{m.Character.Name}, messaging.ParticipantSight(user.Character, room)))
	} else {
		actor := actions.UserActorAtSight(user, room)
		actions.Consider(actor, target)
	}
```

(`mobs.GetInstance(0)` is nil for a player target, and `CheckPlayerHarm(nil)` allows, so players are rated as before.)

- [ ] **Step 4: Run the consider tests and the root guards that scan player lines**

Run: `go test ./internal/usercommands/ -run 'Consider' -count=1 && go test . -run 'Guard|Sight|Messaging|Lookup' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/consider.go internal/usercommands/consider_protected_test.go
git commit -m "fix(consider): a mob you may not fight says so instead of rating the odds (#286)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: #293 Bandit fighter drops no spare sword and vest

**Model: haiku**

Owner call 2026-10-10: drop the spare copies. The equipped longsword and vest still drop at the mob's 25% `itemdropchance`.

**Files:**
- Modify: `_datafiles/world/dogmud/mobs/north_road/284-bandit_fighter.yaml:43-45`

- [ ] **Step 1: Delete the carried list**

Delete exactly these three lines (43-45), leaving `equipment:` directly after `gold: 30`:

```yaml
  items:
    - itemid: 20068
    - itemid: 10026
```

- [ ] **Step 2: Check the mob still loads**

Run: `go test ./internal/mobs/ -count=1 && go test ./internal/behaviortree/ -run 'Bandit' -count=1 && go test . -run 'Mob|Content' -count=1`
Expected: PASS. The isolated boot in Task 15 confirms the full load.

- [ ] **Step 3: Commit**

```bash
git add _datafiles/world/dogmud/mobs/north_road/284-bandit_fighter.yaml
git commit -m "fix(content): bandit fighter no longer carries a spare sword and vest (#293)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: #339 Delete the dead cast-chance function and its knob

**Model: sonnet**

Tier 1 of the issue only (the function and the knob). Tier 2, collapsing `SpellBook`, touches saves and is not in the spec.

**Files:**
- Modify: `internal/characters/spells.go:1-75`
- Modify: `internal/configs/config.balance.go:630`
- Modify: `internal/configs/config.balance.spells.go:48-50`
- Modify: `_datafiles/config.yaml:1935-1939`

- [ ] **Step 1: Confirm nothing calls either one**

Run: `grep -rn "GetBaseCastSuccessChance\|SpellProficiencyCastsPerPoint" --include=*.go --include=*.yaml --include=*.template --include=*.md internal modules _datafiles *.go`
Expected: exactly `internal/characters/spells.go:19`, `:50`, `config.balance.go:630`, `config.balance.spells.go:48`, `:49`, `_datafiles/config.yaml:1939`. Any other hit: stop and report.

- [ ] **Step 2: Delete the function**

In `spells.go`, delete from the comment `/*` (line 16, `All spells should have a 10% minimum chance of success.`) through the closing `}` of `GetBaseCastSuccessChance` (line 75), and the blank line after it. Then remove the now-unused imports `"math"`, `"github.com/GoMudEngine/GoMud/internal/configs"`, `"github.com/GoMudEngine/GoMud/internal/skills"` and `"github.com/GoMudEngine/GoMud/internal/spells"`. Run `gofmt -w internal/characters/spells.go && go build ./internal/characters/`; if the build names another unused or missing import, follow it.

- [ ] **Step 3: Delete the knob from Go**

In `config.balance.go`, delete the line:

```go
	SpellProficiencyCastsPerPoint ConfigInt   `yaml:"SpellProficiencyCastsPerPoint"` // Casts needed per 1 proficiency point (default 50)
```

In `config.balance.spells.go`, delete:

```go
	if b.SpellProficiencyCastsPerPoint < 1 {
		b.SpellProficiencyCastsPerPoint = 50
	}
```

Run `gofmt -w internal/configs/config.balance.go internal/configs/config.balance.spells.go` (the struct block may realign).

- [ ] **Step 4: Delete the knob from `config.yaml`**

In this worktree `config.yaml` is the committed blob, but confirm before touching it:

Run: `git ls-files -v _datafiles/config.yaml` (expect `H`, not `S`) and `git diff HEAD --stat -- _datafiles/config.yaml` (expect no output). If either differs, stop and report.

Then delete exactly these five lines (1935-1939), which follow `SpellFoldsSkillFactor: 25`:

```yaml
  #
  #
  # Proficiency: successful casts needed to gain 1 proficiency point
  #   with a specific spell. Higher proficiency reduces fizzle chance.
  SpellProficiencyCastsPerPoint: 50
```

so `SpellFoldsSkillFactor: 25` is followed by the blank line and the `# ── DIFFICULTY` header. Run `git diff --stat -- _datafiles/config.yaml` and confirm `1 file changed, 5 deletions(-)`.

- [ ] **Step 5: Build and test**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/characters/ ./internal/configs/ -count=1 && go test . -run 'Config|Lighting' -count=1`
Expected: no gofmt output; PASS

- [ ] **Step 6: Commit**

```bash
git add internal/characters/spells.go internal/configs/config.balance.go internal/configs/config.balance.spells.go _datafiles/config.yaml
git commit -m "refactor(spells): delete the dead cast-chance function and its proficiency knob (#339)" -m "GetBaseCastSuccessChance had no callers and was the only reader of SpellProficiencyCastsPerPoint. Tier 2 (collapsing SpellBook) is left for later." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: #362 Module commands are held to the helpfile test

**Model: sonnet**

Module commands reach `userCommands` only when `plugins.Load` runs at boot, which a test binary never does, and `usercommands` cannot import `modules` (cycle). So the test reads the registrations from the module sources, the way the root guards read source, and proves the scan works by requiring names known to be registered each way.

**Files:**
- Modify: `internal/usercommands/helpfile_completeness_test.go` (imports; append a test)
- Create: `modules/aicompanion/files/datafiles/templates/help/companion-court.template`
- Create: `modules/aicompanion/files/datafiles/templates/help/companion-boundary.template`
- Create: `modules/aicompanion/files/datafiles/templates/help/companion-ask.template`

- [ ] **Step 1: Write the failing test**

Add `"io/fs"` and `"regexp"` to the imports of `helpfile_completeness_test.go`, and append:

```go
// moduleUserCommandCall and moduleRegisterCommandCall match the two ways a
// module registers a user command:
//
//	plug.AddUserCommand(`name`, handler, allowWhenDowned, isAdminOnly)
//	usercommands.RegisterCommand(`name`, handler, disabledWhenDowned, allowedInCombat, isAdminOnly)
var (
	moduleUserCommandCall = regexp.MustCompile(
		"AddUserCommand\\(\\s*[`\"]([^`\"]+)[`\"]\\s*,\\s*[^,]+,\\s*(?:true|false)\\s*,\\s*(true|false)\\s*\\)")
	moduleRegisterCommandCall = regexp.MustCompile(
		"usercommands\\.RegisterCommand\\(\\s*[`\"]([^`\"]+)[`\"]\\s*,\\s*[^,]+,\\s*(?:true|false)\\s*,\\s*(?:true|false)\\s*,\\s*(true|false)\\s*\\)")
)

// TestHelpFileCompleteness_ModuleCommands is TestHelpFileCompleteness_Commands
// for the commands modules register (#362). Those reach userCommands only when
// plugins.Load runs at boot, never in a test binary, so they are read from the
// module sources. Their help may live in the world templates or in the
// module's own embedded templates, which the help command reads first.
func TestHelpFileCompleteness_ModuleCommands(t *testing.T) {
	root := helpDataRoot(t) // <repo>/_datafiles/world/dogmud
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	modulesDir := filepath.Join(repoRoot, "modules")

	type moduleCommand struct {
		name, module string
		admin        bool
	}
	var found []moduleCommand
	err := filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(modulesDir, path)
		if err != nil {
			return err
		}
		module := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range moduleUserCommandCall.FindAllStringSubmatch(string(src), -1) {
			found = append(found, moduleCommand{name: m[1], module: module, admin: m[2] == "true"})
		}
		for _, m := range moduleRegisterCommandCall.FindAllStringSubmatch(string(src), -1) {
			found = append(found, moduleCommand{name: m[1], module: module, admin: m[2] == "true"})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", modulesDir, err)
	}

	// The scan must be able to find what is there: one plugin-helper
	// command, one admin one, and one direct registration.
	seen := map[string]bool{}
	for _, c := range found {
		seen[c.name] = true
	}
	for _, want := range []string{"auction", "ai-flag", "companion-stay"} {
		if !seen[want] {
			t.Fatalf("the module scan did not find %q; the patterns no longer match how modules register commands (found %d)", want, len(found))
		}
	}

	userHelpDir := filepath.Join(root, "templates", "help")
	adminHelpDir := filepath.Join(root, "templates", "admincommands", "help")
	var missing []string
	for _, c := range found {
		if commandHelpSkip[c.name] {
			continue
		}
		target := c.name
		if alias, ok := commandHelpAliases[c.name]; ok {
			target = alias
		}
		moduleTemplates := filepath.Join(modulesDir, c.module, "files", "datafiles", "templates")
		if c.admin {
			if !helpFileExistsAt(
				filepath.Join(adminHelpDir, "command."+target+".template"),
				filepath.Join(adminHelpDir, "command."+target+".md"),
				filepath.Join(moduleTemplates, "admincommands", "help", "command."+target+".template"),
			) {
				missing = append(missing, c.name+" (admin, module "+c.module+")")
			}
			continue
		}
		if !helpFileExistsAt(
			filepath.Join(userHelpDir, target+".template"),
			filepath.Join(userHelpDir, target+".md"),
			filepath.Join(moduleTemplates, "help", target+".template"),
		) {
			missing = append(missing, c.name+" (module "+c.module+") expected "+
				filepath.Join("modules", c.module, "files", "datafiles", "templates", "help", target+".template"))
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("module commands missing help files (%d):\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
}
```

- [ ] **Step 2: Run it and confirm it fails on exactly three**

Run: `go test ./internal/usercommands/ -run TestHelpFileCompleteness_ModuleCommands -count=1`
Expected: FAIL listing `companion-ask`, `companion-boundary` and `companion-court` (module aicompanion), and nothing else. If it lists another, write its help the same way and name it in the commit.

- [ ] **Step 3: Write the three help files**

`modules/aicompanion/files/datafiles/templates/help/companion-court.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">companion-court</ansi>

Says yes to a closer bond with your companion. Nothing about a romance
moves a step unless you say so with this command, and only when your
companion is ready for it: if it is not the moment, they let you know.

If you drew a line earlier with <ansi fg="command">companion-boundary friendship</ansi>,
this lifts it. Nothing else changes; the rest is up to the two of you.

This works only once you have agreed that your companion may think for
themselves (see <ansi fg="command">help aicompanion</ansi>). A plain companion answers
with a few set lines, and nothing more grows between you.

<ansi fg="yellow">━━━ Usage ━━━</ansi>

  <ansi fg="command">companion-court</ansi>    Say yes to the next step.
```

`modules/aicompanion/files/datafiles/templates/help/companion-boundary.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">companion-boundary</ansi>

Draws a line with your companion: friendship, and nothing more. They
remember it for good and will not raise it again. If things between you
had already grown closer, you go back to being friends.

<ansi fg="yellow">━━━ Usage ━━━</ansi>

  <ansi fg="command">companion-boundary friendship</ansi>   Keep things as they are.
  <ansi fg="command">companion-boundary none</ansi>         Lift the line you drew.

<ansi fg="command">companion-court</ansi> lifts the line too.
```

`modules/aicompanion/files/datafiles/templates/help/companion-ask.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">companion-ask</ansi>

Sends your companion to ask a townsperson or creature here about
something for you. Only when you send them this way do your companion's
words count with that one, and only on that topic, for a short while.

You must be able to see who you are sending them to, and someone's
companion cannot be asked.

<ansi fg="yellow">━━━ Usage ━━━</ansi>

  <ansi fg="command">companion-ask <who> about <what></ansi>

  <ansi fg="command">companion-ask smith about the ore</ansi>
```

Check width: `for f in court boundary ask; do sed 's/<[^>]*>//g' modules/aicompanion/files/datafiles/templates/help/companion-$f.template | tr -d '\r' | awk -v f=$f 'length($0) > 80 { print f": "length($0)": "$0 }'; done` prints nothing.

- [ ] **Step 4: Run the helpfile tests and the module**

Run: `go test ./internal/usercommands/ -run 'TestHelpFileCompleteness' -count=1 && go build ./... && go test ./modules/aicompanion/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/helpfile_completeness_test.go modules/aicompanion/files/datafiles/templates/help/companion-court.template modules/aicompanion/files/datafiles/templates/help/companion-boundary.template modules/aicompanion/files/datafiles/templates/help/companion-ask.template
git commit -m "test(help): module commands are held to the helpfile test; three companion help pages (#362)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: #253 ASCII mode drops heavy rules and markers, and keeps dashes as hyphens

**Model: haiku**

Owner call 2026-10-10: drop the glyphs. Em and en dashes become "-" because dropping them would glue the words either side together. Only `━` of the heavy family ships in server text today; the whole family is listed so a new template cannot bring one back unconverted.

**Files:**
- Modify: `internal/util/util.go:1186-1188` (inside `unicodeToAscii`)
- Modify: `internal/util/util_test.go:1192-1213` (rows in `TestConvertToAscii`)
- Modify: `internal/util/context.md:73-78`

- [ ] **Step 1: Write the failing rows**

In `TestConvertToAscii`, before the `{"unmapped high rune passthrough", ...}` row, add:

```go
		// #253: heavy rules and the pet, companion, death and counter
		// markers are dropped; dashes become a hyphen.
		{"heavy rule dropped", "━━━ Usage ━━━", " Usage "},
		{"heavy box dropped", "┏┓┗┛┃┣┫┳┻╋", ""},
		{"markers dropped", "♥friend ♦companion ☠dead", "friend companion dead"},
		{"counter glyph dropped", "⚔ COUNTER!", " COUNTER!"},
		{"em dash is a hyphen", "locked in — nowhere", "locked in - nowhere"},
		{"en dash is a hyphen", "arms 3–6", "arms 3-6"},
```

- [ ] **Step 2: Run it and confirm the new rows fail**

Run: `go test ./internal/util/ -run TestConvertToAscii -count=1`
Expected: the six new subtests FAIL; every old row PASSES.

- [ ] **Step 3: Add the table rows**

In `unicodeToAscii`, directly after the line

```go
	'≈': "~", '⌂': "#", '◆': "*", '●': "o", '○': "o",
```

add:

```go
	// Heavy box rules (#253): dropped, not drawn. Only ━ ships in server
	// text (help section rules and the banner rule); the rest of the heavy
	// family is here so a new template cannot bring one back unconverted.
	'━': "", '┃': "", '┅': "", '┇': "", '┉': "", '┋': "", '╍': "", '╏': "",
	'┏': "", '┓': "", '┗': "", '┛': "",
	'┣': "", '┫': "", '┳': "", '┻': "", '╋': "",
	'╸': "", '╹': "", '╺': "", '╻': "",
	// Pet, companion, death and counter markers (#253): dropped; the word
	// beside each ("friend", "companion", "dead", "COUNTER!") still says it.
	'♥': "", '♦': "", '☠': "", '⚔': "",
	// Em and en dash: a hyphen, not nothing, so the words either side stay
	// apart (#253).
	'—': "-", '–': "-",
```

Run `gofmt -w internal/util/util.go` (the map realigns).

- [ ] **Step 4: Document it**

In `internal/util/context.md`, after the sentence ending `as the no-break space (U+00A0) of the crit banners has.`, add:

```
Heavy box rules (`━` and family) and the `♥ ♦ ☠ ⚔` markers map to nothing,
because the words beside them carry the meaning; em and en dashes map to `-`,
because dropping them would glue two words together (#253).
```

- [ ] **Step 5: Run the util tests and the ASCII readers**

Run: `go test ./internal/util/ ./internal/users/ ./internal/connections/ -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/util/util.go internal/util/util_test.go internal/util/context.md
git commit -m "fix(ascii): drop heavy rules and markers, keep dashes as hyphens (#253)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Gate, patch notes, PR

**Model: sonnet**

**Files:**
- Modify: `docs/PATCH_NOTES.md` (new section at the top)

- [ ] **Step 1: gofmt, build, lint**

Load the `dogmud-shipping` skill. Then, from the worktree:

Run: `gofmt -l internal/ modules/` (expect no output; also run `gofmt -l *.go` for the root files)
Run: `go build ./...`
Run: `~/go/bin/golangci-lint run --new-from-merge-base=origin/master` (expect `0 issues`; check `golangci-lint --version` against `.github/workflows/` first)

- [ ] **Step 2: Touched-package tests**

Run: `go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/combat/ ./internal/actions/ ./internal/items/ ./internal/shops/ ./internal/util/ ./internal/characters/ ./internal/configs/ ./internal/forager/ ./internal/parser/ ./internal/templates/ ./internal/users/ ./internal/connections/ ./internal/narration/ ./modules/auctions/ ./modules/aicompanion/ -count=1`
Run: `go test . -count=1`
Expected: PASS. A failure in a test this plan did not touch: check it on a clean detached worktree of master (never `git stash`; the stash is shared across worktrees) before treating it as ours.

- [ ] **Step 3: Shuffle the new tests only**

Run: `go test ./internal/usercommands/ -run 'TestGetAll(From)?_.*Corpse|TestPartyList_CharmedCompanionListedOnce|TestEat_SpoiledFoodIsRefused|TestStorageRemove_BySlotNumber|TestConsider_ProtectedMobSaysYouCantFightIt|TestHelpFileCompleteness_ModuleCommands' -shuffle=on -count=3`
Run: `go test ./internal/mobcommands/ -run 'TestMobEmote_LeadingAtIsStripped|TestMobEat_|TestMobTauntTriad_TargetReadsYourResolve' -shuffle=on -count=3`
Run: `go test ./internal/combat/ -run 'SecondPerson|CounteredReadsYourWill|TestTauntTriad_WithDefenderDamage' -shuffle=on -count=3`
Run: `go test ./internal/shops/ -run 'GoldReserve' -shuffle=on -count=3 && go test ./internal/items/ -run TestIsSpoiledFood -shuffle=on -count=3 && go test ./internal/util/ -run TestConvertToAscii -shuffle=on -count=3`
Expected: PASS every time. A failure under shuffle is a missing `t.Cleanup` in the new test: fix the test, not the order.

- [ ] **Step 4: Isolated boot**

`C:/tmp/dogmud-boot-check-loose2` is the boot worktree; it is removed in this step. The committed `config.yaml` is used as is (it already carries Task 12's change), so nothing is copied in.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check-loose2 HEAD
cat > C:/tmp/dogmud-boot-check-loose2/boot-overrides.yaml <<'EOF'
Network.TelnetPort: [33339]
Network.LocalPort: 9995
Network.HttpPort: 8099
Network.HttpsPort: 0
Network.AIPort: 0
EOF
go build -C C:/tmp/dogmud-boot-check-loose2 -o boot-check.exe .
```

Start it hidden and keep its PID (PowerShell):

```powershell
$env:CONFIG_PATH = 'C:\tmp\dogmud-boot-check-loose2\boot-overrides.yaml'; $env:LOG_NOCOLOR = '1'
$p = Start-Process -FilePath 'C:\tmp\dogmud-boot-check-loose2\boot-check.exe' -WorkingDirectory 'C:\tmp\dogmud-boot-check-loose2' -RedirectStandardOutput 'C:\tmp\dogmud-boot-check-loose2\boot.log' -RedirectStandardError 'C:\tmp\dogmud-boot-check-loose2\boot.err' -WindowStyle Hidden -PassThru
$p.Id
```

Wait (Monitor with an until-loop, not `sleep`) until `boot.log` contains `Server Ready` or a panic, at most 180 seconds. Then:

Run: `grep -c "Server Ready" C:/tmp/dogmud-boot-check-loose2/boot.log` (want 1)
Run: `grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" C:/tmp/dogmud-boot-check-loose2/boot.log C:/tmp/dogmud-boot-check-loose2/boot.err` (want 0 for each; a zero count exits 1, so run it on its own)
Run: `grep -iE "steppe_forager_delivery|284-bandit|unreachable waypoint" C:/tmp/dogmud-boot-check-loose2/boot.log` (want no error lines)

Stop it by that PID only (never by name or port): `Stop-Process -Id <PID> -Force`. Then remove the worktree: `git worktree remove --force C:/tmp/dogmud-boot-check-loose2`; if Windows still holds the exe, `Remove-Item -Recurse -Force 'C:\tmp\dogmud-boot-check-loose2'` and then `git worktree prune`.

- [ ] **Step 5: Patch notes**

Add this section at the top of `docs/PATCH_NOTES.md`, directly under `# DOGMud Patch Notes` and above `## 2026-10-10: Loose ends`:

```markdown
## 2026-10-10: More loose ends

- `get all lookout corpse` and `get all from lookout corpse` now take from
  the corpse you named, not whichever body fell last.
- `party list` shows a charmed companion once, as your companion.
- When someone turns your taunt back on you, or a creature's taunt or howl
  lands, the line now reads "your will" or "your resolve", not "their".
- `consider` on someone you cannot fight now says so, rather than weighing
  the odds of a fight that will never happen.
- `storage remove` followed by a slot number takes out what is in that
  slot. The help for `storage`, `sell`, `give` and `show` now lists every
  form of each command and points to the others.
- Bandit fighters on the North Road no longer always drop a spare sword and
  vest. Their own sword and vest still drop now and then.
- Creatures no longer leave a stray "@" in their emotes, and no longer eat
  food that has gone bad.
- With the ASCII character set on, decorative rules and markers no longer
  come through as stray symbols, and long dashes show as hyphens.
```

Read it once against `dogmud-player-copy`: 80 columns, no raw numbers, no em or en dashes (`grep -n "[—–]" docs/PATCH_NOTES.md | head -5` must show nothing in the new section).

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs(patch-notes): loose issues sweep 2" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: PR**

Write the body to a file in the session scratchpad (not `C:/tmp`):

```markdown
Fourteen small fixes clear of the Messaging M6 slice 1 files.

Fixes #217
Fixes #244
Fixes #262
Fixes #273
Fixes #277
Fixes #308
Fixes #287
Fixes #346
Fixes #266
Fixes #286
Fixes #293
Fixes #339
Fixes #362
Fixes #253

Notes:
- #262 also fixes the same defect in ordinary taunts and howls (the target read "their resolve" about their own). The new `GetConvictionDamageDescriptionToTarget`, `TauntTriad.WithDefenderDamage` and `TauntResult.DmgDescToTarget` are not yet in combat/actions context.md, which the M6 branch owns; they get a line after M6 merges.
- #308 also makes `storage remove <slot#>` work; it never reached the slot branch.
- #266 also drops room 507 from Halix's VendorRooms in internal/forager/territory.go.
- #339 is Tier 1 only; collapsing SpellBook is left for later.

Left out on purpose: issues 264, 261 and 283 (M7 wording), 241 (a line order in a line-keyed guard file), 440 (own branch), 249 and 236 (M6).

Gate: gofmt clean, build, golangci-lint 0 issues, touched-package tests, new tests under -shuffle=on, isolated boot reached Server Ready.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

The left-out issues are named without `#` so no closing keyword or link lands on them.

```bash
git push -u origin fix/loose-issues-sweep-2
gh pr create --repo pruuk/DOGMud --base master --head fix/loose-issues-sweep-2 --title "Loose issues sweep 2: 14 small fixes" --body-file <scratchpad body file>
```

Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Then `gh pr checks <n> --repo pruuk/DOGMud --watch`, and confirm with `gh run list --repo pruuk/DOGMud --branch fix/loose-issues-sweep-2` that every expected workflow ran. The owner merges and deploys.
