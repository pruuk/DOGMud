package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// #940 (owner ruling 2026-11-22): a copy a quest handed out fades whenever it
// leaves its holder's hands (actions.VanishQuestGiven). No single function
// sees every way an item leaves a character, so this guard lists every call to
// a method named RemoveItem in production code, keyed by file and enclosing
// function, and fails on one that is not classified below. A new way out added
// later fails CI here until someone decides what quest gear does there.
//
// The match is syntactic (the method name, not the receiver type), like the
// other call guards in this package, so RemoveItem on a room, a container, a
// corpse, storage or a pet is listed too, as not-a-leave: those take an item
// out of something that is not a character's hands.
//
// Classes:
//   - consumed: the item is used up, eaten, spent or destroyed; nothing
//     leaves to anyone.
//   - replaced-in-place: the item comes out and goes straight back to the same
//     holder (equip, rename, a rollback).
//   - quest-given-checked: a way out that calls actions.VanishQuestGiven (or
//     refuses quest gear) before this call, so quest gear never reaches it.
//   - not-a-leave: the receiver is not a player's or companion's carried
//     inventory, or the call is a mob's own stock handling.

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
	reason string
}

// removeItemSites is keyed "file|function" (a method as Recv.Name). Keyed by
// function, not line, so edits do not break it.
var removeItemSites = map[string]removeSite{
	vanishQuestGivenFile + "|" + vanishQuestGivenFunc: {removeQuestChecked, "the vanish helper itself"},

	// Voluntary ways out: each fades quest gear (or refuses it) first.
	"internal/actions/transfer.go|TransferItemToFloor":           {removeQuestChecked, "drop: fades quest gear before the floor"},
	"internal/actions/transfer.go|TransferItemBetweenChars":      {removeQuestChecked, "give: GiveItemToChar fades it toward another player; usercommands.Give hands it back from an NPC; a tree's return_item and a companion's return keep the mark"},
	"internal/actions/plant.go|plantOnMob":                       {removeQuestChecked, "Plant fades quest gear before dispatch"},
	"internal/actions/plant.go|plantOnPlayer":                    {removeQuestChecked, "Plant fades quest gear before dispatch"},
	"internal/actions/plant.go|plantInContainer":                 {removeQuestChecked, "Plant fades quest gear before dispatch"},
	"internal/actions/sell.go|sellOneToMerchant":                 {removeQuestChecked, "QuoteSale refuses quest gear before the sale"},
	"internal/actions/sell_bauble.go|sellBaubleToMerchant":       {removeQuestChecked, "reached only after QuoteSale, which refuses quest gear"},
	"internal/actions/buy.go|validatePurchase":                   {removeQuestChecked, "trade-in: findTradeInItem never picks quest gear"},
	"internal/usercommands/give.go|giveToPet":                    {removeQuestChecked, "own pet is a companion; another player's pet or no room fades it first"},
	"internal/usercommands/guild.go|guildDonate":                 {removeQuestChecked, "donate fades quest gear first"},
	"internal/usercommands/mail.go|Mail":                         {removeQuestChecked, "quest gear fades instead and the mail goes without it"},
	"internal/usercommands/put.go|Put":                           {removeQuestChecked, "fades quest gear before the container; the second call is container overflow"},
	"internal/usercommands/stash.go|Stash":                       {removeQuestChecked, "stash fades quest gear first"},
	"internal/usercommands/storage.go|storageAddQuiet":           {removeQuestChecked, "storage add and stow fade quest gear first"},
	"internal/mobcommands/put.go|Put":                            {removeQuestChecked, "a companion's put fades quest gear first; the second call is container overflow"},
	"modules/auctions/auctions.go|AuctionsModule.auctionCommand": {removeQuestChecked, "quest gear fades at the listing instead"},

	// The item is used up, spent or destroyed: nobody receives it.
	"internal/actions/combat_reload.go|chamberNextRound":        {removeConsumed, "an emptied ammo bundle"},
	"internal/actions/salvage.go|salvageItem":                   {removeConsumed, "salvage breaks the item down"},
	"internal/behaviortree/actions_quest.go|actTakeItem":        {removeConsumed, "a quest tree takes the item"},
	"internal/conversation/playerstate.go|removeBackpackItem":   {removeConsumed, "a topic's requiresItem takes the item"},
	"internal/dialogue/engine.go|applyQuestEffects":             {removeConsumed, "a topic's requiresItem takes the item"},
	"internal/questengine/bridge.go|GameBridge.ConsumeItem":     {removeConsumed, "a quest step consumes the item"},
	"internal/usercommands/give.go|Give":                        {removeConsumed, "the quest engine consumed a give"},
	"internal/usercommands/go.go|Go":                            {removeConsumed, "a key is absorbed into the keyring"},
	"internal/usercommands/lock.go|Lock":                        {removeConsumed, "a key is absorbed into the keyring"},
	"internal/usercommands/unlock.go|Unlock":                    {removeConsumed, "a key is absorbed into the keyring"},
	"modules/cleanup/cleanup.go|CleanupModule.userTrashCommand": {removeConsumed, "trash destroys the item"},
	"modules/cleanup/cleanup.go|CleanupModule.mobTrashCommand":  {removeConsumed, "trash destroys the item"},

	// Out and straight back to the same holder.
	"internal/actions/remove_equip.go|equipItem":           {removeReplaced, "equip moves it from the pack to the body"},
	"internal/usercommands/admin.rename.go|Rename":         {removeReplaced, "admin rename returns the same copy to the pack"},
	"internal/usercommands/admin.redescribe.go|Redescribe": {removeReplaced, "admin redescribe returns the same copy to the pack"},
	"internal/usercommands/guild.go|guildTake":             {removeReplaced, "rollback of a vault take that failed"},
	"internal/usercommands/give.go|handBackQuestGear":      {removeReplaced, "an NPC hands quest gear back to its giver"},
	"modules/aicompanion/cooking.go|equipStartingKit":      {removeReplaced, "a companion wears its own starting kit"},

	// Not a character's hands: a room, container, corpse, storage or pet, or
	// an NPC's own stock handling, or a theft (involuntary; the copy keeps
	// its mark).
	"internal/actions/get.go|TakeFloorItem":                               {removeNotALeave, "the room floor"},
	"internal/actions/search.go|searchWholeRoom":                          {removeNotALeave, "the room floor"},
	"internal/actions/steal.go|stealFromContainer":                        {removeNotALeave, "a container"},
	"internal/actions/steal.go|stealHouseholdBauble":                      {removeNotALeave, "the room floor"},
	"internal/actions/steal.go|takeFromMob":                               {removeNotALeave, "a theft from a mob: involuntary"},
	"internal/actions/steal_pocket.go|pocketAttempt.resolve":              {removeNotALeave, "a bauble picked from a mob's pocket: involuntary"},
	"internal/behaviortree/actions_forager.go|dumpFernwayLoadIntoCrate":   {removeNotALeave, "a forager NPC's cargo"},
	"internal/behaviortree/actions_forager.go|dumpSatchelToLockbox":       {removeNotALeave, "a forager NPC's cargo"},
	"internal/behaviortree/actions_wagon.go|actDistributeCargoToHostiles": {removeNotALeave, "a wagon's cargo"},
	"internal/caravan/cargo_handoff.go|TransferCargoToRunner":             {removeNotALeave, "caravan cargo"},
	"internal/caravan/cargo_handoff.go|TransferAllCargoBack":              {removeNotALeave, "caravan cargo"},
	"internal/caravan/import_arrival.go|handleImportArrival":              {removeNotALeave, "caravan cargo"},
	"internal/caravan/visit.go|VisitVendorsInRoomOpts":                    {removeNotALeave, "caravan cargo"},
	"internal/ferry/factor.go|bankLeftoverCargo":                          {removeNotALeave, "ferry factor cargo"},
	"internal/forager/chest_backfill.go|BackfillVendorFromChests":         {removeNotALeave, "a container"},
	"internal/forager/vendor_sell.go|SellToVendor":                        {removeNotALeave, "a forager NPC's own goods"},
	"internal/hooks/mob_equip_best_floor_item.go|EquipBestFloorItem":      {removeNotALeave, "the room floor"},
	"internal/usercommands/get.go|Get":                                    {removeNotALeave, "a container, or the player's own pet handing back"},
	"internal/usercommands/get.go|takeCorpseItem":                         {removeNotALeave, "a corpse"},
	"internal/usercommands/look.go|Look":                                  {removeNotALeave, "the room floor"},
	"internal/usercommands/look.go|lookRoom":                              {removeNotALeave, "the room floor"},
	"internal/usercommands/loot.go|lootCorpseAll":                         {removeNotALeave, "a corpse"},
	"internal/usercommands/storage.go|storageRemoveQuiet":                 {removeNotALeave, "storage, toward the player"},
	"internal/usercommands/use.go|Use":                                    {removeNotALeave, "a container"},
	"modules/aicompanion/loot.go|mobCompanionLoot":                        {removeNotALeave, "a corpse"},
	"modules/aicompanion/loot.go|mobCompanionTakeout":                     {removeNotALeave, "a container"},
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

// TestEveryRemoveItemIsClassifiedForQuestGear fails on a RemoveItem call whose
// file and function are not in removeItemSites, and on an entry no call
// matches any more.
func TestEveryRemoveItemIsClassifiedForQuestGear(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{"RemoveItem": true})
	if files < 100 || len(calls) < 30 {
		t.Fatalf("scanned %d files and found %d RemoveItem calls: the scan is not reaching the source", files, len(calls))
	}
	byFile := map[string]func(int) string{}
	seen := map[string]bool{}
	var unclassified []string
	for _, c := range calls {
		lookup, ok := byFile[c.file]
		if !ok {
			lookup = enclosingFuncs(t, c.file)
			byFile[c.file] = lookup
		}
		key := c.file + "|" + lookup(c.line)
		seen[key] = true
		if _, ok := removeItemSites[key]; !ok {
			unclassified = append(unclassified, key+" (line "+strconv.Itoa(c.line)+")")
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Errorf("RemoveItem called where quest gear (#940) has not been decided. Make a way out call "+
			"actions.VanishQuestGiven first, then classify the site in removeItemSites with a reason:\n  %s",
			strings.Join(unclassified, "\n  "))
	}
	if !seen[vanishQuestGivenFile+"|"+vanishQuestGivenFunc] {
		t.Errorf("the scan never saw %s in %s: the guard cannot find calls", vanishQuestGivenFunc, vanishQuestGivenFile)
	}
	var stale []string
	for key := range removeItemSites {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("removeItemSites entries with no RemoveItem call any more; remove them:\n  %s", strings.Join(stale, "\n  "))
	}
	for key, site := range removeItemSites {
		switch site.class {
		case removeConsumed, removeReplaced, removeQuestChecked, removeNotALeave:
		default:
			t.Errorf("%s: unknown class %q", key, site.class)
		}
		if strings.TrimSpace(site.reason) == "" {
			t.Errorf("%s: no reason", key)
		}
	}
}
