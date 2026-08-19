package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	mobcmd "github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var refusalDisclosurePattern = regexp.MustCompile(`(?i)(?:\d|%|\bpercent\b|\branks?\b|\bmodifiers?\b)`)
var refusalFailureTonePattern = regexp.MustCompile(`(?i)\b(?:failed|failure|error|botched?)\b`)

func assertVoluntaryRefusalOutput(t *testing.T, lines []string, pool characters.Pool) {
	t.Helper()
	want := map[characters.Pool]string{
		characters.PoolStamina:    "You are too spent to manage that right now.",
		characters.PoolConviction: "You cannot muster the resolve for that right now.",
	}[pool]
	require.NotEmpty(t, want, "test must independently specify the pool-aware refusal prose")
	require.Len(t, lines, 1, "a voluntary refusal must emit exactly one private line")
	require.Equal(t, 1, strings.Count(lines[0], want), "the exact refusal prose must appear once")
	require.Empty(t, refusalDisclosurePattern.FindString(lines[0]), "refusal leaked tuning or numeric detail")
	require.Empty(t, refusalFailureTonePattern.FindString(lines[0]), "refusal used failure/error narration")
	require.Less(t, len([]rune(lines[0])), 120, "refusal should remain concise")
}

func TestRhetoricWrappersRefuseAtomicallyWithOnePoolAwareLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T, *rooms.Room) ([]string, *characters.Character)
	}{
		{
			name: "taunt",
			run: func(t *testing.T, room *rooms.Room) ([]string, *characters.Character) {
				user := usersForRhetoricRefusal(t)
				target := mobs.GetInstance(100)
				user.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
				_, err := Taunt("#100", user, room, 0)
				require.NoError(t, err)
				return events.DrainQueuedMessagesForTest(user.UserId), user.Character
			},
		},
		{
			name: "rally",
			run: func(t *testing.T, room *rooms.Room) ([]string, *characters.Character) {
				user := usersForRhetoricRefusal(t)
				_, err := Rally("", user, room, 0)
				require.NoError(t, err)
				return events.DrainQueuedMessagesForTest(user.UserId), user.Character
			},
		},
		{
			name: "warcry",
			run: func(t *testing.T, room *rooms.Room) ([]string, *characters.Character) {
				user := usersForRhetoricRefusal(t)
				_, err := Warcry("", user, room, 0)
				require.NoError(t, err)
				return events.DrainQueuedMessagesForTest(user.UserId), user.Character
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			room := rooms.LoadRoom(1)
			lines, char := tc.run(t, room)
			assertVoluntaryRefusalOutput(t, lines, characters.PoolConviction)
			require.Zero(t, char.Conviction)
			require.Empty(t, char.Cooldowns)
			require.Zero(t, char.GetSkillUseCount("rhetoric"))
			require.Zero(t, char.AttacksThisRound)
			require.Empty(t, char.Conditions)
		})
	}
}

func usersForRhetoricRefusal(t *testing.T) *users.UserRecord {
	t.Helper()
	user := users.GetByUserId(1)
	user.Character.Conviction = 0
	user.Character.Cooldowns = characters.Cooldowns{}
	user.Character.Skills = map[string]int{}
	user.Character.Conditions = nil
	events.DrainQueuedMessagesForTest(user.UserId)
	return user
}

func TestMobRhetoricWrappersKeepPrivateRefusalSilent(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(string, *mobs.Mob, *rooms.Room) (bool, error)
	}{
		{"taunt", mobcmd.Taunt},
		{"rally", mobcmd.Rally},
		{"warcry", mobcmd.Warcry},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			mob, room := mobs.GetInstance(100), rooms.LoadRoom(1)
			mob.Character.Conviction = 0
			mob.Character.Cooldowns = characters.Cooldowns{}
			mob.Character.SetAggro(1, 0, characters.DefaultAttack)
			beforeAggro := *mob.Character.Aggro
			for _, userID := range []int{1, 2} {
				events.DrainQueuedMessagesForTest(userID)
			}
			handled, err := tc.run("", mob, room)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, beforeAggro, *mob.Character.Aggro)
			require.Zero(t, mob.Character.Conviction)
			require.Empty(t, mob.Character.Cooldowns)
			require.Zero(t, mob.Character.GetSkillUseCount("rhetoric"))
			for _, userID := range []int{1, 2} {
				require.Empty(t, events.DrainQueuedMessagesForTest(userID), "mob refusal leaked to user %d", userID)
			}
		})
	}
}
