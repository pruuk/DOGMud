package aicompanion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Perception rule: the companion is told only what a player standing where
// it stands could see or already knows about itself. Hidden characters are
// left out, secret exits are left out, and nobody's condition is given as a
// number. Everything here reads live game state, under the mud lock, and
// returns plain text so no game pointer escapes to the model goroutine.

// healthWords mirrors the bands of characters.GetHealthAppearance without
// its markup, so the model gets words, never numbers.
// healthWords describes a character's condition against the ceiling it can
// actually reach (see healthPct: reserving gear lowers it).
func healthWords(ch *characters.Character) string {
	cur, max := ch.Health, ch.EffectivePoolMax(characters.PoolHealth)
	if max <= 0 {
		return `unhurt`
	}
	pct := cur * 100 / max
	switch {
	case pct < 15:
		return `about to die`
	case pct < 50:
		return `in bad shape`
	case pct < 80:
		return `cut and bruised`
	case pct < 100:
		return `a few scratches`
	}
	return `unhurt`
}

func purseWords(gold int) string {
	switch {
	case gold <= 0:
		return `no coin at all`
	case gold < 20:
		return `a few coins`
	case gold < 200:
		return `a modest purse`
	case gold < 2000:
		return `a comfortable purse`
	}
	return `a heavy purse`
}

// describeSituation builds the WHERE YOU ARE NOW section for one companion.
func describeSituation(mob *mobs.Mob, owner *users.UserRecord) string {
	if mob == nil {
		return ``
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return ``
	}

	var b strings.Builder
	sight := sightOf(mob, room)
	dark := sight != messaging.SightFull

	// A place you cannot see is not a place you can name.
	if dark {
		b.WriteString("Place: you cannot make out where you are.\n")
	} else {
		fmt.Fprintf(&b, "Place: %s.\n", strings.TrimSpace(room.Title))
	}
	fmt.Fprintf(&b, "Time: %s.\n", timeWords(gametime.GetDate()))
	if dark {
		if sight == messaging.SightShapes {
			b.WriteString("You can make out shapes moving, no more than that.\n")
		} else {
			b.WriteString("You can see nothing at all here.\n")
		}
	} else {
		desc := strings.Join(strings.Fields(room.GetDescription()), ` `)
		if len(desc) > 420 {
			desc = desc[:420]
			if i := strings.LastIndexAny(desc, `.!?`); i > 200 {
				desc = desc[:i+1]
			}
		}
		if desc != `` {
			fmt.Fprintf(&b, "Around you: %s\n", desc)
		}

		exits := make([]string, 0, len(room.Exits))
		for name, ex := range room.Exits {
			if ex.Secret {
				continue
			}
			exits = append(exits, name)
		}
		sort.Strings(exits)
		if len(exits) > 0 {
			fmt.Fprintf(&b, "Ways out: %s.\n", strings.Join(exits, `, `))
		}
	}

	// People.
	var people []string
	ownerHere := false
	for _, uid := range room.GetPlayers() {
		u := users.GetByUserId(uid)
		if u == nil || u.Character == nil {
			continue
		}
		if owner != nil && uid == owner.UserId {
			ownerHere = true
			continue
		}
		if !mob.Character.Perceives(u.Character) || dark {
			continue
		}
		people = append(people, u.Character.Name)
	}
	if owner != nil && owner.Character != nil {
		ownerVisible := ownerHere && !dark && mob.Character.Perceives(owner.Character)
		if ownerHere && !ownerVisible {
			// You know their voice and their step; you cannot see the
			// state of them, whether it is the dark or their own craft
			// keeping them out of sight.
			fmt.Fprintf(&b, "%s is somewhere close by, out of sight.\n", owner.Character.Name)
		} else if ownerHere {
			fmt.Fprintf(&b, "%s is here with you (%s)%s.\n", owner.Character.Name,
				healthWords(owner.Character), ownerLooks(owner))
		} else {
			fmt.Fprintf(&b, "%s is not here with you.\n", owner.Character.Name)
		}
	}
	if len(people) > 0 {
		fmt.Fprintf(&b, "Other people here: %s.\n", strings.Join(people, `, `))
	}

	// Creatures and other NPCs.
	var others []string
	if !dark {
		for _, mid := range room.GetMobs() {
			if mid == mob.InstanceId {
				continue
			}
			m := mobs.GetInstance(mid)
			if m == nil || !mob.Character.Perceives(&m.Character) {
				continue
			}
			others = append(others, m.Character.Name)
		}
	}
	if len(others) > 0 {
		fmt.Fprintf(&b, "Also here: %s.\n", strings.Join(others, `, `))
	}

	// Things lying about, as a player's `look` would show them.
	if !dark {
		if things := roomThings(room); len(things) > 0 {
			fmt.Fprintf(&b, "Lying about: %s.\n", strings.Join(things, `, `))
		}
	}

	// Yourself.
	self := healthWords(&mob.Character)
	if mob.Character.IsInCombat() {
		self += `, and in a fight`
	}
	fmt.Fprintf(&b, "You: %s.\n", self)

	var worn []string
	for _, it := range mob.Character.Equipment.GetAllItems() {
		if it.ItemId < 1 {
			continue
		}
		worn = append(worn, it.ModelName())
	}
	if len(worn) > 0 {
		fmt.Fprintf(&b, "You have on you: %s.\n", strings.Join(worn, `, `))
	}
	var carried []string
	for _, it := range mob.Character.Items {
		if it.ItemId < 1 {
			continue
		}
		carried = append(carried, it.ModelName())
	}
	if len(carried) > 0 {
		fmt.Fprintf(&b, "In your pack: %s.\n", strings.Join(carried, `, `))
	}
	fmt.Fprintf(&b, "Your purse: %s.\n", purseWords(mob.Character.Gold))

	return b.String()
}

// roomThings lists what is visibly lying in a room: floor items, bodies,
// unhidden containers and loose coin. Container contents are not listed;
// a player has to look inside, and so will the companion (phase 3).
func roomThings(room *rooms.Room) []string {
	var out []string
	for _, it := range room.Items {
		if it.ItemId < 1 || it.IsFixture() {
			continue
		}
		out = append(out, it.ModelName())
	}
	for _, c := range room.Corpses {
		if c.CorpseName != `` {
			out = append(out, c.CorpseName)
			continue
		}
		out = append(out, `the body of `+c.Character.Name)
	}
	names := make([]string, 0, len(room.Containers))
	for name, ct := range room.Containers {
		if ct.Hidden {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out = append(out, names...)
	if room.Gold > 0 {
		out = append(out, `some coins on the ground`)
	}
	const maxThings = 15
	if len(out) > maxThings {
		out = append(out[:maxThings], `and more besides`)
	}
	return out
}

// otherPlayersPresent counts the players in a room, other than the speaker,
// that the companion can actually perceive: someone it cannot see is not
// company, and should not stop the owner's words counting as spoken to it.
func otherPlayersPresent(room *rooms.Room, speakerUserId int, mob *mobs.Mob) int {
	if room == nil || mob == nil {
		return 0
	}
	n := 0
	for _, uid := range room.GetPlayers() {
		if uid == speakerUserId {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil || u.Character == nil || !mob.Character.Perceives(u.Character) {
			continue
		}
		n++
	}
	return n
}

// timeWords describes the game time the way a person would: the part of
// the day and the month, as a player sees from the game's clock.
func timeWords(d gametime.GameDate) string {
	var part string
	switch h := d.Hour24; {
	case d.Night && h >= 3 && h < 6:
		part = `the small hours before dawn`
	case d.Night:
		part = `night`
	case h < 9:
		part = `early morning`
	case h < 12:
		part = `morning`
	case h < 14:
		part = `midday`
	case h < 17:
		part = `afternoon`
	default:
		part = `evening`
	}
	return fmt.Sprintf(`%s, in the month of %s`, part, gametime.MonthName(d.Month))
}

// ownerLooks is what anyone standing there can see of the owner: their
// kind and what they have in hand.
func ownerLooks(owner *users.UserRecord) string {
	var parts []string
	if sp := species.GetSpecies(owner.Character.SpeciesId); sp != nil && sp.Name != `` {
		parts = append(parts, strings.ToLower(sp.Name))
	}
	if w := owner.Character.Equipment.Weapon; w.ItemId > 0 {
		parts = append(parts, `carrying `+w.ModelName())
	}
	if len(parts) == 0 {
		return ``
	}
	return `; ` + strings.Join(parts, `, `)
}

// sightOf asks the engine what the companion can see here, exactly as it
// decides for a player: blindness first, then the room's light, night
// vision, and infrared (which shows shapes only).
func sightOf(mob *mobs.Mob, room *rooms.Room) messaging.SightDecision {
	if mob == nil {
		return messaging.SightNone
	}
	return messaging.ParticipantSight(&mob.Character, room)
}

// cannotSee reports whether the companion cannot make out names and detail
// here: blind, or in the dark without the eyes for it. Shapes-only counts,
// because a shape is not a name.
func cannotSee(mob *mobs.Mob, room *rooms.Room) bool {
	return sightOf(mob, room) != messaging.SightFull
}
