package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Nobody may pickpocket a companion, the thief's own included: a charmed
// one (the predicate mobs.CheckPlayerHarm refuses first) or one bonded to
// the AI companion (owner ruling 2026-09-29). Nothing is rolled, taken or
// trained.
func TestSteal_RefusesAnyCompanion(t *testing.T) {
	actor := newStealPlayerActor(200, 8)
	cases := map[string]func(m *mobs.Mob){
		`someone's charmed companion`:       func(m *mobs.Mob) { m.Character.Charmed = characters.NewCharm(4242, 10, ``) },
		`the thief's own charmed companion`: func(m *mobs.Mob) { m.Character.Charmed = characters.NewCharm(actor.GetUserId(), 10, ``) },
		`an AI companion`: func(m *mobs.Mob) {
			companionai.SetBondedCheck(func(id int) bool { return id == testMobInstId })
		},
	}
	for name, setup := range cases {
		target := newStealTestMob(testMobInstId, 50, 1)
		mobs.SetInstanceForTest(testMobInstId, target)
		setup(target)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
		awards := len(actor.awards)

		result := Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId})

		companionai.SetBondedCheck(nil)
		mobs.SetInstanceForTest(testMobInstId, nil)
		if result.Reason != "companion" || result.Succeeded || result.Pending || target.Character.Gold != 50 || len(actor.awards) != awards {
			t.Errorf("%v: refused, nothing taken or trained: %+v gold=%d", name, result, target.Character.Gold)
		}
	}
}

// recordingStealActor keeps what the thief is told.
type recordingStealActor struct {
	*stubActorWithId
	told []string
}

func (r *recordingStealActor) SendText(_ messaging.Category, msg string) {
	r.told = append(r.told, msg)
}

func (r *recordingStealActor) SendBlankLine(_ messaging.Category) {
	r.told = append(r.told, "")
}

// M7 slice 3 (#371): the companion refusal names the mob only for a thief
// who can see it.
func TestSteal_CompanionRefusalHidesTheNameInTheDark(t *testing.T) {
	for _, lit := range []bool{true, false} {
		actor := &recordingStealActor{stubActorWithId: newStealPlayerActor(200, 8)}
		if lit {
			lamp := 60
			actor.room.Lamp = &lamp
		} else {
			actor.room.Biome = "cave"
		}
		target := newStealTestMob(testMobInstId, 50, 1)
		target.Character.Charmed = characters.NewCharm(4242, 10, ``)
		mobs.SetInstanceForTest(testMobInstId, target)
		delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))

		Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId})

		mobs.SetInstanceForTest(testMobInstId, nil)
		told := strings.Join(actor.told, "\n")
		if lit != strings.Contains(told, "Bandit") {
			t.Errorf("lit=%v: the name shows only in light; told %q", lit, told)
		}
	}
}
