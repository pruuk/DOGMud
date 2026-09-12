package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Slice C delivery path: a player buff must travel the events.Buff event, so
// the Buff_ApplyBuffs hook runs and narrates the start through
// buffs.BuffSpec.StartUserNotice. A direct character-level add
// (Character.AddBuff / Character.AddBuffScaled) applies the buff in place and
// queues nothing, so the hook never runs and the holder reads no line at all.
// That is not a theoretical gap: a playtest drank a Purging Draught and took
// fifty rounds of Purging Weakness in complete silence, because the drink path
// called Character.AddBuffScaled directly.
//
// The event path for a player is users.UserRecord.AddBuff /
// UserRecord.AddBuffScaled. Every remaining direct add under the command and
// action packages needs a reason recorded here, keyed "file|line". A mob
// holder has no client, so a mob add is legitimately direct; a buff flagged
// silent-start has, by authored intent, no start line of its own.
//
// When a legitimate direct add moves, update its line number here. When a new
// one appears, either route it through the user record or record why it cannot
// be.
var buffApplyPathAllowlist = map[string]string{
	// Rally and warcry give party members buffs 80 and 79. Both are flagged
	// silent-start: the command narrates the rally for the whole party itself,
	// and the synchronous apply is what lets that narration read the members'
	// post-buff state in the same tick.
	"internal/usercommands/rally.go|57":   "party member gets buff 80, which is silent-start; the rally command narrates it and relies on the synchronous apply",
	"internal/usercommands/rally.go|87":   "party member gets buff 79, which is silent-start; the rally command narrates it and relies on the synchronous apply",
	"internal/usercommands/rally.go|117":  "charmed mob, not a player; a mob holder has no client to read a start line",
	"internal/usercommands/warcry.go|57":  "party member gets buff 79, which is silent-start; the warcry command narrates it and relies on the synchronous apply",
	"internal/usercommands/warcry.go|91":  "party member gets buff 80, which is silent-start; the warcry command narrates it and relies on the synchronous apply",
	"internal/usercommands/warcry.go|121": "charmed mob, not a player; a mob holder has no client to read a start line",

	// An admin-spawned mob is pinned into its gear with the perma-gear buff.
	"internal/usercommands/character.go|413": "the holder is a MOB (m.Character), and buff 99 is a perma-gear pin, not something a player reads",

	// Combat.
	"internal/actions/combat_throttle.go|147": "the throttle move narrates the choke as it lands and must apply buff 89 in the same tick; buff 89 is flagged silent-start",
}

// directBuffAddPattern matches a character-level buff add reached through a
// field: user.Character.AddBuff, target.Char.AddBuffScaled, and so on. It does
// not match the primitives themselves (characters.Character.AddBuff's own body,
// or the Buff_ApplyBuffs hook, which is the sanctioned consumer of the event
// and lives outside the scanned packages).
var directBuffAddPattern = regexp.MustCompile(`\.(?:Character|Char)\.AddBuff(?:Scaled)?\(`)

func TestPlayerBuffsTravelTheEventPath(t *testing.T) {
	scanDirs := []string{
		filepath.Join("internal", "usercommands"),
		filepath.Join("internal", "actions"),
	}

	var problems []string
	seen := map[string]bool{}

	for _, dir := range scanDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("cannot read %s: %v", dir, err)
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
		if len(files) == 0 {
			t.Fatalf("no non-test .go files found under %s; the guard would pass vacuously", dir)
		}
		sort.Strings(files)

		for _, path := range files {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("cannot open %s: %v", path, err)
			}
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				line := scanner.Text()
				if !directBuffAddPattern.MatchString(line) {
					continue
				}
				// Slash-normalised so the key reads the same on every platform.
				key := fmt.Sprintf("%s|%d", filepath.ToSlash(path), lineNo)
				if _, ok := buffApplyPathAllowlist[key]; ok {
					seen[key] = true
					continue
				}
				problems = append(problems, fmt.Sprintf(
					"%s: %s\n      a player buff applied this way is SILENT: route it through users.UserRecord.AddBuff / AddBuffScaled, or add %q to buffApplyPathAllowlist with a reason",
					key, strings.TrimSpace(line), key))
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("cannot scan %s: %v", path, err)
			}
			f.Close()
		}
	}

	// A stale allowlist entry is a silent hole: the line it pardons has moved,
	// so the real add at the new line is no longer pardoned by anything and the
	// entry pardons whatever now sits there.
	var stale []string
	for key := range buffApplyPathAllowlist {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		problems = append(problems, fmt.Sprintf(
			"%s: allowlisted but no direct buff add is on that line any more; find where it moved and update the key", key))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("%d buff delivery path problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}
