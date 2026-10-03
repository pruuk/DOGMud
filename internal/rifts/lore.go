package rifts

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// lore.go: lore objects, writings and rubble.
//
// A lore object (Profile.Lore) is something a player studies with `look`.
// Each distinct one studied says `lore_progress`; the Lore.Needed-th says
// `lore_unlocked` and lets the player read the profile's writings for good,
// in this rift and every one after. A writing (Profile.Writings) is a noun
// anyone can look at; a reader is also given the fragment of the story it
// holds. A rubble pile (Profile.Rubble) is searched once, by whoever gets to
// it first (ClaimRubble).
//
// What a player has learned is kept in their character's MiscData, so it is
// saved with the character:
//
//	rift-lore-seen:<profile>  []string  lore placements studied (until unlocked)
//	rift-reader:<profile>     bool      the permanent tag
//	rift-fragments:<profile>  []int     writings read (1-based), for reference

func loreSeenKey(p *Profile) string  { return `rift-lore-seen:` + p.Id }
func readerKey(p *Profile) string    { return `rift-reader:` + p.Id }
func fragmentsKey(p *Profile) string { return `rift-fragments:` + p.Id }

// IsReader reports whether u can read p's writings.
func IsReader(u *users.UserRecord, p *Profile) bool {
	if u == nil || p == nil {
		return false
	}
	v, _ := u.Character.GetMiscData(readerKey(p)).(bool)
	return v
}

// LoreStudied is how many distinct lore objects of p u has studied (Needed
// or more once they are a reader).
func LoreStudied(u *users.UserRecord, p *Profile) int {
	if IsReader(u, p) {
		return p.Lore.Needed
	}
	return len(stringList(u.Character.GetMiscData(loreSeenKey(p))))
}

// OnLook handles a player looking at something in a rift room: a lore object
// teaches, a writing is read (or not). The look itself has already shown the
// noun's description; this adds what the player makes of it.
func OnLook(userId, roomId int, target string) {
	run, rr := RoomInfo(roomId)
	if run == nil || rr == nil {
		return
	}
	u := users.GetByUserId(userId)
	room := rooms.LoadRoom(roomId)
	if u == nil || room == nil {
		return
	}
	noun, _ := room.FindNoun(strings.ToLower(strings.TrimSpace(target)))
	if noun == `` {
		return
	}
	p := run.Profile
	if rr.Memory != nil && p.Memory != nil && noun == p.Memory.Noun {
		lookTable(u, run, rr)
		return
	}
	if rr.LoreToken != `` && noun == p.Lore.Noun {
		studyLore(u, p, rr.LoreToken)
		return
	}
	if rr.Writing >= 0 && rr.Writing < len(p.Writings) && noun == p.Writings[rr.Writing].Noun {
		readWriting(u, p, rr.Writing)
	}
}

func studyLore(u *users.UserRecord, p *Profile, token string) {
	// A reader, or a lore object already studied, adds nothing: the look
	// itself has said all there is to say.
	if IsReader(u, p) {
		return
	}
	seen := stringList(u.Character.GetMiscData(loreSeenKey(p)))
	for _, t := range seen {
		if t == token {
			return
		}
	}
	seen = append(seen, token)
	if len(seen) >= p.Lore.Needed {
		u.Character.SetMiscData(readerKey(p), true)
		u.Character.SetMiscData(loreSeenKey(p), nil)
		u.SendText(messaging.CategorySystem, p.Msg(`lore_unlocked`))
		return
	}
	u.Character.SetMiscData(loreSeenKey(p), seen)
	u.SendText(messaging.CategorySystem, p.Msg(`lore_progress`))
}

func readWriting(u *users.UserRecord, p *Profile, idx int) {
	if !IsReader(u, p) {
		u.SendText(messaging.CategorySystem, p.Msg(`writing_unreadable`))
		return
	}
	u.SendText(messaging.CategorySystem, wrap(p.Writings[idx].Fragment))

	read := intList(u.Character.GetMiscData(fragmentsKey(p)))
	for _, n := range read {
		if n == idx+1 {
			return
		}
	}
	u.Character.SetMiscData(fragmentsKey(p), append(read, idx+1))
}

// ResetLore forgets everything u has learned of p (admin, for testing).
func ResetLore(u *users.UserRecord, p *Profile) {
	if u == nil || p == nil {
		return
	}
	u.Character.SetMiscData(loreSeenKey(p), nil)
	u.Character.SetMiscData(readerKey(p), nil)
	u.Character.SetMiscData(fragmentsKey(p), nil)
}

// FragmentsRead lists the writings (1-based) u has read of p.
func FragmentsRead(u *users.UserRecord, p *Profile) []int {
	return intList(u.Character.GetMiscData(fragmentsKey(p)))
}

// stringList reads a []string back from MiscData, which comes back from a
// saved character as []any.
func stringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return nil
}

// intList is stringList for []int.
func intList(v any) []int {
	switch t := v.(type) {
	case []int:
		return append([]int(nil), t...)
	case []any:
		out := make([]int, 0, len(t))
		for _, e := range t {
			switch n := e.(type) {
			case int:
				out = append(out, n)
			case float64:
				out = append(out, int(n))
			}
		}
		return out
	}
	return nil
}

// ClaimRubble settles a player's `search <feature>` against a rift room's
// rubble pile. handled is false unless featureName is the pile in that room.
// The first search of a pile claims its find: it returns the bauble tier for
// the caller to deliver and tells the player something is there. Any later
// search tells them the pile has been picked over.
func ClaimRubble(u *users.UserRecord, roomId int, featureName string) (tier string, claimed bool, handled bool) {
	run, rr := RoomInfo(roomId)
	if run == nil || rr == nil || !rr.Rubble || u == nil {
		return ``, false, false
	}
	p := run.Profile
	if !strings.EqualFold(featureName, p.Rubble.Noun) {
		return ``, false, false
	}
	if rr.RubbleSearched {
		u.SendText(messaging.CategorySystem, p.Msg(`rubble_empty`))
		return ``, false, true
	}
	rr.RubbleSearched = true
	u.SendText(messaging.CategorySystem, p.Msg(`rubble_found`))
	findLost(u, p) // now and then, something someone else lost here (lost.go)
	return p.Rubble.Tier[rr.Pool], true, true
}

// AmbientPlace is the rift's say about a room's generated ambient events
// (roomlife.PlaceHook, installed by modules/rifts): its profile's chance and
// setting, and no time of day. False for a room outside every rift.
func AmbientPlace(roomId int) (chance int, setting string, ok bool) {
	run := RunForRoom(roomId)
	if run == nil || run.Profile == nil {
		return 0, ``, false
	}
	return run.Profile.AmbientGeneratedChance, run.Profile.AmbientSetting, true
}

// MakeReader makes u a reader of p's writing at once, as if they had
// studied every lore object (admin `rift lore read`, for testing).
func MakeReader(u *users.UserRecord, p *Profile) {
	u.Character.SetMiscData(readerKey(p), true)
	u.Character.SetMiscData(loreSeenKey(p), nil)
}
