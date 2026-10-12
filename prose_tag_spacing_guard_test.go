package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Words and tags need a space between them. A tag is invisible, so
// "you<ansi fg=...>FUMBLE</ansi>" renders "youFUMBLE". The dash rewrite once
// dropped these spaces in dozens of lines. The same rewrite also produced
// the joined words "youare", "theyare" and "weare".
var (
	wordTagJoin  = regexp.MustCompile(`[A-Za-z]<ansi fg=`)
	joinedAreRun = regexp.MustCompile(`\b(you|they|we)are\b`)
)

func TestProseTagSpacing(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud")
	scanned := 0
	for _, d := range proseDashDirs {
		base := filepath.Join(root, d)
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("%s: %v; the guard would check nothing there", base, err)
		}
		err := filepath.WalkDir(base, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				scanned++
				if wordTagJoin.MatchString(line) {
					t.Errorf("%s:%d word joined to a tag (needs a space): %s", path, n, strings.TrimSpace(line))
				}
				if joinedAreRun.MatchString(line) {
					t.Errorf("%s:%d joined words (you are / they are / we are): %s", path, n, strings.TrimSpace(line))
				}
			}
			return sc.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
