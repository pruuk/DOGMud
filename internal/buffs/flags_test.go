package buffs

import (
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
	re := regexp.MustCompile("Flag = `([a-z-]+)`")
	declared := map[Flag]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
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
