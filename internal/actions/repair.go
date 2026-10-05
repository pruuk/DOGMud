package actions

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Repair (wilderness trades). Gear and tools wear (items.Item.Wear) and
// break; `repair` mends them. Two ways:
//
//   - yourself, for free, when you could craft the item: you know a recipe
//     that makes it, your skill meets its minimum, its station is here and you
//     carry its tool (RepairRecipe);
//   - by a merchant of the item's trade standing here, for gold: a smith for
//     metal weapons, armour and tools, a woodworker for bows and staves, a
//     tailor for leather and cloth (RepairDiscipline, RepairCost).

// repairTrades is the order an item's vendor categories are read in when no
// recipe makes it.
var repairTrades = []string{
	shops.CraftSupportBlacksmithing,
	shops.CraftSupportWoodwork,
	shops.CraftSupportTailoring,
	shops.CraftSupportJewelcrafting,
}

// recipesFor lists the recipes whose output is itemId, by recipe id.
func recipesFor(itemId int) []*crafting.RecipeSpec {
	out := []*crafting.RecipeSpec{}
	for _, r := range crafting.GetAll() {
		if r.Output.ItemId == itemId && !crafting.IsEnchantingRecipe(r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].RecipeId < out[b].RecipeId })
	return out
}

// RepairDiscipline is the trade that repairs an item: the skill of a recipe
// that makes it; else woodwork for a bow, crossbow or sling; else the first
// of its vendor categories that is a repair trade; else blacksmithing.
func RepairDiscipline(itm items.Item) string {
	if rs := recipesFor(itm.ItemId); len(rs) > 0 {
		return rs[0].Skill
	}
	spec := itm.GetRawSpec()
	if items.IsShooter(spec) {
		return shops.CraftSupportWoodwork
	}
	for _, trade := range repairTrades {
		for _, c := range spec.VendorCategories {
			if c == trade {
				return trade
			}
		}
	}
	return shops.CraftSupportBlacksmithing
}

// RepairRecipe returns a recipe that lets char repair itm here: one that
// makes it, known, with the skill, the station and the tool. nil when char
// could not craft it here.
func RepairRecipe(char *characters.Character, room *rooms.Room, itm items.Item) *crafting.RecipeSpec {
	if char == nil || room == nil {
		return nil
	}
	for _, r := range recipesFor(itm.ItemId) {
		if !char.HasRecipe(r.RecipeId) {
			continue
		}
		if char.GetSkillLevel(skills.SkillTag(r.Skill)) < r.SkillMinimum {
			continue
		}
		if !StationSatisfied(char, r.Station, room.Station) || !ToolSatisfied(char, r) {
			continue
		}
		return r
	}
	return nil
}

// RepairCost is what a merchant charges to mend itm: its graded value times
// how worn it is times Balance.RepairCostRatio, at least 1 gold.
func RepairCost(itm items.Item) int {
	f := itm.WearFraction()
	if f <= 0 {
		return 0
	}
	ratio := float64(configs.GetBalanceConfig().RepairCostRatio)
	if ratio <= 0 {
		return 0 // RepairCostRatio 0: merchants mend for free
	}
	cost := int(math.Ceil(float64(shops.GradedValue(itm)) * f * ratio))
	if cost < 1 {
		cost = 1
	}
	return cost
}

// Repairer finds a merchant here who works in trade.
func Repairer(room *rooms.Room, trade string) *mobs.Mob {
	if room == nil {
		return nil
	}
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		if m := mobs.GetInstance(mobId); m != nil && m.ShopCraftSupport == trade {
			return m
		}
	}
	return nil
}

// wornItemPtr finds the character's own copy of itm, worn or carried, so
// repair can change it in place.
func wornItemPtr(char *characters.Character, itm items.Item) *items.Item {
	for _, p := range char.Equipment.GetAllItemPtrs() {
		if p.Equals(itm) {
			return p
		}
	}
	for j := range char.Items {
		if char.Items[j].Equals(itm) {
			return &char.Items[j]
		}
	}
	return nil
}

// damagedGear lists the worn and carried items that have any wear.
func damagedGear(char *characters.Character) []items.Item {
	out := []items.Item{}
	for _, p := range char.Equipment.GetAllItemPtrs() {
		if p.Wear > 0 && p.Durability() > 0 {
			out = append(out, *p)
		}
	}
	for _, itm := range char.Items {
		if itm.Wear > 0 && itm.Durability() > 0 {
			out = append(out, itm)
		}
	}
	return out
}

// tradeName is a trade as a player reads it in a repair line.
func tradeName(trade string) string {
	switch trade {
	case shops.CraftSupportBlacksmithing:
		return `a smith`
	case shops.CraftSupportWoodwork:
		return `a woodworker`
	case shops.CraftSupportTailoring:
		return `a tailor`
	case shops.CraftSupportJewelcrafting:
		return `a jeweler`
	}
	return `a ` + trade + ` merchant`
}

// ListRepairs tells the actor what is worn and where it can be mended.
func ListRepairs(actor Actor) {
	char := actor.GetCharacter()
	room := actor.GetRoom()
	gear := damagedGear(char)
	if len(gear) == 0 {
		actor.SendText(messaging.CategorySystem, `Nothing you carry or wear needs mending.`)
		return
	}
	lines := []string{`Your gear in need of mending:`}
	for _, itm := range gear {
		trade := RepairDiscipline(itm)
		how := fmt.Sprintf(`take it to %s`, tradeName(trade))
		if RepairRecipe(char, room, itm) != nil {
			how = `you can mend it yourself here`
		} else if m := Repairer(room, trade); m != nil {
			how = fmt.Sprintf(`<ansi fg="mobname">%s</ansi> will mend it for <ansi fg="gold">%d gold</ansi>`, m.Character.Name, RepairCost(itm))
		}
		lines = append(lines, fmt.Sprintf(`  %s: %s`, itm.DisplayName(), how))
	}
	lines = append(lines, `Type <ansi fg="command">repair <item></ansi> to mend one.`)
	actor.SendText(messaging.CategorySystem, strings.Join(lines, "\n"))
}

// RepairResult is the outcome of a repair.
type RepairResult struct {
	Repaired bool
	Self     bool
	Cost     int
	Reason   string
}

// Repair mends the named item: by the actor's own hand when they could craft
// it here, else by a merchant of its trade here, for gold.
func Repair(actor Actor, name string) RepairResult {
	res := RepairResult{}
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		res.Reason = `no character or room`
		return res
	}
	found, _, ok := char.FindItem(name)
	if !ok {
		actor.SendText(messaging.CategorySystem, `You don't have that.`)
		res.Reason = `no item`
		return res
	}
	itm := wornItemPtr(char, found)
	if itm == nil {
		actor.SendText(messaging.CategorySystem, `You need to be wearing or carrying it, not keeping it in a bag.`)
		res.Reason = `not reachable`
		return res
	}
	if itm.Durability() <= 0 {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(`Your %s is not the kind of thing that wears out.`, itm.DisplayName()))
		res.Reason = `does not wear`
		return res
	}
	if itm.Wear <= 0 {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(`Your %s is in good order already.`, itm.DisplayName()))
		res.Reason = `not worn`
		return res
	}

	// Yourself: you could craft it here.
	if r := RepairRecipe(char, room, *itm); r != nil {
		itm.Repair()
		res.Repaired, res.Self = true, true
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="green">You work the wear out of your <ansi fg="itemname">%s</ansi> the way you would make one new. It is as good as the day it was finished.</ansi>`,
			itm.DisplayName()))
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="username">%s</ansi> sets to work mending a piece of gear.`, char.Name), actor.GetUserId())
		return res
	}

	// A merchant of its trade, for gold.
	trade := RepairDiscipline(*itm)
	m := Repairer(room, trade)
	if m == nil {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`Nobody here mends that. Take your %s to %s, or mend it yourself at the right workbench if you can make one. (<ansi fg="command">help repair</ansi>)`,
			itm.DisplayName(), tradeName(trade)))
		res.Reason = `no repairer`
		return res
	}
	cost := RepairCost(*itm)
	if char.Gold < cost {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> wants <ansi fg="gold">%d gold</ansi> to mend your %s, and you don't have it.`,
			m.Character.Name, cost, itm.DisplayName()))
		res.Reason = `cannot afford`
		res.Cost = cost
		return res
	}
	char.Gold -= cost
	if inv := shops.GetShopInventory(m.Zone, int(m.MobId), m.HomeRoomId); inv != nil {
		inv.Gold += cost
		if err := shops.SaveShop(m.Zone, int(m.MobId), m.HomeRoomId); err != nil {
			mudlog.Error("REPAIR", "msg", "SaveShop failed", "error", err)
		}
	} else {
		m.Character.Gold += cost
	}
	if actor.GetUserId() != 0 {
		events.AddToQueue(events.EquipmentChange{UserId: actor.GetUserId(), GoldChange: -cost})
	}
	itm.Repair()
	res.Repaired, res.Cost = true, cost
	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="green"><ansi fg="mobname">%s</ansi> takes your <ansi fg="itemname">%s</ansi> and <ansi fg="gold">%d gold</ansi>, and hands it back mended.</ansi>`,
		m.Character.Name, itm.DisplayName(), cost))
	return res
}
