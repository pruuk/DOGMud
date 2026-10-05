package rifts

// instance.go: who may share a run. A run belongs to whoever enters it
// first and their party; nobody else can get in, so two players who are not
// in a party together never meet inside a rift.
//
// Routing (routePortal) has no side effects: it only names a room. The
// entry guard (EntryGuard), which runs on the actual move, is where a player
// is committed to a run: they claim it (claims), so the next stranger at the
// same crystal is sent to a fresh run of their own, and their party,
// following a moment later, is let in beside them.

import (
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/parties"
)

// claimFor is how long a claim made at the portal holds before the claimant
// arrives (their RoomChange makes them a member) or gives up.
const claimFor = 30 * time.Second

// sameParty reports whether two players are in one party (or are one
// player).
func sameParty(a, b int) bool {
	if a == b {
		return true
	}
	// Both accepted members: parties.Get also answers for a player who has
	// only been invited, and an invite nobody accepted joins no one.
	p := parties.Get(a)
	return p != nil && p.IsMember(a) && p.IsMember(b)
}

// company is everyone the run belongs to: its members and anyone who has
// claimed it at the portal and is still on their way in.
func (run *Run) company() []int {
	ids := make([]int, 0, len(run.Members)+len(run.claims))
	for uid := range run.Members {
		ids = append(ids, uid)
	}
	t := now()
	for uid, until := range run.claims {
		if !run.Members[uid] && t.Before(until) {
			ids = append(ids, uid)
		}
	}
	sort.Ints(ids)
	return ids
}

// claimed reports whether userId has a live claim on the run.
func (run *Run) claimed(userId int) bool {
	until, ok := run.claims[userId]
	return ok && now().Before(until)
}

// mayJoin reports whether userId, not yet a member, may come into the run
// through its portal: it is open, and either it belongs to nobody yet (a
// fresh run) or userId is in a party with everyone it belongs to.
func (run *Run) mayJoin(userId int) bool {
	if !run.PortalOpen {
		return false
	}
	co := run.company()
	if len(co) == 0 {
		return !run.Entered
	}
	// In a party with everyone already in it: someone who left the party
	// cannot bring a stranger in beside those still in it.
	for _, uid := range co {
		if !sameParty(uid, userId) {
			return false
		}
	}
	return true
}

// claim commits userId to the run (EntryGuard, on the move through the
// portal). A site's waiting run that is claimed is no longer the site's to
// hand out: the next stranger gets a new one.
func (run *Run) claim(userId int) {
	if run.claims == nil {
		run.claims = map[int]time.Time{}
	}
	run.claims[userId] = now().Add(claimFor)
	if site := sites[run.OriginRoomId]; site != nil && site.run == run {
		site.run = nil
	}
}

// partyRunAt is the open run at a site that userId belongs to or may join
// as a party member, or nil.
func partyRunAt(site *Site, userId int) *Run {
	for _, run := range Runs() {
		if run.OriginRoomId != site.RoomId || !run.PortalOpen || run.Profile.Id != site.ProfileId {
			continue
		}
		if run.Members[userId] || run.claimed(userId) {
			return run
		}
		if len(run.company()) > 0 && run.mayJoin(userId) {
			return run
		}
	}
	return nil
}
