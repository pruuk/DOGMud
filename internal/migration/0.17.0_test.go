package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const oldUserSave = `userid: 7
username: probe
character:
  name: Probe
  buffs:
    list:
    - buffid: 122
      triggersleft: 4
      stacks:
      - roundsleft: 3
        amount: -2
    - buffid: 79
      permabuff: true
  pet:
    type: dog
    buffids: [12, 13]
  shop:
  - itemid: 5
  - buffid: 4
    price: 10
  miscdata:
    pinnacle_bandolier_buffs: [51]
    unrelated: keep
`

const oldAltsSave = `- name: AltOne
  buffs:
    list:
    - buffid: 3
      permabuff: true
- name: AltTwo
  pet:
    type: cat
    buffids: [9]
`

const oldRoomInstance = `roomid: 1001
containers:
  chest:
    lock:
      difficulty: 3
      trapbuffids: [45]
    gold: 7
  crate:
    gold: 1
`

// overrideItemBody is an enchanted item whose saved spec copy carries every
// buff-spelled item key.
const overrideItemBody = `itemid: 20061
enchantments: 1
overrides:
  itemid: 20061
  name: Zephyr Treads
  buffids: [5]
  wornbuffids: [98]
  damage:
    diceroll: 1d4
    critbuffids: [31, 32]
enchantbaseline:
  damage:
    diceroll: 1d2
    critbuffids: [33]
`

func indentItem(first, rest string) string {
	lines := strings.Split(strings.TrimSuffix(overrideItemBody, "\n"), "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = first + lines[i]
		} else {
			lines[i] = rest + lines[i]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// overrideItem renders the item as a list element at indent.
func overrideItem(indent string) string { return indentItem(indent+"- ", indent+"  ") }

// overrideValue renders the item as a mapping value at indent.
func overrideValue(indent string) string { return indentItem(indent, indent) }

// assertOverrideItem checks one migrated overrideItem decoded into items.Item.
func assertOverrideItem(t *testing.T, it items.Item) {
	t.Helper()
	require.NotNil(t, it.Spec, "overrides block must survive")
	assert.Equal(t, []int{5}, it.Spec.ConditionIds)
	assert.Equal(t, []int{98}, it.Spec.WornConditionIds)
	assert.Equal(t, []int{31, 32}, it.Spec.Damage.CritConditionIds)
	require.NotNil(t, it.EnchantBaseline)
	assert.Equal(t, []int{33}, it.EnchantBaseline.Damage.CritConditionIds)
}

func writeFixture(t *testing.T, dir, rel, body string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0644))
	return p
}

func readMigrated(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	for _, old := range []string{"buffid", "buffids:", "permabuff", "pinnacle_bandolier_buffs", "BuffsEnabled"} {
		assert.NotContains(t, string(raw), old, "%s still carries %s", p, old)
	}
	return raw
}

// assertUnchanged proves a file was not rewritten: exact bytes and mtime.
func assertUnchanged(t *testing.T, p, body string, before time.Time) {
	t.Helper()
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, body, string(raw))
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, before, info.ModTime(), "the mtime is what proves the file was not rewritten; byte equality alone can coincide with a rewrite")
}

func mtime(t *testing.T, p string) time.Time {
	t.Helper()
	info, err := os.Stat(p)
	require.NoError(t, err)
	return info.ModTime()
}

func TestConditionKeys_UserSave(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/7.yaml", oldUserSave)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	raw := readMigrated(t, p)
	assert.NotContains(t, string(raw), "buffs:")

	var u users.UserRecord
	require.NoError(t, yaml.Unmarshal(raw, &u))
	require.Len(t, u.Character.Conditions.List, 2)
	assert.Equal(t, 122, u.Character.Conditions.List[0].ConditionId)
	assert.Len(t, u.Character.Conditions.List[0].Stacks, 1)
	assert.Equal(t, 79, u.Character.Conditions.List[1].ConditionId)
	assert.True(t, u.Character.Conditions.List[1].Permanent)
	assert.Equal(t, []int{12, 13}, u.Character.Pet.ConditionIds)
	require.Len(t, u.Character.Shop, 2)
	assert.Equal(t, 4, u.Character.Shop[1].ConditionId)
	assert.NotNil(t, u.Character.MiscData["pinnacle_bandolier_conditions"])
	assert.Equal(t, "keep", u.Character.MiscData["unrelated"])
}

func TestConditionKeys_UserSaveItemOverrides(t *testing.T) {
	dir := t.TempDir()
	body := "userid: 11\nusername: enchanter\ncharacter:\n  name: Enchanter\n  items:\n" +
		overrideItem("  ") +
		"  equipment:\n    feet:\n" + overrideValue("      ") +
		"itemstorage:\n  slots:\n  - count: 1\n    item:\n" + overrideValue("      ") +
		"  items:\n" + overrideItem("  ")
	p := writeFixture(t, dir, "users/11.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	raw := readMigrated(t, p)
	var u users.UserRecord
	require.NoError(t, yaml.Unmarshal(raw, &u))
	require.Len(t, u.Character.Items, 1)
	assertOverrideItem(t, u.Character.Items[0])
	assertOverrideItem(t, u.Character.Equipment.Feet)
	require.Len(t, u.ItemStorage.Slots, 1)
	assertOverrideItem(t, u.ItemStorage.Slots[0].Item)
	require.Len(t, u.ItemStorage.Items, 1)
	assertOverrideItem(t, u.ItemStorage.Items[0])
}

func TestConditionKeys_MobInstanceEquipment(t *testing.T) {
	dir := t.TempDir()
	body := "schema_version: 1\nequipment:\n  weapon:\n" + overrideValue("    ")
	p := writeFixture(t, dir, "mobs.instances/frostfang/55-1001.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var d mobs.MobInstanceData
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &d))
	require.NotNil(t, d.Equipment)
	assertOverrideItem(t, d.Equipment.Weapon)
}

func TestConditionKeys_ShopAffixedStock(t *testing.T) {
	dir := t.TempDir()
	body := "gold: 100\nstarting_gold: 100\nlast_restock: 0\ninventory: []\naffixed_stock:\n- price: 40\n  item:\n" +
		overrideValue("    ")
	p := writeFixture(t, dir, "shops/greenford/12-room300.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var inv shops.ShopInventory
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &inv))
	require.Len(t, inv.AffixedStock, 1)
	assertOverrideItem(t, inv.AffixedStock[0].Item)
	assert.Equal(t, 40, inv.AffixedStock[0].Price)
}

func TestConditionKeys_GuildVault(t *testing.T) {
	dir := t.TempDir()
	body := "tag: WOLF\nname: Wolves\nleaderuserid: 3\nmembers:\n- userid: 3\n  charactername: Alpha\n  rank: leader\n  joined: 2026-09-01T10:02:03.000000456Z\ncreated: 2026-09-01T10:02:03Z\nvault:\n" + overrideItem("")
	p := writeFixture(t, dir, "guilds/wolf.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var g guilds.Guild
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &g))
	require.Len(t, g.Vault, 1)
	assertOverrideItem(t, g.Vault[0])
	require.Len(t, g.Members, 1)
	assert.Equal(t, time.Date(2026, 9, 1, 10, 2, 3, 456, time.UTC), g.Members[0].Joined.UTC(), "a timestamp must survive the round trip")
}

// genericPath walks a decoded document by key names and list indexes.
func genericPath(t *testing.T, raw []byte, path ...any) any {
	t.Helper()
	var node any
	require.NoError(t, yaml.Unmarshal(raw, &node))
	for _, seg := range path {
		switch s := seg.(type) {
		case string:
			m, ok := node.(map[any]any)
			require.True(t, ok, "expected a mapping at %q", s)
			v, ok := m[s]
			require.True(t, ok, "missing key %q", s)
			node = v
		case int:
			l, ok := node.([]any)
			require.True(t, ok, "expected a list at %d", s)
			require.Greater(t, len(l), s)
			node = l[s]
		}
	}
	return node
}

func TestConditionKeys_SealedCrate(t *testing.T) {
	dir := t.TempDir()
	body := "roomid: 4038\ncapacity: 10\nitems:\n" + overrideItem("")
	p := writeFixture(t, dir, "crates/4038-fernway_shipment.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	// cratePayload is unexported; its Items field is []items.Item.
	var payload struct {
		Items []items.Item `yaml:"items"`
	}
	raw := readMigrated(t, p)
	require.NoError(t, yaml.Unmarshal(raw, &payload))
	require.Len(t, payload.Items, 1)
	assertOverrideItem(t, payload.Items[0])
}

func TestConditionKeys_AuctionPluginData(t *testing.T) {
	dir := t.TempDir()
	body := "ActiveAuction:\n  itemdata:\n" + overrideValue("    ") +
		"  sellername: Seller\nSeizedQueue:\n- Item:\n" + overrideValue("    ") +
		"  Count: 1\n"
	p := writeFixture(t, dir, "plugin-data/auctions-v1.0/auctionhistory.plugin.dat", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	raw := readMigrated(t, p)
	assert.Equal(t, []any{98}, genericPath(t, raw, "SeizedQueue", 0, "Item", "overrides", "wornconditionids"))
	assert.Equal(t, []any{98}, genericPath(t, raw, "ActiveAuction", "itemdata", "overrides", "wornconditionids"))
	assert.Equal(t, []any{33}, genericPath(t, raw, "ActiveAuction", "itemdata", "enchantbaseline", "damage", "critconditionids"))
}

func TestConditionKeys_ConfigOverrides(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "config-overrides.yaml", "Server:\n  NextRoomId: 6471\nModules:\n  weather:\n    BuffsEnabled: false\n")
	require.NoError(t, migrateConditionKeysIn(dir, false))

	raw := readMigrated(t, p)
	assert.Equal(t, false, genericPath(t, raw, "Modules", "weather", "ConditionsEnabled"))
	assert.Equal(t, 6471, genericPath(t, raw, "Server", "NextRoomId"))
}

func TestConditionKeys_OverridesOutsideDataFilesAndReload(t *testing.T) {
	dir := t.TempDir()
	outside := writeFixture(t, t.TempDir(), "config-production.yaml", "Modules:\n  weather:\n    BuffsEnabled: false\n")
	reloads := 0
	reload := func() error { reloads++; return nil }

	require.NoError(t, migrateConditionKeys(dir, outside, false, reload))
	assert.Equal(t, false, genericPath(t, readMigrated(t, outside), "Modules", "weather", "ConditionsEnabled"))
	assert.Equal(t, 1, reloads, "a rewritten overrides file must be reloaded, or Run's SetVal writes the stale in-memory key back")

	require.NoError(t, migrateConditionKeys(dir, outside, false, reload))
	assert.Equal(t, 1, reloads, "an untouched overrides file needs no reload")

	inside := writeFixture(t, dir, "config-overrides.yaml", "Modules:\n  weather:\n    BuffsEnabled: true\n")
	require.NoError(t, migrateConditionKeys(dir, inside, false, reload))
	assert.Equal(t, 2, reloads, "an overrides file under DataFiles is rewritten once by the walk and reloaded")
	assert.Equal(t, true, genericPath(t, readMigrated(t, inside), "Modules", "weather", "ConditionsEnabled"))

	body := "Modules:\n  weather:\n    BuffsEnabled: true\n"
	dry := writeFixture(t, t.TempDir(), "o.yaml", body)
	require.NoError(t, migrateConditionKeys(dir, dry, true, reload))
	assert.Equal(t, 2, reloads, "a dry run never reloads")
	raw, err := os.ReadFile(dry)
	require.NoError(t, err)
	assert.Equal(t, body, string(raw))
}

func TestConditionKeys_AltsList(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/7.alts.yaml", oldAltsSave)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var alts []characters.Character
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &alts))
	require.Len(t, alts, 2)
	require.Len(t, alts[0].Conditions.List, 1)
	assert.Equal(t, 3, alts[0].Conditions.List[0].ConditionId)
	assert.True(t, alts[0].Conditions.List[0].Permanent)
	assert.Equal(t, []int{9}, alts[1].Pet.ConditionIds)
}

func TestConditionKeys_LegacyAltsList(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/someone-alts.yaml", oldAltsSave)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var alts []characters.Character
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &alts))
	require.Len(t, alts, 2)
	require.Len(t, alts[0].Conditions.List, 1)
	assert.Equal(t, 3, alts[0].Conditions.List[0].ConditionId)
	assert.Equal(t, []int{9}, alts[1].Pet.ConditionIds)
}

func TestConditionKeys_RoomInstance(t *testing.T) {
	dir := t.TempDir()
	body := oldRoomInstance + "items:\n" + overrideItem("") + "stash:\n" + overrideItem("")
	body = strings.Replace(body, "    gold: 7\n", "    gold: 7\n    items:\n"+overrideItem("    "), 1)
	p := writeFixture(t, dir, "rooms.instances/frostfang/1001.yaml", body)
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var r rooms.Room
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &r))
	assert.Equal(t, []int{45}, r.Containers["chest"].Lock.TrapConditionIds)
	assert.Equal(t, 1, r.Containers["crate"].Gold)
	require.Len(t, r.Containers["chest"].Items, 1)
	assertOverrideItem(t, r.Containers["chest"].Items[0])
	require.Len(t, r.Items, 1)
	assertOverrideItem(t, r.Items[0])
	require.Len(t, r.Stash, 1)
	assertOverrideItem(t, r.Stash[0])
}

func TestConditionKeys_GenericKeyWithoutListIsKept(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/12.yaml", "character:\n  name: Keep\n  miscdata:\n    buffs: 3\n  buffs:\n    list: []\n")
	require.NoError(t, migrateConditionKeysIn(dir, false))

	var u users.UserRecord
	require.NoError(t, yaml.Unmarshal(readMigrated(t, p), &u))
	assert.Equal(t, 3, u.Character.MiscData["buffs"], "a buffs key that is not a {list: ...} record must not be renamed")
	assert.Nil(t, u.Character.MiscData["conditions"])
}

func TestConditionKeys_OldSpellingValueIsNotRewritten(t *testing.T) {
	dir := t.TempDir()
	body := "character:\n  name: Talker\n  miscdata:\n    keywords: [\"condition\", \"buff\"]\n"
	p := writeFixture(t, dir, "users/13.yaml", body)
	before := mtime(t, p)
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, migrateConditionKeysIn(dir, false))
	assertUnchanged(t, p, body, before)
}

func TestConditionKeys_FilesWithoutOldSpellingAreNeverParsed(t *testing.T) {
	dir := t.TempDir()
	jsonBody := `{"cells":[{"x":1,"y":2}],"version":"0.2.0"}`
	jsonPath := writeFixture(t, dir, "plugin-data/weather-v0.2.0/simstate.plugin.dat", jsonBody)
	corruptBody := "name: [unclosed\n\t: : :\n"
	corruptPath := writeFixture(t, dir, "rooms/frostfang/1.yaml", corruptBody)
	jsonBefore, corruptBefore := mtime(t, jsonPath), mtime(t, corruptPath)

	time.Sleep(20 * time.Millisecond)
	require.NoError(t, migrateConditionKeysIn(dir, false), "a corrupt file with no buff spelling is not the migration's business")
	assertUnchanged(t, jsonPath, jsonBody, jsonBefore)
	assertUnchanged(t, corruptPath, corruptBody, corruptBefore)
}

func TestConditionKeys_SecondRunIsANoOp(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/7.yaml", oldUserSave)
	require.NoError(t, migrateConditionKeysIn(dir, false))
	first, err := os.ReadFile(p)
	require.NoError(t, err)
	before := mtime(t, p)

	time.Sleep(20 * time.Millisecond)
	require.NoError(t, migrateConditionKeysIn(dir, false))
	assertUnchanged(t, p, string(first), before)
}

func TestConditionKeys_UntouchedFileNotRewritten(t *testing.T) {
	dir := t.TempDir()
	body := "userid: 8\nusername: clean\ncharacter:\n  name: Clean\n"
	p := writeFixture(t, dir, "users/8.yaml", body)
	before := mtime(t, p)

	time.Sleep(20 * time.Millisecond)
	require.NoError(t, migrateConditionKeysIn(dir, false))
	assertUnchanged(t, p, body, before)
}

func TestConditionKeys_DryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "users/7.yaml", oldUserSave)
	require.NoError(t, migrateConditionKeysIn(dir, true))
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, oldUserSave, string(raw))
}

func TestConditionKeys_CollisionIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "users/9.yaml", "character:\n  buffs:\n    list: []\n  conditions:\n    list: []\n")
	err := migrateConditionKeysIn(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "9.yaml")
}

func TestConditionKeys_UnparseableIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "users/10.yaml", "character: [unclosed\n  buffs:\n")
	err := migrateConditionKeysIn(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.yaml")
}

func TestConditionKeys_MixedListWithOldKeyIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "misc/odd.yaml", "- 3\n- buffids: [1]\n")
	err := migrateConditionKeysIn(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "odd.yaml")
}

// TestConditionKeys_MultiDocumentWithOldKeyIsAnError proves the migration
// refuses a multi-document file rather than silently keeping only its first
// document, which is what decodeOrdered's yaml.v2 round trip does today.
func TestConditionKeys_MultiDocumentWithOldKeyIsAnError(t *testing.T) {
	dir := t.TempDir()
	body := "character:\n  buffs:\n    list:\n    - buffid: 3\n---\nusername: x\n"
	p := writeFixture(t, dir, "users/11.yaml", body)
	before := mtime(t, p)
	time.Sleep(20 * time.Millisecond)

	err := migrateConditionKeysIn(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "11.yaml")
	assertUnchanged(t, p, body, before)
}

// TestConditionKeys_MergeKeyWithOldKeyIsAnError proves the migration refuses
// a file using a YAML merge key rather than silently dropping the merged
// fields, which is what decodeOrdered's yaml.v2 round trip does today.
func TestConditionKeys_MergeKeyWithOldKeyIsAnError(t *testing.T) {
	dir := t.TempDir()
	body := "base: &b {gold: 1}\nstock:\n  <<: *b\n  item:\n    itemid: 5\n    overrides:\n      wornbuffids: [98]\n"
	p := writeFixture(t, dir, "shops/z/1.yaml", body)
	before := mtime(t, p)
	time.Sleep(20 * time.Millisecond)

	err := migrateConditionKeysIn(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1.yaml")
	assertUnchanged(t, p, body, before)
}

// TestConditionKeys_MergeKeyWithoutOldKeyIsIgnored proves the merge-key check
// only runs on a file the walker would otherwise rewrite: no old spelling
// means no parse, so a merge key elsewhere in DataFiles is not this
// migration's business.
func TestConditionKeys_MergeKeyWithoutOldKeyIsIgnored(t *testing.T) {
	dir := t.TempDir()
	body := "base: &b {gold: 1}\nstock:\n  <<: *b\n  item:\n    itemid: 5\n"
	p := writeFixture(t, dir, "shops/z/1.yaml", body)
	before := mtime(t, p)
	time.Sleep(20 * time.Millisecond)

	require.NoError(t, migrateConditionKeysIn(dir, false))
	assertUnchanged(t, p, body, before)
}

func TestConditionKeys_MissingFoldersAreNotErrors(t *testing.T) {
	require.NoError(t, migrateConditionKeysIn(t.TempDir(), false))
	require.NoError(t, migrateConditionKeysIn(filepath.Join(t.TempDir(), "absent"), false))
}
