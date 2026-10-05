package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/timber"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Lumberjacking (wilderness trades, phase 4). `survey trees` reads the stand
// in a forest room; `chop` starts a timed job on the Salvaging activity, keyed
// ChopActivityPrefix; ResolveChop finishes it. The stand itself (species,
// trees left, regrowth) is internal/timber, stored on the room.

// ChopActivityPrefix keys a felling job on the Salvaging activity.
const ChopActivityPrefix = `chop:`

// Chance in 100 that a felled tree also gives its bark (when the species has
// a bark item).
const chopBarkChance = 40

// RoomStand returns the room's stand, seeding it on first use and applying
// regrowth since it was last touched. ok is false when nothing grows here.
// The stand is written back to the room before returning.
func RoomStand(room *rooms.Room, now uint64) (timber.Stand, *timber.Species, bool) {
	if room == nil {
		return timber.Stand{}, nil, false
	}
	// Rift rooms are rebuilt every run; a stand there would be fresh each
	// day (review fix, as RoomVein).
	if room.GetTempData(`rift_run`) != nil {
		return timber.Stand{}, nil, false
	}
	// The room's own biome, else its zone's default. Read directly rather than
	// through GetBiome, which substitutes the registry default for an unknown
	// id and would make a mistyped biome choppable or not by accident.
	biome := room.Biome
	if biome == `` {
		biome = rooms.GetZoneBiome(room.Zone)
	}
	pool := timber.Pool(room.Zone, biome)
	if len(pool) == 0 {
		return timber.Stand{}, nil, false
	}
	b := configs.GetBalanceConfig()

	st, have := timber.LoadStand(room)
	if have && timber.GetSpecies(st.Species) == nil {
		have = false // the species was removed from timber.yaml; start over
	}
	if !have {
		sp := timber.PickSpecies(pool, neighbourSpecies(room), util.Rand)
		st = timber.NewStand(sp, int(b.TimberStandMin), int(b.TimberStandMax), now, util.Rand)
	} else if st.Regrow(now, int(b.TimberRegrowRounds)) {
		st.Species = timber.PickSpecies(pool, neighbourSpecies(room), util.Rand)
	}
	timber.SaveStand(room, st)
	return st, timber.GetSpecies(st.Species), true
}

// neighbourSpecies lists the species of the stands already growing in the
// rooms next door. Rooms not yet seeded contribute nothing.
func neighbourSpecies(room *rooms.Room) []string {
	out := []string{}
	for _, ex := range room.Exits {
		if ex.RoomId == room.RoomId {
			continue
		}
		n := rooms.LoadRoom(ex.RoomId)
		if n == nil {
			continue
		}
		if st, ok := timber.LoadStand(n); ok {
			out = append(out, st.Species)
		}
	}
	return out
}

// ChopTarget is the target's side of a felling roll: easier than a carcass
// for a common tree, harder for each tier above it.
func ChopTarget(tier int) float64 {
	b := configs.GetBalanceConfig()
	return float64(tier-1)*float64(b.TimberTierDifficulty) - float64(b.TimberEase)
}

// ChopRounds is how long felling a tree of this tier takes before axe speed.
func ChopRounds(tier int) int {
	return int(configs.GetBalanceConfig().TimberChopRoundsBase) + tier - 1
}

// LogsFor is how many logs one felling yields: one, plus one per
// GatherStatPerBonusUnit of Strength above 100, plus one for a fine or better
// felling, capped by Balance.TimberMaxLogs.
func LogsFor(strength int, grade items.Quality) int {
	n := 1 + StatBonusUnits(strength)
	if grade >= items.QualityFine {
		n++
	}
	if max := int(configs.GetBalanceConfig().TimberMaxLogs); n > max {
		n = max
	}
	return n
}

// SpeciesKnown reports whether the surveyor can name the species: common
// woods (tier 1 and 2) always; rarer ones take a Perception and Search roll
// against a difficulty that rises with the tier. The surveyor has to see the
// bark and the leaf, so the score pays the sight ramp here.
func SpeciesKnown(actor Actor, sp *timber.Species) bool {
	if sp == nil {
		return false
	}
	if sp.Tier <= 2 {
		return true
	}
	char := actor.GetCharacter()
	score := CalcSearchScore(char) * messaging.SightMult(char, actor.GetRoom())
	return contest.AgainstDifficulty(score, 100+15*float64(sp.Tier-2)).Success
}

// standWord describes how much timber is left.
func standWord(st timber.Stand) string {
	switch {
	case st.Stock <= 0:
		return `cut back to stumps`
	case st.Stock*4 >= st.Max*3:
		return `a thick stand`
	case st.Stock*5 >= st.Max*2:
		return `a fair stand, partly cut`
	default:
		return `thinned, with only a few good trees left`
	}
}

// SurveyTrees describes the stand in the actor's room.
func SurveyTrees(actor Actor) {
	room := actor.GetRoom()
	now := util.GetRoundCount()
	st, sp, ok := RoomStand(room, now)
	if !ok {
		actor.SendText(messaging.CategorySystem, `There is no timber worth cutting here. Look for a forest, deep woods or a marsh.`)
		return
	}
	name := `an unfamiliar hardwood you can't put a name to`
	known := SpeciesKnown(actor, sp)
	if known {
		name = sp.Name
	}
	lines := []string{fmt.Sprintf(`The trees here are mostly <ansi fg="itemname">%s</ansi>: %s.`, name, standWord(st))}
	if known && sp.Note != `` {
		lines = append(lines, sp.Note)
	}
	if st.Stock <= 0 {
		b := configs.GetBalanceConfig()
		wait := st.RoundsToNextTree(now, int(b.TimberRegrowRounds))
		lines = append(lines, fmt.Sprintf(`Nothing here is ready to fell. Give it %s.`, roughWait(wait)))
	} else if sp.Tier >= 3 {
		lines = append(lines, `It is hard, close-grained wood, slow to fell.`)
	}
	if axe, has := gather.BestTool(actor.GetCharacter(), items.ToolAxe); !has {
		lines = append(lines, `You'll need an axe to fell any of it. (<ansi fg="command">help tools</ansi>)`)
	} else if known && axe.Tier < items.ToolTier(sp.MinAxe()) {
		lines = append(lines, AxeTooPoor(sp))
	}
	actor.SendText(messaging.CategorySystem, strings.Join(lines, "\n"))
}

// AxeTooPoor is what a woodcutter is told when their best axe cannot bite
// into this species.
func AxeTooPoor(sp *timber.Species) string {
	return fmt.Sprintf(`The %s is too hard for your axe: it would only chip the edge. You need a %s axe or better.`,
		sp.Name, items.ToolTier(sp.MinAxe()))
}

// roughWait turns rounds into words a player can plan by.
func roughWait(rounds uint64) string {
	secs := rounds * uint64(configs.GetTimingConfig().RoundSeconds)
	switch {
	case secs <= 0:
		return `a moment`
	case secs < 120:
		return `a minute or two`
	case secs < 3600:
		return fmt.Sprintf(`about %d minutes`, (secs+59)/60)
	default:
		return `an hour or more`
	}
}

// ChopResult is the outcome of one felling.
type ChopResult struct {
	Reason  string
	Roll    gather.Result
	Logs    int
	Species string
	Dropped bool // something was too heavy to carry and fell at the actor's feet
}

// ResolveChop finishes a felling job in the actor's room.
func ResolveChop(actor Actor) ChopResult {
	res := ChopResult{}
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		res.Reason = `no character or room`
		return res
	}
	now := util.GetRoundCount()
	st, sp, ok := RoomStand(room, now)
	if !ok || sp == nil {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">There is nothing here to fell.</ansi>`)
		res.Reason = `not choppable`
		return res
	}
	res.Species = sp.Name
	if st.Stock <= 0 {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">Someone has felled the last good tree here before you.</ansi>`)
		res.Reason = `no stock`
		return res
	}

	if axe, has := gather.BestTool(char, items.ToolAxe); has && axe.Tier < items.ToolTier(sp.MinAxe()) {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">`+AxeTooPoor(sp)+`</ansi>`)
		res.Reason = `axe too poor`
		return res
	}

	roll := gather.Roll(char, room, gather.JobChop, ChopTarget(sp.Tier))
	res.Roll = roll
	if roll.NoTool {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">You need an axe to fell a tree.</ansi>`)
		res.Reason = `no tool`
		return res
	}
	defer WearUsedTool(actor, roll.Tool, roll.HasTool)

	taken := []items.Item{}
	if roll.Success {
		st.Fell(now)
		timber.SaveStand(room, st)

		res.Logs = LogsFor(char.GetStatValue(`strength`), roll.Grade)
		give := func(itemId, qty int) {
			for i := 0; i < qty; i++ {
				itm := items.New(itemId)
				if !itm.IsValid() {
					return
				}
				itm.Quality = roll.Grade
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
		give(sp.LogItemId, res.Logs)
		if branch := items.FindSpecByComponentTag(`branch`); branch != nil {
			give(branch.ItemId, 1+util.Rand(2))
		}
		if sp.BarkItemId != 0 && util.Rand(100) < chopBarkChance {
			give(sp.BarkItemId, 1)
		}
	}

	actorLine := messaging.NoLine
	observer := messaging.Say(messaging.CategoryMobIdle, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> swings an axe at the trunk of a tree, the strokes ringing through the wood.`, actor.GetName()))
	if actor.IsPlayer() {
		if roll.Success {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="green">The %s creaks, leans and comes down with a crash. You limb it and buck it into lengths: %s.</ansi>`,
				sp.Name, summarizeTaken(taken)))
		} else {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="red">Your axe keeps glancing off the %s. You stop before you ruin the trunk.</ansi>`, sp.Name))
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
	if res.Dropped && actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, `<ansi fg="yellow">You can't carry it all; the rest lies at your feet.</ansi>`)
	}
	return res
}
