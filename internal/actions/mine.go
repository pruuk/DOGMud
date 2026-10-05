package actions

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mining"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Mining (wilderness trades). `prospect` reads the vein in a cave, mountain
// or cliff room; `mine` starts a timed job on the Salvaging activity, keyed
// MineActivityPrefix; ResolveMine finishes it. The vein itself (ore, loads
// left, refilling) is internal/mining, stored on the room. Like felling, it
// is labour: strength and vitality and the pick, no skill.

// MineActivityPrefix keys a mining job on the Salvaging activity.
const MineActivityPrefix = `mine:`

// RoomVein returns the room's vein, seeding it on first use and applying
// refilling since it was last touched. ok is false when nothing can be mined
// here. The vein is written back to the room before returning.
func RoomVein(room *rooms.Room, now uint64) (mining.Vein, *mining.Ore, bool) {
	if room == nil {
		return mining.Vein{}, nil, false
	}
	// Rift rooms are rebuilt every run with empty long-term data, so a vein
	// there would be a fresh, full one each day. Rifts have their own ore
	// through forage instead (review fix). rift_run is the temp key
	// internal/rifts stamps on every room it builds (rifts imports actions,
	// so the key is read here rather than calling rifts.IsRiftRoom).
	if room.GetTempData(`rift_run`) != nil {
		return mining.Vein{}, nil, false
	}
	biome := room.Biome
	if biome == `` {
		biome = rooms.GetZoneBiome(room.Zone)
	}
	pool := mining.Pool(room.RoomId, room.Zone, biome)
	if len(pool) == 0 {
		return mining.Vein{}, nil, false
	}
	b := configs.GetBalanceConfig()

	v, have := mining.LoadVein(room)
	if have && mining.GetOre(v.Ore) == nil {
		have = false // the ore was removed from mining.yaml; start over
	}
	if !have {
		ore := mining.PickOre(pool, neighbourOres(room), util.Rand)
		v = mining.NewVein(ore, int(b.MiningVeinMin), int(b.MiningVeinMax), now, util.Rand)
	} else if v.Refill(now, int(b.MiningRegrowRounds)) {
		v.Ore = mining.PickOre(pool, neighbourOres(room), util.Rand)
	}
	mining.SaveVein(room, v)
	return v, mining.GetOre(v.Ore), true
}

// neighbourOres lists the ores of the veins already found next door.
func neighbourOres(room *rooms.Room) []string {
	out := []string{}
	for _, ex := range room.Exits {
		if ex.RoomId == room.RoomId {
			continue
		}
		n := rooms.LoadRoom(ex.RoomId)
		if n == nil {
			continue
		}
		if v, ok := mining.LoadVein(n); ok {
			out = append(out, v.Ore)
		}
	}
	return out
}

// MineTarget is the target's side of a mining roll: a little easier than a
// carcass for common ore, harder for each tier above it.
func MineTarget(tier int) float64 {
	b := configs.GetBalanceConfig()
	return float64(tier-1)*float64(b.MiningTierDifficulty) - float64(b.MiningEase)
}

// MineRounds is how long one mining job on an ore of this tier takes before
// pick speed.
func MineRounds(tier int) int {
	return int(configs.GetBalanceConfig().MiningRoundsBase) + tier - 1
}

// OreFor is how many loads of ore one job yields: one, plus one per
// GatherStatPerBonusUnit of Strength above 100, plus one for a fine or
// better job, capped by Balance.MiningMaxOre.
func OreFor(strength int, grade items.Quality) int {
	n := 1 + StatBonusUnits(strength)
	if grade >= items.QualityFine {
		n++
	}
	if max := int(configs.GetBalanceConfig().MiningMaxOre); n > max {
		n = max
	}
	return n
}

// OreKnown reports whether the prospector can name the ore: common ores
// (tier 1 and 2) always; rarer ones take an eye for it, Perception of at
// least 100 + 5 per tier above 2, paying the sight ramp. No skill, no roll:
// a keen-eyed miner always knows silver when they see it.
func OreKnown(actor Actor, ore *mining.Ore) bool {
	if ore == nil {
		return false
	}
	if ore.Tier <= 2 {
		return true
	}
	char := actor.GetCharacter()
	eye := float64(char.GetStatValue(`perception`)) * messaging.SightMult(char, actor.GetRoom())
	return eye >= 100+5*float64(ore.Tier-2)
}

// PickTooPoor is what a miner is told when their best pick cannot work this
// ore.
func PickTooPoor(ore *mining.Ore) string {
	return fmt.Sprintf(`The %s is too hard for your pick: it only rings off the rock. You need a %s pick or better.`,
		ore.Name, items.ToolTier(ore.MinPick))
}

// veinWord describes how much ore is left.
func veinWord(v mining.Vein) string {
	switch {
	case v.Stock <= 0:
		return `worked out`
	case v.Stock*4 >= v.Max*3:
		return `a rich seam`
	case v.Stock*5 >= v.Max*2:
		return `a fair seam, partly worked`
	default:
		return `nearly worked out, with only a little left`
	}
}

// Prospect describes the vein in the actor's room.
func Prospect(actor Actor) {
	room := actor.GetRoom()
	now := util.GetRoundCount()
	v, ore, ok := RoomVein(room, now)
	if !ok {
		actor.SendText(messaging.CategorySystem, `There is no ore worth digging here. Look in caves, on mountainsides and along cliffs.`)
		return
	}
	name := `a glinting ore you can't put a name to`
	known := OreKnown(actor, ore)
	if known {
		name = ore.Name + ` ore`
	}
	lines := []string{fmt.Sprintf(`The rock here carries <ansi fg="itemname">%s</ansi>: %s.`, name, veinWord(v))}
	if known && ore.Note != `` {
		lines = append(lines, ore.Note)
	}
	if v.Stock <= 0 {
		b := configs.GetBalanceConfig()
		lines = append(lines, fmt.Sprintf(`Nothing here is worth the digging. Give it %s.`,
			roughWait(v.RoundsToNextLoad(now, int(b.MiningRegrowRounds)))))
	} else if ore.Tier >= 3 {
		lines = append(lines, `It is hard, stubborn rock, slow to work.`)
	}
	if pick, has := gather.BestTool(actor.GetCharacter(), items.ToolPick); !has {
		lines = append(lines, `You'll need a pick to work it. (<ansi fg="command">help tools</ansi>)`)
	} else if known && pick.Tier < items.ToolTier(ore.MinPick) {
		lines = append(lines, PickTooPoor(ore))
	}
	actor.SendText(messaging.CategorySystem, strings.Join(lines, "\n"))
}

// MineResult is the outcome of one mining job.
type MineResult struct {
	Reason  string
	Roll    gather.Result
	Loads   int
	Ore     string
	Gem     string // the gem's name when one turned up
	Dropped bool   // something was too heavy to carry and fell at the actor's feet
}

// gemChance is the chance a successful job also turns up a gem.
func gemChance(perception int, pickTier items.ToolTier) float64 {
	base := float64(configs.GetBalanceConfig().MiningGemChance)
	if base <= 0 {
		return 0 // MiningGemChance 0 turns gems off
	}
	c := base * float64(perception) / 100.0 * gather.RareMult(pickTier)
	return math.Max(0.005, math.Min(0.25, c))
}

// ResolveMine finishes a mining job in the actor's room.
func ResolveMine(actor Actor) MineResult {
	res := MineResult{}
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		res.Reason = `no character or room`
		return res
	}
	now := util.GetRoundCount()
	v, ore, ok := RoomVein(room, now)
	if !ok || ore == nil {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">There is nothing here to mine.</ansi>`)
		res.Reason = `not mineable`
		return res
	}
	res.Ore = ore.Name
	if v.Stock <= 0 {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">Someone has worked out the last of the seam before you.</ansi>`)
		res.Reason = `no stock`
		return res
	}
	if pick, has := gather.BestTool(char, items.ToolPick); has && pick.Tier < items.ToolTier(ore.MinPick) {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">`+PickTooPoor(ore)+`</ansi>`)
		res.Reason = `pick too poor`
		return res
	}

	roll := gather.Roll(char, room, gather.JobMine, MineTarget(ore.Tier))
	res.Roll = roll
	if roll.NoTool {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">You need a pick to mine.</ansi>`)
		res.Reason = `no tool`
		return res
	}
	defer WearUsedTool(actor, roll.Tool, roll.HasTool)

	taken := []items.Item{}
	give := func(itemId, qty int, grade items.Quality) {
		for i := 0; i < qty; i++ {
			itm := items.New(itemId)
			if !itm.IsValid() {
				return
			}
			itm.Quality = grade
			if char.StoreItem(itm) {
				if actor.GetUserId() != 0 {
					events.AddToQueue(events.ItemOwnership{UserId: actor.GetUserId(), Item: itm, Gained: true})
				}
			} else {
				room.AddItem(itm, false)
				res.Dropped = true
			}
			taken = append(taken, itm)
		}
	}
	if roll.Success {
		v.Dig(now)
		mining.SaveVein(room, v)
		res.Loads = OreFor(char.GetStatValue(`strength`), roll.Grade)
		give(ore.ItemId, res.Loads, roll.Grade)

		pickTier := items.ToolTierCrude
		if roll.HasTool {
			pickTier = roll.Tool.Tier
		}
		if float64(util.Rand(10000))/10000.0 < gemChance(char.GetStatValue(`perception`), pickTier) {
			if gem, ok := mining.PickGem(int(pickTier), util.Rand); ok {
				before := len(taken)
				give(gem.ItemId, 1, roll.Grade)
				if len(taken) > before {
					res.Gem = taken[len(taken)-1].DisplayName()
				}
			}
		}
	}

	actorLine := messaging.NoLine
	observer := messaging.Say(messaging.CategoryMobIdle, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> swings a pick at the rock face, the blows ringing off the stone.`, actor.GetName()))
	if actor.IsPlayer() {
		if roll.Success {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="green">The rock gives with a crack and you pry the ore loose: %s.</ansi>`, summarizeTaken(taken)))
		} else {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="red">Your pick skids off the %s seam. You stop before you shatter it.</ansi>`, ore.Name))
		}
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    actorLine,
		Actee:    messaging.NoLine,
		Observer: observer,
	}, messaging.Audience{
		Actor:     actor,
		ActorId:   actor.GetUserId(),
		ActorName: actor.GetName(),
		ActeeName: messaging.NoName,
		Room:      room,
	})
	if res.Gem != `` && actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="yellow">Something glints in the spoil: %s!</ansi>`, res.Gem))
	}
	if res.Dropped && actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, `<ansi fg="yellow">You can't carry it all; the rest lies at your feet.</ansi>`)
	}
	return res
}
