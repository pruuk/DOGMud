package mobs

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Charm's protection for shopkeepers is not written anywhere in charm's own
// code. It is inherited twice over: charm is type: harmsingle, so InitiateCast
// routes it through rejectHarmTarget -> CheckPlayerHarm -> HarmBlockedNonCombatant
// for any non_combatant mob, and CharmImmune is a second gate inside
// applyMobEffect_charm.
//
// Both gates are per-mob DATA. A shopkeeper authored with neither flag is
// charmable, and nothing else in the tree would notice: the code is fine, the
// tests are green, and a player walks off with the merchant.
//
// 97 mobs carry a shop block today and every one of them is covered. This test
// exists entirely for the NEXT one. Spec 11.3.3 names it as the deliverable.
func TestShopMobsCannotBeCharmed(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// Anchor on the source file, never on the working directory: test binaries
	// in this repo do not reliably run with the package dir as CWD (internal/
	// actions/economy_test.go chdirs to the repo root and every test in a
	// package shares one binary), so a relative path passes or fails by order.
	mobsDir := filepath.Join(filepath.Dir(thisFile), "..", "..",
		"_datafiles", "world", "dogmud", "mobs")

	if _, err := os.Stat(mobsDir); err != nil {
		t.Fatalf("mob data directory not found at %s: %v", mobsDir, err)
	}

	var shopMobs, unprotected []string

	err := filepath.Walk(mobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		// Line-oriented on purpose: a shop block is a top-level key, and a full
		// YAML parse would drag the whole MobSpec schema (and its validation)
		// into a test that only asks one question about three keys.
		var covered, charmImmune, nonCombatant bool
		for _, line := range strings.Split(string(raw), "\n") {
			// The shop block is INDENTED under a parent key, so match the
			// trimmed line rather than a prefix. A prefix match silently found
			// zero shop mobs, and the empty-set guard below is what caught it.
			trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))

			// A noncombat archetype is the second population that must not be
			// charmable. A 2026-08-24 playtest charmed the Thornwall street
			// performer in the middle of the city and walked it away: its
			// archetype is noncombat_questgiver and it is hostile: false, but it
			// carried neither flag, so nothing stopped it. It and the city
			// beggar were the only two of 383 that had been missed.
			if strings.HasPrefix(trimmed, "behavior_archetype: noncombat") {
				covered = true
			}

			switch trimmed {
			case "shop:":
				covered = true
			case "charm_immune: true":
				charmImmune = true
			case "non_combatant: true":
				nonCombatant = true
			}
		}
		if !covered {
			return nil
		}

		rel, _ := filepath.Rel(mobsDir, path)
		shopMobs = append(shopMobs, rel)
		if !charmImmune && !nonCombatant {
			unprotected = append(unprotected, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", mobsDir, err)
	}

	if len(shopMobs) == 0 {
		t.Fatal("found no mobs to check, so this test is not testing anything -- " +
			"the walk or the key match is wrong")
	}

	if len(unprotected) > 0 {
		sort.Strings(unprotected)
		t.Errorf("%d of %d shop or noncombat-archetype mobs can be CHARMED. A "+
			"merchant that can be charmed can be walked out of its own shop, and "+
			"a noncombat townsperson can be marched around the city as a "+
			"puppet.\n\nAdd charm_immune: true (or non_combatant: true if it "+
			"should not fight at all) to:\n  %s",
			len(unprotected), len(shopMobs), strings.Join(unprotected, "\n  "))
	}
}
