package guards

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// M3 item 8 deleted ConsistentAttackMessages and the seeded message pick it
// drove (docs/superpowers/specs/completed/2026-09-16-messaging-m3-item8-combat-messages-design.md).
//
// The knob was upstream GoMud's and it worked there, because upstream keeps
// its role pools equal: one ItemId seed gave a coordinated triad and a
// per-weapon voice at once. Stage 9.2 expanded the dogmud pools and switched
// it off in the same commit, and that switch is also what replaced the single
// shared index with an independent draw per audience. Stage 9.5 then added
// skill tiers and the pools drifted unequal, so flipping it back would no
// longer have coordinated anything.
//
// It is superseded, not broken. Reintroducing it would reverse a deliberate
// design decision and undo the coordination this arc exists to deliver, so
// this guard keeps the names from coming back.
//
// Its counterpart is the coordination itself, pinned in
// internal/items/attack_messages_test.go.

// consistentAttackGuardBannedNames are the identifiers and config keys the
// deletion removed. Each is distinctive enough that a match is a real
// reintroduction rather than an unrelated word.
var consistentAttackGuardBannedNames = []string{
	"ConsistentAttackMessages",
	"msgSeed",
	"seedNum",
	"GetForSkillLevel",
}

// consistentAttackGuardSkipDir skips version control, vendored trees and the
// docs tree. Docs are exempt on purpose: the spec, the plans and the patch
// notes all NAME the deleted knob in order to record why it went, and a guard
// that forbade writing about a deletion would be worse than no guard.
func consistentAttackGuardSkipDir(name string) bool {
	if name == ".git" || name == "docs" || name == "node_modules" || name == "vendor" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

// consistentAttackGuardBanned reports whether an identifier is one of the
// deleted names. GetForSkillLevel matches its -With variant too.
func consistentAttackGuardBanned(ident string) string {
	for _, name := range consistentAttackGuardBannedNames {
		if strings.HasPrefix(ident, name) {
			return name
		}
	}
	return ""
}

// consistentAttackGuardGoIdents tokenizes a Go file and returns the banned
// identifiers it actually declares or uses, with line numbers.
//
// Tokenizing rather than grepping is the whole point. A grep flags the prose
// comments in attack_messages.go and snapshot_test.go that EXPLAIN why the
// knob went, which is documentation worth keeping, not a reintroduction. Only
// a real token.IDENT means the mechanism is back.
//
// Note this is an ABSENCE guard, so comments cause false FAILURES here. The
// repo's other lesson, that a commented-out call let a guard pass, applies to
// PRESENCE guards asserting a required call is wired up; there a comment
// causes a false pass. Opposite direction, opposite treatment.
func consistentAttackGuardGoIdents(path string, src []byte) map[int]string {
	out := map[int]string{}
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, 0) // 0: do not emit comments
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.IDENT {
			continue
		}
		if name := consistentAttackGuardBanned(lit); name != "" {
			out[fset.Position(pos).Line] = name
		}
	}
	return out
}

// TestConsistentAttackMessagesStaysDeleted fails if any banned name reappears
// in Go source or YAML config.
//
// This file is itself the one place the names may appear, since it has to
// spell them to look for them.
func TestConsistentAttackMessagesStaysDeleted(t *testing.T) {
	// This file's own path, from the compiler: the working directory is the
	// repo root (TestMain), not this file's folder.
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot find this file to skip it")
	}
	self, err := filepath.Abs(here)
	if err != nil {
		t.Fatalf("resolving this file's own path: %v", err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatalf("this file's own path %s does not exist, so the self-skip below would skip nothing: %v", self, err)
	}

	type hit struct {
		path string
		line int
		name string
		text string
	}
	var hits []hit
	scanned, selfSeen := 0, false

	walkErr := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The walk root itself is reported with Name() == ".", which the
			// dotfile rule below would match, returning SkipDir on the root
			// and scanning nothing. The first version of this guard did
			// exactly that and passed in 0.00s while the banned names were
			// sitting in internal/.
			if path == "." {
				return nil
			}
			if consistentAttackGuardSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if abs == self {
			selfSeen = true
			return nil
		}
		scanned++
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(body), "\n")

		if ext == ".go" {
			for line, name := range consistentAttackGuardGoIdents(path, body) {
				text := ""
				if line-1 < len(lines) {
					text = strings.TrimSpace(lines[line-1])
				}
				hits = append(hits, hit{path, line, name, text})
			}
			return nil
		}

		// YAML has no tokenizer here, and the config KEY is the thing that
		// matters, so a plain scan is right for it.
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, name := range consistentAttackGuardBannedNames {
				if strings.Contains(line, name) {
					hits = append(hits, hit{path, i + 1, name, strings.TrimSpace(line)})
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking the repo: %v", walkErr)
	}
	// The walk must reach the tree, and must meet this file, or the self-skip
	// matched nothing and this file's own spellings went unscanned by accident.
	if scanned < 500 || !selfSeen {
		t.Fatalf("scanned %d .go/.yaml files and met this file: %v; the walk is not seeing the repo", scanned, selfSeen)
	}

	for _, h := range hits {
		t.Errorf("%s:%d reintroduces %q: %s", h.path, h.line, h.name, h.text)
	}
	if len(hits) > 0 {
		t.Log("ConsistentAttackMessages and its seeded pick were deleted by M3 item 8. " +
			"Combat narration now renders every audience from one coordinated index; " +
			"a per-weapon seed would reverse that. See the spec before re-adding anything here.")
	}
}
