package guards

import (
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Conditions unification slice 3 renamed every on-disk and wire spelling of
// buff to condition (docs/superpowers/specs/completed/2026-09-15-conditions-unification-slice-3-disk-wire-design.md).
// These tests pin the new literal keys, values and strings, and that the old
// ones no longer bind: loaders ignore unknown keys, so a stale key would load
// as a silent zero value.

func TestWireFreeze_CharacterSaveKeys(t *testing.T) {
	defer conditions.SeedConditionRecordsForTest()()

	c := characters.Character{}
	c.Conditions.Validate(true)
	require.True(t, c.Conditions.AddConditionMagnitude(conditions.ConditionIdBleeding, 3, -2))
	require.True(t, c.Conditions.AddConditionMagnitude(conditions.ConditionIdBleeding, 5, -3))
	c.Conditions.List = append(c.Conditions.List, &conditions.Condition{ConditionId: conditions.ConditionIdWarcry, Permanent: true, TriggersLeft: 1})

	out, err := yaml.Marshal(&c)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	assert.NotContains(t, doc, "buffs")
	held, ok := doc["conditions"].(map[any]any)
	require.True(t, ok, "the character save must use the `conditions:` key; got:\n%s", out)
	list, ok := held["list"].([]any)
	require.True(t, ok, "the held records must stay under `list:`")
	require.Len(t, list, 2)

	bleed := list[0].(map[any]any)
	assert.EqualValues(t, conditions.ConditionIdBleeding, bleed["conditionid"], "a record's id must be under `conditionid:`")
	assert.NotContains(t, bleed, "buffid")
	assert.Contains(t, bleed, "triggersleft")
	assert.Contains(t, bleed, "stacks")
	stack := bleed["stacks"].([]any)[0].(map[any]any)
	assert.Contains(t, stack, "roundsleft")
	assert.Contains(t, stack, "amount")

	warcry := list[1].(map[any]any)
	assert.Equal(t, true, warcry["permanent"], "a permanent record must be under `permanent:`")
	assert.NotContains(t, warcry, "permabuff")

	var back characters.Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Len(t, back.Conditions.List, 2)
	assert.Equal(t, conditions.ConditionIdBleeding, back.Conditions.List[0].ConditionId)
	assert.Len(t, back.Conditions.List[0].Stacks, 2)
	assert.True(t, back.Conditions.List[1].Permanent)

	var stale characters.Character
	require.NoError(t, yaml.Unmarshal([]byte("buffs:\n  list:\n  - buffid: 3\n    permabuff: true\n"), &stale))
	assert.Empty(t, stale.Conditions.List, "the old `buffs:` key must no longer bind; saves are migrated by 0.17.0")
}

func TestWireFreeze_ConditionFileKeys(t *testing.T) {
	var s conditions.ConditionSpec
	doc := "conditionid: 950\nname: Probe\ntriggerrate: 1 round\ntriggercount: 2\n" +
		"start_remove_conditions: [3]\neffects:\n  damage_mult: magnitude\nflags:\n  - stacking\n" +
		"tick_pool: health\ntick_from_magnitude: true\n"
	require.NoError(t, yaml.Unmarshal([]byte(doc), &s))
	assert.Equal(t, 950, s.ConditionId, "`conditionid:` must name the record")
	assert.Equal(t, "Probe", s.Name)
	assert.Equal(t, 2, s.TriggerCount)
	assert.Equal(t, "health", s.TickPool)
	assert.True(t, s.TickFromMagnitude)
	assert.Equal(t, []int{3}, s.StartRemoveConditions, "`start_remove_conditions:` must parse")
	assert.True(t, s.Effects[conditions.EffectDamageMult].UsesMagnitude)
	assert.Equal(t, []conditions.Flag{conditions.Stacking}, s.Flags)

	var old conditions.ConditionSpec
	require.NoError(t, yaml.Unmarshal([]byte("buffid: 950\nstart_remove_buffs: [3]\n"), &old))
	assert.Zero(t, old.ConditionId, "`buffid:` must no longer bind")
	assert.Empty(t, old.StartRemoveConditions, "`start_remove_buffs:` must no longer bind")
}

func TestWireFreeze_ShippedConditionFilesLoadInBothWorlds(t *testing.T) {
	// This package's TestMain sets up no logger; conditions.LoadDataFiles logs.
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	repo := repoRoot(t)
	for _, world := range []string{"dogmud", "default"} {
		t.Run(world, func(t *testing.T) {
			cfg := configs.GetConfig()
			cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repo, "_datafiles", "world", world))
			cfg.Network.LogoutRounds = 3
			configs.SetConfigForTest(t, cfg)
			restore := conditions.SeedConditionsForTest(nil)
			defer restore()
			conditions.LoadDataFiles()
			assert.NotEmpty(t, conditions.GetAllConditionIds(), "the %s world's conditions/ folder must load", world)
		})
	}
}

func TestWireFreeze_SpeciesAndSpellValues(t *testing.T) {
	var sp species.Species
	require.NoError(t, yaml.Unmarshal([]byte("name: Probe\nconditionids: [7]\n"), &sp))
	assert.Equal(t, []int{7}, sp.ConditionIds, "species `conditionids:` must parse")

	var spell spells.SpellData
	require.NoError(t, yaml.Unmarshal([]byte("spellid: probe\nname: Probe\neffect_type: condition\ncondition_ids: [3]\n"), &spell))
	assert.Equal(t, "condition", spell.EffectType)
	assert.Equal(t, []int{3}, spell.ConditionIds, "`condition_ids:` must parse")
}

func TestWireFreeze_MessagingCategoryStrings(t *testing.T) {
	assert.Equal(t, "condition-apply", messaging.CategoryConditionApply.String())
	assert.Equal(t, "condition-expire", messaging.CategoryConditionExpire.String())
}
