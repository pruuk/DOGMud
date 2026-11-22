package guards

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// #940 (owner ruling 2026-11-22): a copy a quest handed out fades whenever it
// leaves its holder's hands (actions.VanishQuestGiven, or
// actions.FadeQuestGivenOffHand for a copy already out of the carry lists).
// No single function sees every way an item leaves a character, so this guard
// lists two kinds of call in production code, keyed by file and enclosing
// function, and pins how many of each a function makes:
//
//   - every call to a method named RemoveItem: the way an item comes out of a
//     character's carry lists;
//   - every call to a method named AddItem with TWO arguments: that shape is
//     Room.AddItem(item, stash), the floor (Container.AddItem and
//     Storage.AddItem take one). This catches a floor drop that never went
//     through RemoveItem: an item displaced by an equip, taken off while
//     overloaded, refused by a pet, or ejected from a bandolier.
//
// A function that is not listed, or that makes more or fewer of these calls
// than its entry says, fails the test, so a new way out (even a second call
// inside a function already listed) fails CI until someone decides what
// quest gear does there and updates the entry.
//
// The match is syntactic (the method name and argument count, not the
// receiver type), like the other call guards in this package, so RemoveItem
// on a room, a container, a corpse, storage or a pet is listed too, as
// not-a-leave.
//
// Classes:
//   - consumed: the item is used up, spent or destroyed; nothing leaves to
//     anyone.
//   - replaced-in-place: the item comes out and goes straight back to the
//     same holder (equip, rename, a rollback, a hand-back).
//   - quest-given-checked: a way out that fades quest gear (or refuses it, or
//     hands it back) before this call, so quest gear never reaches it.
//   - not-a-leave: the item is not in a player's or companion's hands: a room,
//     a container, a corpse, storage, a spawn, an NPC's own stock, or a theft
//     from a mob (involuntary; the copy keeps its mark).

type removeSiteClass string

const (
	removeConsumed       removeSiteClass = "consumed"
	removeReplaced       removeSiteClass = "replaced-in-place"
	removeQuestChecked   removeSiteClass = "quest-given-checked"
	removeNotALeave      removeSiteClass = "not-a-leave"
	vanishQuestGivenFile                 = "internal/actions/quest_given.go"
	vanishQuestGivenFunc                 = "VanishQuestGiven"
)

type removeSite struct {
	class  removeSiteClass
	calls  int // how many matching calls the function makes
	reason string
}

// removeItemSites is keyed "file|function" (a method as Recv.Name). Keyed by
// function, not line, so edits do not break it; the call count pins a new
// call inside a listed function.
var removeItemSites = map[string]removeSite{
	vanishQuestGivenFile + "|" + vanishQuestGivenFunc: {removeQuestChecked, 1, "the vanish helper itself"},

	// Voluntary ways out: each fades quest gear (or refuses it) first.
	"internal/actions/transfer.go|TransferItemToFloor":           {removeQuestChecked, 1, "drop: fades quest gear before the floor"},
	"internal/actions/transfer.go|TransferItemBetweenChars":      {removeQuestChecked, 1, "give: questGivenLeaves fades it here for every caller; a tree's return_item and a companion's return keep the mark"},
	"internal/actions/plant.go|plantOnMob":                       {removeQuestChecked, 1, "plantQuestGearFades fades it at success, before this"},
	"internal/actions/plant.go|plantOnPlayer":                    {removeQuestChecked, 1, "plantQuestGearFades fades it at success, before this"},
	"internal/actions/plant.go|plantInContainer":                 {removeQuestChecked, 1, "plantQuestGearFades fades it at success, before this"},
	"internal/actions/sell.go|sellOneToMerchant":                 {removeQuestChecked, 1, "QuoteSale refuses quest gear before the sale"},
	"internal/actions/sell_bauble.go|sellBaubleToMerchant":       {removeQuestChecked, 1, "reached only after QuoteSale, which refuses quest gear"},
	"internal/actions/buy.go|validatePurchase":                   {removeQuestChecked, 1, "trade-in: findTradeInItem never picks quest gear"},
	"internal/usercommands/give.go|giveToPet":                    {removeQuestChecked, 1, "own pet is a companion; another player's pet or no room hands quest gear back first"},
	"internal/usercommands/guild.go|guildDonate":                 {removeQuestChecked, 1, "donate fades quest gear first"},
	"internal/usercommands/mail.go|Mail":                         {removeQuestChecked, 1, "quest gear fades instead and the mail goes without it"},
	"internal/usercommands/put.go|Put":                           {removeQuestChecked, 2, "fades quest gear before the container; the second call is container overflow"},
	"internal/usercommands/stash.go|Stash":                       {removeQuestChecked, 1, "stash fades quest gear first"},
	"internal/usercommands/storage.go|storageAddQuiet":           {removeQuestChecked, 1, "storage add and stow fade quest gear first"},
	"internal/mobcommands/put.go|Put":                            {removeQuestChecked, 2, "a companion's put fades quest gear first; the second call is container overflow"},
	"modules/auctions/auctions.go|AuctionsModule.auctionCommand": {removeQuestChecked, 1, "quest gear fades at the listing instead"},

	// The item is used up, spent or destroyed: nobody receives it.
	"internal/actions/combat_reload.go|chamberNextRound":        {removeConsumed, 1, "an emptied ammo bundle"},
	"internal/actions/salvage.go|salvageItem":                   {removeConsumed, 2, "salvage breaks the item down; quest gear crumbles to nothing (owner ruling 2026-11-22)"},
	"internal/behaviortree/actions_quest.go|actTakeItem":        {removeConsumed, 1, "a quest tree takes the item"},
	"internal/conversation/playerstate.go|removeBackpackItem":   {removeConsumed, 1, "a topic's requiresItem takes the item"},
	"internal/dialogue/engine.go|applyQuestEffects":             {removeConsumed, 1, "a topic's requiresItem takes the item"},
	"internal/questengine/bridge.go|GameBridge.ConsumeItem":     {removeConsumed, 2, "a quest step consumes the item"},
	"internal/usercommands/give.go|Give":                        {removeConsumed, 1, "the quest engine consumed a give"},
	"internal/usercommands/go.go|Go":                            {removeConsumed, 1, "a key is absorbed into the keyring"},
	"internal/usercommands/lock.go|Lock":                        {removeConsumed, 2, "a key is absorbed into the keyring"},
	"internal/usercommands/unlock.go|Unlock":                    {removeConsumed, 2, "a key is absorbed into the keyring"},
	"modules/cleanup/cleanup.go|CleanupModule.userTrashCommand": {removeConsumed, 1, "trash destroys the item"},
	"modules/cleanup/cleanup.go|CleanupModule.mobTrashCommand":  {removeConsumed, 1, "trash destroys the item"},

	// Out and straight back to the same holder.
	"internal/actions/remove_equip.go|equipItem":           {removeReplaced, 1, "equip moves it from the pack to the body"},
	"internal/usercommands/admin.rename.go|Rename":         {removeReplaced, 1, "admin rename returns the same copy to the pack"},
	"internal/usercommands/admin.redescribe.go|Redescribe": {removeReplaced, 1, "admin redescribe returns the same copy to the pack"},
	"internal/usercommands/guild.go|guildTake":             {removeReplaced, 1, "rollback of a vault take that failed"},
	"internal/usercommands/give.go|giveQuestGearToNPC":     {removeReplaced, 2, "quest gear into an NPC's hands for its tree, and back to the giver when the tree declines"},
	"modules/aicompanion/cooking.go|equipStartingKit":      {removeReplaced, 1, "a companion wears its own starting kit"},

	// Not a character's hands.
	"internal/actions/get.go|TakeFloorItem":                               {removeNotALeave, 1, "the room floor"},
	"internal/actions/search.go|searchWholeRoom":                          {removeNotALeave, 1, "the room floor"},
	"internal/actions/steal.go|stealFromContainer":                        {removeNotALeave, 1, "a container"},
	"internal/actions/steal.go|stealHouseholdBauble":                      {removeNotALeave, 1, "the room floor"},
	"internal/actions/steal.go|takeFromMob":                               {removeNotALeave, 1, "a theft from a mob: involuntary"},
	"internal/actions/steal_pocket.go|pocketAttempt.resolve":              {removeNotALeave, 1, "a bauble picked from a mob's pocket: involuntary"},
	"internal/behaviortree/actions_forager.go|dumpFernwayLoadIntoCrate":   {removeNotALeave, 1, "a forager NPC's cargo"},
	"internal/behaviortree/actions_forager.go|dumpSatchelToLockbox":       {removeNotALeave, 1, "a forager NPC's cargo"},
	"internal/behaviortree/actions_wagon.go|actDistributeCargoToHostiles": {removeNotALeave, 1, "a wagon's cargo"},
	"internal/caravan/cargo_handoff.go|TransferCargoToRunner":             {removeNotALeave, 1, "caravan cargo"},
	"internal/caravan/cargo_handoff.go|TransferAllCargoBack":              {removeNotALeave, 1, "caravan cargo"},
	"internal/caravan/import_arrival.go|handleImportArrival":              {removeNotALeave, 1, "caravan cargo"},
	"internal/caravan/visit.go|VisitVendorsInRoomOpts":                    {removeNotALeave, 1, "caravan cargo"},
	"internal/ferry/factor.go|bankLeftoverCargo":                          {removeNotALeave, 1, "ferry factor cargo"},
	"internal/forager/chest_backfill.go|BackfillVendorFromChests":         {removeNotALeave, 1, "a container"},
	"internal/forager/vendor_sell.go|SellToVendor":                        {removeNotALeave, 1, "a forager NPC's own goods"},
	"internal/hooks/mob_equip_best_floor_item.go|EquipBestFloorItem":      {removeNotALeave, 1, "the room floor"},
	"internal/usercommands/get.go|Get":                                    {removeNotALeave, 2, "a container, or the player's own pet handing back"},
	"internal/usercommands/get.go|takeCorpseItem":                         {removeNotALeave, 1, "a corpse"},
	"internal/usercommands/look.go|Look":                                  {removeNotALeave, 1, "the room floor"},
	"internal/usercommands/look.go|lookRoom":                              {removeNotALeave, 2, "the room floor"},
	"internal/usercommands/loot.go|lootCorpseAll":                         {removeNotALeave, 1, "a corpse"},
	"internal/usercommands/storage.go|storageRemoveQuiet":                 {removeNotALeave, 1, "storage, toward the player"},
	"internal/usercommands/use.go|Use":                                    {removeNotALeave, 1, "a container"},
	"modules/aicompanion/loot.go|mobCompanionLoot":                        {removeNotALeave, 1, "a corpse"},
	"modules/aicompanion/loot.go|mobCompanionTakeout":                     {removeNotALeave, 1, "a container"},
}

// floorAddSites lists every function that puts an item on a room floor
// (a two-argument AddItem), with the same classes and pinned counts. A floor
// drop of a character's own item must fade quest gear first.
var floorAddSites = map[string]removeSite{
	// A held item reaching the floor: quest gear fades first.
	"internal/actions/transfer.go|TransferItemToFloor":              {removeQuestChecked, 1, "drop: VanishQuestGiven first"},
	"internal/actions/remove_equip.go|equipItem":                    {removeQuestChecked, 1, "displaced gear that will not fit the pack: FadeQuestGivenOffHand first"},
	"internal/actions/remove_equip.go|removeWorn":                   {removeQuestChecked, 1, "taken off while overloaded: FadeQuestGivenOffHand first"},
	"internal/hooks/NewRound_AutoHeal.go|ejectSpoiledFromBandolier": {removeQuestChecked, 1, "a spoiled bomb out of the bandolier: FadeQuestGivenOffHand first"},
	"internal/usercommands/get.go|Get":                              {removeQuestChecked, 1, "taken from a pet that cannot hold it back: FadeQuestGivenOffHand first"},
	"internal/usercommands/give.go|giveToPet":                       {removeQuestChecked, 1, "a pet throws a gift down; quest gear is handed back before this"},
	"internal/usercommands/put.go|Put":                              {removeQuestChecked, 1, "container overflow; quest gear faded before it went in"},
	"internal/mobcommands/put.go|Put":                               {removeQuestChecked, 1, "container overflow; quest gear faded before it went in"},
	"internal/usercommands/stash.go|Stash":                          {removeQuestChecked, 1, "the stash; quest gear faded first"},
	"internal/acquire/acquire.go|AcquireOrDrop":                     {removeQuestChecked, 1, "a new item too heavy to carry; a quest hand-out falls marked, the same copy it would be in hand"},

	// Not an item a character was holding.
	"internal/actions/get.go|TakeFloorItem":                          {removeNotALeave, 1, "rollback of a pickup to the floor it came from"},
	"internal/actions/search_bauble.go|BaubleDelivery.deliver":       {removeNotALeave, 3, "a found bauble too heavy to carry"},
	"internal/actions/steal.go|takeFromMob":                          {removeNotALeave, 2, "loot from a theft, too heavy for the thief: involuntary for the mob"},
	"internal/behaviortree/actions_room.go|actSpawnItemInRoom":       {removeNotALeave, 1, "a tree spawns an item"},
	"internal/hooks/Death_MobLoot.go|dropMobLootAndSetCorpse":        {removeNotALeave, 1, "a dead mob's loot: involuntary; a companion's quest copy keeps its mark"},
	"internal/hooks/NewRound_MobRoundTick.go|tickMobCrafting":        {removeNotALeave, 1, "an NPC crafter's output"},
	"internal/hooks/combat_shared_helpers.go|tryWeaponBreak":         {removeNotALeave, 1, "the broken replacement item, new"},
	"internal/hooks/mob_equip_best_floor_item.go|EquipBestFloorItem": {removeNotALeave, 1, "a mob's floor pickup put back"},
	"internal/questengine/bridge.go|GameBridge.SpawnItem":            {removeNotALeave, 1, "a quest spawns an item"},
	"internal/rooms/rooms.go|Room.RepeatSpawnItem":                   {removeNotALeave, 1, "a room spawn"},
	"internal/rooms/rooms.go|Room.UpdateCorpses":                     {removeNotALeave, 1, "a decaying corpse's loot"},
	"internal/rooms/save_and_load.go|SaveRoomTemplate":               {removeNotALeave, 2, "template save restores floor items"},
	"internal/usercommands/admin.item.go|item_Spawn":                 {removeNotALeave, 1, "admin spawn"},
}

// enclosingFuncs maps each line of a file to the name of the top-level
// function or method that holds it.
func enclosingFuncs(t *testing.T, path string) func(line int) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	type span struct {
		from, to int
		name     string
	}
	var spans []span
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			recv := fd.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			if id, ok := recv.(*ast.Ident); ok {
				name = id.Name + "." + name
			}
		}
		spans = append(spans, span{fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line, name})
	}
	return func(line int) string {
		for _, s := range spans {
			if line >= s.from && line <= s.to {
				return s.name
			}
		}
		return "(package level)"
	}
}

// countCallSites counts the calls keep admits, by "file|function".
func countCallSites(t *testing.T, calls []goCall, keep func(goCall) bool) map[string]int {
	t.Helper()
	byFile := map[string]func(int) string{}
	counts := map[string]int{}
	for _, c := range calls {
		if !keep(c) {
			continue
		}
		lookup, ok := byFile[c.file]
		if !ok {
			lookup = enclosingFuncs(t, c.file)
			byFile[c.file] = lookup
		}
		counts[c.file+"|"+lookup(c.line)]++
	}
	return counts
}

// checkClassified compares counted call sites against a site table and
// returns one line per difference.
func checkClassified(counts map[string]int, sites map[string]removeSite) []string {
	var bad []string
	for key, n := range counts {
		site, ok := sites[key]
		switch {
		case !ok:
			bad = append(bad, fmt.Sprintf("%s: %d call(s), not classified", key, n))
		case site.calls != n:
			bad = append(bad, fmt.Sprintf("%s: %d call(s), the entry says %d (a call was added or removed: decide quest gear there, then update the entry)", key, n, site.calls))
		}
	}
	for key, site := range sites {
		if _, ok := counts[key]; !ok {
			bad = append(bad, fmt.Sprintf("%s: no call any more; remove the entry", key))
		}
		switch site.class {
		case removeConsumed, removeReplaced, removeQuestChecked, removeNotALeave:
		default:
			bad = append(bad, fmt.Sprintf("%s: unknown class %q", key, site.class))
		}
		if strings.TrimSpace(site.reason) == "" {
			bad = append(bad, fmt.Sprintf("%s: no reason", key))
		}
	}
	sort.Strings(bad)
	return bad
}

// TestEveryRemoveItemIsClassifiedForQuestGear fails on a RemoveItem call in a
// function removeItemSites does not list, on a listed function whose count
// changed, and on a stale entry.
func TestEveryRemoveItemIsClassifiedForQuestGear(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{"RemoveItem": true})
	if files < 100 || len(calls) < 30 {
		t.Fatalf("scanned %d files and found %d RemoveItem calls: the scan is not reaching the source", files, len(calls))
	}
	counts := countCallSites(t, calls, func(goCall) bool { return true })
	if counts[vanishQuestGivenFile+"|"+vanishQuestGivenFunc] == 0 {
		t.Fatalf("the scan never saw %s in %s: the guard cannot find calls", vanishQuestGivenFunc, vanishQuestGivenFile)
	}
	if bad := checkClassified(counts, removeItemSites); len(bad) > 0 {
		t.Errorf("RemoveItem sites and quest gear (#940): make a way out fade quest gear first "+
			"(actions.VanishQuestGiven), then classify it in removeItemSites:\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestEveryFloorDropIsClassifiedForQuestGear is the same check for every
// two-argument AddItem (Room.AddItem): an item reaching the floor.
func TestEveryFloorDropIsClassifiedForQuestGear(t *testing.T) {
	calls, _ := walkGoCalls(t, map[string]bool{"AddItem": true})
	counts := countCallSites(t, calls, func(c goCall) bool { return len(c.call.Args) == 2 })
	if len(counts) < 10 {
		t.Fatalf("found %d functions putting items on a floor: the scan is not reaching the source", len(counts))
	}
	if bad := checkClassified(counts, floorAddSites); len(bad) > 0 {
		t.Errorf("Room.AddItem sites and quest gear (#940): make a floor drop of a held item fade quest gear first "+
			"(actions.FadeQuestGivenOffHand), then classify it in floorAddSites:\n  %s", strings.Join(bad, "\n  "))
	}
}
