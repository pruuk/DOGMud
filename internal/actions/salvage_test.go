package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ---------------------------------------------------------------------------
// salvage-test-specific helpers
// ---------------------------------------------------------------------------

// salvageFakeActor is a minimal Actor with a configurable room. It records
// SendText messages for assertion. It satisfies the full Actor interface.
type salvageFakeActor struct {
	awardRecorder // records Actor.AwardResolved calls
	char          *characters.Character
	room          *rooms.Room
	name          string
	isPlayer      bool
	userId        int
	mobInstId     int
	sent          []string
}

func newSalvageFakeActor(t *testing.T, name string, room *rooms.Room, isPlayer bool, userId int) *salvageFakeActor {
	t.Helper()
	c := &characters.Character{
		Name:  name,
		Buffs: buffs.New(),
	}
	c.Stats.Perception.ValueAdj = 100
	return &salvageFakeActor{
		char:     c,
		room:     room,
		name:     name,
		isPlayer: isPlayer,
		userId:   userId,
	}
}

func newSalvageMobActor(t *testing.T, mob *mobs.Mob, room *rooms.Room) *salvageFakeActor {
	t.Helper()
	c := &characters.Character{
		Name:  mob.Character.Name,
		Buffs: buffs.New(),
	}
	return &salvageFakeActor{
		char:      c,
		room:      room,
		name:      mob.Character.Name,
		isPlayer:  false,
		mobInstId: mob.InstanceId,
	}
}

func (a *salvageFakeActor) GetCharacter() *characters.Character    { return a.char }
func (a *salvageFakeActor) GetRoom() *rooms.Room                   { return a.room }
func (a *salvageFakeActor) GetName() string                        { return a.name }
func (a *salvageFakeActor) IsPlayer() bool                         { return a.isPlayer }
func (a *salvageFakeActor) GetUserId() int                         { return a.userId }
func (a *salvageFakeActor) GetMobInstanceId() int                  { return a.mobInstId }
func (a *salvageFakeActor) AddBuff(_ int, _ string)                {}
func (a *salvageFakeActor) OnSkillUse(_ string) bool               { return false }
func (a *salvageFakeActor) OnStatUse(_ string) bool                { return false }
func (a *salvageFakeActor) SendRoomCommunication(_ string, _ bool) {}
func (a *salvageFakeActor) SendText(_ messaging.Category, msg string) {
	a.sent = append(a.sent, msg)
}

// newSalvageTestRoom builds a minimal Room with the specified RoomId.
func newSalvageTestRoom(t *testing.T, roomId int) *rooms.Room {
	t.Helper()
	return &rooms.Room{RoomId: roomId}
}

// newSalvageTestMob builds a minimal Mob with an InstanceId.
func newSalvageTestMob(t *testing.T, instId int, name string, roomId int) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		InstanceId: instId,
	}
	m.Character.Name = name
	m.Character.Buffs = buffs.New()
	m.Character.RoomId = roomId
	return m
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSalvage_NoTargetFails confirms an empty options struct
// (no corpse, no item) returns a reason without rolling.
func TestSalvage_NoTargetFails(t *testing.T) {
	room := newSalvageTestRoom(t, 9401)
	user := newSalvageFakeActor(t, "SalvageTester", room, true, 1)

	result := Salvage(user, SalvageOptions{})

	if result.Succeeded {
		t.Error("expected Succeeded=false with no target")
	}
	if result.Reason == "" {
		t.Error("expected Reason to be set")
	}
	if result.RollHappened {
		t.Error("expected RollHappened=false with no target")
	}
}

// TestSalvage_CorpseModeNoCorpse confirms TargetCorpse with no
// eligible corpse in the room returns Failure cleanly.
func TestSalvage_CorpseModeNoCorpse(t *testing.T) {
	room := newSalvageTestRoom(t, 9402)
	mob := newSalvageTestMob(t, 9998, "SalvageMob", room.RoomId)
	actor := newSalvageMobActor(t, mob, room)

	result := Salvage(actor, SalvageOptions{TargetCorpse: true})

	if result.Succeeded {
		t.Error("expected Succeeded=false with no corpse in room")
	}
}

// TestSalvage_ItemModeInvalidUuid confirms TargetItemUuid pointing
// at a non-existent UUID returns Failure with a reason.
func TestSalvage_ItemModeInvalidUuid(t *testing.T) {
	room := newSalvageTestRoom(t, 9403)
	user := newSalvageFakeActor(t, "SalvageTester2", room, true, 2)

	result := Salvage(user, SalvageOptions{TargetItemUuid: "nonexistent-uuid"})

	if result.Succeeded {
		t.Error("expected Succeeded=false for invalid UUID")
	}
}

// TestSalvage_CorpseVanished_NamesTheMob verifies that when the player's
// specific target corpse has vanished by the final activity tick, the failure
// message names the mob ("The goblin corpse is no longer here.") rather than
// the generic fallback. Regression guard for the 2.9 lift that dropped the
// mob name from this message.
func TestSalvage_CorpseVanished_NamesTheMob(t *testing.T) {
	const mobId = 8801
	goblin := &mobs.Mob{}
	goblin.Character.Name = "goblin"
	cleanup := mobs.SeedMobsForTest(map[int]*mobs.Mob{mobId: goblin}, map[int]*mobs.Mob{})
	defer cleanup()

	room := newSalvageTestRoom(t, 9405) // no corpses present
	user := newSalvageFakeActor(t, "SalvageTester3", room, true, 3)

	result := Salvage(user, SalvageOptions{
		TargetCorpse:             true,
		TargetCorpseMobId:        mobId,
		TargetCorpseRoundCreated: 123,
	})

	if result.RollHappened {
		t.Error("expected RollHappened=false when the corpse has vanished")
	}
	if len(user.sent) == 0 {
		t.Fatal("expected a player-facing vanished-corpse message")
	}
	joined := strings.Join(user.sent, " ")
	if !strings.Contains(joined, "goblin") {
		t.Errorf("vanished-corpse message should name the mob 'goblin'; got %q", joined)
	}
}

// TestSalvage_MobActorSilent confirms MobActor.SendText is not called.
func TestSalvage_MobActorSilent(t *testing.T) {
	room := newSalvageTestRoom(t, 9404)
	mob := newSalvageTestMob(t, 9997, "SalvageSilentMob", room.RoomId)
	actor := newSalvageMobActor(t, mob, room)

	_ = Salvage(actor, SalvageOptions{TargetCorpse: true})

	if len(actor.sent) != 0 {
		t.Errorf("MobActor should be silent; got %d messages", len(actor.sent))
	}
}

// ---------------------------------------------------------------------------
// U10b-1 Task 16: the salvage award
// ---------------------------------------------------------------------------

// A salvage that recovers NOTHING still awards, at the loss weight, and awards
// exactly ONCE per command.
//
// Both halves matter and neither was pinned before:
//
//   - This site is a CUT. It paid a FULL event whether or not anything came
//     back, so a salvage that returned nothing trained as much as one that
//     returned everything.
//   - ONCE PER COMMAND, NOT PER UNIT. RollSalvageReturnsFromSpec rolls each
//     salvage_returns entry independently, so a rich item rolls many times.
//     They are one resolved action, the same rule Search follows across its six
//     tiers. THREE entries here, so a per-unit implementation reports 3.
//
// Driven through the ITEM path rather than the corpse path. A corpse is only
// eligible when crafting.LookupCorpseSalvage finds a group for it, so "recovers
// nothing" is unreachable there without a seam into that package's private
// table -- an earlier draft of this test aimed at the corpse path and silently
// SKIPPED, which is worse than no test.
//
// Determinism: SalvageMinChance and SalvageMaxChance are both pinned to 0, so
// CalcSalvageChance returns 0 and every per-entry roll fails. No dice pinning.
func TestSalvage_ARecoveryOfNothingAwardsOnceAtTheLossWeight(t *testing.T) {
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	// U10b-1b: SalvageMin/MaxChance no longer decide anything — salvage is a
	// contest. Force a certain loss by inflating the difficulty and suppressing
	// the mercy floor, which otherwise rescues 15% of losses.
	//
	// SalvageFloor takes a tiny POSITIVE value, not 0: the validator corrects
	// <=0 back to 0.15 because a 0 floor is not a legal shipped value.
	cfg.Balance.CraftBaseDifficulty = 1000000
	cfg.Balance.CraftSkillMinWeight = 0
	cfg.Balance.SalvageFloor = configs.ConfigFloat(1e-12)
	configs.SetConfigForTest(t, cfg)

	const salvageItemId = 77401
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		salvageItemId: {
			ItemId: salvageItemId,
			Name:   "test scrap",
			SalvageReturns: []items.SalvageReturn{
				{ItemTag: "scrap-a", Quantity: 1},
				{ItemTag: "scrap-b", Quantity: 1},
				{ItemTag: "scrap-c", Quantity: 1},
			},
		},
	})
	defer restoreItems()

	room := newSalvageTestRoom(t, 9410)
	actor := newSalvageFakeActor(t, "SalvageTester", room, true, 61)

	target := items.New(salvageItemId)
	actor.char.StoreItem(target)

	result := Salvage(actor, SalvageOptions{TargetItemUuid: target.UUID.String()})

	if !result.RollHappened {
		t.Fatalf("fixture did not reach the salvage roll (Reason=%q); this test must not skip", result.Reason)
	}
	if len(result.MaterialIds) > 0 {
		t.Fatalf("chance was pinned to 0 but %d materials came back", len(result.MaterialIds))
	}
	if got := len(actor.awards); got != 1 {
		t.Fatalf("a resolved salvage produced %d awards, want exactly 1 per command (3 salvage_returns entries rolled)", got)
	}
	if actor.awards[0].won {
		t.Error("a salvage that recovered nothing reported won=true; it must pay the failure fraction")
	}
	if _, n := actor.awardedCandidate(string(skills.Salvage)); n != 1 {
		t.Errorf("the award named the salvage skill %d times, want 1", n)
	}
}

// A salvage target in the BANDOLIER is found, not just one in the backpack.
//
// Character.StoreItem auto-routes potions and throwables into an equipped
// bandolier, and salvageItem used to scan char.Items alone -- so a carried item
// was invisible to the code meant to salvage it and the player got "no longer
// in your backpack" about something they were carrying.
//
// ⚠️ The live case is a DECLINING potion or a THROWABLE, not a spoiled one:
// NewRound_AutoHeal ejects PhaseSpoiled potions back to the backpack, but
// salvage accepts PhaseDeclining too and throwables are never age-ejected.
// This fixture uses a plain bandolier-routed item because the routing, not the
// aging phase, is what the lookup was blind to.
func TestSalvage_FindsATargetInTheBandolier(t *testing.T) {
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.SalvageMinChance = 0
	cfg.Balance.SalvageMaxChance = 0
	configs.SetConfigForTest(t, cfg)

	const bandolierItemId = 77402
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		bandolierItemId: {
			ItemId:         bandolierItemId,
			Name:           "bandolier scrap",
			SalvageReturns: []items.SalvageReturn{{ItemTag: "scrap-a", Quantity: 1}},
		},
	})
	defer restoreItems()

	room := newSalvageTestRoom(t, 9411)
	actor := newSalvageFakeActor(t, "SalvageBandolier", room, true, 62)

	target := items.New(bandolierItemId)
	// Placed directly in the bandolier slice: StoreItem's routing needs an
	// equipped bandolier belt, and the lookup is what is under test here.
	actor.char.PotionItems = append(actor.char.PotionItems, target)

	result := Salvage(actor, SalvageOptions{TargetItemUuid: target.UUID.String()})

	if result.Reason == "item not found" {
		t.Fatal("salvage could not find an item in the bandolier; the lookup is still backpack-only")
	}
	if !result.RollHappened {
		t.Fatalf("salvage found the bandolier item but did not resolve (Reason=%q)", result.Reason)
	}
	if got := len(actor.awards); got != 1 {
		t.Errorf("a resolved bandolier salvage produced %d awards, want 1", got)
	}
}
