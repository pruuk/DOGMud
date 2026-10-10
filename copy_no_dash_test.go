package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// noDashCopyFiles are the source files whose string literals must carry no
// em or en dash (#219, #383): player copy uses commas, colons and full stops.
// Comments may keep dashes. The list is the files the #219 fixes touched plus
// the other files the same copy PR edits, so a line added there later cannot
// bring a dash back. Widen it file by file as the #383 sweep reaches more.
var noDashCopyFiles = []string{
	"internal/usercommands/stand.go",
	"internal/usercommands/eat.go",
	"internal/usercommands/jail.go",
	"internal/usercommands/say.go",
	"internal/usercommands/get.go",
	"internal/usercommands/drop.go",
	"internal/usercommands/equip.go",
	"internal/hooks/NewRound_DoCombat_helpers.go",
	// #449, the sight-gates follow-ups.
	"internal/usercommands/loot.go",
	"internal/usercommands/go.go",
	"internal/usercommands/usercommands.go",
	"internal/usercommands/rally.go",
	"internal/usercommands/warcry.go",
	"internal/hooks/spell_resolution.go",
	"internal/combat/combat_helpers.go",
	// #287, the flee refusals and the AI rate-limit notice.
	"internal/usercommands/flee.go",
	"main.go",
}

func TestCopyFilesHaveNoDashesInStringLiterals(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(here)
	fset := token.NewFileSet()
	for _, rel := range noDashCopyFiles {
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		scanned := 0
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			scanned++
			if strings.ContainsAny(lit.Value, "—–") {
				t.Errorf("%s: string literal carries a dash: %s", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
		if scanned == 0 {
			t.Errorf("%s: scanned no string literals; the walk is not reaching the file", rel)
		}
	}
}
