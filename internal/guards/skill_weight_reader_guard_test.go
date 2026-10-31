package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Skill weight knee (owner 2026-10-31): a skill rank's contribution to any
// additive score goes through configs.WeightedSkill, which applies the knee.
// A site that multiplies a rank by the SkillWeight knob itself skips the knee
// in silence. This guard finds every selector named SkillWeight outside
// internal/configs.

// directSkillWeightReads reports every `x.SkillWeight` selector in src.
func directSkillWeightReads(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == `SkillWeight` {
			bad = append(bad, rel+`:`+strconv.Itoa(fset.Position(sel.Pos()).Line)+`: reads SkillWeight directly; use configs.WeightedSkill`)
		}
		return true
	})
	return bad
}

func TestSkillWeightReaderGuard(t *testing.T) {
	root := repoRoot(t)
	skip := filepath.Join(root, `internal`, `configs`) + string(filepath.Separator)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			if strings.HasPrefix(path, skip) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, b := range directSkillWeightReads(t, filepath.ToSlash(rel), src) {
				t.Error(b)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf(`scanned only %d Go files; the walk is broken`, scanned)
	}
}

// TestSkillWeightReaderGuardCanFail feeds the finder one direct read and one
// helper call.
func TestSkillWeightReaderGuardCanFail(t *testing.T) {
	src := []byte(`package p

func f(bal B, r float64) {
	_ = r * float64(bal.SkillWeight)
	_ = configs.WeightedSkill(r)
}
`)
	bad := directSkillWeightReads(t, `p/p.go`, src)
	if len(bad) != 1 || !strings.HasPrefix(bad[0], `p/p.go:4: reads SkillWeight directly`) {
		t.Fatalf(`want exactly the direct read on line 4 reported, got %v`, bad)
	}
}
