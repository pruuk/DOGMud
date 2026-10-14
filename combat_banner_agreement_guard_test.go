package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

// Rows 26-27 of the M6 ledger (#370): the record layout stops a line pairing
// with the wrong blow by position, but a record can still be written with
// mismatched roles. The bold banner word (SHRED, FLAWLESS BITE) is the one
// part of a strong hit that every role repeats, so it must agree across the
// roles of a slot.

// bannerGuardFiles are the converted combat-message files. Each batch adds its
// files; the last batch replaces this list with every file in the store.
var bannerGuardFiles = []string{"bite", "claws", "gore", "maul", "slashing", "bludgeoning", "stabbing", "slam", "throttle", "cleaving", "whipping", "pounce", "sting", "staff", "sceptre", "wand", "siphon"}

var bannerTag = regexp.MustCompile(`<ansi fg="[a-z]+-bold">([^<]+)</ansi>`)

func bannerWords(line string) []string {
	var out []string
	for _, m := range bannerTag.FindAllStringSubmatch(line, -1) {
		out = append(out, strings.Fields(strings.ToUpper(m[1]))...)
	}
	return out
}

// wordsAgree: the shorter word (three letters or more) is a prefix of the
// longer, or both share their first five letters. SHRED/SHREDS and
// EVISCERATE/EVISCERATION agree; CRUSHES/DEVASTATED do not.
func wordsAgree(x, y string) bool {
	s, l := x, y
	if len(s) > len(l) {
		s, l = l, s
	}
	if len(s) >= 3 && strings.HasPrefix(l, s) {
		return true
	}
	return len(x) >= 5 && len(y) >= 5 && x[:5] == y[:5]
}

func bannersAgree(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !wordsAgree(a[i], b[i]) {
			return false
		}
	}
	return true
}

// separateMayOmitBanner names the roles that may carry no banner in a
// separate split: the attacker's room watches the attack leave through the
// exit and cannot see it land (owner 2026-10-14). A banner it does carry must
// still agree with the actor's.
var separateMayOmitBanner = map[string]bool{"observer": true}

// slotFailures compares every role of each slot against the actor's line.
// Roles in mayOmit may carry no banner at all.
func slotFailures(where string, roles map[string][]string, mayOmit map[string]bool) []string {
	var out []string
	actor := roles["actor"]
	for i := range actor {
		want := bannerWords(actor[i])
		for role, lines := range roles {
			if role == "actor" || i >= len(lines) {
				continue
			}
			got := bannerWords(lines[i])
			if len(got) == 0 && mayOmit[role] {
				continue
			}
			if (len(want) > 0 || len(got) > 0) && !bannersAgree(want, got) {
				out = append(out, fmt.Sprintf("%s[%d] %s %v against actor %v", where, i, role, got, want))
			}
		}
	}
	return out
}

func bannerFailures(name string, g *items.WeaponAttackMessageGroup) []string {
	var out []string
	tiers := []struct {
		name string
		get  func(items.SkillTieredMessages) items.MessageOptions
	}{
		{"beginner", func(s items.SkillTieredMessages) items.MessageOptions { return s.Beginner }},
		{"expert", func(s items.SkillTieredMessages) items.MessageOptions { return s.Expert }},
		{"master", func(s items.SkillTieredMessages) items.MessageOptions { return s.Master }},
	}
	strs := func(m items.MessageOptions) []string {
		r := make([]string, len(m))
		for i, x := range m {
			r[i] = string(x)
		}
		return r
	}
	for intensity, opts := range g.Options {
		for _, tier := range tiers {
			out = append(out, slotFailures(fmt.Sprintf("%s %s together %s", name, intensity, tier.name), map[string][]string{
				"actor":    strs(tier.get(opts.Together.ToAttacker)),
				"actee":    strs(tier.get(opts.Together.ToDefender)),
				"observer": strs(tier.get(opts.Together.ToRoom)),
			}, nil)...)
			out = append(out, slotFailures(fmt.Sprintf("%s %s separate %s", name, intensity, tier.name), map[string][]string{
				"actor":           strs(tier.get(opts.Separate.ToAttacker)),
				"actee":           strs(tier.get(opts.Separate.ToDefender)),
				"observer":        strs(tier.get(opts.Separate.ToAttackerRoom)),
				"remote_observer": strs(tier.get(opts.Separate.ToDefenderRoom)),
			}, separateMayOmitBanner)...)
		}
	}
	return out
}

func TestCombatBannerWordsAgreeAcrossRoles(t *testing.T) {
	for _, name := range bannerGuardFiles {
		path := filepath.Join("_datafiles", "world", "dogmud", "combat-messages", name+".yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var g items.WeaponAttackMessageGroup
		if err := yaml.Unmarshal(raw, &g); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, f := range bannerFailures(name, &g) {
			t.Errorf("banner mismatch: %s", f)
		}
		if n := bannerRecordCount(&g); n == 0 {
			t.Errorf("%s: no record carries a banner word, the guard checked nothing", name)
		}
	}
}

// bannerRecordCount counts the actor lines (one per record) holding at least
// one banner word, so a file the guard cannot see into fails loudly.
func bannerRecordCount(g *items.WeaponAttackMessageGroup) int {
	n := 0
	for _, opts := range g.Options {
		for _, s := range []items.SkillTieredMessages{opts.Together.ToAttacker, opts.Separate.ToAttacker} {
			for _, tier := range []items.MessageOptions{s.Beginner, s.Expert, s.Master} {
				for _, line := range tier {
					if len(bannerWords(string(line))) > 0 {
						n++
					}
				}
			}
		}
	}
	return n
}

func TestBannerWordsAgreeRule(t *testing.T) {
	agree := [][2]string{{"SHRED", "SHREDS"}, {"MAUL", "MAULED"}, {"EVISCERATE", "EVISCERATION"}, {"TEAR", "TEARS"}, {"HEW", "HEWS"}}
	differ := [][2]string{{"CRUSHES", "DEVASTATED"}, {"CHOP", "CLEAVE"}, {"DEVASTATING", "FLAWLESS"}}
	for _, p := range agree {
		if !wordsAgree(p[0], p[1]) {
			t.Errorf("%s and %s should agree", p[0], p[1])
		}
	}
	for _, p := range differ {
		if wordsAgree(p[0], p[1]) {
			t.Errorf("%s and %s should differ", p[0], p[1])
		}
	}
}

func TestBannerGuardCatchesAMismatchedSlot(t *testing.T) {
	b := func(w string) string { return `x <ansi fg="cyan-bold">` + w + `</ansi>!` }
	roles := map[string][]string{
		"actor":    {b("DEVASTATING CRITICAL"), b("SHRED")},
		"actee":    {b("FLAWLESS BITE"), b("SHREDS")},
		"observer": {b("FLAWLESS BITE"), "no banner here"},
	}
	got := slotFailures("probe", roles, nil)
	if len(got) != 3 {
		t.Fatalf("want 3 failures (two roles at [0], the missing banner at [1]), got %d: %v", len(got), got)
	}
}

// In a separate split the attacker's room watches the shot leave and cannot
// see it land, so its line may carry no banner (owner 2026-10-14); a banner it
// does carry must still agree.
func TestBannerGuardLetsTheAttackerRoomOmitInSeparate(t *testing.T) {
	b := func(w string) string { return `x <ansi fg="cyan-bold">` + w + `</ansi>!` }
	roles := map[string][]string{
		"actor":           {b("DEVASTATES"), b("SHRED")},
		"actee":           {b("DEVASTATES"), b("SHREDS")},
		"observer":        {"fires through the exit", b("FLAWLESS")},
		"remote_observer": {b("DEVASTATES"), b("SHREDS")},
	}
	got := slotFailures("probe", roles, separateMayOmitBanner)
	if len(got) != 1 {
		t.Fatalf("want 1 failure (the wrong banner at [1]; the missing one at [0] is allowed), got %d: %v", len(got), got)
	}
	if got := slotFailures("probe", map[string][]string{
		"actor":           {b("DEVASTATES")},
		"actee":           {b("DEVASTATES")},
		"observer":        {b("DEVASTATES")},
		"remote_observer": {"a shot lands"},
	}, separateMayOmitBanner); len(got) != 1 {
		t.Fatalf("the defender's room sees the hit and must keep the banner, got %d: %v", len(got), got)
	}
}
