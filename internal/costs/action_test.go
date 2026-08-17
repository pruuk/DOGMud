package costs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
)

func TestSpecForCompleteActionMatrix(t *testing.T) {
	tests := []struct {
		action   Action
		name     string
		skill    skills.SkillTag
		source   SkillSource
		physical bool
	}{
		{ActionAttack, "attack", "", SkillEquippedCombat, true},
		{ActionDodge, "dodge", skills.UnarmedCombat, SkillFixed, true},
		{ActionParry, "parry", skills.WeaponCombat, SkillFixed, true},
		{ActionBlock, "block", skills.WeaponCombat, SkillFixed, true},
		{ActionMove, "move", skills.Search, SkillFixed, true},
		{ActionFlee, "flee", skills.Skullduggery, SkillFixed, true},
		{ActionQuell, "quell", skills.Spellcasting, SkillFixed, false},
		{ActionDefy, "defy", skills.Rhetoric, SkillFixed, false},
		{ActionShoot, "shoot", skills.RangedCombat, SkillFixed, true},
		{ActionReload, "reload", skills.RangedCombat, SkillFixed, true},
		{ActionBash, "bash", skills.WeaponCombat, SkillFixed, true},
		{ActionTrip, "trip", skills.UnarmedCombat, SkillFixed, true},
		{ActionKick, "kick", skills.UnarmedCombat, SkillFixed, true},
		{ActionGrapple, "grapple", skills.UnarmedCombat, SkillFixed, true},
		{ActionGrappleMaintain, "grapple-maintain", skills.UnarmedCombat, SkillFixed, true},
		{ActionHamstring, "hamstring", skills.UnarmedCombat, SkillFixed, true},
		{ActionRake, "rake", skills.UnarmedCombat, SkillFixed, true},
		{ActionMaul, "maul", skills.UnarmedCombat, SkillFixed, true},
		{ActionPounce, "pounce", skills.UnarmedCombat, SkillFixed, true},
		{ActionGore, "gore", skills.UnarmedCombat, SkillFixed, true},
		{ActionDrain, "drain", skills.UnarmedCombat, SkillFixed, true},
		{ActionThrottle, "throttle", skills.UnarmedCombat, SkillFixed, true},
		{ActionThrow, "throw", skills.Skullduggery, SkillFixed, true},
		{ActionSneak, "sneak", skills.Skullduggery, SkillFixed, true},
		{ActionTaunt, "taunt", skills.Rhetoric, SkillFixed, false},
		{ActionRally, "rally", skills.Rhetoric, SkillFixed, false},
		{ActionWarcry, "warcry", skills.Rhetoric, SkillFixed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.action); got != tt.name {
				t.Fatalf("action name = %q, want %q", got, tt.name)
			}
			got := SpecFor(tt.action)
			if got.Skill != tt.skill || got.SkillSource != tt.source || got.Physical != tt.physical {
				t.Fatalf("SpecFor(%q) = %+v, want skill=%q source=%v physical=%v", tt.action, got, tt.skill, tt.source, tt.physical)
			}
		})
	}
}

func TestSpecForUnregisteredActionUsesNoSkillOrLoad(t *testing.T) {
	got := SpecFor(Action("not-a-real-action"))
	if got.Skill != "" || got.SkillSource != SkillNone || got.Physical {
		t.Fatalf("unregistered spec = %+v, want inert zero value", got)
	}
}
