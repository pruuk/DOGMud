package items

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// The slam split (#541, owner rulings S2 and S4, 2026-10-16) gave each body
// its own combat-message file. These guards hold the two promises the split
// made: every line fits 80 visible columns, and the amorphous and elemental
// files name no body part those creatures lack.

// slamSplitFiles are the four files the split wrote or rewrote.
var slamSplitFiles = []ItemSubType{Slam, Trample, Lash, Blast}

// slamSplitMaxColumns is the hard wrap for player copy.
const slamSplitMaxColumns = 80

// slamSplitWidestTokens are the longest value each token can render. Names are
// measured at 20 characters. {stance}, {position} and {momentum} come from
// characters.Calculate*String (internal/characters/combat_tokens.go) and
// {bodypart} from combat.bodyParts (internal/combat/descriptions.go); neither
// package can be imported here, so the longest values are written out.
// {itemname} is filled per file from the species that use it.
var slamSplitWidestTokens = map[string]string{
	`{actor}`:     strings.Repeat(`A`, 20),
	`{actee}`:     strings.Repeat(`E`, 20),
	`{actortype}`: `user`,
	`{acteetype}`: `user`,
	`{stance}`:    `aggressive`,
	`{position}`:  `grappling on the ground`,
	`{momentum}`:  `on the defensive`,
	`{bodypart}`:  `right shoulder`,
	`{damage}`:    `999`,
}

var slamSplitTag = regexp.MustCompile(`</?ansi[^>]*>`)

var slamSplitToken = regexp.MustCompile(`\{[a-z]+\}`)

// slamSplitRecord is one line of one role, with where it lives.
type slamSplitRecord struct {
	where string
	text  string
}

// slamSplitRecords returns every role line of a combat-message file.
func slamSplitRecords(t *testing.T, sub ItemSubType) []slamSplitRecord {
	t.Helper()
	path := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "combat-messages", string(sub)+".yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Options map[string]map[string]map[string][]map[string]string `yaml:"options"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []slamSplitRecord
	for intensity, sections := range doc.Options {
		for section, tiers := range sections {
			for tier, recs := range tiers {
				for i, rec := range recs {
					for role, text := range rec {
						out = append(out, slamSplitRecord{
							where: strings.Join([]string{string(sub), intensity, section, tier, strconv.Itoa(i + 1), role}, `/`),
							text:  text,
						})
					}
				}
			}
		}
	}
	if len(out) < 300 {
		t.Fatalf("%s: only %d role lines decoded; the walk is broken, not the data", path, len(out))
	}
	return out
}

// slamSplitLongestItemName returns the longest unarmedname among the dogmud
// species whose natural_attack is sub.
func slamSplitLongestItemName(t *testing.T, sub ItemSubType) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "species", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no species files found: %v", err)
	}
	longest := ``
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		var sp struct {
			NaturalAttack string `yaml:"natural_attack"`
			UnarmedName   string `yaml:"unarmedname"`
		}
		if yaml.Unmarshal(raw, &sp) != nil || sp.NaturalAttack != string(sub) {
			continue
		}
		if utf8.RuneCountInString(sp.UnarmedName) > utf8.RuneCountInString(longest) {
			longest = sp.UnarmedName
		}
	}
	if longest == `` {
		t.Fatalf("no dogmud species uses %q", sub)
	}
	return longest
}

// slamSplitRender fills every token with its widest value and strips tags.
func slamSplitRender(t *testing.T, line, itemName string) string {
	t.Helper()
	out := strings.ReplaceAll(line, `{itemname}`, itemName)
	for tok, v := range slamSplitWidestTokens {
		out = strings.ReplaceAll(out, tok, v)
	}
	if left := slamSplitToken.FindString(out); left != `` {
		t.Errorf("token %s has no widest value in this guard: %s", left, line)
	}
	return slamSplitTag.ReplaceAllString(out, ``)
}

// S4: no spillover. Every line of the four files fits 80 visible columns with
// 20-character names, the longest unarmedname that uses the file and the
// longest value of every other token.
func TestSlamSplitLinesFitEightyColumns(t *testing.T) {
	for _, sub := range slamSplitFiles {
		itemName := slamSplitLongestItemName(t, sub)
		for _, r := range slamSplitRecords(t, sub) {
			out := slamSplitRender(t, r.text, itemName)
			if n := utf8.RuneCountInString(out); n > slamSplitMaxColumns {
				t.Errorf("%s renders %d columns with {itemname}=%q: %s", r.where, n, itemName, out)
			}
		}
	}
}

// The width check must be able to fail: a line just past 80 once rendered.
func TestSlamSplitWidthCheck_CanFail(t *testing.T) {
	line := `<ansi fg="{actortype}">{actor}</ansi> hurls their <ansi fg="item">{itemname}</ansi> at <ansi fg="{acteetype}">{actee}</ansi> while {position}.`
	if n := utf8.RuneCountInString(slamSplitRender(t, line, `servo-driven fists`)); n <= slamSplitMaxColumns {
		t.Fatalf("a %d-column line passed; the render is not filling tokens", n)
	}
}

// slamSplitForbiddenBodyParts are the body parts the slimes, fungal colonies
// and energy elementals lack (spec section 2).
var slamSplitForbiddenBodyParts = regexp.MustCompile(`(?i)\b(feet|foot|head|chest|shoulder|footwork)\b`)

// The amorphous and elemental bodies have no feet, head, chest or shoulder,
// so no line of lash.yaml or blast.yaml may name one. The actee's own body is
// only ever named through {bodypart}, which this check does not expand.
func TestSlamSplitLashAndBlastNameNoMissingBodyPart(t *testing.T) {
	for _, sub := range []ItemSubType{Lash, Blast} {
		for _, r := range slamSplitRecords(t, sub) {
			if w := slamSplitForbiddenBodyParts.FindString(r.text); w != `` {
				t.Errorf("%s names %q, which this body lacks: %s", r.where, w, r.text)
			}
		}
	}
}

func TestSlamSplitBodyPartCheck_CanFail(t *testing.T) {
	for _, line := range []string{
		`You plant your feet and surge at {actee}.`,
		`{actor} lowers their head.`,
		`Your footwork carries you clear.`,
		`{actor} drives a Shoulder into you!`,
	} {
		if slamSplitForbiddenBodyParts.FindString(line) == `` {
			t.Errorf("missed the body part in %q", line)
		}
	}
	for _, line := range []string{
		`You lash {actee} across their {bodypart}.`,
		`{actor} rears back, headlong into the fight.`,
	} {
		if w := slamSplitForbiddenBodyParts.FindString(line); w != `` {
			t.Errorf("flagged %q in %q, which names no missing body part", w, line)
		}
	}
}
