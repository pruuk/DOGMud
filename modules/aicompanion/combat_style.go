package aicompanion

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Fighting styles. Every companion's blow by blow is the engine's scripted
// combat AI: the mob template's behavior_archetype (a companion_* tree in
// _datafiles/world/dogmud/behaviors/archetypes) picks the special moves
// and spells that suit them each round, under the engine's own gates and
// the shared special-move cooldown. What this file adds is the part no
// scripted tree can see: her owner.
//
//   - mend_below: a healer mends her owner mid-fight (packmate healing in
//     the trees never reaches a player).
//   - ward_owner: she wards her owner once as a fight opens.
//   - opener surprise: from hiding, her first blow is a surprise attack.
//   - the model's own choice of spell, cast once.
//
// Every one of these is an ordinary mob command (cast, attack), so the
// roll, the cost, the cooldown and the interruption rules are the engine's.

// castReady reports whether she could begin a cast this moment: not busy
// casting, crafting or the like, and not inside the shared special-move
// cooldown that casts, bashes and kicks all draw on.
func castReady(mob *mobs.Mob) bool {
	return !mob.Character.IsActing() && actions.SpecialMoveReady(&mob.Character)
}

// bestSpell is the strongest spell she could cast right now that does
// `what` (mends, wards) for someone else: a single-target helping spell,
// the costliest she can pay for.
func bestSpell(mob *mobs.Mob, what string) (spellOption, bool) {
	var best spellOption
	found := false
	for _, o := range spellsReady(mob) {
		if o.What != what || o.Harm || o.SelfOK {
			continue
		}
		if !found || o.Cost > best.Cost {
			best, found = o, true
		}
	}
	return best, found
}

// tendOwner is a healer's reflex: the command that mends or wards her owner
// now, or "" for nothing to do.
func (m *AICompanionModule) tendOwner(c *controller, mob *mobs.Mob, u *users.UserRecord, room *rooms.Room, ownerPct int) string {
	f := c.fight
	p := c.profile
	if f == nil || p == nil || u == nil || u.Character == nil || f.Stance == `flee` {
		return ``
	}
	if p.Combat.MendBelow <= 0 && !p.Combat.WardOwner {
		return ``
	}
	if !castReady(mob) {
		return ``
	}
	if p.Combat.MendBelow > 0 && ownerPct < p.Combat.MendBelow && u.Character.Health > 0 {
		if o, ok := bestSpell(mob, `mends`); ok {
			c.mind.addLine(Line{Kind: `event`, Text: `You mended ` + u.Character.Name + ` in the thick of it.`}, m.cfg.WorkingMemoryLines)
			return fmt.Sprintf(`cast %s @%d`, o.Id, u.UserId)
		}
	}
	if p.Combat.WardOwner && !f.Warded {
		f.Warded = true // tried once a fight, whether or not it is in reach
		if o, ok := bestSpell(mob, `wards`); ok {
			return fmt.Sprintf(`cast %s @%d`, o.Id, u.UserId)
		}
	}
	return ``
}

// fightCastCommand turns the spell the model called for into a cast. It
// returns "" with keep=true when the spell should wait for a better moment
// (she is busy, or the cooldown is still on), and "" with keep=false when it
// can never go as asked (she no longer knows it or cannot pay, the enemy is
// gone, or her owner could not harm them).
func fightCastCommand(c *controller, mob *mobs.Mob, u *users.UserRecord, room *rooms.Room, s *fightSpell) (cmd string, keep bool) {
	if s == nil || room == nil {
		return ``, false
	}
	known := false
	for _, o := range spellsReady(mob) {
		if o.Id == s.Id {
			known = true
			break
		}
	}
	if !known {
		return ``, false
	}
	if s.Harm && c.fight != nil && c.fight.Stance == `hold_back` {
		return ``, false // holding back is not joining in
	}
	if !castReady(mob) {
		return ``, true
	}
	if s.Harm {
		if s.Area {
			if ok, _ := areaHarmAllowed(u, room, mob, c.profile); !ok {
				return ``, false
			}
			return `cast ` + s.Id, false
		}
		target := s.AtMob
		if target == 0 {
			// The enemy she is fighting, under the same rule as a special
			// move: hers to choose, so held to her owner's rules.
			if !mayStrikeCurrent(u, room, mob) {
				return ``, false
			}
			cur := mob.Character.CurrentCombatTarget()
			if cur.MobInstanceId == 0 {
				return ``, false
			}
			target = cur.MobInstanceId
		} else if t := mobs.GetInstance(target); t == nil || t.Character.RoomId != room.RoomId || t.Character.Health <= 0 ||
			!mayStrike(u, room, target) {
			return ``, false
		}
		return fmt.Sprintf(`cast %s #%d`, s.Id, target), false
	}
	switch {
	case s.SelfOK || s.AtSelf:
		return `cast ` + s.Id, false
	case s.AtOwner && u != nil && u.Character != nil && u.Character.RoomId == room.RoomId:
		return fmt.Sprintf(`cast %s @%d`, s.Id, u.UserId), false
	case s.AtMob != 0:
		// A helping spell on a creature (soothing a hurt dog that fights
		// beside them, say): only one still here.
		if t := mobs.GetInstance(s.AtMob); t != nil && t.Character.RoomId == room.RoomId {
			return fmt.Sprintf(`cast %s #%d`, s.Id, s.AtMob), false
		}
		return ``, false
	}
	return `cast ` + s.Id, false
}

// resolveFightSpell reads the model's spell choice against what she knows
// and can pay for right now, and the e-refs of this fight.
func resolveFightSpell(mob *mobs.Mob, f *fightState, ref string, at string) *fightSpell {
	o, ok := findSpellOption(spellsReady(mob), ref)
	if !ok {
		return nil
	}
	s := &fightSpell{Id: o.Id, Name: o.Name, Harm: o.Harm, Area: o.Area, SelfOK: o.SelfOK}
	at = strings.ToLower(strings.TrimSpace(at))
	switch at {
	case `owner`:
		if s.Harm {
			return nil // never at her owner
		}
		s.AtOwner = true
	case `self`, `yourself`:
		if s.Harm {
			return nil
		}
		s.AtSelf = true
	case ``:
		if !s.Harm {
			s.AtSelf = true
		}
	default:
		id, ok := f.Refs[at]
		if !ok || id <= 0 {
			return nil
		}
		s.AtMob = id
	}
	return s
}

// surpriseOpener is the first blow of a fight struck from hiding, for a
// companion whose profile fights that way: an ordinary attack, which the
// engine turns into a surprise attack because she is hidden
// (actions.EngageAggroType). It goes for whatever is fighting her owner,
// and otherwise any enemy here her owner could harm.
func (m *AICompanionModule) surpriseOpener(c *controller, mob *mobs.Mob, u *users.UserRecord, room *rooms.Room, round uint64, enemies map[int]string) bool {
	if c.profile == nil || c.profile.Combat.Opener != `surprise` || c.fight == nil || c.fight.Stance == `hold_back` {
		return false
	}
	if !mob.Character.IsHidden() || u == nil || room == nil {
		return false
	}
	target := threatTo(room, u.UserId)
	if target == 0 || !mayStrike(u, room, target) {
		target = 0
		for id := range enemies {
			if mayStrike(u, room, id) && (target == 0 || id < target) {
				target = id
			}
		}
	}
	if target == 0 {
		return false
	}
	mob.Command(fmt.Sprintf(`attack #%d`, target))
	c.fight.LastReflex = round
	c.fight.TargetId = target
	c.mind.addLine(Line{Kind: `event`, Text: `You struck first, from hiding.`}, m.cfg.WorkingMemoryLines)
	return true
}

// canSneak reports whether slipping out of sight is part of who she is: a
// profile that trains skullduggery or opens a fight from hiding.
func canSneak(p *Profile) bool {
	if p == nil {
		return false
	}
	if p.Combat.Opener == `surprise` {
		return true
	}
	return p.Archetype.Skills[`skullduggery`] > 0
}

// combatRule is the fighting paragraph of the system prompt, written for
// this companion: how they fight, the moves that suit them, the spells
// they could reach for, and what they do on their own.
func combatRule(p *Profile, owner string, knowsSpells bool) string {
	var b strings.Builder
	b.WriteString("- Fighting: your body fights on its own, as it always has")
	if a := strings.TrimSpace(p.Combat.Approach); a != `` {
		b.WriteString(": " + strings.TrimSuffix(a, `.`))
	}
	b.WriteString(". In a fight you choose the plan in combat: stance fight, protect (put yourself between " + owner + " and harm), hold_back (do not join in; you still defend yourself) or flee; which enemy to go for; when you would run; melee or ranged")
	if moves := movesFor(p); len(moves) > 0 {
		b.WriteString("; a special move to try next when one would help (" + strings.Join(moves, ", ") + ")")
	} else {
		b.WriteString("; no special moves, which are not your way of fighting (leave move none)")
	}
	if knowsSpells {
		b.WriteString("; and a spell you know to cast next (spell: its [m] ref; spell_at: owner, self or an [e] ref)")
	}
	b.WriteString(".")
	if len(movesFor(p)) > 0 {
		b.WriteString(" Your body only manages a move when it can (the right gear in hand, the foe where you need them) and not twice in a breath.")
	}
	if p.Combat.MendBelow > 0 {
		b.WriteString(" You mend " + owner + " yourself when they are badly hurt, without needing to plan it.")
	}
	if p.Combat.WardOwner {
		b.WriteString(" As a fight opens you ward " + owner + " if you can.")
	}
	if p.Combat.Opener == `surprise` {
		b.WriteString(" If you are hidden when a fight starts, your first blow takes them by surprise, so sneak before a fight you can see coming.")
	}
	b.WriteString(" Choose as the person you are: how brave you are, how you feel about " + owner + ", whether the fight is just. Keep speech in a fight to a few words. Outside a fight leave combat unchanged.\n")
	return b.String()
}
