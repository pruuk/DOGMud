package main

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Parity slice 6 made one listener, hooks.RoomChangeShadowFollow, the only
// place a shadower follows its quarry, and put the shadow's state behind
// shared bodies in internal/actions (ShadowTargetOf, ClearShadow, EndShadow,
// ShadowSenseRoll, ShadowingConditionId). Before that there were two follow
// sites, a loop in usercommands/go.go and a mob-only hook, each reading the
// misc-data keys and condition 87 itself, each with its own copy of the end
// logic, and neither moved a mob shadower. These tests fail if that forks
// again.

// shadowGuardCode is path's Go source with every comment removed (parsed
// without ParseComments and printed back), so a comment naming a key, a
// condition id or a deleted function neither fails nor satisfies a check.
func shadowGuardCode(path string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// shadowGuardWalk calls fn with the comment-free code of every production Go
// file under internal/ and modules/, and fails the test when it parsed fewer
// than 50 (a walk that sees nothing proves nothing).
func shadowGuardWalk(t *testing.T, fn func(rel, code string)) {
	t.Helper()
	parsed := 0
	for _, root := range []string{"internal", "modules"} {
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
			code, perr := shadowGuardCode(path)
			if perr != nil {
				// A syntax error is the compiler's to report, and another
				// root test may create and remove a scratch file mid-walk.
				return nil
			}
			parsed++
			fn(filepath.ToSlash(path), code)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if parsed < 50 {
		t.Fatalf("parsed only %d files; the walk is not seeing the tree, so a pass proves nothing", parsed)
	}
}

// The two misc-data keys are read and written only in internal/actions. On
// master before this slice five files outside it named them.
func TestShadowKeysLiveInActions(t *testing.T) {
	pattern := regexp.MustCompile(`shadow-target-(user|mob)`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if strings.HasPrefix(rel, "internal/actions/") {
			seen = true
			return
		}
		t.Errorf("%s names a shadow misc-data key; use actions.ShadowTargetOf, ClearShadow or EndShadow", rel)
	})
	if !seen {
		t.Fatal("no shadow key found in internal/actions: the pattern cannot match, so this guard proves nothing")
	}
}

// Condition 87 is named by literal only in internal/actions/shadow.go, as
// actions.ShadowingConditionId. On master before this slice go.go, the
// skullduggery command and a hooks const each carried the literal.
func TestShadowConditionIsNamedOnce(t *testing.T) {
	pattern := regexp.MustCompile(`Condition\(87\b|=\s*87\b`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if rel == "internal/actions/shadow.go" {
			seen = true
			return
		}
		t.Errorf("%s names condition 87 by literal; use actions.ShadowingConditionId", rel)
	})
	if !seen {
		t.Fatal("no condition 87 literal found in internal/actions/shadow.go: the pattern cannot match")
	}
}

// A shadow follow is ShadowTargetOf and a queued Command in one file. Only the
// listener does that, with one of each: one follow dispatch for every kind of
// shadower. On master before this slice go.go and the mob hook each followed.
func TestOneShadowFollowSite(t *testing.T) {
	const site = "internal/hooks/RoomChange_ShadowFollow.go"
	target := regexp.MustCompile(`ShadowTargetOf\(`)
	command := regexp.MustCompile(`\.Command\(`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !target.MatchString(code) || !command.MatchString(code) {
			return
		}
		if rel != site {
			t.Errorf("%s reads a shadow target and queues a command; shadow following lives in %s alone", rel, site)
			return
		}
		seen = true
		if n := len(target.FindAllString(code, -1)); n != 1 {
			t.Errorf("%s calls ShadowTargetOf %d times, want 1", site, n)
		}
		if n := len(command.FindAllString(code, -1)); n != 1 {
			t.Errorf("%s queues %d commands, want 1 follow dispatch for every kind of shadower", site, n)
		}
	})
	if !seen {
		t.Fatalf("%s does not read a shadow target and queue a command: the listener is gone or the patterns cannot match", site)
	}
}

// shadowTargetReaders is every production file that may read a shadow target.
// TestOneShadowFollowSite only sees a follow written in one file; a lookup in
// one hooks file handing off to a dispatch in another would slip past it. So
// a new reader anywhere fails here and must be reviewed before it is added.
var shadowTargetReaders = map[string]string{
	"internal/actions/shadow.go":                         "the shared bodies",
	"internal/hooks/RoomChange_ShadowFollow.go":          "the one follow site",
	"internal/hooks/MobDeath_TrackingCleanup.go":         "clears shadows on a dead quarry (ClearShadow)",
	"internal/hooks/PlayerDespawn_TrackingCleanup.go":    "clears shadows on a departed quarry (ClearShadow)",
	"internal/usercommands/skill.skullduggery.shadow.go": "shadow stop (EndShadow)",
}

func TestShadowTargetReadersArePinned(t *testing.T) {
	pattern := regexp.MustCompile(`ShadowTargetOf\(`)
	seen := map[string]bool{}
	shadowGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if _, ok := shadowTargetReaders[rel]; !ok {
			t.Errorf("%s reads a shadow target; shadow following lives in internal/hooks/RoomChange_ShadowFollow.go alone. If this reader is not a follow, add it to shadowTargetReaders with its reason", rel)
			return
		}
		seen[rel] = true
	})
	for rel, why := range shadowTargetReaders {
		if !seen[rel] {
			t.Errorf("shadowTargetReaders lists %s (%s) but it no longer reads a shadow target; remove it", rel, why)
		}
	}
}

// The forked follow and end logic this slice deleted stays deleted.
func TestDeletedShadowForksStayDeleted(t *testing.T) {
	pattern := regexp.MustCompile(`\bshadowIsTargetingUser\b|\bshadowDetectionRoll\b|\binlineShadowEnd\b|\bendShadow\(|\bMobRoomChangeShadowFollow\b|\bgetShadowTargetUserId\b`)
	shadowGuardWalk(t, func(rel, code string) {
		if loc := pattern.FindStringIndex(code); loc != nil {
			t.Errorf("%s names %q, deleted by parity slice 6; use the shared bodies in internal/actions", rel, code[loc[0]:loc[1]])
		}
	})
	// Proof the pattern can match: its own alternatives, as they were spelled
	// on master before this slice.
	for _, probe := range []string{"shadowIsTargetingUser(u, 1)", "shadowDetectionRoll(a, b, r)", "inlineShadowEnd(u, s)", "endShadow(u, s)", "MobRoomChangeShadowFollow)", "getShadowTargetUserId(u)"} {
		if !pattern.MatchString(probe) {
			t.Fatalf("the deleted-name pattern does not match %q: it cannot see what it guards", probe)
		}
	}
	if pattern.MatchString("actions.EndShadow(actor, reason)") {
		t.Fatal("the deleted-name pattern matches the shared EndShadow; it would fail every caller")
	}
}
