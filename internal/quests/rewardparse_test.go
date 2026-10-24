package quests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Moved from internal/hooks (Quest_ItemGrants_test.go, Quest_RecipeGrants_test.go,
// Quest_SkillGrants_test.go, Quest_StatGrants_test.go). These pin the parse of
// the reward strings; the application logic lives in the hooks handler.

func TestParseItemGrants_Empty(t *testing.T) {
	if g := ParseItemGrants(""); g != nil {
		t.Errorf("empty item_info: want nil, got %v", g)
	}
}

func TestParseItemGrants_SingleBareId(t *testing.T) {
	// a bare itemid with no colon defaults to quantity one
	g := ParseItemGrants("30058")
	if len(g) != 1 || g[0].ItemId != 30058 || g[0].Qty != 1 {
		t.Errorf("single bare id: got %+v", g)
	}
}

func TestParseItemGrants_SingleWithQty(t *testing.T) {
	g := ParseItemGrants("30036:3")
	if len(g) != 1 || g[0].ItemId != 30036 || g[0].Qty != 3 {
		t.Errorf("single with qty: got %+v", g)
	}
}

func TestParseItemGrants_Multi(t *testing.T) {
	g := ParseItemGrants("30036:3,30028:2,30058")
	if len(g) != 3 {
		t.Fatalf("multi grant: want 3, got %d (%+v)", len(g), g)
	}
	if g[0].ItemId != 30036 || g[0].Qty != 3 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].ItemId != 30028 || g[1].Qty != 2 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
	if g[2].ItemId != 30058 || g[2].Qty != 1 {
		t.Errorf("grant[2]: got %+v", g[2])
	}
}

func TestParseItemGrants_TrimsWhitespace(t *testing.T) {
	g := ParseItemGrants(" 30036 : 3 , 30058 ")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%+v)", len(g), g)
	}
	if g[0].ItemId != 30036 || g[0].Qty != 3 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].ItemId != 30058 || g[1].Qty != 1 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseItemGrants_SkipsMalformed(t *testing.T) {
	// empty entries, non-numeric id, non-positive qty are all skipped
	g := ParseItemGrants("30036:2,,abc,30058:0,30059:-1,30028:1")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%+v)", len(g), g)
	}
	if g[0].ItemId != 30036 || g[0].Qty != 2 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].ItemId != 30028 || g[1].Qty != 1 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseRecipeGrants_Empty(t *testing.T) {
	if g := ParseRecipeGrants(""); g != nil {
		t.Errorf("empty recipe_info: want nil, got %v", g)
	}
}

func TestParseRecipeGrants_Single(t *testing.T) {
	g := ParseRecipeGrants("iron-dagger")
	if len(g) != 1 || g[0] != "iron-dagger" {
		t.Errorf("single grant: got %v", g)
	}
}

func TestParseRecipeGrants_Multi(t *testing.T) {
	g := ParseRecipeGrants("iron-dagger,iron-buckler")
	if len(g) != 2 {
		t.Fatalf("multi grant: want 2, got %d (%v)", len(g), g)
	}
	if g[0] != "iron-dagger" {
		t.Errorf("grant[0]: want iron-dagger, got %q", g[0])
	}
	if g[1] != "iron-buckler" {
		t.Errorf("grant[1]: want iron-buckler, got %q", g[1])
	}
}

func TestParseRecipeGrants_TrimsWhitespace(t *testing.T) {
	g := ParseRecipeGrants(" iron-dagger , iron-buckler ")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%v)", len(g), g)
	}
	if g[0] != "iron-dagger" {
		t.Errorf("grant[0]: want iron-dagger, got %q", g[0])
	}
	if g[1] != "iron-buckler" {
		t.Errorf("grant[1]: want iron-buckler, got %q", g[1])
	}
}

func TestParseRecipeGrants_SkipsEmptyEntries(t *testing.T) {
	// trailing comma, double-comma: empties are skipped
	g := ParseRecipeGrants("iron-dagger,,healing-salve,")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%v)", len(g), g)
	}
	if g[0] != "iron-dagger" {
		t.Errorf("grant[0]: want iron-dagger, got %q", g[0])
	}
	if g[1] != "healing-salve" {
		t.Errorf("grant[1]: want healing-salve, got %q", g[1])
	}
}

func TestParseSkillGrants_Empty(t *testing.T) {
	if g := ParseSkillGrants(""); g != nil {
		t.Errorf("empty skill_info: want nil, got %v", g)
	}
}

func TestParseSkillGrants_LegacySingle(t *testing.T) {
	g := ParseSkillGrants("map:1")
	if len(g) != 1 || g[0].Skill != "map" || g[0].Level != 1 {
		t.Errorf("single grant: got %+v", g)
	}
}

func TestParseSkillGrants_MultiSkill(t *testing.T) {
	g := ParseSkillGrants("weapon-combat:1,unarmed-combat:1")
	if len(g) != 2 {
		t.Fatalf("multi grant: want 2, got %d (%+v)", len(g), g)
	}
	if g[0].Skill != "weapon-combat" || g[0].Level != 1 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].Skill != "unarmed-combat" || g[1].Level != 1 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseSkillGrants_TrimsWhitespaceAndCase(t *testing.T) {
	g := ParseSkillGrants(" Weapon-Combat : 2 , Unarmed-Combat:3 ")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%+v)", len(g), g)
	}
	if g[0].Skill != "weapon-combat" || g[0].Level != 2 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].Skill != "unarmed-combat" || g[1].Level != 3 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseSkillGrants_SkipsMalformed(t *testing.T) {
	// no colon, unparseable level, and empty skill are each dropped;
	// the one valid entry survives.
	g := ParseSkillGrants("garbage,:5,foo:notanumber,salvage:2")
	if len(g) != 1 || g[0].Skill != "salvage" || g[0].Level != 2 {
		t.Errorf("want only salvage:2, got %+v", g)
	}
}

func TestParseStatGrants_Empty(t *testing.T) {
	if g := ParseStatGrants(""); g != nil {
		t.Errorf("empty stat_info: want nil, got %v", g)
	}
}

func TestParseStatGrants_Single(t *testing.T) {
	g := ParseStatGrants("strength:5")
	if len(g) != 1 || g[0].Stat != "strength" || g[0].Amount != 5 {
		t.Errorf("single grant: got %+v", g)
	}
}

func TestParseStatGrants_Multi(t *testing.T) {
	g := ParseStatGrants("strength:3,vitality:2")
	if len(g) != 2 {
		t.Fatalf("multi grant: want 2, got %d (%+v)", len(g), g)
	}
	if g[0].Stat != "strength" || g[0].Amount != 3 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].Stat != "vitality" || g[1].Amount != 2 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseStatGrants_TrimsWhitespaceAndCase(t *testing.T) {
	g := ParseStatGrants(" Strength : 4 , Willpower:1 ")
	if len(g) != 2 {
		t.Fatalf("want 2, got %d (%+v)", len(g), g)
	}
	if g[0].Stat != "strength" || g[0].Amount != 4 {
		t.Errorf("grant[0]: got %+v", g[0])
	}
	if g[1].Stat != "willpower" || g[1].Amount != 1 {
		t.Errorf("grant[1]: got %+v", g[1])
	}
}

func TestParseStatGrants_SkipsMalformed(t *testing.T) {
	// no colon, unparseable amount, and empty name are each dropped;
	// the one valid entry survives.
	g := ParseStatGrants("garbage,:5,strength:notanumber,perception:2")
	if len(g) != 1 || g[0].Stat != "perception" || g[0].Amount != 2 {
		t.Errorf("want only perception:2, got %+v", g)
	}
}

func TestParseStatGrants_AllSixStats(t *testing.T) {
	g := ParseStatGrants(
		"strength:1,dexterity:1,perception:1,vitality:1,willpower:1,charisma:1",
	)
	if len(g) != 6 {
		t.Fatalf("want 6 grants, got %d (%+v)", len(g), g)
	}
	expected := []string{
		"strength", "dexterity", "perception",
		"vitality", "willpower", "charisma",
	}
	for i, want := range expected {
		if g[i].Stat != want || g[i].Amount != 1 {
			t.Errorf("grant[%d]: want {%s 1}, got %+v", i, want, g[i])
		}
	}
}

// Added by the move (plan task 2).

func TestParseSkillGrants(t *testing.T) {
	require.Nil(t, ParseSkillGrants(""))
	require.Equal(t, []SkillGrant{{Skill: "weapon-combat", Level: 1}}, ParseSkillGrants("Weapon-Combat:1"))
	require.Equal(t, []SkillGrant{{Skill: "a", Level: 2}, {Skill: "b", Level: 3}}, ParseSkillGrants(" a:2 , b:3 ,nocolon, c:x"))
}

func TestParseStatGrants(t *testing.T) {
	require.Nil(t, ParseStatGrants(""))
	require.Equal(t, []StatGrant{{Stat: "strength", Amount: 3}, {Stat: "vitality", Amount: 2}}, ParseStatGrants("Strength:3,vitality:2,bad"))
}
