package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// rawEventsMessageAllowed lists the only production files that may construct
// events.Message directly. Everything else sends through UserRecord.SendText or
// the Room.SendText* family, which run messaging.RenderForRecipient (category,
// normalize, sight gate, color). Progression was the last known bypass until
// M3 item 6; code in internal/characters uses notifyProgression.
var rawEventsMessageAllowed = map[string]string{
	"internal/rooms/rooms.go":                "the pipeline's own fan-out: Room sends queue per-recipient Messages after rendering",
	"internal/users/userrecord.go":           "UserRecord.SendText itself, the end of the per-user pipeline",
	"internal/usercommands/print.go":         "the print debug command, which echoes its raw argument by purpose",
	"internal/usercommands/admin.bauble.go":  "bauble corpus list/export: a table and a YAML paste target the normalizer would corrupt (capitalize, a/an, dup-word collapse, a stray period)",
	"internal/hooks/hooks.go":                "listener registration: events.Message{} is a type key, not a send",
	"internal/hooks/Message_SendMessages.go": "the listener; the match is its comment naming direct events.Message{RoomId} constructions",
}

func TestNoRawEventsMessageOutsidePipeline(t *testing.T) {
	pattern := regexp.MustCompile(`events\.Message\{`)
	seen := map[string]bool{}
	var outside []string

	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel := filepath.ToSlash(path)
			for i, line := range strings.Split(string(src), "\n") {
				if !pattern.MatchString(line) {
					continue
				}
				if _, ok := rawEventsMessageAllowed[rel]; ok {
					seen[rel] = true
					continue
				}
				outside = append(outside, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// Each allowlist entry must still match, which also proves the pattern can
	// match at all: an empty "outside" list from a blind pattern proves nothing.
	for file, why := range rawEventsMessageAllowed {
		if !seen[file] {
			t.Errorf("allowlist entry %s (%s) no longer constructs events.Message; remove it", file, why)
		}
	}
	sort.Strings(outside)
	for _, o := range outside {
		t.Errorf("raw events.Message outside the pipeline: %s\n  send through UserRecord.SendText or Room.SendText*; from internal/characters use notifyProgression", o)
	}
}
