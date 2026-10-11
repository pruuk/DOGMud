# Loose Issues Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix fourteen small open issues (#461, #459, #289, #269, #271, #267, #302, #284, #296, #270, #307, #361, #357, #306, #305) in one branch.

**Spec:** `docs/superpowers/specs/2026-10-10-loose-issues-sweep-design.md` (owner approved 2026-10-10).

**Architecture:** Each task is independent and touches its own files. No new packages. Behavioural changes get a test that fails first; copy, YAML and comment changes do not.

**Tech Stack:** Go, testify, Go `text/template` world templates, YAML content.

**Worktree:** `C:\Users\Calabe Davis\workspace\DOGMud\.claude\worktrees\loose-issues-sweep`, branch `worktree-loose-issues-sweep`. Run every command from there. Never `cd` to the main checkout.

---

## Ground rules for every task

- **Do not touch these files** (the Messaging M6 slice 1 session, branch `feat/messaging-m6-slice1`, is editing them): anything in `internal/conditions/`; `internal/hooks/` spell, condition and round-tick files; `internal/actions/combat_*.go`; `internal/behaviortree/actions_item_proc.go`, `action_cast_best_in_category.go`; `internal/combat/ai.go`; `internal/characters/combat.go`, `internal/characters/conditions.go`; `internal/events/eventtypes.go`; `internal/users/userrecord.go`; `internal/mobs/mobs.go`; `keywords.yaml`; the `context.md` of conditions, characters, events, users, mobs, actions, behaviortree, combat and hooks. If a task seems to need one of these, stop and report.
- Stage named paths only. Never `git add -A` or `git add .`.
- Never edit a file with a Python read-modify-write. Use the Edit tool.
- `_datafiles/config.yaml` is not touched by this plan.
- No em dashes or en dashes in player text, comments or commit messages.
- Player text: 80 columns, no raw numbers for damage or chances.
- Every commit message ends with:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- Test command form: `go test ./internal/<pkg>/ -run '<Regex>' -count=1`.

## Facts verified against source (master 47c0d6902)

| Fact | Where |
|---|---|
| `roominfo.template` line 13 reads `$room.SkillTraining`; no such field on `Room` (removed in 0f83dfc96) | `_datafiles/world/{dogmud,default}/templates/admincommands/ingame/roominfo.template:13` |
| `adminRoom_Info(args []string, user, room)`; `args == ["info"]` renders the current room; needs permission `room.info` | `internal/usercommands/admin.room.dispatcher.go:136-162` |
| `templates.Process` returns the literal `[TEMPLATE ERROR]` on an exec error | `internal/templates/templates.go:223,261` |
| `cmdPartyInvite` calls `parties.New` at 173-175, before `AimBySight` (185) | `internal/usercommands/party.go:166-226` |
| `Party(rest, user, room, flags)` routes `invite` to `cmdPartyInvite` | `internal/usercommands/party.go:20,41` |
| `parties.New(userId)`, `parties.Get(userId)`, `(*Party).Disband()` | `internal/parties/parties.go:137,152,356` |
| `showHidden := rest == "all+"`; `if rest == "all+"` injects every quest; Secret filter skipped when `showHidden` | `internal/usercommands/quests.go:34-62` |
| `users.RoleAdmin == "admin"`; `user.Role` field | `internal/users/userrecord.go:31` |
| Floor gold branch: `if args[0] == goldName \|\| (len(args[0]) < 5 && goldName[0:len(args[0])-1] == args[0])`; the prefix half never matches anything but `gold` | `internal/usercommands/get.go:575-576` |
| `items.LoadDataFiles()` panics at 868, 885, 892; `casing.AssertCanonical` also panics | `internal/items/itemspec.go:861-897`, `internal/casing/validate.go:9` |
| `reload items` prints "Items reloaded." unconditionally | `internal/usercommands/admin.reload.go:41-43` |
| `portal loot` with no room: `mob.Command("portal home;drop all")` then `return true, fmt.Errorf(...)`; `world.go` logs any error with `mudlog.Warn` | `internal/mobcommands/portal.go:47-53`, `world.go:1106-1108` |
| Unhandled mob command: `mob.Command(fmt.Sprintf("emote looks a little confused (%s %s).", ...))` | `world.go:1112-1116` |
| `mudlog.Debug`, `mudlog.Warn` exist | `internal/mudlog/mudlog.go:86,94` |
| Player default surrender policy `{SurrenderAutoTap, 15}` set only in `New()`; mobs overwrite theirs at spawn; only `status`/`set` read a player's | `internal/characters/character.go:432`, `internal/mobs/mobs.go:483-491` |
| `Help` calls `GetHelpContents(rest)`; on error prints `No help found for "%s"` | `internal/usercommands/help.go:85-89` |
| 50 admin help templates `admincommands/help/command.<name>.template` | `_datafiles/world/dogmud/templates/admincommands/help/` |
| `useDogmudTemplates(t)` points templates at the shipped dogmud world | `internal/usercommands/template_freeze_test.go:41` |
| Arm keys: `HandSlot.Label` "wielded", "offhand", "extra arm 1".."extra arm 4"; `ArmLabel(arm)` returns that key; its only player-facing use is the equip line via `EquipResult.ArmLabel` | `internal/characters/hand_slots.go:30-73`, `internal/actions/remove_equip.go:19-41`, `internal/usercommands/equip.go:153-157` |
| Keys `"extra arm 2"`, `"extra wrist 1"` etc. are looked up by `IsBlockedBy2H`, `GetSlotPointer`, enchant slots; they stay | `internal/characters/worn.go:184-236`, `internal/usercommands/enchant_slot.go:128-135` |
| `AllSlots` display labels: "Arm 3".."Arm 6", "Wrist", "Wrist", "Wrist 3".."Wrist 6"; GMCP sends this label | `internal/characters/worn.go:53-63`, `modules/gmcp/gmcp.Char.go:899` |
| `FindItem` source strings for arms and wrists: "extra arm", "extra arm", "extra arm 3", "extra arm 4", "worn - wrist" x2, "extra wrist 1".."4"; shown by `look` ("You look at the X <source>:"); `salvageableSource` only checks backpack and bandolier | `internal/characters/inventory.go:516-536`, `internal/usercommands/look.go:309-316`, `internal/usercommands/salvage.go:220-226` |
| Inventory templates print "Wrist:" for both natural wrists | `templates/character/inventory.template:26,28`, `inventory-look.template:27,29` |
| Switching target: if the old target is gone it is immediate; otherwise `ChanceToSwitchTarget` roll, success applies after 1 round | `internal/usercommands/target.go:170-205` |
| Sunstone sets `rarity_tier: 40` on the line after `value:` | `_datafiles/world/dogmud/items/armor-20000/light/20099-sunstone.yaml:13-14` |
| `TriggerCombatInterrupt` only in `transitions.go:36` and `activity/context.md:151` (plus archived docs) | grep |
| Behaviour tree evictors: `EvictArchetype`, `EvictTree`, `EvictRoomTree`, `EvictItemTree` | `internal/behaviortree/engine.go:184,195,203,268` |

---

### Task 1: #461 Admin `room info` template error

**Files:**
- Modify: `_datafiles/world/dogmud/templates/admincommands/ingame/roominfo.template:13`
- Modify: `_datafiles/world/default/templates/admincommands/ingame/roominfo.template:13`
- Create: `internal/usercommands/admin_room_info_template_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #461: `room info` rendered [TEMPLATE ERROR] because roominfo.template read
// Room.SkillTraining, a field removed in 0f83dfc96. This renders the shipped
// template against a real room so a later field removal fails here.
func TestRoomInfo_ShippedTemplateRenders(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	admin, room := getTestUserAndRoom(t)
	oldRole := admin.Role
	admin.Role = users.RoleAdmin
	t.Cleanup(func() { admin.Role = oldRole })
	events.DrainQueuedMessagesForTest(admin.UserId)

	handled, err := adminRoom_Info([]string{"info"}, admin, room)
	require.NoError(t, err)
	require.True(t, handled)

	out := strings.Join(events.DrainQueuedMessagesForTest(admin.UserId), "\n")
	assert.NotContains(t, out, "[TEMPLATE ERROR]")
	assert.Contains(t, out, "RoomId:")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestRoomInfo_ShippedTemplateRenders -count=1`
Expected: FAIL, output contains `[TEMPLATE ERROR]`. If it fails for another reason (a different missing field, a nil registry), read the template error, fix the test setup or note the extra cause, and keep the cause in the commit message. If it PASSES, the cause is not `SkillTraining`: stop and report.

- [ ] **Step 3: Delete the Training line in both templates**

In both `roominfo.template` copies, delete exactly this line (line 13):

```
<ansi fg="yellow-bold">Training:</ansi>       {{ if eq (len $room.SkillTraining) 0 }}None{{ else }}{{- range $index, $skill := $room.SkillTraining }}[{{ $skill }}] {{ end -}}{{ end }}
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/usercommands/ -run TestRoomInfo_ShippedTemplateRenders -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/admin_room_info_template_test.go _datafiles/world/dogmud/templates/admincommands/ingame/roominfo.template _datafiles/world/default/templates/admincommands/ingame/roominfo.template
git commit -m "fix(admin): room info template no longer reads the removed SkillTraining field (#461)"
```

---

### Task 2: #459 A refused party invite creates a party

**Files:**
- Modify: `internal/usercommands/party.go:166-226`
- Create: `internal/usercommands/party_invite_refused_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #459: an invite that is refused (nobody by that name here) must not leave
// the inviter leading an empty party, which then blocks `follow`.
func TestPartyInvite_RefusedInviteCreatesNoParty(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	if p := parties.Get(user.UserId); p != nil {
		p.Disband()
	}
	t.Cleanup(func() {
		if p := parties.Get(user.UserId); p != nil {
			p.Disband()
		}
	})
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Party("invite Zzyzxnobody", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Nil(t, parties.Get(user.UserId), "a refused invite must not create a party")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestPartyInvite_RefusedInviteCreatesNoParty -count=1`
Expected: FAIL, "a refused invite must not create a party".

- [ ] **Step 3: Create the party only when the invite is sent**

In `cmdPartyInvite`, replace:

```go
	// Not in a party? Create one.
	if currentParty == nil {
		currentParty = parties.New(user.UserId)
	}

	if !currentParty.IsLeader(user.UserId) {
```

with:

```go
	// A player already in a party must lead it to invite. A player in no
	// party gets one only once the invite is actually sent (#459), so a
	// refused invite does not leave them leading an empty party.
	if currentParty != nil && !currentParty.IsLeader(user.UserId) {
```

Then replace:

```go
	if currentParty.InvitePlayer(invitePlayerId) {
```

with:

```go
	if currentParty == nil {
		currentParty = parties.New(user.UserId)
	}

	if currentParty.InvitePlayer(invitePlayerId) {
```

Read the function after the edit: every use of `currentParty` before this point must be nil-safe (only the leader check, which now guards nil).

- [ ] **Step 4: Run the test and the package's party tests**

Run: `go test ./internal/usercommands/ -run 'Party' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/party.go internal/usercommands/party_invite_refused_test.go
git commit -m "fix(party): create the party only when an invite is sent (#459)"
```

---

### Task 3: #289 `quests all+` is admin-only

**Files:**
- Modify: `internal/usercommands/quests.go:34-48`
- Create: `internal/usercommands/quests_all_plus_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #289: `quests all+` lists every quest, secret ones included. Owner call
// 2026-10-10: admins only; anyone else gets the plain `all` view.
func TestQuestsShowHidden_AdminOnly(t *testing.T) {
	player := &users.UserRecord{Role: users.RoleUser}
	admin := &users.UserRecord{Role: users.RoleAdmin}

	assert.False(t, questsShowHidden(`all+`, player))
	assert.True(t, questsShowHidden(`all+`, admin))
	assert.False(t, questsShowHidden(`all`, admin))
	assert.False(t, questsShowHidden(``, admin))
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestQuestsShowHidden_AdminOnly -count=1`
Expected: FAIL to compile, `undefined: questsShowHidden`.

- [ ] **Step 3: Add the helper and use it**

Add below the `Quests` function in `quests.go`:

```go
// questsShowHidden reports whether `quests all+`, which lists every quest
// including unstarted and secret ones, applies. Admins only (#289); anyone
// else typing it gets the plain `all` view.
func questsShowHidden(rest string, user *users.UserRecord) bool {
	return rest == `all+` && user.Role == users.RoleAdmin
}
```

In `Quests`, replace:

```go
	showHidden := rest == `all+`
	showComplete := (rest == `all`) || showHidden
```

with:

```go
	showHidden := questsShowHidden(rest, user)
	showComplete := rest == `all` || rest == `all+`
```

and replace:

```go
	if rest == `all+` {
		for _, quest := range quests.GetAllQuests() {
```

with:

```go
	if showHidden {
		for _, quest := range quests.GetAllQuests() {
```

Delete the empty `} else {\n\n\t}` branch that follows that block.

- [ ] **Step 4: Run the test**

Run: `go test ./internal/usercommands/ -run 'TestQuests' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/quests.go internal/usercommands/quests_all_plus_test.go
git commit -m "fix(quests): quests all+ is admin-only (#289)"
```

---

### Task 4: #269 Items named "Gold..." are picked up

**Files:**
- Modify: `internal/usercommands/get.go:575-576`
- Create: `internal/usercommands/get_gold_named_item_test.go`

Note: the spec said "gold or a prefix of it". The existing prefix check is dead (it compares `goldName[0:len-1]` to the whole word, which never matches), so today only the exact word `gold` reaches the pile. Keep that: the gold pile is taken only when the whole argument is the single word `gold`.

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

const goldWireItemId = 999981

// seedGoldWireRoom is seedFixtureRoom (get_fixture_test.go) with a Gold Wire
// on the floor and no gold pile. It seeds its own item registry because
// items.SeedItemsForTest REPLACES the registry rather than adding to it.
func seedGoldWireRoom(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430)) // midsummer noon: nothing refused as blind
	t.Cleanup(util.ResetRoundCountForTest)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		goldWireItemId: {ItemId: goldWireItemId, Name: "Gold Wire", NameSimple: "wire", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	origItems := user.Character.Items
	user.Character.Items = nil
	t.Cleanup(func() { user.Character.Items = origItems })
	wire := items.New(goldWireItemId)
	room.AddItem(wire, false)
	t.Cleanup(func() { room.RemoveItem(wire, false) })
	oldGold := room.Gold
	room.Gold = 0
	t.Cleanup(func() { room.Gold = oldGold })
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

// #269: a floor item whose name starts with "Gold" was read as the gold pile,
// so `get gold wire` and `get all` both answered "There's no gold to grab."
func TestGet_ItemNamedGoldIsPickedUp(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	_, err := Get("gold wire", user, room, 0)
	require.NoError(t, err)
	assert.NotContains(t, sentTo(user), "There's no gold to grab.")
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get gold wire picks up the Gold Wire")
}

// `get all` reaches the same item by its name.
func TestGetAll_PicksUpAnItemNamedGold(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	Get("all", user, room, 0)
	assert.NotContains(t, sentTo(user), "There's no gold to grab.")
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get all picks up the Gold Wire")
}

// Plain `get gold` still means the gold pile.
func TestGet_GoldStillMeansThePile(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	Get("gold", user, room, 0)
	assert.Contains(t, sentTo(user), "There's no gold to grab.")
}
```

Imports for this file: `testing`, and from `github.com/GoMudEngine/GoMud/internal/`: `configs`, `events`, `gametime`, `items`, `rooms`, `users`, `util`; plus testify `assert` and `require` (the same set `get_fixture_test.go` uses).

- [ ] **Step 2: Run them and confirm the first two fail**

Run: `go test ./internal/usercommands/ -run 'TestGet_ItemNamedGold|TestGetAll_PicksUpAnItemNamedGold|TestGet_GoldStillMeansThePile' -count=1`
Expected: the two Gold Wire tests FAIL; `TestGet_GoldStillMeansThePile` PASSES.

- [ ] **Step 3: Narrow the gold-pile branch**

In `get.go`, replace:

```go
		goldName := `gold`
		if args[0] == goldName || (len(args[0]) < 5 && goldName[0:len(args[0])-1] == args[0]) {
```

with:

```go
		// Only the bare word is the gold pile. "gold wire" is an item, and
		// `get all` arrives here with each item's name (#269).
		if len(args) == 1 && args[0] == `gold` {
```

If `goldName` is used later in the branch, keep its declaration above the `if`.

- [ ] **Step 4: Run the tests and the get suite**

Run: `go test ./internal/usercommands/ -run 'TestGet' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/get.go internal/usercommands/get_gold_named_item_test.go
git commit -m "fix(get): items named Gold are picked up, not read as the gold pile (#269)"
```

---

### Task 5: #271 `reload items` reports a bad YAML file

**Files:**
- Modify: `internal/items/itemspec.go:860-897`
- Modify: `internal/usercommands/admin.reload.go:41-43`
- Create: `internal/items/load_data_files_error_test.go`
- Modify: `internal/items/context.md` (document `LoadDataFilesE`)

- [ ] **Step 1: Write the failing test**

```go
package items

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #271: a broken item file made `reload items` panic, and the admin was
// never told. LoadDataFilesE returns the error, names the file, and leaves
// the previous items live.
func TestLoadDataFilesE_BadYamlReturnsErrorAndKeepsItems(t *testing.T) {
	const keptId = 999971
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		keptId: {ItemId: keptId, Name: "Kept Probe", Type: Object},
	}))

	dir := t.TempDir()
	itemDir := filepath.Join(dir, "items")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "999972-broken_probe.yaml"),
		[]byte("itemid: 999972\nname: [unclosed\n"), 0o644))

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dir)
	configs.SetConfigForTest(t, cfg)

	err := LoadDataFilesE()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken_probe", "the error names the file that broke")
	require.NotNil(t, GetItemSpec(keptId), "the previous items stay loaded")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/items/ -run TestLoadDataFilesE_BadYamlReturnsErrorAndKeepsItems -count=1`
Expected: FAIL to compile, `undefined: LoadDataFilesE`.

- [ ] **Step 3: Split the loader**

Replace the whole `LoadDataFiles` function (from `// file self loads due to init()` to its closing brace) with:

```go
// LoadDataFiles loads every item spec and the combat and defense message
// sets at boot. It panics on a load error: the server cannot run without
// them. A live reload calls LoadDataFilesE instead.
func LoadDataFiles() {
	if err := LoadDataFilesE(); err != nil {
		panic(err)
	}
}

// LoadDataFilesE is LoadDataFiles for `reload items`: it returns the error
// instead of panicking (#271). Each set is swapped in only after it loads,
// so a set that fails leaves the previous one live.
func LoadDataFilesE() (err error) {
	// casing.AssertCanonical panics on a non-canonical name; a reload
	// reports that like any other load error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	start := time.Now()

	dataPath := string(configs.GetFilePathsConfig().DataFiles)
	tmpItems, err := fileloader.LoadAllFlatFiles[int, *ItemSpec](dataPath + `/items`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/items`)
	}

	for id, spec := range tmpItems {
		if spec.Name != "" {
			casing.AssertCanonical(spec.Name, "item", fmt.Sprintf("%d", id))
		}
		if spec.DisplayName != "" {
			casing.AssertCanonical(spec.DisplayName, "item displayname", fmt.Sprintf("%d", id))
		}
	}

	items = tmpItems
	rebuildAuthoredKeywords()

	tmpAttackMessages, err := fileloader.LoadAllFlatFiles[ItemSubType, *WeaponAttackMessageGroup](dataPath + `/combat-messages`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/combat-messages`)
	}

	attackMessages = tmpAttackMessages

	tmpDefenseMessages, err := fileloader.LoadAllFlatFiles[DefencePool, *DefenseMessageGroup](dataPath + `/defense-messages`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/defense-messages`)
	}

	defenseMessages = tmpDefenseMessages

	mudlog.Info("itemspec.LoadDataFiles()", "itemLoadedCount", len(items), "attackMessageCount", len(attackMessages), "defenseMessageCount", len(defenseMessages), "Time Taken", time.Since(start))

	return nil
}
```

Before replacing, diff this block against the current function body. If the current body has any line not shown above, keep it.

- [ ] **Step 4: Run the test**

Run: `go test ./internal/items/ -run TestLoadDataFilesE_BadYamlReturnsErrorAndKeepsItems -count=1`
Expected: PASS. If it fails only on "names the file", the fileloader error omits the path: in `internal/fileloader/fileloader.go`, find where the YAML unmarshal error is returned inside `LoadAllFlatFiles` and wrap it with the file path (`fmt.Errorf("%s: %w", path, err)`), then rerun.

- [ ] **Step 5: Tell the admin**

In `admin.reload.go`, replace:

```go
	case `items`:
		items.LoadDataFiles()
		user.SendText(messaging.CategorySystem, `Items reloaded.`)
		return true, nil
```

with:

```go
	case `items`:
		if err := items.LoadDataFilesE(); err != nil {
			user.SendText(messaging.CategorySystem,
				`<ansi fg="red">Items reload failed. The items loaded before are still in use.</ansi>`)
			user.SendText(messaging.CategorySystem, err.Error())
			return true, nil
		}
		user.SendText(messaging.CategorySystem, `Items reloaded.`)
		return true, nil
```

- [ ] **Step 6: Document it**

In `internal/items/context.md`, find the line naming `LoadDataFiles` and add after it:

```
- `LoadDataFilesE() error`: the reload form of `LoadDataFiles`. Returns the load error (file named) instead of panicking; a set that fails to load leaves the previous set live. `reload items` uses it.
```

If `context.md` does not name `LoadDataFiles`, add the line to its function list. Run `python tools/context_md_audit.py` and confirm no new findings for `internal/items`.

- [ ] **Step 7: Build and run both packages**

Run: `go build ./... && go test ./internal/items/ -count=1 && go test ./internal/usercommands/ -run 'Reload' -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/items/itemspec.go internal/items/load_data_files_error_test.go internal/items/context.md internal/usercommands/admin.reload.go
git commit -m "fix(reload): reload items reports a broken file instead of panicking (#271)"
```

(Add `internal/fileloader/fileloader.go` to the `git add` if Step 4 changed it.)

---

### Task 6: #267 Loot goblin portal is not an error

**Files:**
- Modify: `internal/mobcommands/portal.go:3-13,47-53`

- [ ] **Step 1: Return success**

Replace:

```go
		if qty == 0 { // could't find any
			// No more rooms with items? Our job is done i guess.

			mob.Command(`portal home;drop all`)

			return true, fmt.Errorf("failed to find worthy room with loot")
		}
```

with:

```go
		if qty == 0 {
			// No room is worth looting, which is a normal outcome: go home
			// and drop the haul. Not an error (#267).
			mudlog.Debug("portal loot", "mobId", mob.MobId, "result", "no worthy room, going home")

			mob.Command(`portal home;drop all`)

			return true, nil
		}
```

Add `"github.com/GoMudEngine/GoMud/internal/mudlog"` to the imports. If `fmt` is now unused, remove it (`go build` will say).

- [ ] **Step 2: Build and test the package**

Run: `go build ./... && go test ./internal/mobcommands/ -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/mobcommands/portal.go
git commit -m "fix(mobs): loot goblin finding no loot room is not logged as an error (#267)"
```

---

### Task 7: #302 Unhandled mob command is logged, not emoted

**Files:**
- Modify: `world.go:1112-1116` and its imports
- Create: `world_unhandled_mob_command_test.go`
- Modify: `internal/mobcommands/planner_command_guard_test.go:22-34,62-66` (comment and message text only)

Owner call 2026-10-10: players see nothing; a warn log names the mob and command. Log each (mob id, command) pair once per process so a mob stuck on a bad verb does not flood the log every tick.

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #302: an unhandled mob command used to emote "looks a little confused
// (east )" to the room. It is now a warn log, once per mob and command.
func TestNoteUnhandledMobCommand_LogsOncePerMobAndCommand(t *testing.T) {
	unhandledMobCommands.Clear()
	t.Cleanup(unhandledMobCommands.Clear)

	assert.True(t, noteUnhandledMobCommand(4242, "east", ""), "first time is logged")
	assert.False(t, noteUnhandledMobCommand(4242, "east", ""), "repeat is not")
	assert.True(t, noteUnhandledMobCommand(4242, "west", ""), "another command is logged")
	assert.True(t, noteUnhandledMobCommand(4343, "east", ""), "another mob is logged")
}
```

Check the root package name first: `head -1 world.go`. Use the same package name in the test.

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test . -run TestNoteUnhandledMobCommand_LogsOncePerMobAndCommand -count=1`
Expected: FAIL to compile, `undefined: noteUnhandledMobCommand`.

- [ ] **Step 3: Replace the emote**

In `world.go`, replace:

```go
	if !handled {
		if len(command) > 0 {
			mob.Command(fmt.Sprintf(`emote looks a little confused (%s %s).`, command, remains))
		}
	}
```

with:

```go
	if !handled && len(command) > 0 {
		noteUnhandledMobCommand(mob.MobId, command, remains)
	}
```

Add at the end of `world.go`:

```go
// unhandledMobCommands remembers which mob and command pairs were already
// logged, so a mob stuck on a bad verb logs it once, not every tick.
var unhandledMobCommands sync.Map

// noteUnhandledMobCommand logs a mob command nothing handled. Players used to
// see "<mob> looks a little confused (east )." (#302); now only the log does.
// Returns whether this call logged.
func noteUnhandledMobCommand(mobId int, command, remains string) bool {
	key := fmt.Sprintf("%d:%s", mobId, command)
	if _, seen := unhandledMobCommands.LoadOrStore(key, true); seen {
		return false
	}
	mudlog.Warn("mob-UnhandledCommand", "mobId", mobId, "command", command, "remains", remains)
	return true
}
```

`sync`, `fmt` and `mudlog` are already imported by `world.go` (lines 10 and 25); confirm with `go build`. `sync.Map.Clear` needs Go 1.23+; check `go.mod`. If older, replace `unhandledMobCommands.Clear()` in the test with `unhandledMobCommands.Range(func(k, _ any) bool { unhandledMobCommands.Delete(k); return true })`.

- [ ] **Step 4: Update the guard test's description**

In `internal/mobcommands/planner_command_guard_test.go`, the comment at lines 22-34 says an unregistered verb emotes to the room. Replace its middle paragraph:

```go
// ⚠️ Nothing validates this at runtime. world.go's TryCommand falls through to
// an unhandled-command path that makes the mob emote
//
//	<name> looks a little confused (<cmd> <rest>).
//
// to the entire room -- so an unregistered verb is not a silent no-op, it is
// visible spam attached to every mob running that planner, every tick.
```

with:

```go
// ⚠️ Nothing validates this at runtime. world.go's TryCommand falls through to
// an unhandled-command path that logs a warning (#302; it used to emote
// "<name> looks a little confused (<cmd> <rest>)." to the room). Either way
// the mob does nothing that tick, every tick.
```

and change the assertion message text `"planner emote \"looks a little confused (%s ...)\" to the "+"whole room. Register it` so it reads `"planner do nothing and log an unhandled command (%s) every tick. Register it`. Keep the `%s` arguments matching the format verbs.

- [ ] **Step 5: Run both tests**

Run: `go test . -run TestNoteUnhandledMobCommand -count=1 && go test ./internal/mobcommands/ -run TestEveryPlannerCommandIsRegistered -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add world.go world_unhandled_mob_command_test.go internal/mobcommands/planner_command_guard_test.go
git commit -m "fix(mobs): an unhandled mob command is logged once, not emoted to the room (#302)"
```

---

### Task 8: #284 Saves with no surrender policy get the default

**Files:**
- Modify: `internal/characters/submission_policy.go` (add the default)
- Modify: `internal/characters/character.go:432`
- Modify: `internal/characters/validate.go` (inside `Validate`, after the machine initialisation)
- Create: `internal/characters/surrender_policy_default_test.go`

- [ ] **Step 1: Write the failing test**

```go
package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #284: a save with no surrender_policy key loaded {AutoTap, 0}, which no
// player can choose (the parser takes 1-100) and which status printed as
// "auto-tap-below 0". Validate gives it the new-character default.
func TestValidate_MissingSurrenderPolicyGetsTheDefault(t *testing.T) {
	c := New()
	c.SurrenderPolicy = SurrenderPolicy{}
	require.NoError(t, c.Validate())
	assert.Equal(t, DefaultPlayerSurrenderPolicy, c.SurrenderPolicy)
	assert.Equal(t, "auto-tap-below 15", c.SurrenderPolicy.String())
}

// A policy the player chose is kept.
func TestValidate_ChosenSurrenderPolicyIsKept(t *testing.T) {
	c := New()
	c.SurrenderPolicy = SurrenderPolicy{Mode: SurrenderNever}
	require.NoError(t, c.Validate())
	assert.Equal(t, SurrenderPolicy{Mode: SurrenderNever}, c.SurrenderPolicy)
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/characters/ -run 'TestValidate_.*SurrenderPolicy' -count=1`
Expected: FAIL to compile, `undefined: DefaultPlayerSurrenderPolicy`. If `c.Validate()` errors or panics for an unrelated reason in a bare `New()` character, copy the setup an existing `Validate` test in `internal/characters` uses (grep `\.Validate()` in `*_test.go`).

- [ ] **Step 3: Add the default and use it**

In `submission_policy.go`, after the `SurrenderPolicy` struct:

```go
// DefaultPlayerSurrenderPolicy is a new character's surrender policy, and the
// one Validate gives a save that has none (#284).
var DefaultPlayerSurrenderPolicy = SurrenderPolicy{Mode: SurrenderAutoTap, HpPctThreshold: 15}
```

In `character.go:432`, replace `SurrenderPolicy:            SurrenderPolicy{Mode: SurrenderAutoTap, HpPctThreshold: 15},` with `SurrenderPolicy:            DefaultPlayerSurrenderPolicy,` (keep the column alignment `gofmt` gives).

In `validate.go`, inside `Validate`, directly after the `if c.Perception == nil { ... }` block:

```go
	// A save written before surrender_policy existed loads the zero value,
	// auto-tap below 0, which no player can choose and which status printed
	// as "auto-tap-below 0" (#284). Mobs set their own policy at spawn.
	if c.SurrenderPolicy == (SurrenderPolicy{}) {
		c.SurrenderPolicy = DefaultPlayerSurrenderPolicy
	}
```

- [ ] **Step 4: Run the tests and the package**

Run: `gofmt -l internal/characters && go test ./internal/characters/ -count=1`
Expected: no gofmt output; PASS

- [ ] **Step 5: Commit**

```bash
git add internal/characters/submission_policy.go internal/characters/character.go internal/characters/validate.go internal/characters/surrender_policy_default_test.go
git commit -m "fix(characters): a save with no surrender policy gets the default (#284)"
```

---

### Task 9: #296 `help <admin command>` shows the admin help

**Files:**
- Modify: `internal/usercommands/help.go:85-89` and add a helper
- Create: `internal/usercommands/help_admin_command_test.go`

- [ ] **Step 1: Confirm the probe topic has no player help**

Run: `ls _datafiles/world/dogmud/templates/help/build.template _datafiles/world/dogmud/templates/admincommands/help/command.build.template`
Expected: the first is missing, the second exists. If `help/build.template` exists, pick another admin command from `admincommands/help/` with no `help/<name>.template` and use it in the test.

- [ ] **Step 2: Write the failing test**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #296: admin help lives in admincommands/help/command.<name>, but `help
// <name>` only looked in help/<name>, so `help build` found nothing.
func TestHelp_AdminCommandShowsAdminHelp(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	oldRole := user.Role
	t.Cleanup(func() { user.Role = oldRole })

	want, err := templates.Process("admincommands/help/command.build", nil, user.UserId)
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(want))

	user.Role = users.RoleAdmin
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Help("build", user, room, 0)
	require.NoError(t, err)
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.NotContains(t, out, "No help found")
	assert.Contains(t, out, strings.TrimSpace(strings.SplitN(want, "\n", 2)[0]))

	user.Role = users.RoleUser
	events.DrainQueuedMessagesForTest(user.UserId)
	Help("build", user, room, 0)
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"),
		`No help found for "build"`, "a player is not shown admin help")
}
```

If the first line of `want` is blank or only markup, compare on the first non-blank line instead.

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestHelp_AdminCommandShowsAdminHelp -count=1`
Expected: FAIL, admin output contains "No help found".

- [ ] **Step 4: Route to the admin help**

In `Help`, replace:

```go
		helpTxt, err = GetHelpContents(rest)
		if err != nil {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`No help found for "%s"`, rest))
			return true, err
		}
```

with:

```go
		helpTxt, err = GetHelpContents(rest)
		if err != nil {
			adminTxt, ok := adminHelpFor(rest, user)
			if !ok {
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`No help found for "%s"`, rest))
				return true, err
			}
			helpTxt, err = adminTxt, nil
		}
```

Add after `Help`:

```go
// adminHelpFor renders an admin command's own help file,
// admincommands/help/command.<name>, for a user allowed to run that command
// (#296). Everyone else, and any topic with no such file, gets false.
func adminHelpFor(topic string, user *users.UserRecord) (string, bool) {
	args := util.SplitButRespectQuotes(strings.ToLower(topic))
	if len(args) == 0 {
		return ``, false
	}
	name := regexp.MustCompile(`[^a-z0-9\-]+`).ReplaceAllString(args[0], ``)
	if name == `` || !user.HasRolePermission(name, true) {
		return ``, false
	}
	tpl := `admincommands/help/command.` + name
	if !templates.Exists(tpl) {
		return ``, false
	}
	out, err := templates.Process(tpl, nil, user.UserId)
	if err != nil {
		return ``, false
	}
	return out, true
}
```

All imports used here are already in `help.go`.

- [ ] **Step 5: Run the help tests**

Run: `go test ./internal/usercommands/ -run 'Help' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/help.go internal/usercommands/help_admin_command_test.go
git commit -m "fix(help): help <admin command> shows the admin help to admins (#296)"
```

---

### Task 10: #270 Arm and wrist slots read the same everywhere

Owner call 2026-10-10: extra arms read "arm 3" to "arm 6"; wrists read "wrist 1" to "wrist 6". Internal slot keys ("extra arm 2", "extra wrist 1", "worn - wrist2") do not change; only text a player reads does.

**Files:**
- Modify: `internal/characters/hand_slots.go` (add `ArmDisplayName`)
- Modify: `internal/actions/remove_equip.go:19,41`
- Modify: `internal/characters/worn.go:61`
- Modify: `internal/characters/inventory.go:521-536`
- Modify: `_datafiles/world/dogmud/templates/character/inventory.template:26,28`
- Modify: `_datafiles/world/dogmud/templates/character/inventory-look.template:27,29`
- Create: `internal/characters/arm_wrist_labels_test.go`

- [ ] **Step 1: Confirm the FindItem sources are display only**

Run: `grep -rn '"extra arm"\|"worn - wrist"\|"extra wrist [1-4]"\|"extra arm [34]"' --include=*.go internal modules | grep -v _test`
Expected: hits only in `inventory.go` (the FindItem list), `worn.go` (`GetSlotPointer` cases), and `enchant_slot.go` (its own key list). If any other file compares a `FindItem` source to one of these strings, stop and report.

- [ ] **Step 2: Write the failing test**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #270: the same extra arm read "extra arm 1" on equip, "Arm 3" in the
// equipment list and "extra arm" on look. Owner call 2026-10-10: arms read
// arm 3 to arm 6, wrists read wrist 1 to wrist 6.
func TestArmDisplayName_MatchesTheEquipmentList(t *testing.T) {
	c := New()
	c.ExtraArms = 4

	assert.Equal(t, "weapon hand", c.ArmDisplayName(1))
	assert.Equal(t, "offhand", c.ArmDisplayName(2))
	for arm, want := range map[int]string{3: "arm 3", 4: "arm 4", 5: "arm 5", 6: "arm 6"} {
		assert.Equal(t, want, c.ArmDisplayName(arm))
	}
	assert.Equal(t, "", c.ArmDisplayName(7))

	c.ExtraArms = 0
	assert.Equal(t, "", c.ArmDisplayName(3), "no such arm")
}

func TestAllSlots_WristsAreNumbered(t *testing.T) {
	var w Worn
	labels := map[string]string{}
	for _, s := range w.AllSlots() {
		labels[s.Key] = s.Label
	}
	assert.Equal(t, "Wrist 1", labels["wrist1"])
	assert.Equal(t, "Wrist 2", labels["wrist2"])
	assert.Equal(t, "Wrist 3", labels["extrawrist1"])
	assert.Equal(t, "Arm 3", labels["extraarm1"])
}

func TestFindItem_ArmAndWristSourcesUseSlotNumbers(t *testing.T) {
	c := New()
	mk := func(id int, name string) items.Item {
		return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name, Type: items.Object}}
	}
	c.Equipment.ExtraArm1 = mk(97001, "Probe Club")
	c.Equipment.Wrist2 = mk(97002, "Probe Bangle")
	c.Equipment.ExtraWrist1 = mk(97003, "Probe Cuff")

	_, src, ok := c.FindItem("probe club")
	assert.True(t, ok)
	assert.Equal(t, "arm 3", src)
	_, src, _ = c.FindItem("probe bangle")
	assert.Equal(t, "worn - wrist 2", src)
	_, src, _ = c.FindItem("probe cuff")
	assert.Equal(t, "worn - wrist 3", src)
}
```

If `items.Item{Spec: ...}` is not how this package's tests build an item, copy the helper `armItem` uses in `internal/usercommands/equip_arm_slot_test.go`.

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/characters/ -run 'TestArmDisplayName|TestAllSlots_WristsAreNumbered|TestFindItem_ArmAndWristSources' -count=1`
Expected: FAIL to compile, `undefined: ArmDisplayName`.

- [ ] **Step 4: Add `ArmDisplayName`**

In `hand_slots.go`, after `ArmLabel`:

```go
// ArmDisplayName is how a player reads arm N (1 to 6): "weapon hand",
// "offhand", then "arm 3" to "arm 6", as the equipment list numbers them
// (#270). "" when the character has no such arm. ArmLabel stays the slot key.
func (c *Character) ArmDisplayName(arm int) string {
	if c.ArmLabel(arm) == `` {
		return ``
	}
	switch arm {
	case 1:
		return `weapon hand`
	case 2:
		return `offhand`
	}
	return fmt.Sprintf(`arm %d`, arm)
}
```

Add `"fmt"` to the imports if absent.

- [ ] **Step 5: Use it on the equip line**

In `internal/actions/remove_equip.go`, change line 41 from `res.ArmLabel = actor.GetCharacter().ArmLabel(arm)` to `res.ArmLabel = actor.GetCharacter().ArmDisplayName(arm)`, and the field comment on line 19 to:

```go
	// ArmLabel is how the player reads the arm the item went into ("weapon
	// hand", "offhand", "arm 3"), set only by EquipItemInArm.
```

- [ ] **Step 6: Number the wrists and fix the FindItem sources**

In `worn.go:61`, change `{"wrist1", "Wrist", &w.Wrist1}, {"wrist2", "Wrist", &w.Wrist2},` to `{"wrist1", "Wrist 1", &w.Wrist1}, {"wrist2", "Wrist 2", &w.Wrist2},`.

In `inventory.go`, in the `slotItems` list inside `FindItem`, change these entries:

```go
		{c.Equipment.ExtraArm1, "arm 3"},
		{c.Equipment.ExtraArm2, "arm 4"},
		{c.Equipment.ExtraArm3, "arm 5"},
		{c.Equipment.ExtraArm4, "arm 6"},
```

```go
		{c.Equipment.Wrist1, "worn - wrist 1"},
		{c.Equipment.Wrist2, "worn - wrist 2"},
		{c.Equipment.ExtraWrist1, "worn - wrist 3"},
		{c.Equipment.ExtraWrist2, "worn - wrist 4"},
		{c.Equipment.ExtraWrist3, "worn - wrist 5"},
		{c.Equipment.ExtraWrist4, "worn - wrist 6"},
```

- [ ] **Step 7: Number the natural wrists in both inventory templates**

In `inventory.template` lines 26 and 28, and `inventory-look.template` lines 27 and 29, change `<ansi fg="yellow">Wrist:      </ansi>` to `<ansi fg="yellow">Wrist 1:    </ansi>` (first wrist) and `<ansi fg="yellow">Wrist 2:    </ansi>` (second). The label plus padding stays 12 characters, so columns still line up with "Wrist 3:    ".

- [ ] **Step 8: Run the tests and every package that asserts on these strings**

Run: `go test ./internal/characters/ ./internal/actions/ ./internal/usercommands/ ./modules/gmcp/ -count=1`
Expected: PASS. A test that asserted the old text ("extra arm 1", "Wrist:", "in your wielded") is updated to the new label; a test that asserted a slot key is a sign Step 1 missed a consumer, so stop and report.

- [ ] **Step 9: Commit**

```bash
git add internal/characters/hand_slots.go internal/characters/worn.go internal/characters/inventory.go internal/characters/arm_wrist_labels_test.go internal/actions/remove_equip.go _datafiles/world/dogmud/templates/character/inventory.template _datafiles/world/dogmud/templates/character/inventory-look.template
git commit -m "fix(equipment): arms read arm 3-6 and wrists wrist 1-6 everywhere (#270)"
```

(Add any test file Step 8 updated.)

---

### Task 11: Copy, YAML and comments (#307, #361, #357, #306, #305)

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/attack.template:82-85`
- Modify: `_datafiles/world/dogmud/items/armor-20000/light/20096-torch.yaml`, `20097-hooded_lantern.yaml`, `20098-umbral_lantern.yaml`
- Modify: `internal/state/activity/transitions.go:36`, `internal/state/activity/context.md:151`
- Modify: `internal/behaviortree/engine.go:82-94`
- Modify: `_datafiles/world/dogmud/mobs/newcomer_antechamber/9614-straw_effigy.yaml:27-34`

- [ ] **Step 1: #307 attack help**

In `attack.template`, replace:

```
When fighting multiple enemies, use <ansi fg="command">attack <target></ansi> to switch targets:
  - Immediately retargets to the new enemy
  - Automatic retargeting when your current target dies
  - Party members can coordinate attacks on the same target
```

with:

```
When fighting multiple enemies, use <ansi fg="command">attack <target></ansi> to switch targets:
  - Switching mid-fight takes skill and quick hands. If it works, you turn
    to the new enemy after a round. If it fails, you stay on your old
    target and lose the round.
  - If your target is already dead or gone, the switch is immediate
  - Party members can coordinate attacks on the same target
```

Each line is under 80 visible columns once the ansi tags are stripped. Run `go test ./internal/usercommands/ -run 'Help' -count=1` and any helpfile width guard (`grep -rln "VisibleWidth" *_test.go internal/usercommands/*_test.go | head`) that covers help templates.

- [ ] **Step 2: #361 rarity tiers**

Insert one line directly after the `value:` line in each file:
- `20096-torch.yaml`: `rarity_tier: 50`
- `20097-hooded_lantern.yaml`: `rarity_tier: 40`
- `20098-umbral_lantern.yaml`: `rarity_tier: 30`

Run: `go test ./internal/items/ -run 'Light|Rarity' -count=1` and `go test . -run 'Rarity' -count=1`.

- [ ] **Step 3: #357 dead constant**

Delete `	TriggerCombatInterrupt    = "combat_interrupt"    // Crafting / Salvaging` from `transitions.go:36` and the `TriggerCombatInterrupt` table row from `internal/state/activity/context.md:151`. Run `gofmt -w internal/state/activity/transitions.go` (the const block realigns), then `go build ./... && go test ./internal/state/... -count=1`.

- [ ] **Step 4: #306 behaviour tree comment**

In `engine.go`, replace the block from `// Negative cache (noTree / noRoomTree) — design note.` through `// TODO(hot-reload): bust cache on file change if/when hot-reload is added.` with:

```go
// Negative cache (noTree / noRoomTree): design note.
//
// The negative cache records mob/room ids whose behavior tree YAML does
// not exist on disk (or whose load failed at file-stat time). An entry
// clears on a successful LoadTree / LoadRoomTree, or when hot-reload
// evicts it: EvictTree, EvictRoomTree, EvictArchetype and EvictItemTree
// below drop both the cached tree and the negative entry, so a file that
// appears on disk is picked up without a restart.
```

Before writing it, read `EvictTree` and `EvictRoomTree` and confirm each deletes from `noTree` / `noRoomTree`. If one does not, say only what is true (name the evictors that do clear it) and report the gap.

- [ ] **Step 5: #305 effigy comment**

In `9614-straw_effigy.yaml`, replace the comment lines from `# Species 19 (dummy) has base vitality 200` through `# so there is no time pressure -- make it effectively unkillable.` with:

```yaml
    # Species 19 (dummy) has base vitality 200 and zero base damage. A large
    # vitality bump keeps the effigy standing through the lesson (no engine
    # "invulnerable" flag exists; high HP is the mechanism). The player is
    # meant to practice on it and FLEE it -- a live target must remain for
    # the cast-spike and trip steps of quest 28. Was 1000, which a slow,
    # confused player auto-swinging for many rounds could still drop. It is
    # hard to kill, not impossible: a veteran with two companions killed it
    # in the U12c-2 playtest. It respawns, so that is harmless.
```

Keep the indentation the file uses. The `--` already in this comment is in a YAML comment, not player text.

- [ ] **Step 6: Check content loads**

Run: `go test . -run 'Content|Guard|Item' -count=1` (root guards that load shipped YAML). Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/attack.template _datafiles/world/dogmud/items/armor-20000/light/20096-torch.yaml _datafiles/world/dogmud/items/armor-20000/light/20097-hooded_lantern.yaml _datafiles/world/dogmud/items/armor-20000/light/20098-umbral_lantern.yaml internal/state/activity/transitions.go internal/state/activity/context.md internal/behaviortree/engine.go _datafiles/world/dogmud/mobs/newcomer_antechamber/9614-straw_effigy.yaml
git commit -m "fix: attack help, light rarity tiers, dead constant, stale comments (#307 #361 #357 #306 #305)"
```

---

### Task 12: Gate, patch notes, PR

**Files:**
- Modify: `docs/PATCH_NOTES.md` (append; the M6 session also appends, whoever merges second rebases)

- [ ] **Step 1: Full pre-push gate**

Load the `dogmud-shipping` skill and run its pre-push gate in its order from this worktree. Expected: all green. A failure in a test this plan did not touch: check whether it fails on master too (`git stash` is shared across worktrees, so use a clean detached worktree of master, not stash) before treating it as ours.

- [ ] **Step 2: Patch notes**

Append one player-facing entry to `docs/PATCH_NOTES.md` in its existing format, written per `dogmud-player-copy`: items named "Gold..." can be picked up; a refused party invite no longer leaves you in an empty party; arms and wrists are numbered the same in every message; `help attack` explains target switching; status shows a real surrender policy. Admin-side items (#461, #271, #296, #289) go in the admin section if the file has one.

- [ ] **Step 3: Local boot smoke**

Per `dogmud-shipping` (detached worktree boot, instance-save wipe) and `dogmud-playtesting` (never kill the user's server; kill only your own PID): as an admin run `room info`, `quests all+`, `help build`, `reload items` with one item file broken then restored; as a player run `quests all+` and `help build`. Record what each printed.

- [ ] **Step 4: PR**

```bash
git push -u origin worktree-loose-issues-sweep
gh pr create --repo pruuk/DOGMud --base master --title "Loose issues sweep: 14 small fixes" --body-file <scratchpad file>
```

The body lists each issue as "Fixes #N" on its own line (these SHOULD auto-close at merge), names #249, #236, #217 and #440 as left out WITHOUT a closing keyword in front of them, summarises the smoke results, and ends with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. The owner merges and deploys.
