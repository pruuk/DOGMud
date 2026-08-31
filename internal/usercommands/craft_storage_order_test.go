package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ⚠️ THIS IS AN ORDERING BUG AND ORDERING IS INVISIBLE TO A NORMAL UNIT TEST.
// Both the storage auto-pull and the enchanting route were individually
// correct; the defect was purely that the enchanting branch `return`ed FIRST.
//
// Reported from prod: `craft honed-edge weapon` answered "You are missing:
// binding-paste" while 152 Binding Paste sat in the player's storage.
// honed-edge is an enchanting recipe (it sets enchant_type + target_type, so
// IsEnchantingRecipe is true), so Craft() handed off to craftEnchanting before
// the pull could run. craftEnchanting does its OWN HasIngredients check, which
// is where that message came from, so components must be on the character
// BEFORE the handoff.
//
// A behavioural test here would need a full user, room, storage and recipe
// registry. The property that actually matters is cheap and exact: in the
// source of Craft(), the auto-pull must appear BEFORE the enchanting route.
func TestCraft_StoragePullPrecedesEnchantingRoute(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "craft.go"))
	require.NoError(t, err)
	src := string(raw)

	// Bound the search to Craft() so craftEnchanting's own body can't confuse it.
	start := strings.Index(src, "func Craft(")
	require.NotEqual(t, -1, start, "func Craft not found; this guard has gone stale")
	end := strings.Index(src[start:], "\nfunc ")
	require.NotEqual(t, -1, end, "could not find the end of func Craft")
	body := src[start : start+end]

	pull := strings.Index(body, "PlanStoragePull")
	route := strings.Index(body, "craftEnchanting(")

	require.NotEqual(t, -1, pull, "the storage auto-pull vanished from Craft()")
	require.NotEqual(t, -1, route, "the enchanting route vanished from Craft()")

	assert.Less(t, pull, route,
		"the storage auto-pull MUST run before Craft() hands off to craftEnchanting; "+
			"otherwise enchanting recipes never pull components and report "+
			"\"You are missing: X\" while X sits in storage")
}

// The exclusion that made the recipe list agree with the old broken behaviour.
// With the pull reordered, claiming an enchant is impossible would be a lie.
func TestStorageCompletable_NoLongerExcludesEnchanting(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "craft.go"))
	require.NoError(t, err)
	src := string(raw)

	start := strings.Index(src, "func storageCompletable(")
	require.NotEqual(t, -1, start, "storageCompletable not found; guard is stale")
	end := strings.Index(src[start:], "\n}")
	require.NotEqual(t, -1, end)
	body := src[start : start+end]

	assert.NotContains(t, body, "IsEnchantingRecipe",
		"storageCompletable must not exclude enchanting recipes any more; "+
			"they pull from storage like everything else")
}
