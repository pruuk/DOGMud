package apiframework

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// A test must never print a key: not a developer's real OPENAI_API_KEY, and
// not a fake one either, since a habit of printing keys in failures is how a
// real one ends up in CI logs. (It happened: a failure message printed the
// key a test had resolved, which on a machine with OPENAI_API_KEY set was
// the real one.) Endpoint redacts itself (String); this guard reads the test
// sources of the packages that handle keys and refuses two shapes:
//
//   - an argument to t.Log/Error/Fatal/Skip that names a key or where one is
//     kept (APIKey, apiKey, ResolveKey, the Authorization header, Getenv);
//   - in this package and the baubles module, a message that talks about a
//     key and prints a value with a string verb (%q, %s), which is how the
//     original leak was written: t.Fatalf("companion key: %q", got). (The
//     AI companion's tests speak of "the owner's key" in many messages that
//     print page text and statuses; they hold no key, and its TestMain
//     clears OPENAI_API_KEY, so only the first rule reads them.)

var (
	keyish   = []string{`APIKey`, `apiKey`, `ResolveKey`, `lastAuth`, `Authorization`, `Getenv`}
	printers = map[string]bool{`Log`: true, `Logf`: true, `Error`: true, `Errorf`: true, `Fatal`: true, `Fatalf`: true, `Skip`: true, `Skipf`: true}
	verbRE   = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?([a-zA-Z%])`)
)

// keyPrints returns a description of every call in src that could print a
// key.
func keyPrints(name string, src []byte, messages bool) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !printers[sel.Sel.Name] {
			return true
		}
		at := fset.Position(call.Pos()).String()
		for i, arg := range call.Args {
			if lit, isLit := arg.(*ast.BasicLit); isLit && i == 0 && lit.Kind == token.STRING {
				continue // the message itself may say "key"
			}
			text := string(src[fset.Position(arg.Pos()).Offset:fset.Position(arg.End()).Offset])
			for _, k := range keyish {
				if strings.Contains(text, k) {
					found = append(found, at+`: prints something holding a key (`+k+`)`)
				}
			}
		}
		if messages && len(call.Args) > 1 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				msg, _ := strconv.Unquote(lit.Value)
				if strings.Contains(strings.ToLower(msg), `key`) {
					for _, m := range verbRE.FindAllStringSubmatch(msg, -1) {
						if v := m[1]; v == `q` || v == `s` {
							found = append(found, at+`: a message about a key prints a value with %`+v)
							break
						}
					}
				}
			}
		}
		return true
	})
	return found, nil
}

func TestNoTestPrintsAKey(t *testing.T) {
	checked := 0
	for dir, messages := range map[string]bool{`.`: true, `../../modules/baubles`: true, `../../modules/aicompanion`: false, `../../modules/npcidle`: true, `../../modules/roomlife`: true, `../../modules/lookdetail`: true, `../../modules/rifts`: true, `../lively`: true} {
		files, err := filepath.Glob(filepath.Join(dir, `*_test.go`))
		if err != nil || len(files) == 0 {
			t.Fatalf("no test files found in %s", dir)
		}
		for _, file := range files {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			found, err := keyPrints(file, src, messages)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range found {
				t.Error(f)
			}
			checked++
		}
	}
	if checked < 10 {
		t.Fatalf("the guard read only %d test files: it is not looking where it should", checked)
	}
}

// The guard must be able to fail: it catches the line that leaked, and the
// other shape, and passes clean messages.
func TestKeyGuardCatchesTheOriginalLeak(t *testing.T) {
	bad := []byte("package x\nfunc T(t *testing.T) {\n\tgot := resolve()\n\tt.Fatalf(\"companion key: %q\", got)\n\tt.Fatalf(\"endpoint %v\", s.Endpoint.APIKey)\n}\n")
	found, err := keyPrints(`bad_test.go`, bad, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("both shapes must be caught, got %d: %v", len(found), found)
	}
	good := []byte("package x\nfunc T(t *testing.T) {\n\tt.Fatal(\"the configured key, trimmed\")\n\tt.Fatalf(\"legacy key (has=%t legacy=%t)\", a, b)\n\tt.Fatalf(\"result: %+v\", res)\n}\n")
	if found, _ := keyPrints(`good_test.go`, good, true); len(found) != 0 {
		t.Fatalf("clean messages pass, got %v", found)
	}
}

// Neither place a server key can be configured shows through the one
// redacted view of the config (slice M's DisplayConfigData, which the boot
// log, both server listings and /viewconfig render): the typed
// APIFramework.APIKey (a ConfigSecret) and the companion's old module-map
// key (a plain map value, redacted by its leaf name).
func TestNoServerKeyReachesTheConfigDisplay(t *testing.T) {
	var c configs.Config
	c.APIFramework.APIKey = `sk-sentinel-typed-0001`
	c.Modules = configs.Modules{`aicompanion`: map[string]any{`APIKey`: `sk-sentinel-module-0002`}}
	shown := 0
	for path, v := range c.DisplayConfigData() {
		shown++
		if strings.Contains(fmt.Sprint(v), `sk-sentinel`) {
			t.Errorf("the config display leaks the sentinel at %v", path)
		}
	}
	if shown < 10 {
		t.Fatalf("the display walked only %d entries: this test could not have found a leak", shown)
	}
}

func TestEndpointRedactsItsKey(t *testing.T) {
	e := Endpoint{BaseURL: `https://api.openai.com/v1`, APIKey: `sk-secret-value`}
	s := ServerSettings{Endpoint: e}
	for _, got := range []string{
		e.String(), e.GoString(),
		sprint(`%v`, e), sprint(`%+v`, e), sprint(`%#v`, e), sprint(`%s`, e),
		sprint(`%v`, s), sprint(`%+v`, s), sprint(`%#v`, s),
	} {
		if strings.Contains(got, `sk-secret`) || !strings.Contains(got, `redacted`) {
			t.Errorf("a printed endpoint must hide its key, got %d bytes", len(got))
		}
	}
}
