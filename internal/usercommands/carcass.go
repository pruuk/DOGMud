package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Carcass work (wilderness trades, phase 2). `skin` and `butcher` take a
// whole section off a corpse; `harvest` lists what is left, or takes one named
// part at a harder roll and a better grade. Each starts a timed job on the
// Salvaging activity (keyed "harvest:<section>:<mobId>"); the round tick
// hands completion to actions.ResolveHarvest.

// Skin handles `skin <corpse>`.
func Skin(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return startCarcassJob(user, room, strings.TrimSpace(rest), actions.HarvestSkin, ``)
}

// Butcher handles `butcher <corpse>`.
func Butcher(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return startCarcassJob(user, room, strings.TrimSpace(rest), actions.HarvestButcher, ``)
}

// Harvest handles `harvest <corpse>` (list what is left) and
// `harvest <part> from <corpse>` / `harvest <corpse> <part>` (take one part).
func Harvest(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	rest = strings.TrimSpace(rest)
	if rest == `` {
		user.SendText(messaging.CategorySystem, `<ansi fg="command">harvest <corpse></ansi> lists what can still be taken. <ansi fg="command">harvest <part> from <corpse></ansi> takes one part with extra care.`)
		return true, nil
	}

	// "fang from wolf"
	if before, after, found := strings.Cut(strings.ToLower(rest), ` from `); found {
		return harvestPart(user, room, strings.TrimSpace(after), strings.TrimSpace(before))
	}

	// "wolf" or "roe deer": the whole phrase names a corpse. Every word must
	// be in the corpse's name, so "wolf fang" (a part) does not list the
	// wolf, while "roe deer" lists the deer rather than cutting its hide.
	if corpse, ok := room.FindCorpse(rest); ok && phraseNamesCorpse(rest, corpse.Character.Name) {
		return listCarcass(user, room, rest)
	}

	// "wolf fang": the longest leading phrase that names a corpse, the rest a part.
	words := strings.Fields(rest)
	for k := len(words) - 1; k >= 1; k-- {
		corpseName := strings.Join(words[:k], ` `)
		if _, ok := room.FindCorpse(corpseName); ok {
			return harvestPart(user, room, corpseName, strings.Join(words[k:], ` `))
		}
	}
	return listCarcass(user, room, rest)
}

// findCarcass resolves a corpse the user may work, replying and returning
// ok=false when they may not.
func findCarcass(user *users.UserRecord, room *rooms.Room, name string) (rooms.Corpse, species.HarvestTable, species.Size, bool) {
	if name == `` {
		user.SendText(messaging.CategorySystem, `Which corpse?`)
		return rooms.Corpse{}, species.HarvestTable{}, ``, false
	}
	corpse, found := room.FindCorpse(name)
	if !found {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">There's no corpse called "%s" here.</ansi>`, name))
		return rooms.Corpse{}, species.HarvestTable{}, ``, false
	}
	if corpse.MobId <= 0 {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't bring yourself to do that.</ansi>`)
		return rooms.Corpse{}, species.HarvestTable{}, ``, false
	}
	if !corpse.LootAllowed(user.UserId, util.GetRoundCount()) {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">That kill isn't yours to work yet.</ansi>`)
		return rooms.Corpse{}, species.HarvestTable{}, ``, false
	}
	table, size, ok := actions.CarcassTable(corpse)
	if !ok || table.Empty() {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">There's nothing to skin or butcher on the <ansi fg="mobname">%s corpse</ansi>.</ansi>`, corpse.Character.Name))
		return rooms.Corpse{}, species.HarvestTable{}, ``, false
	}
	return corpse, table, size, true
}

func startCarcassJob(user *users.UserRecord, room *rooms.Room, corpseName, section, part string) (bool, error) {
	if !user.Character.IsFree() {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You're already busy working on something.</ansi>`)
		return true, nil
	}
	if !messaging.CanSeeClearly(user.Character, room) {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't see well enough to work a carcass here.</ansi>`)
		return true, nil
	}
	corpse, table, size, ok := findCarcass(user, room, corpseName)
	if !ok {
		return true, nil
	}

	if part == `` && len(actions.SectionEntries(table, &corpse, section)) == 0 {
		msg := fmt.Sprintf(`There's nothing to %s on the <ansi fg="mobname">%s corpse</ansi>.`, section, corpse.Character.Name)
		if section == actions.HarvestSkin && corpse.Skinned {
			msg = fmt.Sprintf(`The <ansi fg="mobname">%s corpse</ansi> has already been skinned.`, corpse.Character.Name)
		} else if section == actions.HarvestButcher && corpse.Butchered {
			msg = fmt.Sprintf(`The <ansi fg="mobname">%s corpse</ansi> has already been butchered.`, corpse.Character.Name)
		}
		user.SendText(messaging.CategorySystem, msg)
		return true, nil
	}

	knife, hasKnife := gather.BestTool(user.Character, items.ToolKnife)
	if !hasKnife {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You need a knife for that. A skinning knife is best, but any one-handed blade will do.</ansi> (<ansi fg="command">help tools</ansi>)`)
		return true, nil
	}

	rounds := gather.Rounds(actions.CarcassRounds(size), knife, true)
	if err := user.Character.Activity.TransitionToSalvaging(
		activity.SalvagingData{
			ItemUuid:    fmt.Sprintf(`%s%s:%d`, actions.HarvestActivityPrefix, section, corpse.MobId),
			RoundsTotal: rounds,
			RoomId:      user.Character.RoomId,
		},
		state.TransitionReason{
			Trigger: activity.TriggerSalvageBegin,
			Actor:   state.ActorRef{UserId: user.UserId},
		},
	); err != nil {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You're already busy working on something.</ansi>`)
		return true, nil
	}
	user.Character.SetMiscData(actions.HarvestRoundKey, int(corpse.RoundCreated))
	if part != `` {
		user.Character.SetMiscData(actions.HarvestPartKey, part)
	} else {
		user.Character.SetMiscData(actions.HarvestPartKey, nil)
	}

	verb := map[string]string{actions.HarvestSkin: `skinning`, actions.HarvestButcher: `butchering`}[section]
	if part != `` {
		verb = `carefully cutting at`
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="yellow">You kneel and begin %s the <ansi fg="mobname">%s corpse</ansi> with your <ansi fg="itemname">%s</ansi>...</ansi>`,
		verb, corpse.Character.Name, knife.Item.DisplayName()))
	return true, nil
}

func harvestPart(user *users.UserRecord, room *rooms.Room, corpseName, partWord string) (bool, error) {
	corpse, table, _, ok := findCarcass(user, room, corpseName)
	if !ok {
		return true, nil
	}
	entry, section, found := actions.FindHarvestPart(table, &corpse, partWord)
	if !found {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">There's no %s left to take from the <ansi fg="mobname">%s corpse</ansi>.</ansi> Try <ansi fg="command">harvest %s</ansi>.`,
			partWord, corpse.Character.Name, strings.ToLower(corpse.Character.Name)))
		return true, nil
	}
	tool, has := gather.BestTool(user.Character, entry.ToolOrDefault())
	if !has {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">You need a %s to take the %s.</ansi> (<ansi fg="command">help tools</ansi>)`,
			strings.ReplaceAll(string(entry.ToolOrDefault()), `_`, ` `), actions.HarvestEntryName(entry)))
		return true, nil
	}
	if entry.MinTool != items.ToolTierNone && tool.Tier < entry.MinTool {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red">Your %s would ruin the %s. It wants a %s %s or better.</ansi> (<ansi fg="command">help tools</ansi>)`,
			tool.Item.NameSimple(), actions.HarvestEntryName(entry), entry.MinTool, actions.ToolName(entry.ToolOrDefault())))
		return true, nil
	}
	return startCarcassJob(user, room, corpseName, section, entry.Key())
}

// listCarcass shows what can still be taken, and with what.
func listCarcass(user *users.UserRecord, room *rooms.Room, corpseName string) (bool, error) {
	corpse, table, _, ok := findCarcass(user, room, corpseName)
	if !ok {
		return true, nil
	}
	lines := []string{fmt.Sprintf(`The <ansi fg="mobname">%s corpse</ansi> could still give:`, corpse.Character.Name)}
	listed := false
	for _, sec := range []string{actions.HarvestSkin, actions.HarvestButcher} {
		for _, e := range actions.SectionEntries(table, &corpse, sec) {
			listed = true
			tool := strings.ReplaceAll(string(e.ToolOrDefault()), `_`, ` `)
			note := ``
			if t, has := gather.BestTool(user.Character, e.ToolOrDefault()); !has {
				note = ` <ansi fg="red">(you have no ` + tool + `)</ansi>`
			} else if e.MinTool != items.ToolTierNone && t.Tier < e.MinTool {
				note = ` <ansi fg="red">(needs a ` + e.MinTool.String() + ` ` + tool + `)</ansi>`
			}
			rare := ``
			if e.Rare {
				rare = ` <ansi fg="yellow">(if you can find it)</ansi>`
			}
			lines = append(lines, fmt.Sprintf(`  <ansi fg="itemname">%s</ansi>: %s, needs a %s%s%s`,
				actions.HarvestEntryName(e), sec, tool, rare, note))
		}
	}
	if !listed {
		lines = append(lines, `  nothing; it has been worked clean.`)
	} else {
		lines = append(lines, `Use <ansi fg="command">skin</ansi> or <ansi fg="command">butcher</ansi> for everything, or <ansi fg="command">harvest <part> from <corpse></ansi> for one part done with extra care.`)
	}
	user.SendText(messaging.CategorySystem, strings.Join(lines, "\n"))
	return true, nil
}

// phraseNamesCorpse reports whether every word of phrase appears among the
// words of a corpse's name (or is the word "corpse").
func phraseNamesCorpse(phrase, corpseName string) bool {
	name := map[string]bool{`corpse`: true}
	for _, w := range strings.Fields(strings.ToLower(corpseName)) {
		name[w] = true
	}
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if !name[w] {
			return false
		}
	}
	return true
}
