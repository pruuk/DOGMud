package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// A room description follows the person looking at it.
func TestGetDetails_DescriptionWrapsToTheViewersWidth(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	viewer.LineWidth = 50

	lines := strings.Split(GetDetails(r, viewer).Description, "\n")
	if len(lines) < 2 {
		t.Fatalf("a long description should fold at 50 columns: %q", lines)
	}
	for _, line := range lines {
		if w := util.VisibleWidth(line); w > 50 {
			t.Errorf("description line is %d columns, want <= 50: %q", w, line)
		}
	}
}

// The minimap keeps its seven columns at the right edge of the viewer's width.
func TestGetDetails_MinimapSitsAtTheViewersRightEdge(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	viewer.LineWidth = 60
	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}

	lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
	for i := range tinymap {
		if w := util.VisibleWidth(lines[i]); w != 60 {
			t.Errorf("line %d beside the minimap is %d columns, want 60: %q", i, w, lines[i])
		}
	}
}

// GetDescriptionFormatted used to ignore its width.
func TestGetDescriptionFormatted_UsesTheWidthItIsGiven(t *testing.T) {
	r := &Room{Description: strings.Repeat("a quiet stone hall ", 12)}
	for _, width := range []int{40, 72} {
		for _, line := range strings.Split(strings.ReplaceAll(r.GetDescriptionFormatted(width, false), "\r", ""), "\n") {
			if w := util.VisibleWidth(line); w > width {
				t.Errorf("width %d: line is %d columns: %q", width, w, line)
			}
		}
	}
	if strings.Count(r.GetDescriptionFormatted(40, false), "\n") <= strings.Count(r.GetDescriptionFormatted(72, false), "\n") {
		t.Error("a narrower width must make more lines")
	}
}
