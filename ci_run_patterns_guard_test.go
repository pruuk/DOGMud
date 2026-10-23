package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// CI and make run named tests with `go test -run "<pattern>" ... <pkg>`. An
// alternative in that pattern that names no test is not an error to go test:
// the step stays green and the test it meant never runs. #375 Task 18 renamed
// TestSmoke_ShippedDialoguePassesTheEditorValidator, and CI kept running the
// old name, which matched nothing. This guard reads every such line in the CI
// files and the Makefile and fails when an alternative matches no Test
// function declared in the package the line tests.

// ciRunFiles are the files whose go test -run lines the guard reads.
func ciRunFiles(t *testing.T) []string {
	t.Helper()
	files := []string{filepath.Join(".github", "actions", "codegen-and-test", "action.yml"), "Makefile"}
	workflows, err := filepath.Glob(filepath.Join(".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return append(files, workflows...)
}

// goTestRunLine matches `go test ... -run "<pattern>" <rest of line>`.
var goTestRunLine = regexp.MustCompile(`go test\b[^\n]*?-run[ =]"([^"]+)"([^\n]*)`)

// testFuncDecl matches a top-level Test function declaration.
var testFuncDecl = regexp.MustCompile(`(?m)^func (Test\w*)\(`)

type ciRunCommand struct {
	where   string // file:line
	pattern string
	pkg     string // ".", "./...", or "./dir/"
}

// parseCIRunCommands lists the go test -run commands in src.
func parseCIRunCommands(name, src string) []ciRunCommand {
	var out []ciRunCommand
	for i, line := range strings.Split(src, "\n") {
		m := goTestRunLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pkg := "."
		for _, tok := range strings.Fields(m[2]) {
			if strings.HasPrefix(tok, ".") {
				pkg = tok
			}
		}
		out = append(out, ciRunCommand{where: name + ":" + strconv.Itoa(i+1), pattern: m[1], pkg: pkg})
	}
	return out
}

// testNamesIn lists the Test functions declared in pkg's _test.go files:
// "." is the repo root alone, "./..." every package, "./dir/" that one dir.
func testNamesIn(t *testing.T, pkg string) []string {
	t.Helper()
	var names []string
	read := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, m := range testFuncDecl.FindAllStringSubmatch(string(data), -1) {
			names = append(names, m[1])
		}
	}
	if pkg == "./..." {
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "vendor", "node_modules", "_datafiles":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				read(path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return names
	}
	paths, err := filepath.Glob(filepath.Join(filepath.FromSlash(pkg), "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		read(p)
	}
	return names
}

// unmatchedRunAlternatives returns each alternative of a -run pattern that
// matches none of names. A pattern written "^(A|B)$" anchors every
// alternative; a subtest part after "/" is ignored.
func unmatchedRunAlternatives(t *testing.T, pattern string, names []string) []string {
	t.Helper()
	top, _, _ := strings.Cut(pattern, "/")
	anchored := false
	if strings.HasPrefix(top, "^(") && strings.HasSuffix(top, ")$") {
		top, anchored = top[2:len(top)-2], true
	}
	var missing []string
	for _, alt := range strings.Split(top, "|") {
		expr := alt
		if anchored {
			expr = "^(?:" + alt + ")$"
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			t.Fatalf("-run alternative %q does not compile: %v", alt, err)
		}
		found := false
		for _, n := range names {
			if re.MatchString(n) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, alt)
		}
	}
	return missing
}

func TestGuard_CIRunPatternsNameRealTests(t *testing.T) {
	commands := 0
	for _, f := range ciRunFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, c := range parseCIRunCommands(filepath.ToSlash(f), string(data)) {
			commands++
			names := testNamesIn(t, c.pkg)
			if len(names) == 0 {
				t.Errorf("%s: package %s declares no tests", c.where, c.pkg)
				continue
			}
			for _, alt := range unmatchedRunAlternatives(t, c.pattern, names) {
				t.Errorf("%s: -run alternative %q matches no Test function in %s, so it runs nothing", c.where, alt, c.pkg)
			}
		}
	}
	// The CI action and the Makefile hold five -run lines today; finding
	// none means the parse broke, not that every name is real.
	if commands < 5 {
		t.Fatalf("found %d go test -run lines; the guard's parse cannot see them", commands)
	}
}

// TestGuard_CIRunPatternsGuardCanFail: the line CI shipped after the Task 18
// rename is refused, and only for the old name.
func TestGuard_CIRunPatternsGuardCanFail(t *testing.T) {
	old := `      run: go test -run "TestSmoke_ServerBootsCleanWithRealData|TestSmoke_ShippedDialoguePassesTheEditorValidator" -timeout 300s .`
	cmds := parseCIRunCommands("action.yml", old)
	if len(cmds) != 1 || cmds[0].pkg != "." {
		t.Fatalf("parse: %+v", cmds)
	}
	missing := unmatchedRunAlternatives(t, cmds[0].pattern, testNamesIn(t, cmds[0].pkg))
	if len(missing) != 1 || missing[0] != "TestSmoke_ShippedDialoguePassesTheEditorValidator" {
		t.Fatalf("the guard did not refuse the old name alone: %v", missing)
	}
	anchored := unmatchedRunAlternatives(t, "^(TestSmoke_ServerBoots)$", testNamesIn(t, "."))
	if len(anchored) != 1 {
		t.Fatalf("an anchored prefix of a real name must not match: %v", anchored)
	}
}
