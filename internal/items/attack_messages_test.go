package items

import (
	"fmt"
	"reflect"
	"testing"
)

// firstPickerForTest always selects index 0.
func firstPickerForTest(n int) int { return 0 }

func TestPoolForUnionsTiersCumulatively(t *testing.T) {
	stm := SkillTieredMessages{
		Beginner: MessageOptions{"b1", "b2"},
		Expert:   MessageOptions{"e1"},
		Master:   MessageOptions{"m1"},
	}
	cases := []struct {
		skill int
		want  []string
	}{
		{10, []string{"b1", "b2"}},
		{33, []string{"b1", "b2"}},
		{34, []string{"b1", "b2", "e1"}},
		{66, []string{"b1", "b2", "e1"}},
		{67, []string{"b1", "b2", "e1", "m1"}},
		{100, []string{"b1", "b2", "e1", "m1"}},
	}
	for _, c := range cases {
		got := stm.PoolFor(c.skill)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("PoolFor(%d) = %v, want %v", c.skill, got, c.want)
		}
	}
}

func TestPoolForEmptyIsNil(t *testing.T) {
	var stm SkillTieredMessages
	if got := stm.PoolFor(100); got != nil {
		t.Errorf("PoolFor on an empty store = %v, want nil so the core treats the role as absent", got)
	}
}

// TestTogetherRenderCoordinatesOneIndexAcrossRoles is the point of the whole
// migration: variant N of each role describes the same moment, so all roles
// must come from ONE index. Picking per role is the defect that shipped twice
// before (melee defence PR #112, taunt PR #115).
func TestTogetherRenderCoordinatesOneIndexAcrossRoles(t *testing.T) {
	m := TogetherMessages{
		ToAttacker: SkillTieredMessages{Beginner: MessageOptions{"a0", "a1", "a2"}},
		ToDefender: SkillTieredMessages{Beginner: MessageOptions{"d0", "d1", "d2"}},
		ToRoom:     SkillTieredMessages{Beginner: MessageOptions{"r0", "r1", "r2"}},
	}
	for idx := 0; idx < 3; idx++ {
		pick := func(n int) int { return idx }
		roles := m.Render(10, nil, pick)
		wantA := fmt.Sprintf("a%d", idx)
		wantD := fmt.Sprintf("d%d", idx)
		wantR := fmt.Sprintf("r%d", idx)
		if roles.Actor != wantA || roles.Actee != wantD || roles.Observer != wantR {
			t.Errorf("index %d: got (%q,%q,%q), want (%q,%q,%q); all roles must come from ONE index",
				idx, roles.Actor, roles.Actee, roles.Observer, wantA, wantD, wantR)
		}
		if roles.ActeeObserver != "" {
			t.Errorf("index %d: together must leave ActeeObserver empty, got %q", idx, roles.ActeeObserver)
		}
	}
}

func TestRenderSubstitutesTokens(t *testing.T) {
	m := TogetherMessages{
		ToAttacker: SkillTieredMessages{Beginner: MessageOptions{"You hit {actee}!"}},
		ToDefender: SkillTieredMessages{Beginner: MessageOptions{"{actor} hits you!"}},
		ToRoom:     SkillTieredMessages{Beginner: MessageOptions{"{actor} hits {actee}!"}},
	}
	roles := m.Render(10, map[TokenName]string{
		TokenActor: "Sable",
		TokenActee: "the goblin",
	}, firstPickerForTest)
	if roles.Actor != "You hit the goblin!" {
		t.Errorf("Actor = %q", roles.Actor)
	}
	if roles.Actee != "Sable hits you!" {
		t.Errorf("Actee = %q", roles.Actee)
	}
	if roles.Observer != "Sable hits the goblin!" {
		t.Errorf("Observer = %q", roles.Observer)
	}
}

// TestRenderTakesExactlyOneDrawPerMessage pins the draw count. It was three
// draws, one per audience. The count is
// load-bearing twice over: it IS the coordination guarantee, and it is what
// shifts the global random stream when this lands.
func TestRenderTakesExactlyOneDrawPerMessage(t *testing.T) {
	draws := 0
	counting := func(n int) int { draws++; return 0 }

	together := TogetherMessages{
		ToAttacker: SkillTieredMessages{Beginner: MessageOptions{"a0", "a1"}},
		ToDefender: SkillTieredMessages{Beginner: MessageOptions{"d0", "d1"}},
		ToRoom:     SkillTieredMessages{Beginner: MessageOptions{"r0", "r1"}},
	}
	together.Render(10, nil, counting)
	if draws != 1 {
		t.Errorf("together took %d draws, want exactly 1; a draw per role is the defect this migration removes", draws)
	}
}

// TestRenderUnequalPoolsRenderNothing documents the contract the content pad
// exists to satisfy: the core refuses to invent a pairing.
func TestRenderUnequalPoolsRenderNothing(t *testing.T) {
	m := TogetherMessages{
		ToAttacker: SkillTieredMessages{Beginner: MessageOptions{"a0", "a1", "a2"}},
		ToDefender: SkillTieredMessages{Beginner: MessageOptions{"d0", "d1"}},
		ToRoom:     SkillTieredMessages{Beginner: MessageOptions{"r0", "r1", "r2"}},
	}
	roles := m.Render(10, nil, firstPickerForTest)
	if roles.Actor != "" || roles.Actee != "" || roles.Observer != "" {
		t.Errorf("unequal pools must render nothing, got %+v", roles)
	}
}

// balancedGroup builds a weapon group that satisfies every required intensity
// with one balanced line per role, so a test can break exactly one thing.
func balancedGroup() *WeaponAttackMessageGroup {
	g := &WeaponAttackMessageGroup{OptionId: "testweapon", Options: AttackTypes{}}
	for _, i := range []Intensity{Prepare, Wait, Miss, Weak, Normal, Heavy, Critical, Fumble} {
		g.Options[i] = AttackOptions{Together: TogetherMessages{
			ToAttacker: SkillTieredMessages{Beginner: MessageOptions{"a"}},
			ToDefender: SkillTieredMessages{Beginner: MessageOptions{"d"}},
			ToRoom:     SkillTieredMessages{Beginner: MessageOptions{"r"}},
		}}
	}
	return g
}

func TestValidateAcceptsABalancedGroup(t *testing.T) {
	if err := balancedGroup().Validate(); err != nil {
		t.Fatalf("a balanced group must validate, got %v", err)
	}
}

func TestValidateRejectsAMissingIntensity(t *testing.T) {
	g := balancedGroup()
	delete(g.Options, Fumble)
	if err := g.Validate(); err == nil {
		t.Fatal("Validate accepted a group with no fumble intensity")
	}
}

// TestValidateRejectsUnequalRolePools is the check the attack store has never
// had and the defence sibling always has. One role one line short is exactly
// the shape the content pad fixed across 446 groups.
func TestValidateRejectsUnequalRolePools(t *testing.T) {
	g := balancedGroup()
	short := g.Options[Critical]
	short.Together.ToRoom = SkillTieredMessages{Beginner: MessageOptions{}}
	g.Options[Critical] = short
	if err := g.Validate(); err == nil {
		t.Fatal("Validate accepted a group whose toroom pool went missing")
	}
}

func TestValidateRejectsABlankVariant(t *testing.T) {
	g := balancedGroup()
	blank := g.Options[Weak]
	blank.Together.ToDefender = SkillTieredMessages{Beginner: MessageOptions{"   "}}
	g.Options[Weak] = blank
	if err := g.Validate(); err == nil {
		t.Fatal("Validate accepted a blank message variant")
	}
}

// TestValidateChecksEachTierSeparately is why equality is per tier and not on
// totals. Both roles hold two lines, so a totals check passes, but index 1
// would pair an expert line against a beginner one.
func TestValidateChecksEachTierSeparately(t *testing.T) {
	g := balancedGroup()
	skewed := g.Options[Normal]
	skewed.Together.ToAttacker = SkillTieredMessages{
		Beginner: MessageOptions{"a0", "a1"},
	}
	skewed.Together.ToDefender = SkillTieredMessages{
		Beginner: MessageOptions{"d0"},
		Expert:   MessageOptions{"d1"},
	}
	skewed.Together.ToRoom = SkillTieredMessages{
		Beginner: MessageOptions{"r0", "r1"},
	}
	g.Options[Normal] = skewed
	if err := g.Validate(); err == nil {
		t.Fatal("Validate accepted equal TOTALS with unequal tiers; equality must be per tier")
	}
}
