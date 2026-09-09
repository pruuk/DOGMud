package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// seedTauntStore installs a band whose lines wrap both names in tag whose
// colour comes from a token, exactly as rhetoric.yaml does.
func seedTauntStore(t *testing.T) {
	t.Helper()
	restore := combat.SeedTauntMessagesForTest(map[combat.TauntIntensity]*combat.TauntMessages{
		combat.TauntHit: {
			ToAttacker: []string{`You sneer at <ansi fg="{targettype}">{target}</ansi>!`},
			ToDefender: []string{`<ansi fg="{sourcetype}">{source}</ansi> sneers at you!`},
			ToRoom:     []string{`<ansi fg="{sourcetype}">{source}</ansi> sneers at <ansi fg="{targettype}">{target}</ansi>!`},
		},
	})
	t.Cleanup(restore)
}

// TestMobTauntTriadUsesTheAuthoredStore is the point of routing the mob side
// through taunt-messages at all: until 2026-09-09 mobcommands hand-rolled its
// own "bellows a thunderous challenge" text and never read the store, so one
// event had two narration sources that drifted apart with nobody to notice.
//
// It also proves the todefender pool is finally reachable. No player could see
// those lines before: a taunted MOB has no client, and PVP ships disabled so a
// player could never be the target of a player's taunt.
func TestMobTauntTriadUsesTheAuthoredStore(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	seedTauntStore(t)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	target := users.GetByUserId(1)
	require.NotNil(t, target)
	litRoom := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, litRoom)
	require.GreaterOrEqual(t, litRoom.GetVisibility(), 1, "fixture room must be lit for this lane")

	litRoom.AddPlayer(target.UserId)
	defer litRoom.RemovePlayer(target.UserId)
	events.DrainQueuedMessagesForTest(target.UserId)

	said := sendMobTauntTriad(combat.TauntHit, "ModerateWounds", messaging.CategoryTauntSuccess,
		mob, target.Character.Name, target, litRoom)
	require.True(t, said, "a seeded store must produce narration")

	lines := events.DrainQueuedMessagesForTest(target.UserId)
	require.Len(t, lines, 1, "the target gets their personal line and is excluded from the room line")
	require.Contains(t, lines[0], "sneers at you",
		"the target must receive the AUTHORED todefender line, not hand-rolled text")
	require.Contains(t, lines[0], mob.Character.Name,
		"a lit room names the taunter")
}

// TestMobTauntTriadAnonymizesInTheDark is the property the mob side cannot
// inherit and must build by hand: sendAudioRoomText delivers on the AUDIO
// channel, which messaging's pipeline never sight-gates and never anonymizes.
//
// This is also why {sourcetype} and {targettype} must resolve to real name
// aliases. messaging.Anonymize matches on username|mobname|petname, so a tag
// outside that set leaves the name in plain view.
func TestMobTauntTriadAnonymizesInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	seedTauntStore(t)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	target := users.GetByUserId(1)
	observer := users.GetByUserId(2)
	require.NotNil(t, target)
	require.NotNil(t, observer)

	// "cave" must be REGISTERED as a dark biome: GetVisibility asks the biome
	// registry, so setting the field alone leaves the room lit.
	restoreBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", DarkArea: true, MovementCost: 1},
	})
	defer restoreBiomes()

	darkRoom := rooms.LoadRoom(2)
	require.NotNil(t, darkRoom)
	darkRoom.Biome = "cave"
	require.Zero(t, darkRoom.GetVisibility(), "fixture room must be unlit for this lane")

	mob.Character.RoomId = 2
	target.Character.RoomId = 2
	observer.Character.RoomId = 2
	darkRoom.AddMob(mob.InstanceId)
	darkRoom.AddPlayer(target.UserId)
	darkRoom.AddPlayer(observer.UserId)

	events.DrainQueuedMessagesForTest(target.UserId)
	events.DrainQueuedMessagesForTest(observer.UserId)

	said := sendMobTauntTriad(combat.TauntHit, "ModerateWounds", messaging.CategoryTauntSuccess,
		mob, target.Character.Name, target, darkRoom)
	require.True(t, said)

	targetLines := events.DrainQueuedMessagesForTest(target.UserId)
	observerLines := events.DrainQueuedMessagesForTest(observer.UserId)
	require.Len(t, targetLines, 1, "the target must not receive the room line as well as their own")
	require.Len(t, observerLines, 1)

	for _, line := range append(targetLines, observerLines...) {
		require.NotContains(t, line, mob.Character.Name,
			"an unsighted player was told the taunter's name")
		// Case-insensitive: the delivery pipeline capitalises a sentence-initial
		// placeholder, so the anonymized line can read "A figure ...".
		require.Contains(t, strings.ToLower(line), "a figure")
	}
	require.NotContains(t, observerLines[0], target.Character.Name,
		"an unsighted observer was told the target's name")
}

// TestMobTauntTriadFallsBackWhenTheStoreIsEmpty pins the contract the caller
// relies on to keep its legacy literals: a context that never loaded world
// data must still narrate rather than fall silent.
func TestMobTauntTriadFallsBackWhenTheStoreIsEmpty(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	restore := combat.SeedTauntMessagesForTest(map[combat.TauntIntensity]*combat.TauntMessages{})
	defer restore()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)

	said := sendMobTauntTriad(combat.TauntHit, "", messaging.CategoryTauntSuccess,
		mob, "Someone", nil, room)
	require.False(t, said, "an empty store must report that it narrated nothing")
}
