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

func craftSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "craft.go"))
	require.NoError(t, err)
	return string(raw)
}

// funcBody returns the source of the named func, bounded so a later function
// cannot leak into the match.
func funcBody(t *testing.T, src, decl string) string {
	t.Helper()
	start := strings.Index(src, decl)
	require.NotEqual(t, -1, start, "%s not found; this guard has gone stale", decl)
	rest := src[start+len(decl):]
	end := strings.Index(rest, "\nfunc ")
	if end == -1 {
		return src[start:]
	}
	return src[start : start+len(decl)+end]
}

// ⚠️ THE GENERAL INVARIANT, not just the enchanting case.
//
// Craft() resolves a recipe and then dispatches down one of several paths, and
// several of them run their OWN HasIngredients check and report "You are
// missing: X". The storage pull must therefore happen BEFORE any of them can
// return, or that path silently never pulls.
//
// That is exactly what shipped: the enchanting route sat above the pull and
// returned first, so `craft honed-edge weapon` reported "You are missing:
// binding-paste" with 152 of them in the player's storage. Fixing only the
// enchanting branch would leave the NEXT dispatch free to reintroduce it.
//
// So this asserts the general property: between resolving the recipe and
// pulling from storage, Craft() may not return at all.
func TestCraft_NothingReturnsBeforeTheStoragePull(t *testing.T) {
	body := funcBody(t, craftSource(t), "func Craft(")

	resolve := strings.Index(body, "FindRecipeByName(rest)")
	pull := strings.Index(body, "ensureComponentsFromStorage(")
	require.NotEqual(t, -1, resolve, "recipe resolution vanished from Craft()")
	require.NotEqual(t, -1, pull, "the storage pull vanished from Craft()")
	require.Less(t, resolve, pull, "the pull must come after recipe resolution")

	// The resolution fallback loop uses `break`, not `return`, so any `return`
	// in this window is a NEW early exit that skips the pull.
	window := body[resolve:pull]
	assert.NotContains(t, window, "return",
		"Craft() returns between resolving the recipe and pulling from storage, so "+
			"that path never draws components and will report \"You are missing: X\" "+
			"while the items sit in storage. Move the dispatch below "+
			"ensureComponentsFromStorage.")
}

// The pull must stay ahead of every known ingredient-checking dispatch.
func TestCraft_StoragePullPrecedesEveryDispatch(t *testing.T) {
	body := funcBody(t, craftSource(t), "func Craft(")
	pull := strings.Index(body, "ensureComponentsFromStorage(")
	require.NotEqual(t, -1, pull)

	for _, dispatch := range []string{
		"craftEnchanting(",       // runs its own HasIngredients
		"actions.InitiateCraft(", // runs its own HasIngredients
	} {
		at := strings.Index(body, dispatch)
		require.NotEqual(t, -1, at, "dispatch %s not found; guard is stale", dispatch)
		assert.Less(t, pull, at, "%s must dispatch AFTER the storage pull", dispatch)
	}
}

// The exclusion that made the recipe list agree with the old broken behaviour.
// With the pull reordered, claiming an enchant is impossible would be a lie.
func TestStorageCompletable_NoLongerExcludesEnchanting(t *testing.T) {
	body := funcBody(t, craftSource(t), "func storageCompletable(")
	assert.NotContains(t, body, "IsEnchantingRecipe",
		"storageCompletable must not exclude enchanting recipes any more; "+
			"they pull from storage like everything else")
}
