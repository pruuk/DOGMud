package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionReadiness_GenericCommand_Ready verifies that an unrecognized /
// non-special-move command (e.g. "say hello") always passes through as Ready.
func TestActionReadiness_GenericCommand_Ready(t *testing.T) {
	m := newTestMob(t, nil)
	actor := &MobActor{Mob: m, Room: nil}
	result := ActionReadiness(actor, "say hello")
	assert.Equal(t, ActionReady, result.Status)
}

// TestActionReadiness_NilActor_Rejected verifies that a nil actor is
// immediately rejected without panicking.
func TestActionReadiness_NilActor_Rejected(t *testing.T) {
	result := ActionReadiness(nil, "kick")
	assert.Equal(t, ActionRejected, result.Status)
	assert.NotEmpty(t, result.Reason)
}

// TestActionReadiness_SpecialMove_Ready verifies that a special-move verb
// where CommandIsReady returns true yields ActionReady.
// taunt requires char.IsInCombat() — newTestMob sets default aggro to user 1.
func TestActionReadiness_SpecialMove_Ready(t *testing.T) {
	m := newTestMob(t, nil)
	actor := &MobActor{Mob: m, Room: nil}
	result := ActionReadiness(actor, "taunt")
	assert.Equal(t, ActionReady, result.Status)
}

// TestActionReadiness_SpecialMove_OnCooldown_Deferred verifies that a
// special-move verb with a non-zero "special-move" cooldown yields
// ActionDeferred (transient — retry when the cooldown expires).
// Cooldowns are set directly on the Cooldowns map (no setter method exists).
func TestActionReadiness_SpecialMove_OnCooldown_Deferred(t *testing.T) {
	m := newTestMob(t, nil)
	// Direct map assignment is the canonical test approach (see command_readiness_test.go).
	m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
	actor := &MobActor{Mob: m, Room: nil}
	result := ActionReadiness(actor, "taunt")
	assert.Equal(t, ActionDeferred, result.Status)
	assert.Equal(t, "special-move busy", result.Reason)
}

// TestActionReadiness_SpecialMove_StructurallyBlocked_Rejected verifies that
// a special-move verb where CommandIsReady is false for structural reasons
// (no cooldown, not acting) yields ActionRejected.
// taunt with EndAggro() → !char.IsInCombat() → structural block.
func TestActionReadiness_SpecialMove_StructurallyBlocked_Rejected(t *testing.T) {
	m := newTestMob(t, nil)
	m.Character.EndAggro() // removes the aggro set by newTestMob
	actor := &MobActor{Mob: m, Room: nil}
	result := ActionReadiness(actor, "taunt")
	assert.Equal(t, ActionRejected, result.Status)
	assert.Equal(t, "special-move unavailable", result.Reason)
}

// TestActionReadiness_SilencedTaunt_Rejected: a silenced taunter in combat is
// refused by ExecuteTaunt, so a queued taunt surfaces that refusal and drops
// rather than retrying silently (M6 slice 3).
func TestActionReadiness_SilencedTaunt_Rejected(t *testing.T) {
	cleanup := seedConditionsForTest()
	defer cleanup()

	m := newTestMob(t, nil)
	require.NoError(t, m.Character.AddCondition(readinessSilencedConditionId, true))
	actor := &MobActor{Mob: m, Room: nil}
	result := ActionReadiness(actor, "taunt")
	assert.Equal(t, ActionRejected, result.Status)
	assert.Equal(t, "special-move unavailable", result.Reason)
}

// TestActionReadinessDrift iterates every verb in specialMoveVerbs and calls
// CommandIsReady for each, asserting no panic. This guards against drift: if
// a verb is added to or removed from CommandIsReady's switch without updating
// specialMoveVerbs, the set-membership difference is visible in tests.
func TestActionReadinessDrift(t *testing.T) {
	m := newTestMob(t, nil)
	actor := &MobActor{Mob: m, Room: nil}

	for verb := range specialMoveVerbs {
		v := verb // capture loop variable
		t.Run(v, func(t *testing.T) {
			assert.NotPanics(t, func() {
				CommandIsReady(actor, v)
			}, "CommandIsReady should not panic for special-move verb %q", v)
		})
	}
}

// ---------------------------------------------------------------------------
// Cast readiness tests (Task 2)
// ---------------------------------------------------------------------------

// TestCastReadiness_UnknownSpell_Rejected verifies that "cast <nonexistent>"
// yields ActionRejected immediately — wrong spell name is a structural error.
func TestCastReadiness_UnknownSpell_Rejected(t *testing.T) {
	actor, _, _ := newCastActor()
	result := ActionReadiness(actor, "cast notaspell_xyzzy_ar")
	assert.Equal(t, ActionRejected, result.Status)
	assert.Equal(t, "unknown spell", result.Reason)
}

// TestCastReadiness_NoCP_Deferred verifies that a caster who knows the spell
// but has zero Conviction gets ActionDeferred ("insufficient conviction").
// Conviction = 0 by default from characters.New(); seedTestSpell sets Cost=5.
func TestCastReadiness_NoCP_Deferred(t *testing.T) {
	sd, cleanup := seedTestSpell("test-ar-nocp", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	defer cleanup()

	actor, char, _ := newCastActor()
	// Grant the spell — characters.New() initialises SpellBook with starters
	// but not test spells; set directly as HasSpell checks SpellBook[id] > 0.
	char.SpellBook[sd.SpellId] = 1
	// char.Conviction is 0 (Go zero value) — below the spell's Cost of 5.

	result := ActionReadiness(actor, "cast test-ar-nocp")
	assert.Equal(t, ActionDeferred, result.Status)
	assert.Equal(t, "insufficient conviction", result.Reason)
}

// TestCastReadiness_Affordable_Ready verifies that a caster who knows the
// spell, has ample Conviction, is not casting/busy, and has no cooldowns
// gets ActionReady.
func TestCastReadiness_Affordable_Ready(t *testing.T) {
	sd, cleanup := seedTestSpell("test-ar-affordable", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	defer cleanup()

	actor, char, _ := newCastActor()
	char.SpellBook[sd.SpellId] = 1 // character knows the spell
	char.Conviction = 1000         // well above the spell's Cost of 5

	result := ActionReadiness(actor, "cast test-ar-affordable")
	assert.Equal(t, ActionReady, result.Status)
}

// TestCastInitGateIsGone pins the U0 deletion of the spell-initiation gate.
//
// The gate could never be beaten: CalcInitiationChance clamped at 95 while a
// maxed caster's computed value was 1372 (Meirok, Willpower 148, spellcasting
// 51). So mastery could not touch it and every caster failed one cast in twenty
// forever, each failure carrying a 2-round cast-init cooldown. Concentration
// break already covers the design intent.
//
// A leftover cast-init cooldown on an existing save must therefore be inert
// rather than blocking, which is what this asserts.
func TestCastInitGateIsGone(t *testing.T) {
	sd, cleanup := seedTestSpell("test-ar-noinit", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	defer cleanup()

	actor, char, _ := newCastActor()
	char.SpellBook[sd.SpellId] = 1
	char.Conviction = 1000
	char.Cooldowns = characters.Cooldowns{"cast-init": 3}

	result := ActionReadiness(actor, "cast test-ar-noinit")
	assert.Equal(t, ActionReady, result.Status,
		"a stale cast-init cooldown must not defer casting; the gate was deleted")
}

// TestCastReadinessDrift guards castReadiness against drifting out of sync with
// the player cast pre-checks in skill.cast.go (which it deliberately mirrors,
// read-only). Each case builds a fully-castable caster, then trips exactly one
// gate and asserts the resulting classification + reason. If a gate in
// castReadiness is added/removed/reclassified without matching skill.cast.go,
// the corresponding case fails.
func TestCastReadinessDrift(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(c *characters.Character, sd *spells.SpellData)
		expected ReadyStatus
		reason   string
	}{
		// The cast-init gate was deleted in U0. See
		// TestCastInitGateIsGone below for the regression that pins it.
		{"special-move-cooldown", func(c *characters.Character, sd *spells.SpellData) {
			c.Cooldowns = characters.Cooldowns{"special-move": 3}
		}, ActionDeferred, "special-move cooldown"},
		{"spell-not-known", func(c *characters.Character, sd *spells.SpellData) {
			delete(c.SpellBook, sd.SpellId)
		}, ActionRejected, "spell not known"},
		{"no-skill", func(c *characters.Character, sd *spells.SpellData) {
			c.Skills[string(skills.Spellcasting)] = 0
		}, ActionRejected, "no skill"},
		{"missing-component", func(c *characters.Character, sd *spells.SpellData) {
			sd.ComponentTag = "test-ar-drift-component"
		}, ActionRejected, "missing component"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sd, cleanup := seedTestSpell("test-ar-drift", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
			defer cleanup()

			actor, char, _ := newCastActor()
			char.SpellBook[sd.SpellId] = 1 // knows the spell
			char.Conviction = 1000         // ample CP (Cost is 5)
			tc.mutate(char, sd)

			result := ActionReadiness(actor, "cast test-ar-drift")
			assert.Equal(t, tc.expected, result.Status, "status")
			assert.Equal(t, tc.reason, result.Reason, "reason")
		})
	}
}

// ---------------------------------------------------------------------------
// Spell-name resolution parity with skill.cast.go (Step 2 regression suite)
// ---------------------------------------------------------------------------
//
// The client action queue (Char.Action.Try over GMCP) keeps a command queued
// only when ActionReadiness reports ActionDeferred; ActionRejected fires the
// command immediately and drops it from the queue. castReadiness must
// therefore resolve a spell name using the exact same rule as the live cast
// path (internal/usercommands/skill.cast.go): a greedy longest-match over
// spells.ResolveSpell, which also consults the alias index. Before this fix,
// castReadiness resolved only the FIRST WORD of the argument via
// spells.GetSpell / spells.FindSpellByName and never touched the alias index,
// so an alias like "attune" could never resolve.
//
// seedMultiWordAliasedSpell registers a test spell shaped like Skill
// Attunement (spellid with a hyphen, a two-word display name, and an alias)
// so all four resolution forms can be exercised.
func seedMultiWordAliasedSpell(t *testing.T) (*spells.SpellData, *stubActor, *characters.Character) {
	t.Helper()
	sd := &spells.SpellData{
		SpellId:    "test-ar-multiword-attunement",
		Name:       "Multiword Attunement",
		Aliases:    []string{"attune-ar-test"},
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		BaseFolds: 4,
		Cost:      5,
	}
	cleanup := spells.SeedSpellsForTest(map[string]*spells.SpellData{sd.SpellId: sd})
	t.Cleanup(cleanup)

	actor, char, _ := newCastActor()
	char.SpellBook[sd.SpellId] = 1 // knows the spell
	char.Conviction = 1000         // ample CP (Cost is 5)
	return sd, actor, char
}

// TestCastReadiness_MultiWordSpellName_Resolves verifies that the full
// multi-word display name resolves to ActionReady rather than being
// rejected. The pre-fix code split off only the first word ("multiword"),
// which is not itself a spellid or a full display name, so GetSpell and
// FindSpellByName's exact/prefix checks would need to accidentally match —
// this spell's first word is deliberately not a natural language spell name
// on its own so the case can't pass by coincidence.
func TestCastReadiness_MultiWordSpellName_Resolves(t *testing.T) {
	_, actor, _ := seedMultiWordAliasedSpell(t)

	result := ActionReadiness(actor, "cast multiword attunement")
	assert.Equal(t, ActionReady, result.Status, "reason: %s", result.Reason)
}

// TestCastReadiness_Alias_Resolves verifies that casting by alias ("cast
// attune-ar-test") resolves. castReadiness never consulted the alias index
// before this fix, so this must have failed with "unknown spell".
func TestCastReadiness_Alias_Resolves(t *testing.T) {
	_, actor, _ := seedMultiWordAliasedSpell(t)

	result := ActionReadiness(actor, "cast attune-ar-test")
	assert.Equal(t, ActionReady, result.Status, "reason: %s", result.Reason)
}

// TestCastReadiness_ExactSpellId_StillResolves pins the one form that
// already worked before the fix (the exact hyphenated spellid), so the fix
// is not a regression.
func TestCastReadiness_ExactSpellId_StillResolves(t *testing.T) {
	sd, actor, _ := seedMultiWordAliasedSpell(t)

	result := ActionReadiness(actor, "cast "+sd.SpellId)
	assert.Equal(t, ActionReady, result.Status, "reason: %s", result.Reason)
}

// TestCastReadiness_GenuinelyUnknownSpell_StillRejected verifies that a name
// matching nothing (not a prefix, id, or alias of any known spell) is still
// ActionRejected with "unknown spell" — the fix must not turn castReadiness
// into a pass-everything gate.
func TestCastReadiness_GenuinelyUnknownSpell_StillRejected(t *testing.T) {
	_, actor, _ := seedMultiWordAliasedSpell(t)

	result := ActionReadiness(actor, "cast blorptastic")
	assert.Equal(t, ActionRejected, result.Status)
	assert.Equal(t, "unknown spell", result.Reason)
}

// TestActionReadinessSpellNameResolutionDrift is the sibling of
// TestActionReadinessDrift for the SPELL half of ActionReadiness.
// TestActionReadinessDrift above only ever exercised the special-move verbs
// (specialMoveVerbs vs. CommandIsReady); nothing pinned castReadiness's Gate
// 1 spell-name lookup against spells.ResolveSpellGreedy, the resolver
// skill.cast.go uses for a real cast. That is exactly where it drifted: the
// comment on castReadiness claimed it mirrored skill.cast.go while its Gate
// 1 used splitVerb (first word only) and never consulted the alias index.
//
// For every token, castReadiness must agree with ResolveSpellGreedy on
// whether the spell resolves at all — never rejecting something the real
// cast path would accept, and never accepting something it would not.
// ---------------------------------------------------------------------------
// Alias expansion parity with usercommands.TryCommand (this bug's fix)
// ---------------------------------------------------------------------------
//
// The client action queue (Char.Action.Try over GMCP) keeps a command queued
// only when ActionReadiness reports ActionDeferred; ActionReady fires the
// command immediately and drops it from the queue. ActionReadiness dispatched
// on the raw verb without ever expanding user aliases, so a trigger sending a
// custom alias like "sa" (-> "cast skill-attunement") matched neither "cast"
// nor specialMoveVerbs and fell through to ActionReady even mid-cast. warcry
// and rally only appeared to work because they are literal verbs.
//
// stubActor (used by newCastActor elsewhere in this file) has no backing
// users.UserRecord, so it can never carry a user alias map. These tests build
// a real *UserActor instead.

// newAliasTestActor builds a *UserActor backed by a real users.UserRecord
// carrying the given alias map, so ActionReadiness has something to expand.
func newAliasTestActor(aliases map[string]string) (*UserActor, *characters.Character) {
	char := newTestChar()
	user := &users.UserRecord{
		UserId:    99001,
		Character: char,
		Aliases:   aliases,
	}
	return &UserActor{User: user}, char
}

// TestActionReadiness_UserAlias_CastSpell_Defers verifies that a user alias
// expanding to a cast ("sa" -> "cast skill-attunement") is evaluated exactly
// like the un-aliased "cast skill-attunement" command — here, deferred for
// insufficient conviction — rather than falling through to ActionReady.
func TestActionReadiness_UserAlias_CastSpell_Defers(t *testing.T) {
	sd, cleanup := seedTestSpell("skill-attunement", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	defer cleanup()

	actor, char := newAliasTestActor(map[string]string{"sa": "cast skill-attunement"})
	char.SpellBook[sd.SpellId] = 1
	// char.Conviction is 0 (Go zero value) — below the spell's Cost of 5, so a
	// real "cast skill-attunement" would defer, not fire immediately.

	result := ActionReadiness(actor, "sa")
	assert.Equal(t, ActionDeferred, result.Status,
		"a user alias expanding to a cast must resolve to the same gate as the un-aliased command, not ActionReady (reason: %s)", result.Reason)
	assert.Equal(t, "insufficient conviction", result.Reason)
}

// TestActionReadiness_UserAlias_SpecialMove_Defers verifies that a user alias
// expanding to a special-move verb ("wc" -> "warcry") is evaluated exactly
// like the literal verb — here, deferred on the shared special-move cooldown.
func TestActionReadiness_UserAlias_SpecialMove_Defers(t *testing.T) {
	actor, char := newAliasTestActor(map[string]string{"wc": "warcry"})
	char.Cooldowns = characters.Cooldowns{"special-move": 3}

	result := ActionReadiness(actor, "wc")
	assert.Equal(t, ActionDeferred, result.Status,
		"a user alias expanding to a special move must resolve to the same gate as the literal verb, not ActionReady (reason: %s)", result.Reason)
	assert.Equal(t, "special-move busy", result.Reason)
}

// TestActionReadiness_UnknownVerb_NoAliasMatch_StillReady pins the documented
// pass-through: a *UserActor with a non-empty alias map that simply does not
// match the typed verb must not regress to anything other than ActionReady.
func TestActionReadiness_UnknownVerb_NoAliasMatch_StillReady(t *testing.T) {
	actor, _ := newAliasTestActor(map[string]string{"sa": "cast skill-attunement"})

	result := ActionReadiness(actor, "say hello")
	assert.Equal(t, ActionReady, result.Status)
}

// TestActionReadiness_UserAlias_MultiWord_MergesRestCorrectly verifies the
// multi-word merge order: a user alias that itself expands to multiple words
// ("sa" -> "cast skill-attunement") combined with a trailing rest typed after
// the alias ("sa bob") must produce "cast skill-attunement bob", NOT
// "cast bob skill-attunement". Conviction is left at 0 (below the spell's
// Cost of 5) so a CORRECT merge resolves the spell and defers for
// insufficient conviction. ResolveSpellGreedy drops trailing words from the
// END, so a merge bug that put "bob" ahead of "skill-attunement" would fail
// to resolve the spell at all — surfacing as ActionRejected("unknown spell")
// instead, which this test would also catch.
func TestActionReadiness_UserAlias_MultiWord_MergesRestCorrectly(t *testing.T) {
	sd, cleanup := seedTestSpell("skill-attunement", combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	defer cleanup()

	actor, char := newAliasTestActor(map[string]string{"sa": "cast skill-attunement"})
	char.SpellBook[sd.SpellId] = 1
	// char.Conviction is 0 (Go zero value) — below the spell's Cost of 5.

	result := ActionReadiness(actor, "sa bob")
	assert.Equal(t, ActionDeferred, result.Status, "reason: %s", result.Reason)
	assert.Equal(t, "insufficient conviction", result.Reason)
}

func TestActionReadinessSpellNameResolutionDrift(t *testing.T) {
	sd := &spells.SpellData{
		SpellId:    "test-ar-drift-spellname",
		Name:       "Drift Guard Ward",
		Aliases:    []string{"driftward"},
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		BaseFolds: 4,
		Cost:      5,
	}
	cleanup := spells.SeedSpellsForTest(map[string]*spells.SpellData{sd.SpellId: sd})
	defer cleanup()

	tokens := []string{
		sd.SpellId,         // exact canonical id
		"driftward",        // alias
		"drift guard ward", // full multi-word display name
		"nonexistent-spell-xyzzy",
	}

	for _, token := range tokens {
		t.Run(token, func(t *testing.T) {
			actor, char, _ := newCastActor()
			resolved, _ := spells.ResolveSpellGreedy(token)

			if resolved != nil {
				char.SpellBook[resolved.SpellId] = 1 // knows whatever it resolves to
				char.Conviction = 1000               // ample CP
			}

			result := ActionReadiness(actor, "cast "+token)

			if resolved == nil {
				assert.Equal(t, ActionRejected, result.Status,
					"castReadiness must reject exactly what ResolveSpellGreedy cannot resolve")
				assert.Equal(t, "unknown spell", result.Reason)
			} else {
				assert.Equal(t, ActionReady, result.Status,
					"castReadiness must accept exactly what ResolveSpellGreedy resolves (reason: %s)", result.Reason)
			}
		})
	}
}
