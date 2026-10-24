package guards

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// craftRoomRenderRE matches a call to the crafting-messages room renderers.
var craftRoomRenderRE = regexp.MustCompile(`crafting\.Craft(RoomLines|HeardLine)\(`)

// craftObserverReadRE matches reading the Observer of a narrated craft's
// roles variable (successRoles, failureRoles, ...); the recipe's *_observer
// slot is unread since messaging M6 row 7. A bare `roles.Observer` is not
// matched: the round-tick hooks also hold condition-trigger narration, whose
// roles variable is named plain `roles`, in the same files as crafting.Phase.
var craftObserverReadRE = regexp.MustCompile(`(?i)\w+roles\.Observer\b`)

// TestCraftRoomLinesLiveInOneBody keeps every craft's room lines in
// actions.NarrateCraftPhase (messaging M6 row 7). Eight sites once sent them
// under three rules; a new site must call the body.
func TestCraftRoomLinesLiveInOneBody(t *testing.T) {
	const home = "internal/actions/craft_phase_lines.go"
	inside := 0
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") ||
				strings.HasPrefix(rel, "internal/crafting/") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			usesCraftPhase := bytes.Contains(src, []byte("crafting.Phase"))
			sc := bufio.NewScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if craftRoomRenderRE.MatchString(line) {
					if rel == home {
						inside++
						continue
					}
					t.Errorf("%s:%d renders a craft room line itself: %s\n  call actions.NarrateCraftPhase instead",
						rel, n, strings.TrimSpace(line))
				}
				if usesCraftPhase && rel != home && craftObserverReadRE.MatchString(line) {
					t.Errorf("%s:%d reads a craft's Observer: %s\n  the room line comes from actions.NarrateCraftPhase",
						rel, n, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	// Proves the walk can find a match at all: the body renders both kinds.
	if inside < 2 {
		t.Fatalf("found %d renderer calls in %s, want at least 2; the walk or the regex is broken", inside, home)
	}
}
