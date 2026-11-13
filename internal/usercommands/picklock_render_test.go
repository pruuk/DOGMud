package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
)

// Every lock, 2 to 32 steps, fits every width, and no step is lost
// (owner 2026-11-12, plan 5: lockpicking was over 80 already).
func TestLockRenderFitsEveryWidth(t *testing.T) {
	useDogmudTemplates(t) // TestMain points DataFiles at an empty folder
	// util.GetLockSequence clamps a lock to 2 to 32 steps.
	for _, width := range []int{30, 42, 80} {
		for n := 2; n <= 32; n++ {
			seq := strings.Repeat("UD", 16)[:n]
			out := lockRenderAt(seq, seq[:n/2], width, false)
			if w := templates.WidestLine(out); w > width {
				t.Errorf("a %d-step lock at %d is %d wide:\n%s", n, width, w, out)
			}
			if got := strings.Count(out, "?"); got != n-n/2 {
				t.Errorf("a %d-step lock at %d shows %d unknown steps, want %d", n, width, got, n-n/2)
			}
		}
	}
}

func TestLockRowsHoldWhatFits(t *testing.T) {
	for _, c := range []struct{ width, per int }{{80, 9}, {42, 4}, {30, 3}} {
		if got := lockStepsPerRow(c.width); got != c.per {
			t.Errorf("at %d a row holds %d steps, want %d", c.width, got, c.per)
		}
	}
}

// An empty lock still draws today's titled, empty table, so TestGetLockRender
// keeps its meaning.
func TestLockRenderEmptyKeepsItsTitle(t *testing.T) {
	useDogmudTemplates(t)
	out := lockRenderAt("", "", 80, false)
	if !strings.Contains(out, "The Lock Sequence Looks like:") {
		t.Errorf("an empty lock lost its title:\n%s", out)
	}
	narrow := lockRenderAt("", "", 30, false)
	if !strings.Contains(narrow, "The lock sequence:") || templates.WidestLine(narrow) > 30 {
		t.Errorf("an empty lock at 30 has the wrong title or is too wide:\n%s", narrow)
	}
}

// A row of one step is "Lock step N:", not "Lock steps N to N:".
func TestLockRowOfOneStepIsSingular(t *testing.T) {
	useDogmudTemplates(t)
	out := lockRenderAt(strings.Repeat("UD", 5), "", 42, false) // 10 steps, 4 a row: 4, 4, 2
	if strings.Contains(out, "Lock step 10:") || !strings.Contains(out, "Lock steps 9 to 10:") {
		t.Errorf("a two-step last row is titled wrongly:\n%s", out)
	}
	out = lockRenderAt(strings.Repeat("UD", 5)[:9], "", 42, false) // 9 steps: 4, 4, 1
	if !strings.Contains(out, "Lock step 9:") || strings.Contains(out, "Lock steps 9 to 9:") {
		t.Errorf("a one-step last row is not singular:\n%s", out)
	}
}
