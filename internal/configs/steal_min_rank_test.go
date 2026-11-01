package configs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// repoFile anchors a path on this test file, not the process CWD.
func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(append([]string{filepath.Dir(here), "..", ".."}, parts...)...)
}

// shippedBalance reads the Balance block of the shipped config.yaml.
func shippedBalance(t *testing.T) Balance {
	t.Helper()
	data, err := os.ReadFile(repoFile(t, "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	return cfg.Balance
}

// StealMinRank (#375 phase 7) is the skullduggery rank every theft needs,
// players and mobs alike. It ships at 1: every character starts at rank 1.
// Absent (zero) or below 1 reads as 1, since rank 0 is no rank at all.
func TestStealMinRank_ShippedAndDefaulted(t *testing.T) {
	if got := shippedBalance(t).StealMinRank; got != 1 {
		t.Fatalf("shipped StealMinRank is %d, want 1", got)
	}
	if defaultStealMinRank != 1 {
		t.Fatalf("defaultStealMinRank is %d, want 1", defaultStealMinRank)
	}
	var absent Balance
	absent.validateCombatSteal()
	if absent.StealMinRank != 1 {
		t.Fatalf("an absent key reads %d, want 1", absent.StealMinRank)
	}
	negative := Balance{StealMinRank: -3}
	negative.validateCombatSteal()
	if negative.StealMinRank != 1 {
		t.Fatalf("a negative value reads %d, want 1", negative.StealMinRank)
	}
	kept := Balance{StealMinRank: 4}
	kept.validateCombatSteal()
	if kept.StealMinRank != 4 {
		t.Fatalf("a set value must be kept: %d", kept.StealMinRank)
	}
}
