package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestFormattedName_MobUsesSmartTitleCase(t *testing.T) {
	f := FormattedName{Name: "captain of the guard", Type: "mobname"}
	out := f.String()
	if !strings.Contains(out, "Captain of the Guard") {
		t.Errorf("mob name not smart-title-cased; got %q", out)
	}
	if strings.Contains(out, "Of The") {
		t.Errorf("minor words should be lowercased; got %q", out)
	}
}

func TestGetFormattedAdjectives(t *testing.T) {
	// Setup: populate adjectiveSwaps with test data
	adjectiveSwaps = map[string]string{
		"charmed":       "♥friend",
		"charmed-short": "♥",
		"hidden":        "hidden",
		"hidden-short":  "?",
		"zombie":        "zOmBie",
	}

	tests := []struct {
		name         string
		excludeShort bool
		expected     []string
	}{
		{
			name:         "Include short adjectives",
			excludeShort: false,
			expected:     []string{"charmed", "charmed-short", "hidden", "hidden-short", "zombie"},
		},
		{
			name:         "Exclude short adjectives",
			excludeShort: true,
			expected:     []string{"charmed", "hidden", "zombie"},
		},
		{
			name:         "Empty adjectiveSwaps",
			excludeShort: false,
			expected:     []string{},
		},
	}

	for _, tt := range tests[:2] {
		t.Run(tt.name, func(t *testing.T) {
			got := GetFormattedAdjectives(tt.excludeShort)
			assert.ElementsMatch(t, tt.expected, got)
		})
	}

	// Test with empty adjectiveSwaps
	adjectiveSwaps = map[string]string{}
	t.Run(tests[2].name, func(t *testing.T) {
		got := GetFormattedAdjectives(tests[2].excludeShort)
		assert.Empty(t, got)
	})
}
func TestGetFormattedAdjective(t *testing.T) {
	// Setup: populate adjectiveSwaps with test data
	adjectiveSwaps = map[string]string{
		"charmed":       "♥friend",
		"charmed-short": "♥",
		"hidden":        "hidden",
		"hidden-short":  "?",
		"zombie":        "zOmBie",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Existing adjective",
			input:    "charmed",
			expected: "♥friend",
		},
		{
			name:     "Existing short adjective",
			input:    "charmed-short",
			expected: "♥",
		},
		{
			name:     "Non-existing adjective returns input",
			input:    "unknown",
			expected: "unknown",
		},
		{
			name:     "Another existing adjective",
			input:    "zombie",
			expected: "zOmBie",
		},
		{
			name:     "Another short adjective",
			input:    "hidden-short",
			expected: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetFormattedAdjective(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}

	// Test with empty adjectiveSwaps
	adjectiveSwaps = map[string]string{}
	t.Run("Empty adjectiveSwaps returns input", func(t *testing.T) {
		got := GetFormattedAdjective("charmed")
		assert.Equal(t, "charmed", got)
	})
}

// The aggro suffix means "this character is fighting YOU". Viewer 0 is a room
// broadcast with no single reader, and a character with no target (or a mob
// target) reports target UserId 0, so the old equality test painted nearly
// every name red in every viewer-0 line.
func TestGetFormattedName_AggroNeedsARealViewer(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	// Health above zero, or the dead suffix wins before aggro is considered.
	idle := &Character{Name: "Grix", Health: 10}
	assert.Equal(t, "", idle.GetMobName(0).Suffix, "no target, viewer 0: not aggro")
	assert.NotContains(t, idle.GetCharacterName(true), "-aggro")

	vsMob := &Character{Name: "Grix", Health: 10}
	vsMob.SetAggro(0, 55, DefaultAttack, 0)
	assert.Equal(t, "", vsMob.GetMobName(0).Suffix, "mob target, viewer 0: not aggro")

	vsPlayer := &Character{Name: "Grix", Health: 10}
	vsPlayer.SetAggro(7, 0, DefaultAttack, 0)
	assert.Equal(t, "aggro", vsPlayer.GetMobName(7).Suffix, "the player it is fighting sees aggro")
	assert.Equal(t, "", vsPlayer.GetMobName(8).Suffix, "another player does not")
	assert.Equal(t, "", vsPlayer.GetMobName(0).Suffix, "a room broadcast does not")
}
