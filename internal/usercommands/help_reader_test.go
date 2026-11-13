package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Help is laid out to the width it is asked for, and keeps the characters
// the screen-reader cleanup would blank (owner ruling 2026-11-12).
func TestGetHelpContentsUsesTheReadersWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	for _, width := range []int{30, 42, 80} {
		out, err := GetHelpContents("look", width)
		require.NoError(t, err)
		assert.LessOrEqual(t, templates.WidestLine(out), width, "width %d:\n%s", width, out)
		assert.Contains(t, out, "[thing]", "brackets survive at %d", width)
		assert.NotContains(t, out, "::", "markers never reach the reader")
		// Each usage row starts its own line. Unmarked, the rows would be
		// rejoined as one paragraph, which fits the width but is wrong: the
		// width check alone cannot tell (plan 4, HUA's point).
		visible := regexp.MustCompile(`</?ansi[^>]*>`).ReplaceAllString(out, "")
		for _, row := range []string{"  look north", "  look dave", "  look sword"} {
			assert.Regexp(t, "(?m)^"+regexp.QuoteMeta(row), visible, "at %d the row %q starts a line", width, row)
		}
	}
}

func TestGetHelpContentsDifferBetweenWidths(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	narrow, err := GetHelpContents("look", 30)
	require.NoError(t, err)
	wide, err := GetHelpContents("look", 80)
	require.NoError(t, err)
	assert.Greater(t, strings.Count(narrow, "\n"), strings.Count(wide, "\n"))
}
