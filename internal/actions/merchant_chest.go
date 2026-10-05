package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/merchantchests"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Merchant chests (internal/merchantchests). Every merchant keeps a locked
// chest in its shop. Working one, whether setting a pick to its lock or
// lifting something out once it is open, is a theft attempt the room may
// notice: the same observer contest as stealing from any room container
// (stealObserverPass), where a merchant's Perception scales with the value
// of its stock, and counts for half while it sleeps.

// sleepingMerchantPerceptionMult is the factor on a character's Perception
// in the theft and detection contests (stealVictimScore,
// CalcDetectionScore): merchantchests.SleepingPerceptionMult for a merchant
// NPC (a mob with a shop list) that is asleep, 1 for anyone else, player
// shopkeepers included.
func sleepingMerchantPerceptionMult(c *characters.Character) float64 {
	if c == nil || !c.IsMob || len(c.Shop) == 0 || !c.HasConditionFlag(conditions.Sleeping) {
		return 1
	}
	return merchantchests.SleepingPerceptionMult()
}

// MerchantChestAct is what the thief is doing to the chest.
type MerchantChestAct int

const (
	MerchantChestPick MerchantChestAct = iota // setting a pick to the lock
	MerchantChestTake                         // lifting gold or goods out
)

// IsMerchantChest reports whether containerName in room is a merchant's
// chest.
func IsMerchantChest(room *rooms.Room, containerName string) bool {
	if room == nil {
		return false
	}
	_, ok := merchantchests.ChestAt(room.RoomId, containerName)
	return ok
}

// merchantChestOwner returns the merchant a chest belongs to when it is in
// the room, nil for any other container or an absent merchant.
func merchantChestOwner(room *rooms.Room, containerName string) *mobs.Mob {
	ch, ok := merchantchests.ChestAt(room.RoomId, containerName)
	if !ok {
		return nil
	}
	return merchantchests.OwnerIn(room, ch)
}

// thiefScore is the thief's half of a theft contest, the same as Steal's:
// Dexterity + skullduggery x SkillWeight, plus StealHiddenBonus while
// hidden, times the thief's own sight ramp.
func thiefScore(char *characters.Character, room *rooms.Room) float64 {
	cfg := configs.GetBalanceConfig()
	score := float64(char.Stats.Dexterity.ValueAdj) +
		float64(char.GetSkillLevel(skills.Skullduggery))*float64(cfg.SkillWeight)
	if char.IsHidden() {
		score += float64(cfg.StealHiddenBonus)
	}
	return score * messaging.SightMult(char, room)
}

// WatchMerchantChest runs the observer contest for actor working a
// merchant's chest in room, and reports whether the thief was caught. A
// container that is not a merchant chest is never watched (false).
//
// Caught, the thief is revealed. When the chest's merchant is in the room
// it is theft from that merchant (thiefCaught: a sleeping merchant wakes,
// the crime is recorded against its factions), and the merchant slams the
// chest shut and locks it, which also rotates its combination. When the
// merchant is away, whoever spotted it only raises the room, as for any
// container theft.
//
// Lifting goods out trains skullduggery, win or lose, as a container theft
// does; setting a pick does not, since picking the lock trains it already.
func WatchMerchantChest(actor Actor, containerName string, act MerchantChestAct) (caught bool) {
	room := actor.GetRoom()
	if room == nil || !IsMerchantChest(room, containerName) {
		return false
	}
	char := actor.GetCharacter()

	unseen, spotterName, spotterMob := stealObserverPass(actor, room, thiefScore(char, room))
	if act == MerchantChestTake {
		actor.AwardResolved(unseen, char.CandidateFor(string(skills.Skullduggery)))
	}
	if unseen {
		return false
	}

	doing := `reaching into`
	if act == MerchantChestPick {
		doing = `working at the lock of`
	}
	nameColor := `mobname`
	if spotterMob == nil {
		nameColor = `username` // a player bystander
	}
	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="%s">%s</ansi> spots you %s the <ansi fg="container">%s</ansi>!`,
		nameColor, spotterName, doing, containerName))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> is caught tampering with the <ansi fg="container">%s</ansi>!`,
		actor.GetName(), containerName),
		actor.GetUserId(),
	)

	owner := merchantChestOwner(room, containerName)
	if owner == nil {
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerSkullduggeryFailed,
		})
		return true
	}

	merchantCatchesThief(actor, owner, room, containerName)
	return true
}

// merchantCatchesThief is what follows when a thief at a merchant's chest is
// spotted while its merchant is in the room, whoever did the spotting (a
// bystander's cry wakes a sleeping merchant as surely as its own eyes):
// thiefCaught against the merchant (wakes it, records the theft against its
// factions), its shout, and the chest slammed shut and locked, which rotates
// its combination. Shared by WatchMerchantChest and stealFromContainer.
func merchantCatchesThief(actor Actor, owner *mobs.Mob, room *rooms.Room, containerName string) {
	thiefCaught(actor, owner, room)
	merchantSay(room, owner, fmt.Sprintf(`Thief! Get your hands off my %s!`, containerName))
	if c, ok := room.Containers[containerName]; ok && c.HasLock() {
		c.Lock.SetLocked()
		room.Containers[containerName] = c
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> slams the <ansi fg="container">%s</ansi> shut and locks it.`,
			owner.Character.Name, containerName))
	}
}
