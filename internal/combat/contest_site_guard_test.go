package combat

// The three U6b Task 18 guards (spec §9). U6 was declared done while its own
// criteria were false, and nothing detected it for four slices — because the
// only guards that existed enumerated CHANNELS, and a channel guard only
// protects channels somebody remembered to name. That is how spell's ×0
// defence survived U6, and how flee, grapple, drift, submission, throw and
// steal survived four slices without appearing on any roadmap row.
//
// Guard 1 therefore enumerates combat.RunContest / contest.Run CALL SITES,
// not channels: every reference must be on an allowlist naming the slice that
// owns it. Guard 2 asserts the skill weight is the ONE Balance.SkillWeight on
// every defence score builder. Guard 3 greps the converted family's source
// for the dead literal weights themselves — knob-greps cannot see literals,
// which is how flee's ×25 hid for the whole arc.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ─── Guard 1: every contest site is owned ────────────────────────────────────

// contestSiteOwners is the allowlist: FILE:FUNC → the slice that owns the
// site. Keys are file:func (spec §9), NOT files — a file-level allowlist
// hides new unowned sites inside allowlisted files, and steal.go/plant.go
// each hold four contests. A reference that appears outside any function
// (a package-level var initialiser) is keyed "file:var <name>".
//
// To add a new opposed contest: route it through combat.ResolveChannelAttack
// (the channel seam) so defence sets, costs, progression, crit and narration
// all arrive for free. Only a contest that genuinely is not a channel attack
// gets a row here, and the row must name the slice that owns converting or
// keeping it — "TODO" is not an owner.
var contestSiteOwners = map[string]string{
	// The seams themselves.
	"internal/combat/run_contest.go:RunContest":                            "the floor seam itself — the ONE place Balance.ContestFloor is read",
	"internal/combat/run_concentration_contest.go:RunConcentrationContest": "the concentration floor seam — the ONE place Balance.ConcentrationFloor is read (U10)",
	"internal/combat/defence_multiplier.go:var channelAttackContestRunner": "the channel seam itself — the contest core behind ResolveChannelAttack",
	"internal/combat/combat_helpers.go:runBestOfAllDefense":                "melee's best-of-all defence core (U6; crit bar unified by U6b task 1)",
	"internal/contest/contest.go:AgainstDifficulty":                        "the contest core itself (difficulty checks share Run's crit semantics)",
	"internal/contest/contest.go:RunWithFloors":                            "the contest core itself (the floor wrapper over Run)",

	// The family of seven, converted by U6b (plus the plan's pre-assigned
	// owners for the sites the review found outside the family — these owner
	// strings come from the plan, not from the implementer).
	"internal/combat/flee.go:ResolveFleeBlockers":                            "U6b task 12 (both the mob-blocker and player-blocker loops)",
	"internal/combat/grapple.go:AttemptGrapple":                              "U6b task 13",
	"internal/combat/submission.go:RollSubmissionAttempt":                    "U6b task 13",
	"internal/hooks/Position_GrappleTick.go:processGrapplePair":              "U6b task 14 (grapple drift — RunContest passed to processGrapplePairWithContest)",
	"internal/actions/plant.go:plantOnMob":                                   "U6b task 15",
	"internal/actions/plant.go:plantOnPlayer":                                "U6b task 15 (two sites: plant roll + observer pass)",
	"internal/actions/plant.go:plantInContainer":                             "U6b task 15",
	"internal/actions/steal.go:stealFromMob":                                 "U6b task 15",
	"internal/actions/steal.go:stealFromPlayer":                              "U6b task 15 (two sites: steal roll + observer pass)",
	"internal/actions/steal.go:stealFromContainer":                           "U6b task 15 (the container-theft observer pass)",
	"internal/actions/sneak.go:Sneak":                                        "U6b task 16 (two sites)",
	"internal/actions/shadow.go:shadowPlayer":                                "U6b task 16",
	"internal/usercommands/skill.skullduggery.shadow.go:shadowDetectionRoll": "U6b task 16",
	"internal/usercommands/go.go:Go":                                         "U6b task 16 (hidden detection on room entry, four sites)",

	// Deliberately unconverted, with the plan's pre-assigned owners.
	"internal/actions/defuse.go:Defuse":                         "deliberate: trap-difficulty contest, converted U4",
	"internal/hooks/NewRound_MobRoundTick.go:tickMobCharmState": "U10c (charm-refresh; redesigned wholesale there)",
	"internal/hooks/charm_spell.go:resolveCharmSpell":           "U10c (the charm-cast contest; the charm family is redesigned wholesale there)",
}

// contestCoreSelectors are the internal/contest entry points a selector call
// can reach. RunWithFloors matters as much as Run: it is the floored core,
// and a new production caller bypassing combat.RunContest would silently
// hardcode its own floor.
var contestCoreSelectors = map[string]bool{
	"Run":               true,
	"RunWithFloors":     true,
	"AgainstDifficulty": true,
}

// collectContestSites walks every non-test .go file under internal/ and
// returns file:func keys for every reference (call OR function-value pass —
// Position_GrappleTick hands combat.RunContest to its runner seam, and a
// call-only guard would miss that shape) to:
//
//   - combat.RunContest / contest.Run / contest.RunWithFloors /
//     contest.AgainstDifficulty selectors, anywhere;
//   - bare RunContest inside internal/combat (its own package);
//   - bare Run / RunWithFloors / AgainstDifficulty inside internal/contest.
//
// Defining FuncDecl names are not references and are skipped by inspecting
// only function bodies.
func collectContestSites(t *testing.T) (sites map[string][]string, scanned int) {
	t.Helper()
	sites = map[string][]string{}
	fset := token.NewFileSet()

	root := filepath.Join("..", "..", "internal")
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				walk(path)
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			// os.ReadDir + ParseFile rather than parser.ParseDir: the latter
			// is deprecated (SA1019) and the lint gate is only-new-issues, so
			// a new file using it fails CI (this exact mistake broke a U9
			// push). We only need syntax trees, not package association.
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			scanned++

			rel := filepath.ToSlash(filepath.Join("internal",
				strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")))
			inCombat := strings.HasPrefix(rel, "internal/combat/")
			inContest := strings.HasPrefix(rel, "internal/contest/")

			record := func(where string, pos token.Pos) {
				key := rel + ":" + where
				sites[key] = append(sites[key], fset.Position(pos).String())
			}

			inspect := func(where string, node ast.Node) {
				// Idents consumed as the Sel of a SelectorExpr must not also
				// count as bare references.
				consumed := map[*ast.Ident]bool{}
				ast.Inspect(node, func(n ast.Node) bool {
					switch v := n.(type) {
					case *ast.SelectorExpr:
						consumed[v.Sel] = true
						pkg, ok := v.X.(*ast.Ident)
						if !ok {
							return true
						}
						if pkg.Name == "combat" && v.Sel.Name == "RunContest" {
							record(where, v.Pos())
						}
						if pkg.Name == "contest" && contestCoreSelectors[v.Sel.Name] {
							record(where, v.Pos())
						}
					case *ast.Ident:
						if consumed[v] {
							return true
						}
						if inCombat && v.Name == "RunContest" {
							record(where, v.Pos())
						}
						if inContest && contestCoreSelectors[v.Name] {
							record(where, v.Pos())
						}
					}
					return true
				})
			}

			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Body != nil {
						inspect(d.Name.Name, d.Body)
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Names) == 0 {
							continue
						}
						inspect("var "+vs.Names[0].Name, vs)
					}
				}
			}
		}
	}
	walk(root)
	return sites, scanned
}

// TestEveryContestSiteIsOwned fails on any contest reference that is not on
// the allowlist above, and on any allowlist row whose site no longer exists —
// the second direction is what catches a broken walk, which would otherwise
// pass forever while protecting nothing.
func TestEveryContestSiteIsOwned(t *testing.T) {
	sites, scanned := collectContestSites(t)

	// Vacuity floors (the U9 guard's lesson): a guard that scans nothing or
	// finds nothing passes no matter what the code does.
	if scanned < 50 {
		t.Fatalf("guard parsed only %d files under internal/; the walk is broken and the guard is vacuous", scanned)
	}
	if len(sites) < len(contestSiteOwners) {
		t.Errorf("guard found %d contest sites but the allowlist names %d; the reference detection is broken",
			len(sites), len(contestSiteOwners))
	}

	found := make([]string, 0, len(sites))
	for key := range sites {
		found = append(found, key)
	}
	sort.Strings(found)

	for _, key := range found {
		if _, ok := contestSiteOwners[key]; !ok {
			t.Errorf("NEW contest site %s (at %s): route it through combat.ResolveChannelAttack (the channel seam), "+
				"or add an entry to contestSiteOwners naming the slice that owns it — an unowned contest is how "+
				"spell's ×0 defence survived U6 for four slices",
				key, strings.Join(sites[key], ", "))
		}
	}
	for key, owner := range contestSiteOwners {
		if _, ok := sites[key]; !ok {
			t.Errorf("stale allowlist entry %s (owner %q): no such contest site exists any more — delete the row, "+
				"or the walk failed to see it and this guard is not protecting anything", key, owner)
		}
	}
}

// ─── Guard 2: one SkillWeight on every defence score ─────────────────────────

// TestEveryChannelUsesUniformDefenceSkillWeight asserts, behaviourally, that
// every defence score's skill term is rank × Balance.SkillWeight — the same
// knob on every channel. Channels differ only in WHICH defences answer them
// (DefenceEntriesFor); every defence scores through GetDefenseScoreFor, so a
// per-channel weight divergence must surface here as a delta that is not
// rank × SkillWeight. The flee family's two in-package score builders are
// driven too — they are contest scores that do not pass through
// GetDefenseScoreFor. (The actions-package builders — sneak, steal, plant,
// detection — cannot be imported from here without a cycle; their linear
// SkillWeight shape is pinned by their own package tests, and guard 3 pins
// the literals they must not regrow.)
func TestEveryChannelUsesUniformDefenceSkillWeight(t *testing.T) {
	weight := float64(configs.GetBalanceConfig().SkillWeight)
	if weight <= 0 {
		t.Fatalf("Balance.SkillWeight is %v; validation should have replaced a non-positive value", weight)
	}

	const rank = 37 // deliberately not a round multiple of anything shipped

	build := func(skill skills.SkillTag, level int) *characters.Character {
		c := &characters.Character{
			Name:   "guard",
			Buffs:  buffs.New(),
			Skills: map[string]int{string(skill): level},
		}
		c.Stats.Dexterity.ValueAdj = 100
		c.Stats.Strength.ValueAdj = 100
		c.Stats.Willpower.ValueAdj = 100
		return c
	}

	defences := []struct {
		defence string
		skill   skills.SkillTag
	}{
		{characters.DefenseDodge, skills.UnarmedCombat},
		{characters.DefenseParry, skills.WeaponCombat},
		{characters.DefenseBlock, skills.WeaponCombat},
		{characters.DefenseQuell, skills.Spellcasting},
		{characters.DefenseDefy, skills.Rhetoric},
	}
	for _, tc := range defences {
		zero := build(tc.skill, 0).GetDefenseScoreFor(tc.defence, true)
		ranked := build(tc.skill, rank).GetDefenseScoreFor(tc.defence, true)
		got := ranked - zero
		want := float64(rank) * weight
		if diff := got - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: rank %d adds %v to the defence score, want rank × SkillWeight = %v — "+
				"a per-channel skill weight has diverged from the ONE Balance.SkillWeight",
				tc.defence, rank, got, want)
		}
	}

	// The flee contest's two sides (U6b task 12).
	fleeBuilders := []struct {
		name  string
		skill skills.SkillTag
		score func(c *characters.Character) float64
	}{
		{"fleeContestScore", skills.Skullduggery, func(c *characters.Character) float64 { return fleeContestScore(c, true) }},
		{"fleeBlockScore", skills.UnarmedCombat, fleeBlockScore},
	}
	for _, tc := range fleeBuilders {
		zero := tc.score(build(tc.skill, 0))
		ranked := tc.score(build(tc.skill, rank))
		got := ranked - zero
		want := float64(rank) * weight
		if diff := got - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: rank %d adds %v, want rank × SkillWeight = %v — flee has regrown its own weight",
				tc.name, rank, got, want)
		}
	}
}

// ─── Guard 3: no legacy skill-weight literal survives ────────────────────────

// The converted family's files. SubSkillWeight is a knob and knob-greps found
// it, but flee's ×25 and drift's ×2.2 were LITERALS, invisible to any config
// audit — so this guard scans source text, spacing-tolerant (flee's third
// literal was written "* 25", and the plan's own first draft missed it with a
// tight pattern).
//
// Deliberately NOT listed, each with the reason it keeps its literal:
//   - internal/actions/skill_helpers.go — CalcSearchScore keeps its
//     SkillMultiplier×25 shape for the Category B consumers (forage, search,
//     track), walled off by spec §3.2.
//   - internal/actions/defuse.go — the trap-difficulty contest's ×25, owned
//     "converted U4" on the guard-1 allowlist.
//   - internal/hooks/NewRound_MobRoundTick.go — the charm-refresh ×25, owned
//     by U10c, which redesigns it wholesale.
//   - internal/combat/calculations.go — SkillMultiplier's own sqrt curve.
var legacyLiteralFiles = []string{
	"internal/combat/flee.go",
	"internal/combat/grapple.go",
	"internal/combat/submission.go",
	"internal/combat/skill_moves.go",
	"internal/hooks/Position_GrappleTick.go",
	"internal/actions/steal.go",
	"internal/actions/plant.go",
	"internal/actions/sneak.go",
	"internal/actions/shadow.go",
	"internal/usercommands/go.go",
	"internal/usercommands/skill.skullduggery.shadow.go",
	"internal/usercommands/throw.go",
}

// The dead weights. \s* tolerates the spaced forms; \b stops ×250 and ×25.0x
// from matching while still catching bare "*25", "* 25" and "* 25.0".
var legacyLiteralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\*\s*25(\.0)?\b`), // the old flat skill weight (flee, steal, sneak, shadow, detection)
	regexp.MustCompile(`\*\s*2\.2\b`),     // drift's old skill coefficient
}

// Drift's other dead coefficient. ×2.0 is too common a multiplier to ban
// world-wide, so it is banned only in the file where it was the drift coef.
var legacyDriftPattern = regexp.MustCompile(`\*\s*2\.0\b`)

// Deleted knob identifiers. The knobs are gone from the config structs, so a
// reference would fail the build anyway — this catches the knob being
// re-DECLARED and re-consumed in one change, which compiles fine.
var legacyIdentifiers = []string{
	"SubSkillWeight",
	"StealSkillMultiplier",
	"SpellAttackSkillFactor",
}

func TestNoLegacySkillWeightLiteralSurvives(t *testing.T) {
	root := filepath.Join("..", "..")

	// Literal scan over the converted family's files.
	for _, rel := range legacyLiteralFiles {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			// A missing file is a fatal guard defect, not a pass: the file
			// list has drifted and the guard is no longer scanning what it
			// claims to.
			t.Fatalf("guard cannot read %s: %v — update legacyLiteralFiles", rel, err)
		}
		text := string(src)
		for _, pattern := range legacyLiteralPatterns {
			if loc := pattern.FindStringIndex(text); loc != nil {
				t.Errorf("%s matches %q at byte %d (%q): the legacy hardcoded skill weight is back — "+
					"use Balance.SkillWeight, the ONE knob",
					rel, pattern.String(), loc[0], snippetAround(text, loc[0]))
			}
		}
	}
	driftFile := "internal/hooks/Position_GrappleTick.go"
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(driftFile)))
	if err != nil {
		t.Fatalf("guard cannot read %s: %v", driftFile, err)
	}
	if loc := legacyDriftPattern.FindStringIndex(string(src)); loc != nil {
		t.Errorf("%s matches %q at byte %d (%q): drift's dead ×2.0 coefficient is back",
			driftFile, legacyDriftPattern.String(), loc[0], snippetAround(string(src), loc[0]))
	}

	// Deleted-identifier scan over everything guard 1 walks.
	scanned := 0
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				walk(path)
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			scanned++
			for _, ident := range legacyIdentifiers {
				if idx := strings.Index(string(src), ident); idx >= 0 {
					t.Errorf("%s mentions deleted knob %s (byte %d): the per-channel weight knobs died with U6b — "+
						"there is ONE Balance.SkillWeight", path, ident, idx)
				}
			}
		}
	}
	walk(filepath.Join(root, "internal"))
	if scanned < 50 {
		t.Fatalf("identifier scan read only %d files; the walk is broken and the guard is vacuous", scanned)
	}
}

// snippetAround returns a short context window for a match report.
func snippetAround(text string, at int) string {
	start := at - 20
	if start < 0 {
		start = 0
	}
	end := at + 20
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}
