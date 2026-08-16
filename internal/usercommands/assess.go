package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Assess lets a player evaluate a corpse to determine its essence and what
// undead forms it could support. It is the primary way to decide which
// raise spell to use before animating a corpse.
func Assess(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	rest = strings.TrimSpace(rest)
	if rest == `` {
		user.SendText(messaging.CategorySystem, `Assess what?`)
		return true, nil
	}

	corpse, found := room.FindCorpse(rest)
	if !found {
		user.SendText(messaging.CategorySystem, `You don't see those remains here.`)
		return true, nil
	}

	if corpse.Character.IsCharmed() {
		user.SendText(messaging.CategorySystem, `These remains were bound to a master. The essence is spent. There is nothing left to raise.`)
		return true, nil
	}

	if !user.Character.TryCooldown(`assess`, "6 rounds") {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf("You need to wait %d more rounds before you can assess again.", user.Character.GetCooldown(`assess`)),
		)
		return true, nil
	}

	// Sum all stat training values as a proxy for the creature's total power.
	stats := corpse.Character.Stats
	totalTraining := stats.Strength.Training +
		stats.Dexterity.Training +
		stats.Perception.Training +
		stats.Vitality.Training +
		stats.Willpower.Training +
		stats.Charisma.Training

	// Describe the power level without exposing raw numbers. Each phrase
	// completes the sentence "You sense ..." on its own, so it needs no
	// dash and no second clause bolted on afterwards.
	var essenceDesc string
	switch {
	case totalTraining >= 500:
		essenceDesc = `overwhelming essence in these remains, more than death can hold`
	case totalTraining >= 300:
		essenceDesc = `immense essence in these remains, a truly mighty creature in life`
	case totalTraining >= 200:
		essenceDesc = `powerful essence in these remains, formidable in life`
	case totalTraining >= 120:
		essenceDesc = `substantial essence in these remains, a strong life force`
	case totalTraining >= 60:
		essenceDesc = `moderate essence in these remains`
	case totalTraining >= 30:
		essenceDesc = `faint residual essence in these remains`
	default:
		essenceDesc = `barely a trace of essence in these remains`
	}

	user.SendText(messaging.CategorySystem, `You study the remains of <ansi fg="mob-corpse">`+corpse.Character.Name+`</ansi>.`)
	user.SendText(messaging.CategorySystem, `You sense `+essenceDesc+`.`)

	// List which undead types this corpse could support, driven by the raise
	// spells' own summon_min_corpse_pool gates so assess and the spells can
	// never disagree. For each supported form, also work out the Conviction
	// it would reserve for THIS caster (skill/mutation-reduced) so the
	// reservation can be disclosed before anything is summoned.
	type raiseGroup struct {
		band       string
		forms      []string
		maxReserve int
	}
	var supported []string
	var groups []*raiseGroup
	for _, form := range []string{`skeleton`, `zombie`, `wraith`, `spectre`, `vampire`, `golem`} {
		sp := spells.GetSpell(`raise-` + form)
		if sp == nil || !sp.SummonRequiresCorpse || totalTraining < sp.SummonMinCorpsePool {
			continue
		}
		supported = append(supported, form)

		reserve := user.Character.CalcCompanionReserve(
			characters.CompanionReserveBase(sp.SummonPetMultiplier))
		band := convictionReserveBand(reserve, user.Character.ConvictionMax.Value)
		var grp *raiseGroup
		for _, g := range groups {
			if g.band == band {
				grp = g
				break
			}
		}
		if grp == nil {
			grp = &raiseGroup{band: band}
			groups = append(groups, grp)
		}
		grp.forms = append(grp.forms, form)
		if reserve > grp.maxReserve {
			grp.maxReserve = reserve
		}
	}

	if len(supported) == 0 {
		user.SendText(messaging.CategorySystem, `The essence is too faint to animate any form.`)
	} else {
		user.SendText(messaging.CategorySystem, `It could sustain: `+strings.Join(supported, `, `)+`.`)
		// Disclose the Conviction reservation per band — descriptive, never
		// a raw number (see the player-facing messages rule in CLAUDE.md).
		for _, g := range groups {
			line := fmt.Sprintf(`Raising %s would set aside %s of your conviction while it serves.`,
				joinRaiseForms(g.forms), g.band)
			if user.Character.WouldBreachReservationCap(characters.PoolConviction, g.maxReserve) {
				line += ` You could not spare that right now.`
			}
			user.SendText(messaging.CategorySystem, line)
		}
	}

	// Trigger manifestation skill progression.
	user.Character.OnSkillUse(string(skills.Manifestation), user.UserId)

	return true, nil
}

// convictionReserveBand maps a companion's Conviction reservation to a
// descriptive band relative to the caster's pool — player-facing text never
// shows the raw number.
func convictionReserveBand(reserve, maxPool int) string {
	if maxPool <= 0 || reserve >= maxPool {
		return `more than your spirit could hold`
	}
	return reserveShareBand(reserve, maxPool)
}

// reserveShareBand is the pool-agnostic half of convictionReserveBand: it names
// what SHARE of a pool a reservation holds, with no flavour committing it to
// Conviction. `stand` uses it to disclose a stamina reservation. Player-facing
// text never shows the raw number.
//
// The edges now live in characters.ReserveShareBand, because the cap refusals
// on `Wear`, summon, charm and the two auto-spawn paths need the same words and
// cannot import usercommands. Kept as a package-local name so assess.go and
// stand.go read unchanged.
func reserveShareBand(reserve, maxPool int) string {
	return characters.ReserveShareBand(reserve, maxPool)
}

// joinRaiseForms renders a form list as prose: "a skeleton", "a skeleton or
// zombie", "a skeleton, zombie, or wraith".
func joinRaiseForms(forms []string) string {
	switch len(forms) {
	case 0:
		return ``
	case 1:
		return `a ` + forms[0]
	case 2:
		return `a ` + forms[0] + ` or ` + forms[1]
	default:
		return `a ` + strings.Join(forms[:len(forms)-1], `, `) + `, or ` + forms[len(forms)-1]
	}
}
