package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/keywords"
)

// TestPartyRanksAreHidden holds the owner's #295 call (2026-10-17): the
// backrank/frontrank feature is disabled and hidden until positioning is
// designed (#600). The two aliases pointed at a `rank` command that never
// existed, so nothing a player can type or read may name party ranks: no
// alias resolves to `rank`, no help topic is one, and no help text mentions
// a front or back rank.
func TestPartyRanksAreHidden(t *testing.T) {
	saved := configs.GetConfig()
	cfg := saved
	cfg.FilePaths.DataFiles = configs.ConfigString(dogmudDataFilesRoot)
	configs.SetConfigForTest(t, cfg)
	keywords.LoadAliases()
	t.Cleanup(func() { configs.SetConfigForTest(t, saved); keywords.LoadAliases() })

	aliases := keywords.GetAllCommandAliases()
	if len(aliases) < 50 {
		t.Fatalf("loaded %d command aliases; the shipped keywords.yaml did not load, so this check proves nothing", len(aliases))
	}
	for _, typed := range []string{"backrank", "frontrank", "rank"} {
		if got := keywords.TryCommandAlias(typed); got != typed {
			t.Errorf("%q still resolves to %q", typed, got)
		}
	}
	for alias, cmd := range aliases {
		if cmd == "rank" || strings.HasPrefix(cmd, "rank ") {
			t.Errorf("alias %q points at the rank command %q", alias, cmd)
		}
	}
	for _, topic := range keywords.GetAllHelpTopics() {
		switch strings.ToLower(topic) {
		case "rank", "backrank", "frontrank":
			t.Errorf("help lists the topic %q", topic)
		}
	}

	helps, err := filepath.Glob(filepath.Join(dogmudDataFilesRoot, "templates", "help", "*.template"))
	if err != nil || len(helps) < 50 {
		t.Fatalf("found %d help templates (err %v); the scan cannot run", len(helps), err)
	}
	for _, f := range helps {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		low := strings.ToLower(string(b))
		for _, w := range []string{"backrank", "frontrank", "front-rank", "back-rank", "front rank", "back rank"} {
			if strings.Contains(low, w) {
				t.Errorf("%s mentions %q", filepath.ToSlash(f), w)
			}
		}
	}
}
