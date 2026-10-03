package main

import (
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rifts"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// baubleSweepSourceNames are every live store the bauble catalog sweep must
// see before it may prune anything: the six registered below and the
// auction house, which registers itself in modules/auctions. A sweep that
// runs while one of them is not registered fails closed
// (baubles.ExpectLiveSources). TestBaubleSweepSourcesMatchTheGuardedRoots
// holds this list, the registered sources and the guarded roots
// (sweepRoots, item_walker_guard_test.go) to the same names.
var baubleSweepSourceNames = []string{`auctions`, `guilds`, `housing`, `mobs`, `rifts`, `rooms`, `shops`, `users`}

// registerBaubleSweepSources tells the bauble catalog sweep
// (internal/baubles/sweep.go) where the live world keeps items. Each walk
// runs under the mud lock and only reads. The auction house registers its
// own in modules/auctions. Every store here has a WalkItems kept complete by
// TestItemWalkersVisitEveryItemField; a new store of items needs a source
// here, a name in baubleSweepSourceNames and a root in
// item_walker_guard_test.go, or TestEveryItemHolderIsASweepRootOrTransient
// fails.
func registerBaubleSweepSources() {
	baubles.ExpectLiveSources(baubleSweepSourceNames...)
	baubles.RegisterLiveSource(`users`, func(visit func(*items.Item)) {
		for _, u := range users.GetAllLoadedUsers() {
			u.WalkItems(visit)
		}
	})
	baubles.RegisterLiveSource(`rooms`, func(visit func(*items.Item)) {
		for _, r := range rooms.LoadedRooms() {
			r.WalkItems(visit)
		}
	})
	baubles.RegisterLiveSource(`mobs`, func(visit func(*items.Item)) {
		for _, id := range mobs.GetAllMobInstanceIds() {
			if m := mobs.GetInstance(id); m != nil {
				m.WalkItems(visit)
			}
		}
	})
	baubles.RegisterLiveSource(`shops`, func(visit func(*items.Item)) {
		for _, s := range shops.AllShops() {
			s.WalkItems(visit)
		}
	})
	baubles.RegisterLiveSource(`guilds`, func(visit func(*items.Item)) {
		for _, g := range guilds.All() {
			g.WalkItems(visit)
		}
	})
	// What players lost in rifts, waiting in rubble for someone else
	// (internal/rifts lost.go).
	baubles.RegisterLiveSource(`rifts`, rifts.WalkLostItems)
	// House containers' contents live in the house records (internal/housing).
	baubles.RegisterLiveSource(`housing`, func(visit func(*items.Item)) {
		for _, h := range housing.AllHouses() {
			h.WalkItems(visit)
		}
	})
}
