package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSkillsRankColumnLinesUp is the 2026-10-17 playtest finding "skills
// columns misalign on long names": the name column was a fixed 12 wide, so
// "blacksmithing" (13) pushed its rank one column right of the others. The
// command passes the longest held name as NameWidth (usercommands
// skillNameWidth); the rank's "[" must then sit in one column on every row.
func TestSkillsRankColumnLinesUp(t *testing.T) {
	data := map[string]any{
		"SkillList": map[string]int{
			"search":         2,
			"blacksmithing":  1,
			"unarmed-combat": 3,
		},
		"SkillCooldowns": map[string]int{"search": 0, "blacksmithing": 0, "unarmed-combat": 0},
		"SkillBlurbs":    map[string]string{},
		"NameWidth":      len("unarmed-combat"),
	}
	out, err := Process("character/skills", data)
	require.NoError(t, err)
	col := -1
	rows := 0
	for _, l := range strings.Split(out, "\n") {
		l = tableFrameTagRE.ReplaceAllString(l, "")
		for _, name := range []string{"search", "blacksmithing", "unarmed-combat"} {
			if strings.HasPrefix(strings.TrimLeft(l, " "), name+" ") {
				at := strings.Index(l, "[")
				require.GreaterOrEqual(t, at, 0, "no rank on %q", l)
				if col == -1 {
					col = at
				} else if at != col {
					t.Errorf("rank of %s starts at column %d, the first row's at %d: %q", name, at, col, l)
				}
				rows++
			}
		}
	}
	require.Equal(t, 3, rows, "rendered:\n%s", out)
}
