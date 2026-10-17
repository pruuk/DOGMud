package util

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// asciiGuardRepoRoot is the repo root, anchored on this file's own path
// because a test binary's working directory is not reliably the package dir.
func asciiGuardRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// asciiGuardSkipLiterals names Go files whose non-ASCII literals are never
// sent to a player, so the guard does not read them.
var asciiGuardSkipLiterals = map[string]string{
	// Invisible and look-alike runes the bauble validator rejects in
	// generated text; they are inputs to a check, never output.
	"internal/baubles/validate.go": "validator input runes",
}

// shippedGlyphs collects every non-ASCII rune the game can send from source:
// the live world's templates and data YAML, config.yaml (the prompt strings),
// and the string and rune literals of every non-test Go file. Comments are not
// read. The localize tree is left out: a translation has no ASCII form.
func shippedGlyphs(t *testing.T) (map[rune]string, int) {
	t.Helper()
	root := asciiGuardRepoRoot(t)
	glyphs := map[rune]string{}
	files := 0

	note := func(rel string, s string) {
		for _, r := range s {
			if r >= utf8.RuneSelf {
				if _, seen := glyphs[r]; !seen {
					glyphs[r] = rel
				}
			}
		}
	}

	// Data: the live world (templates, YAML) and config.yaml.
	world := filepath.Join(root, "_datafiles", "world", "dogmud")
	err := filepath.WalkDir(world, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "localize" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".template") && !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		note(filepath.ToSlash(rel), string(b))
		files++
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", world, err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	note("_datafiles/config.yaml", string(cfg))
	files++

	// Go: string and rune literals only.
	for _, dir := range []string{"internal", "modules", "."} {
		base := filepath.Join(root, dir)
		walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The root pass reads only the root's own files.
				if dir == "." && p != base {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if _, skip := asciiGuardSkipLiterals[rel]; skip {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fset := token.NewFileSet()
			var s scanner.Scanner
			s.Init(fset.AddFile(p, -1, len(src)), src, nil, 0)
			for {
				_, tok, lit := s.Scan()
				if tok == token.EOF {
					break
				}
				if tok != token.STRING && tok != token.CHAR {
					continue
				}
				if v, err := strconv.Unquote(lit); err == nil {
					note(rel, v)
				} else {
					note(rel, lit)
				}
			}
			files++
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", base, walkErr)
		}
	}
	return glyphs, files
}

// TestAsciiModeCoversEveryShippedGlyph is the guard behind the ASCII charset
// mode: every non-ASCII rune a template, config string or Go literal can send
// must come out of ConvertToAscii as pure ASCII. The 2026-10-17 playtests saw
// star-field dots, wave bars, the shield emoji and the prompt's pointer reach
// ASCII clients unconverted because the table only knew the box glyphs.
func TestAsciiModeCoversEveryShippedGlyph(t *testing.T) {
	glyphs, files := shippedGlyphs(t)

	// The scan must reach the sources it guards, or an empty pass proves
	// nothing.
	if files < 1000 {
		t.Fatalf("scanned only %d files; the walk is not reaching the repo", files)
	}
	for _, want := range []rune{'»', '·', '▂', '🛡', '═'} {
		if _, ok := glyphs[want]; !ok {
			t.Fatalf("the scan did not find %q, which ships today; it cannot be trusted", want)
		}
	}

	var bad []string
	for r, where := range glyphs {
		out := ConvertToAscii(string(r))
		for i := 0; i < len(out); i++ {
			if out[i] >= 0x80 {
				bad = append(bad, strconv.QuoteRune(r)+" (first seen in "+where+") converts to "+strconv.Quote(out))
				break
			}
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d shipped glyphs have no ASCII form; add each to unicodeToAscii:\n%s",
			len(bad), strings.Join(bad, "\n"))
	}
}

// TestAsciiModeRendersPlaytestLines runs the exact lines the 2026-10-17
// playtests saw reach an ASCII client and checks every byte is ASCII.
func TestAsciiModeRendersPlaytestLines(t *testing.T) {
	lines := []string{
		"                   ·      ˙        ·          ˙       ·    ˙  ",
		"                   ▂▃▄▅▆▇▆▅▄▃▄▅▆▇▆▅▄▃▂▃▄▅▆▇▆▅▄▃▂▃▄▅▆",
		"˙    ·         *         ·    ˙",
		"🛡 SHIELD SLAM! Arena Champion catches your blow on their shield and slams you",
		"🛡 Block Crit -> SHIELD SLAM",
		"[* HP:#########. SP:#########. CP:########## Prone] » Steppe Hare[bruised] :",
		"❀       ˙      ✦        ❀        ·     ✿  ",
		"     ╲╿╱   ╲│╱    ╲╿╱    ╲│╱   ╲╿╱     ",
		"▀▄▀▚▄▀▄▚▀▄▀▚▄▀▄▚▀▄▀▚▄▀▄▚▀▄▀▚▄▀▄▚▀▄▀▚",
		"      ░▒▓▒░  ▟▙  ░▒▓▓▒░  ▟▙  ░▒▒░      ",
		"⠀⠀⢀⡤⠒⠒⠢⢄⡀⠀⠀⢠⡏⠉⠉⠉⠑⠒⠤⣀⠀⠀",
	}
	for _, in := range lines {
		out := ConvertToAscii(in)
		for i := 0; i < len(out); i++ {
			if out[i] >= 0x80 {
				t.Errorf("ConvertToAscii(%q) = %q: byte %d is 0x%x", in, out, i, out[i])
				break
			}
		}
	}
	if got := ConvertToAscii("🛡 SHIELD SLAM!"); got != "SHIELD SLAM!" {
		t.Errorf("shield emoji should drop with its space, got %q", got)
	}
	if got := ConvertToAscii("» Steppe Hare"); got != "> Steppe Hare" {
		t.Errorf("prompt pointer should read >, got %q", got)
	}
}
