package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The menu guard (#375 phase 0): every numbered choice renders and resolves
// through internal/menu. A numbered-list template render or the copied
// menu-index range check anywhere else is a regression.
var menuGuardPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"tables/numbered-list`),
	regexp.MustCompile(`restNum > 0 && restNum <= len\(`),
}

// stripLineComments drops everything after "//" on each line, so a comment
// that mentions a retired pattern is not code.
func stripLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if idx := strings.Index(l, "//"); idx >= 0 {
			lines[i] = l[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// menuGuardFileViolations reports the retired patterns found in one file.
func menuGuardFileViolations(path string) []string {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	if strings.Contains(filepath.ToSlash(path), "internal/menu/") {
		return nil
	}
	b, _ := os.ReadFile(path)
	code := stripLineComments(string(b))
	var out []string
	for _, re := range menuGuardPatterns {
		if re.MatchString(code) {
			out = append(out, path+": "+re.String())
		}
	}
	return out
}

// menuGuardViolations walks each root recursively. The flat roots are
// scanned for their own *.go files only (the repo root's world.go, main.go).
func menuGuardViolations(t *testing.T, roots ...string) []string {
	return menuGuardViolationsIn(t, roots, nil)
}

func menuGuardViolationsIn(t *testing.T, roots, flatRoots []string) []string {
	var out []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			out = append(out, menuGuardFileViolations(path)...)
			return nil
		})
	}
	for _, root := range flatRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				out = append(out, menuGuardFileViolations(filepath.Join(root, e.Name()))...)
			}
		}
	}
	return out
}

func TestMenuGuard_NoNumberedMenusOutsideTheMenuPackage(t *testing.T) {
	if v := menuGuardViolationsIn(t, []string{"..", "../../modules"}, []string{"../.."}); len(v) > 0 {
		t.Fatalf("numbered menu code outside internal/menu:\n%s", strings.Join(v, "\n"))
	}
}

func TestMenuGuard_CanFail(t *testing.T) {
	dir := t.TempDir()
	bad := "package x\nfunc f() { _ = \"tables/numbered-list\"; if restNum > 0 && restNum <= len(xs) {} }\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := menuGuardViolations(t, dir); len(v) != 2 {
		t.Fatalf("guard missed a planted violation: %v", v)
	}
	if v := menuGuardViolationsIn(t, nil, []string{dir}); len(v) != 2 {
		t.Fatalf("flat root scan missed a planted violation: %v", v)
	}
}

func TestMenuGuard_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	ok := "package x\n// the retired \"tables/numbered-list\" template\nfunc f() {} // restNum > 0 && restNum <= len(xs)\n"
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte(ok), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := menuGuardViolations(t, dir); len(v) != 0 {
		t.Fatalf("guard flagged comment-only mentions: %v", v)
	}
}
