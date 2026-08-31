package spells

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ⚠️ Owner found Meirok had learned `core-drain` in play — the Core Guardian's
// signature recharge, whose own description reads "Mob-castable only ... not a
// spell any living caster could learn". Nothing enforced that; it was prose.
//
// The discovery pool filtered known / quest-gated / school / skill and had no
// mob-only concept at all.
func TestGetEligibleSpells_ExcludesMobOnlyForPlayers(t *testing.T) {
	restore := allSpells
	t.Cleanup(func() { allSpells = restore })
	allSpells = map[string]*SpellData{
		"core-drain":   {SpellId: "core-drain", Difficulty: 45, MobOnly: true, Schools: []string{SchoolVital}},
		"repair-pulse": {SpellId: "repair-pulse", Difficulty: 10, MobOnly: true, Schools: []string{SchoolVital}},
		"mend-wounds":  {SpellId: "mend-wounds", Difficulty: 10, Schools: []string{SchoolVital}},
	}

	player := GetEligibleSpells(map[string]int{}, 99, SchoolVital)
	assert.NotContains(t, player, "core-drain", "a player must never discover a boss ability")
	assert.NotContains(t, player, "repair-pulse",
		"repair-pulse is difficulty 10 — reachable by a near-novice, which is why the skill gate alone is not enough")
	assert.Contains(t, player, "mend-wounds", "ordinary spells must still be discoverable")

	// ⚠️ Mobs learn through the SAME seam. Excluding these outright would stop a
	// Core Guardian ever learning its own signature ability.
	mob := GetEligibleSpellsForMob(map[string]int{}, 99, SchoolVital)
	assert.Contains(t, mob, "core-drain", "mobs must still be able to learn mob-only spells")
	assert.Contains(t, mob, "repair-pulse")
	assert.Contains(t, mob, "mend-wounds")
}

// The skill gate is NOT the protection, and this pins why. Meirok has
// spellcasting 51 against core-drain's difficulty 45, so he cleared the bar
// honestly. U10b-3 raised that bar from 0 to 45 and narrowed the exposure; it
// never closed it.
func TestGetEligibleSpells_SkillGateAloneDoesNotProtect(t *testing.T) {
	restore := allSpells
	t.Cleanup(func() { allSpells = restore })
	allSpells = map[string]*SpellData{
		"core-drain": {SpellId: "core-drain", Difficulty: 45, MobOnly: true, Schools: []string{SchoolVital}},
	}

	require.GreaterOrEqual(t, 51, RequiredSkillFor(45),
		"fixture assumption: a spellcasting-51 caster clears difficulty 45")
	assert.Empty(t, GetEligibleSpells(map[string]int{}, 51, SchoolVital),
		"clearing the skill bar must not be enough to discover a boss ability")
}

// ⚠️ DRIFT GUARD. The flag and the authored prose must not disagree. Every
// spell whose own description claims it is mob-castable only must carry
// mob_only: true, or the next one authored that way leaks into the player pool
// exactly as core-drain did.
func TestSpellFiles_MobOnlyProseMatchesFlag(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..",
		"_datafiles", "world", "dogmud", "spells")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "spell directory must be readable")
	require.NotEmpty(t, entries)

	claimsMobOnly := regexp.MustCompile(`(?i)mob.castable only|mob.cast only|not a spell any living caster|mob only`)
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		body := string(raw)
		if !claimsMobOnly.MatchString(body) {
			continue
		}
		checked++
		assert.Regexp(t, `(?m)^mob_only:\s*true`, body,
			"%s says it is mob-only in prose but does not set mob_only: true", e.Name())
	}
	assert.GreaterOrEqual(t, checked, 3,
		"expected at least the three known mob-only spells; if this drops, the regex stopped matching and the guard is vacuous")
}
