package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// mobnameArticlePattern matches an article written directly before a mobname
// tag. Below clear sight Anonymize swaps the whole tag body for "a figure", so
// "A <mobname>water-rat</mobname> emerges" reads "A a figure emerges" (#382
// playtest, room 4121). A creature named in flavour prose that is not a real
// mob in the room takes no identity tag at all.
var mobnameArticlePattern = regexp.MustCompile(`(?i)\b(a|an) <ansi fg="mobname`)

func TestDogmudContentHasNoArticleBeforeMobnameTag(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud")
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			if mobnameArticlePattern.MatchString(line) {
				rel, _ := filepath.Rel(filepath.Dir(here), path)
				t.Errorf("%s:%d: article before a mobname tag: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("scanned no YAML files; the walk is not reaching the world data")
	}
}
