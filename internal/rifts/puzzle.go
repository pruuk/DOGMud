package rifts

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// puzzle.go: the puzzle kinds the engine checks itself (riddle, sequence)
// and the entry trap. Templates bring the words; this file brings the rules.

// Answer handles `answer <words>` in room. handled is false when the room
// holds no riddle, so the caller can say so in its own words.
func Answer(u *users.UserRecord, room *rooms.Room, words string) (handled bool) {
	run, rr := RoomInfo(room.RoomId)
	if run == nil || rr == nil || rr.Template.Puzzle == nil || rr.Template.Puzzle.Kind != `riddle` {
		return false
	}
	ps := rr.Template.Puzzle
	if rr.PuzzleSolved {
		u.SendText(messaging.CategorySystem, `That has already been answered.`)
		return true
	}
	given := normalizeAnswer(words)
	for _, a := range ps.Answers {
		if given != `` && given == normalizeAnswer(a) {
			solve(u, run, rr, room)
			return true
		}
	}
	punish(u, ps)
	return true
}

// Touch handles `touch <thing>` in room. It returns false when thing is not
// one of the room's nouns, so the caller can say it is not here.
func Touch(u *users.UserRecord, room *rooms.Room, thing string) (handled bool) {
	noun, _ := room.FindNoun(strings.ToLower(strings.TrimSpace(thing)))
	if noun == `` {
		return false
	}
	run, rr := RoomInfo(room.RoomId)
	if run == nil || rr == nil || rr.Template.Puzzle == nil || rr.Template.Puzzle.Kind != `sequence` || rr.PuzzleSolved || !inSequence(rr.Template.Puzzle, noun) {
		u.SendText(messaging.CategorySystem, fmt.Sprintf(`You touch the <ansi fg="noun">%s</ansi>. Nothing happens.`, noun))
		return true
	}
	ps := rr.Template.Puzzle
	if ps.Sequence[rr.SeqProgress] != noun {
		rr.SeqProgress = 0
		punish(u, ps)
		return true
	}
	rr.SeqProgress++
	if rr.SeqProgress >= len(ps.Sequence) {
		solve(u, run, rr, room)
		return true
	}
	u.SendText(messaging.CategorySystem, run.Profile.Msg(`touch_progress`, noun))
	return true
}

// inSequence reports whether noun is one of the things a sequence puzzle
// asks to be touched. Touching anything else (a door, the floor) is harmless.
func inSequence(ps *PuzzleSpec, noun string) bool {
	for _, n := range ps.Sequence {
		if n == noun {
			return true
		}
	}
	return false
}

func normalizeAnswer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return -1
	}, s)
	for _, article := range []string{`the `, `an `, `a `} {
		s = strings.TrimPrefix(s, article)
	}
	return strings.Join(strings.Fields(s), ` `)
}

// solve marks rr's puzzle solved, opens its sealed door and pays out.
func solve(u *users.UserRecord, run *Run, rr *RiftRoom, room *rooms.Room) {
	ps := rr.Template.Puzzle
	rr.PuzzleSolved = true
	for _, d := range rr.Doors {
		d.Sealed = false
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategorySystem, wrap(ps.Solved)),
		Actee:    messaging.NoLine, // a puzzle has no viewpoint
		Observer: messaging.Say(messaging.CategoryRoomDescription, run.Profile.Msg(`puzzle_unsealed`)),
	}, messaging.Audience{
		Actor:     u,
		ActorId:   u.UserId,
		ActorName: messaging.NoName,
		ActeeName: messaging.NoName,
		Room:      room,
	})
	switch ps.Reward {
	case `key`:
		if !rr.KeyAwarded {
			rr.KeyAwarded = true
			giveKey(u, run.Profile, room)
			u.SendText(messaging.CategorySystem, run.Profile.Msg(`puzzle_key`))
		}
	case `ore`:
		if len(run.Profile.OreItems) > 0 {
			if name, carried := giveItem(u, run.Profile.OreItems[rng(len(run.Profile.OreItems))], room); name != `` {
				where := `You take up`
				if !carried {
					where = `Your hands are full; on the floor lies`
				}
				u.SendText(messaging.CategoryLoot, fmt.Sprintf(`%s <ansi fg="item">%s</ansi>.`, where, name))
			}
		}
	}
}

// punish tells u they got it wrong and applies the puzzle's conditions.
func punish(u *users.UserRecord, ps *PuzzleSpec) {
	u.SendText(messaging.CategorySystem, wrap(ps.Wrong))
	for _, id := range ps.WrongConditionIds {
		u.AddCondition(id, `trap`)
	}
}

// springTrap tests u against rr's trap on their first entry: a Perception
// roll against the trap's difficulty. A single unopposed check, so it uses
// dice.RollStat like other static checks.
func springTrap(u *users.UserRecord, run *Run, rr *RiftRoom) {
	trap := rr.Template.Trap
	if trap == nil || rr.trapChecked[u.UserId] {
		return
	}
	rr.trapChecked[u.UserId] = true
	perception := float64(u.Character.Stats.Perception.ValueAdj)
	if dice.RollStat(perception).Value >= float64(trap.Difficulty) {
		u.SendText(messaging.CategorySystem, wrap(trap.Avoided))
		return
	}
	u.SendText(messaging.CategoryWarning, wrap(trap.Triggered))
	ids := trap.ConditionIds
	if len(ids) == 0 {
		ids = run.Profile.TrapConditionIds
	}
	for _, id := range ids {
		u.AddCondition(id, `trap`)
	}
}
