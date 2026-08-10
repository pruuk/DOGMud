package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// durableWriteExemptions lists the packages allowed to call os.WriteFile
// directly, with the reason each one is genuinely not living state.
//
// Keys are repo-relative directories. A file is exempt if its directory matches
// a key or sits underneath one.
var durableWriteExemptions = map[string]string{
	// The hardened implementation itself, plus SaveRoundCount, which is
	// deliberately unhardened for a measured reason documented at the call site.
	"internal/util": "owns the durable write; SaveRoundCount opts out on purpose",

	// One-shot upgrades run at boot before players connect. A failed migration
	// is reported and halts; there is no concurrent reader to tear against, and
	// re-running the migration is the recovery path.
	"internal/migration": "boot-time one-shot upgrades, no concurrent readers",

	// Developer and test tooling. Output is scratch, regenerated on demand, and
	// never read back by a running game.
	"internal/devtools":         "generates scratch content for a human to review",
	"internal/playtestenv":      "disposable docker environment scaffolding",
	"internal/playtestrun":      "disposable harness run scaffolding",
	"internal/playtestprofiles": "MUST stay a plain write: chunk 2.3 depends on in-place truncation preserving the inode owner",

	// Build-time code generation, run by a developer, never by the server.
	"cmd/generate": "codegen tool; output is source, regenerated on demand",
}

// durableWriteFileExemptions lists individual FILES allowed to hand-roll the
// temp-and-rename dance, for cases where util.Save's []byte signature does not
// fit. Keyed by repo-relative path. Deliberately per-file rather than
// per-directory so the rest of a high-value package stays guarded.
var durableWriteFileExemptions = map[string]string{
	// Streams fixed-size binary records rather than holding one []byte, so it
	// cannot call util.Save. It does the work correctly: f.Sync before close,
	// temp removed on failure, and util.SyncDir after the rename.
	"internal/users/index_rebuild.go": "streaming binary writer; syncs content and directory itself",
}

// exemptFile reports whether a repo-relative path is exempt by directory or by
// explicit file entry.
func exemptFile(rel string) bool {
	if _, ok := durableWriteFileExemptions[rel]; ok {
		return true
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	for exempt := range durableWriteExemptions {
		if dir == exempt || strings.HasPrefix(dir, exempt+"/") {
			return true
		}
	}
	return false
}

// TestLivingStateWritesAreDurable fails when a package outside the exemption
// list writes a file with os.WriteFile instead of util.Save.
//
// This is the recurrence guard for roadmap chunk 2.8. Wave 2 hardened
// util.SafeSave with an fsync, on the reasoning that a temp-file-and-rename is
// atomic but not durable: without the flush, the rename can be recorded while
// the data is still in the page cache, and a power loss leaves an
// atomically-renamed EMPTY file. That is the precise corruption the dance
// exists to prevent.
//
// The problem was that the audit which followed grepped the stores it already
// suspected and stopped. Fifteen writers were missed, including
// users.SaveUser — a player's entire character, inventory, gold, progression
// and quest state — which kept its own hand-rolled copy of the unhardened
// pattern through an entire wave about durable writes. Several others
// (bounties, facts, goals, knowledge, fileloader) had independently reinvented
// the same tmp+rename, so the codebase carried six divergent copies of one
// contract and hardening one of them fixed only one.
//
// A grep-and-judgement audit cannot hold that line, because the next writer is
// added by someone who never read the audit. This test holds it instead.
//
// If you are adding a genuinely non-living-state write, add the directory to
// durableWriteExemptions with a reason. If you cannot write the reason, the
// write probably wants util.Save.
func TestLivingStateWritesAreDurable(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if exemptFile(rel) {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			// A syntax error is the compiler's problem to report, not this
			// test's. Skipping it keeps failures attributable.
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "WriteFile" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "os" {
				return true
			}
			pos := fset.Position(call.Pos())
			offenders = append(offenders,
				rel+":"+itoa(pos.Line)+" os.WriteFile")
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d write(s) bypass the durable path (roadmap chunk 2.8).\n"+
			"Use util.Save, which is safe by default, or add the directory to\n"+
			"durableWriteExemptions in this file with a reason it is not living state.\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// TestNoHandRolledTempRename fails when a file both writes a `.tmp`/`.new`
// sibling and renames it — the hand-rolled durable-write pattern.
//
// os.Rename on its own is legitimate (zone renames move real files, the
// harness moves fixtures). What this rejects is a package reimplementing
// util.SafeSave, because every such copy has so far omitted the fsync and each
// one has to be found and fixed independently.
func TestNoHandRolledTempRename(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if exemptFile(rel) {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}

		var suffixLine, renameLine int
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if suffixLine == 0 && (v.Value == `".tmp"` || v.Value == `".new"` ||
					v.Value == "`.tmp`" || v.Value == "`.new`") {
					suffixLine = fset.Position(v.Pos()).Line
				}
			case *ast.CallExpr:
				sel, ok := v.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Rename" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if ok && pkg.Name == "os" && renameLine == 0 {
					renameLine = fset.Position(v.Pos()).Line
				}
			}
			return true
		})

		if suffixLine > 0 && renameLine > 0 {
			offenders = append(offenders,
				rel+":"+itoa(suffixLine)+" builds a temp sibling and renames it (os.Rename at :"+
					itoa(renameLine)+")")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d file(s) hand-roll the durable-write dance instead of calling util.Save.\n"+
			"Every previous copy of this pattern omitted the fsync, making it atomic but not durable.\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// itoa avoids pulling strconv in for two call sites.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
