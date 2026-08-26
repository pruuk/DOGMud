package actions

// surprise_uncontested_hit_guard_test.go — U10d Task 16, guard 3.
//
// THE BUG THIS GUARDS. The deleted actions.SurpriseAttack burst swung every
// equipped weapon at the target BEFORE combat was joined and applied damage
// with NO DEFENDER CONTEST AT ALL. Its own source said so:
//
//	"surprise attack has NO HIT RESOLUTION ... there is no defender term
//	 anywhere: the target's stats, skills and defences never enter. A surprise
//	 attack against a novice and against the Elemental King resolve
//	 identically."
//
// U10d replaced it with ONE opening strike inside the ordinary combat round,
// which crits only when it WINS its contest. The whole point of the redesign is
// that a defender now gets to answer, so the guard has to make an uncontested
// surprise hit unreachable rather than merely absent today.
//
// Three parts, in the style of the U6b site guards
// (internal/combat/contest_site_guard_test.go), whose lesson was that a guard
// enumerating CHANNELS only protects the channels somebody remembered to name —
// so guard 2 below enumerates raw-damage CALL SITES instead, and asserts BOTH
// directions (an unowned site fails; a stale allowlist row also fails, which is
// what catches a broken walk that would otherwise pass forever).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/require"
)

// ─── part 1: the engagement seam lands no damage ─────────────────────────────

// TestEngageAggroTypeLandsNoDamage is the behavioural half. The burst lived
// inside what is now EngageAggroType, and fired at exactly the moment this test
// reproduces: a hidden attacker, a live target, a free special-move cooldown.
// If anyone re-adds a pre-combat swing there, the target's health moves and
// this fails.
func TestEngageAggroTypeLandsNoDamage(t *testing.T) {
	room := newAggroTestRoom()

	attacker := newAggroAttackerMob(9820)
	addHiddenBuff(&attacker.Character)
	// A burst would have swung with these; fists were its fallback, so it lands
	// damage even with no weapon equipped.
	attacker.Character.Stats.Strength.ValueAdj = 100
	attacker.Character.Stats.Dexterity.ValueAdj = 100

	victim := newAggroAttackerMob(9821)
	victim.Character.HealthMax.Base = 500
	victim.Character.HealthMax.Recalculate()
	victim.Character.Health = 500
	victim.Character.StaminaMax.Base = 200
	victim.Character.StaminaMax.Recalculate()
	victim.Character.Stamina = 200

	require.True(t, attacker.Character.IsHidden(), "precondition: attacker is hidden")
	require.Zero(t, attacker.Character.Cooldowns["special-move"],
		"precondition: the special-move cooldown is free, so nothing refuses the opener")

	got := EngageAggroType(
		NewMobActorInRoom(attacker, room),
		NewMobActorInRoom(victim, room),
	)

	// Precondition, and it matters: if the opener were refused, "no damage"
	// would be true for an uninteresting reason.
	require.Equal(t, characters.SurpriseAttack, got,
		"precondition: this is the exact situation the deleted burst fired in")

	require.Equal(t, 500, victim.Character.Health,
		"typing an engagement as a surprise attack must not damage the target. The pre-combat "+
			"burst is deleted: the surprise is the OPENING STRIKE of the ordinary combat round, "+
			"which the defender gets to answer. An uncontested auto-hit has come back.")
	require.Equal(t, 200, victim.Character.Stamina,
		"the engagement seam must not touch the target's pools at all")
}

// ─── part 2: every raw-damage site is owned ──────────────────────────────────

// rawDamageSiteOwners is the allowlist: FILE:FUNC → what contest that damage
// sits behind. combat.CalcRawDamage is the single entry point to the damage
// pipeline (internal/combat/damage_pipeline.go), so any new way to hurt a
// character has to appear here — including a re-added surprise burst, which is
// how the deleted one computed its damage.
//
// A row must name the contest, or state plainly that the site is not an attack
// on another actor. "TODO" is not an owner. To add a contested attack, route it
// through combat.ResolveChannelAttack (the channel seam) and say so in the row.
var rawDamageSiteOwners = map[string]string{
	// Behind combat.ResolveChannelAttack (ChannelSocial), which runs the
	// defender's quell/defy set before the damage below.
	"internal/actions/combat_taunt.go:ExecuteTaunt":          "contested: ResolveChannelAttack(ChannelSocial) at combat_taunt.go",
	"internal/actions/combat_counter.go:executeCounterTaunt": "contested: ResolveChannelAttack(ChannelSocial) at combat_counter.go",

	// Behind combat.ResolveChannelAttack (ChannelRanged). Both sites in Throw
	// are downstream of the same per-target contest: the second is the ordinary
	// damage branch, the first is the FUMBLE branch, which harms the thrower.
	"internal/usercommands/throw.go:Throw": "contested: ResolveChannelAttack(ChannelRanged) per target (two sites: the fumble self-harm and the hit)",

	// Melee's best-of-all defence contest (runBestOfAllDefense) has already
	// settled by the time these build the swing's damage.
	"internal/combat/combat_helpers.go:buildDamageParams": "contested: melee's runBestOfAllDefense settles before the swing is priced",

	// The special-move seam runs its own contest through defenceContestRunner.
	"internal/combat/skill_moves.go:executeSkillMoveWithRunner": "contested: the skill-move seam's own defenceContestRunner",

	// Spell damage is computed here and consumed downstream of the spell's
	// contest; this function applies nothing itself.
	"internal/hooks/combat_shared_helpers.go:calcSpellDamageForCharacter": "pure calculation, consumed downstream of the spell contest",

	// Crit follow-up effects, which only exist because the round's contest was
	// already won decisively.
	"internal/hooks/combat_shared_helpers.go:applyCritEffects": "contested: only reachable from a crit the round's own contest produced",

	// Not attacks. No defender exists to contest them.
	"internal/combat/calculations.go:PowerScore":              "not an attack: PowerScore only SCORES a character, it applies no damage",
	"internal/usercommands/inventory.go:checkSpoiledGrenades": "not an attack: a spoiled grenade detonating in its owner's own pack, self-harm only",
}

// collectRawDamageSites walks every non-test .go file under internal/ and
// returns file:func keys for references (call OR function-value pass) to
// combat.CalcRawDamage, plus bare CalcRawDamage inside internal/combat.
//
// Defining FuncDecl names are not references: only function bodies and var
// initialisers are inspected. Comments are dropped by parser mode 0, so the
// several files that merely DISCUSS CalcRawDamage do not register.
func collectRawDamageSites(t *testing.T) (sites map[string][]string, scanned int) {
	t.Helper()
	sites = map[string][]string{}
	fset := token.NewFileSet()

	root := internalDirForGuard(t)
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err, "read %s", dir)
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
			// os.ReadDir + ParseFile rather than the deprecated parser.ParseDir
			// (SA1019; the lint gate is only-new-issues, so a new file using it
			// fails CI).
			file, err := parser.ParseFile(fset, path, nil, 0)
			require.NoError(t, err, "parse %s", path)
			scanned++

			rel := filepath.ToSlash(filepath.Join("internal",
				strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")))
			inCombat := strings.HasPrefix(rel, "internal/combat/")

			record := func(where string, pos token.Pos) {
				sites[rel+":"+where] = append(sites[rel+":"+where], fset.Position(pos).String())
			}

			inspect := func(where string, node ast.Node) {
				consumed := map[*ast.Ident]bool{}
				ast.Inspect(node, func(n ast.Node) bool {
					switch v := n.(type) {
					case *ast.SelectorExpr:
						consumed[v.Sel] = true
						if pkg, ok := v.X.(*ast.Ident); ok &&
							pkg.Name == "combat" && v.Sel.Name == "CalcRawDamage" {
							record(where, v.Pos())
						}
					case *ast.Ident:
						if !consumed[v] && inCombat && v.Name == "CalcRawDamage" {
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
						if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
							inspect("var "+vs.Names[0].Name, vs)
						}
					}
				}
			}
		}
	}
	walk(root)
	return sites, scanned
}

func TestEveryRawDamageSiteIsOwned(t *testing.T) {
	sites, scanned := collectRawDamageSites(t)

	// Vacuity floors (the U9 guard's lesson): a guard that scans nothing, or
	// finds nothing, passes no matter what the code does.
	require.GreaterOrEqual(t, scanned, 50,
		"guard parsed only %d files under internal/; the walk is broken and the guard is vacuous", scanned)
	// t.Errorf, not require: the stale-row loop below names WHICH site went
	// missing, and a fail-fast assertion here would hide it.
	if len(sites) < len(rawDamageSiteOwners) {
		t.Errorf("guard found %d raw-damage sites but the allowlist names %d; the reference "+
			"detection is broken", len(sites), len(rawDamageSiteOwners))
	}

	found := make([]string, 0, len(sites))
	for key := range sites {
		found = append(found, key)
	}
	sort.Strings(found)

	for _, key := range found {
		if _, ok := rawDamageSiteOwners[key]; !ok {
			t.Errorf("NEW raw-damage site %s (at %s): every way to hurt a character must name the "+
				"contest it sits behind. If it is an attack, route it through "+
				"combat.ResolveChannelAttack so the defender answers, then add a row to "+
				"rawDamageSiteOwners — an uncontested attack is exactly what the deleted surprise "+
				"burst was.", key, strings.Join(sites[key], ", "))
		}
	}
	for key, owner := range rawDamageSiteOwners {
		if _, ok := sites[key]; !ok {
			t.Errorf("stale allowlist entry %s (owner %q): no such raw-damage site exists any more — "+
				"delete the row, or the walk failed to see it and this guard is not protecting anything",
				key, owner)
		}
	}
}

// ─── part 3: the deleted burst's own symbols ─────────────────────────────────

// deletedBurstIdentifiers are the names the uncontested burst was built from.
// The types and knobs are gone from the source, so a plain reference would fail
// the build — this catches them being re-DECLARED and re-consumed in one
// change, which compiles fine and is precisely how a deleted feature comes
// back.
var deletedBurstIdentifiers = []string{
	"SurpriseAttackOpts",
	"SurpriseAttackResult",
	"SurpriseAttackOffhandPenalty",
	"SurpriseAttackExtraArm1Penalty",
	"SurpriseAttackExtraArm2Penalty",
	"SurpriseAttackExtraArm3Penalty",
	"SurpriseAttackExtraArm4Penalty",
}

func TestDeletedSurpriseBurstSymbolsStayDeleted(t *testing.T) {
	root := internalDirForGuard(t)
	scanned := 0

	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err, "read %s", dir)
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
			require.NoError(t, err, "read %s", path)
			scanned++
			for _, ident := range deletedBurstIdentifiers {
				if idx := strings.Index(string(src), ident); idx >= 0 {
					t.Errorf("%s mentions %s (byte %d): the pre-combat surprise burst was deleted by "+
						"U10d because it applied damage with no defender contest. The surprise is the "+
						"opening strike of the ordinary combat round now.", path, ident, idx)
				}
			}
		}
	}
	walk(root)

	require.GreaterOrEqual(t, scanned, 50,
		"identifier scan read only %d files; the walk is broken and the guard is vacuous", scanned)
}
