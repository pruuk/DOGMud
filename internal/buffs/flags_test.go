package buffs

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every Flag constant declared in buffspec.go must be in AllFlags, or the
// load-time guard would reject a buff that uses a perfectly good flag.
func TestAllFlagsNamesEveryDeclaredConstant(t *testing.T) {
	src, err := os.ReadFile("buffspec.go")
	require.NoError(t, err)
	// Every character but the delimiter, so a constant declared with an
	// unexpected spelling (an underscore, a digit, a capital) is caught rather
	// than quietly skipped by a narrow character class. The All sentinel is the
	// empty string and is deliberately not in AllFlags, so it is skipped.
	re := regexp.MustCompile("Flag = `([^`]*)`")
	declared := map[Flag]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		if m[1] == "" {
			continue
		}
		declared[Flag(m[1])] = true
	}
	require.NotEmpty(t, declared)
	listed := map[Flag]bool{}
	for _, f := range AllFlags {
		listed[f] = true
	}
	for f := range declared {
		assert.True(t, listed[f], "declared flag %q is missing from AllFlags", f)
	}
	for f := range listed {
		assert.True(t, declared[f], "AllFlags lists %q, which no constant declares", f)
	}
}

func TestUnknownFlagIsRejectedAtLoad(t *testing.T) {
	assert.NoError(t, (&BuffSpec{BuffId: 1, Name: "Fine", Flags: []Flag{Poison, NightVision}}).ValidateFlags())
	err := (&BuffSpec{BuffId: 65, Name: "Cat's Eye Draught", Flags: []Flag{"night-vision"}}).ValidateFlags()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "night-vision")
	assert.Contains(t, err.Error(), "65")
}

// Ids clear of the other fixtures in this package.
const (
	flagsTestUnknownFlagBuffId = 9421
	flagsTestCleanBuffId       = 9422
)

// The load-time guard is what makes a misspelled flag a boot failure instead
// of a buff that silently does nothing, so the walk over the loaded registry
// is a named function a test can call. With it inlined in LoadDataFiles there
// was nothing red to see: no test loads world YAML.
func TestValidateLoadedFlagsPanicsOnAnUnknownFlag(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		flagsTestUnknownFlagBuffId: {BuffId: flagsTestUnknownFlagBuffId, Name: "Test Draught", Flags: []Flag{"night-vision"}},
	})
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic for a buff carrying a flag the engine does not declare")
		}
		msg := fmt.Sprint(r)
		assert.Contains(t, msg, "night-vision", "the panic must name the flag")
		assert.Contains(t, msg, "9421", "and the buff id")
	}()
	ValidateLoadedFlags()
}

func TestValidateLoadedFlagsAcceptsACleanRegistry(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		flagsTestCleanBuffId: {BuffId: flagsTestCleanBuffId, Name: "Test Clean", Flags: []Flag{Poison, PoisonImmunity}},
	})
	defer restore()

	ValidateLoadedFlags()
}
