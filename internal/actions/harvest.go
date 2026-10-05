package actions

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gather"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Carcass work (wilderness trades, phase 2): skin, butcher, and harvest one
// named part. The commands in usercommands start a timed activity; when it
// completes, the round tick calls ResolveHarvest, which does all the work
// here.
//
// What a carcass gives comes from its species `harvest:` table merged with
// the mob's own (mobs.ResolveHarvest). How WELL it comes off is one
// gather.Roll for the job: the grade, capped by the knife, and further capped
// per entry by the tool that entry needs (a cleaver for bone, a bone saw for
// horn and antler). Rare parts are noticed on a Perception roll. A carcass
// that has gone stale gives one grade worse and, late enough, no meat.

// HarvestActivityPrefix keys a carcass job on the Salvaging activity:
// "harvest:<section>:<mobId>". MiscData carries the corpse's RoundCreated and
// the targeted part, if any, for the resolver.
const (
	HarvestActivityPrefix = `harvest:`
	HarvestRoundKey       = `harvest_corpse_round_created`
	HarvestPartKey        = `harvest_part`
)

// Harvest sections.
const (
	HarvestSkin    = `skin`
	HarvestButcher = `butcher`
)

// HarvestOptions names the carcass and what to take from it.
type HarvestOptions struct {
	MobId        int
	RoundCreated uint64
	Section      string // HarvestSkin or HarvestButcher
	Part         string // non-empty: take only this entry (HarvestEntry.Key), at +difficulty and +1 grade
}

// HarvestResult is the outcome of one completed carcass job.
type HarvestResult struct {
	Reason     string // non-empty when nothing was rolled
	Roll       gather.Result
	Taken      []items.Item
	Missed     []string // entry display names skipped for want of the right tool
	TooPoor    []string // parts that were there but wanted a better tool than the one at hand
	HideRuined bool     // butchering an unskinned carcass ruined its hide
}

// HarvestTake is one entry that will come off the carcass, with its final
// quantity and grade.
type HarvestTake struct {
	Entry species.HarvestEntry
	Qty   int
	Grade items.Quality
}

// CarcassTable is the merged harvest table and body size for a corpse's mob,
// or ok=false for a player corpse or an unknown mob.
func CarcassTable(c rooms.Corpse) (table species.HarvestTable, size species.Size, ok bool) {
	if c.MobId <= 0 {
		return species.HarvestTable{}, ``, false
	}
	spec := mobs.GetMobSpec(mobs.MobId(c.MobId))
	if spec == nil {
		return species.HarvestTable{}, ``, false
	}
	size = species.Medium
	if sp := species.GetSpecies(c.Character.SpeciesId); sp != nil && sp.Size != `` {
		size = sp.Size
	}
	return mobs.ResolveHarvest(spec), size, true
}

// SectionEntries returns the entries of one section still on the carcass.
func SectionEntries(table species.HarvestTable, c *rooms.Corpse, section string) []species.HarvestEntry {
	var src []species.HarvestEntry
	switch section {
	case HarvestSkin:
		if c.Skinned {
			return nil
		}
		src = table.Skin
	case HarvestButcher:
		if c.Butchered {
			return nil
		}
		src = table.Butcher
	}
	out := []species.HarvestEntry{}
	for _, e := range src {
		if !c.PartTaken(e.Key()) {
			out = append(out, e)
		}
	}
	return out
}

// HarvestEntrySpec is the item spec an entry produces: its ItemId when set,
// else the cheapest item carrying its tag.
func HarvestEntrySpec(e species.HarvestEntry) *items.ItemSpec {
	if e.ItemId > 0 {
		return items.GetItemSpec(e.ItemId)
	}
	return items.FindSpecByComponentTag(e.Item)
}

// HarvestEntryName is the player-facing name of what an entry produces.
func HarvestEntryName(e species.HarvestEntry) string {
	if spec := HarvestEntrySpec(e); spec != nil {
		return strings.ToLower(spec.Name)
	}
	return strings.ReplaceAll(e.Key(), `-`, ` `)
}

// FindHarvestPart matches a player's word ("fang", "antler", "pelt") against
// the entries still on the carcass, in both sections. Matches the tag first,
// then any word of the item name.
func FindHarvestPart(table species.HarvestTable, c *rooms.Corpse, word string) (species.HarvestEntry, string, bool) {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == `` {
		return species.HarvestEntry{}, ``, false
	}
	type cand struct {
		e   species.HarvestEntry
		sec string
	}
	var all []cand
	for _, sec := range []string{HarvestSkin, HarvestButcher} {
		for _, e := range SectionEntries(table, c, sec) {
			all = append(all, cand{e, sec})
		}
	}
	singular := strings.TrimSuffix(word, `s`)
	for _, x := range all {
		if strings.EqualFold(x.e.Key(), word) || strings.EqualFold(x.e.Key(), singular) {
			return x.e, x.sec, true
		}
	}
	for _, x := range all {
		name := HarvestEntryName(x.e)
		if name == word || strings.HasPrefix(name, word) {
			return x.e, x.sec, true
		}
		for _, w := range strings.Fields(name) {
			ws := strings.TrimSuffix(w, `s`)
			if w == word || w == singular || ws == word || ws == singular {
				return x.e, x.sec, true
			}
		}
	}
	return species.HarvestEntry{}, ``, false
}

// CarcassDifficulty is the target's side of a carcass roll: the mob's
// statpool (its power budget) and body size.
func CarcassDifficulty(statPool int, size species.Size) float64 {
	b := configs.GetBalanceConfig()
	d := float64(statPool)*float64(b.GatherStatPoolDifficulty) - float64(b.GatherCarcassEase)
	switch size {
	case species.Medium:
		d += float64(b.GatherSizeDifficultyMedium)
	case species.Large:
		d += float64(b.GatherSizeDifficultyLarge)
	}
	return d
}

// CarcassRounds is how long a carcass job takes before the tool's speed.
func CarcassRounds(size species.Size) int {
	b := configs.GetBalanceConfig()
	switch size {
	case species.Small:
		return int(b.GatherJobRoundsSmall)
	case species.Large:
		return int(b.GatherJobRoundsLarge)
	}
	return int(b.GatherJobRoundsMedium)
}

// JobForSection is the gather job behind a section.
func JobForSection(section string) gather.Job {
	if section == HarvestButcher {
		return gather.JobButcher
	}
	return gather.JobSkin
}

// planInputs are everything planHarvest needs, so it can be tested without
// a world.
type planInputs struct {
	Entries     []species.HarvestEntry
	Size        species.Size
	Grade       items.Quality // the job roll's grade (already capped by the knife)
	Staleness   float64
	BonusUnits  int                                         // extra units on the first non-rare entry
	Perception  int                                         // for rare parts
	ToolTier    func(items.ToolType) (items.ToolTier, bool) // best tool the gatherer has of a type
	Rand        func() float64                              // 0..1
	IsPerishing func(species.HarvestEntry) bool             // meat or organ: lost when the carcass is late
}

// planHarvest decides what comes off: per entry, whether the right tool is
// at hand, whether a rare part was noticed, the quantity and the grade.
func planHarvest(in planInputs) (takes []HarvestTake, missed []species.HarvestEntry) {
	b := configs.GetBalanceConfig()
	grade := in.Grade
	if in.Staleness >= float64(b.CorpseStaleGradeAt) && grade > items.QualityCrude {
		grade--
	}
	meatLost := in.Staleness >= float64(b.CorpseMeatLostAt)

	bonusGiven := false
	for _, e := range in.Entries {
		if meatLost && in.IsPerishing != nil && in.IsPerishing(e) {
			continue
		}
		tier, ok := in.ToolTier(e.ToolOrDefault())
		if !ok {
			missed = append(missed, e)
			continue
		}
		if e.Rare {
			base := e.Chance
			if base <= 0 {
				base = float64(b.GatherRareBaseChance)
			}
			if base <= 0 {
				continue // GatherRareBaseChance 0: rare parts are off
			}
			// Perception notices the part; a better tool gets it off whole.
			chance := base * float64(in.Perception) / 100.0 * gather.RareMult(tier)
			chance = math.Max(0.02, math.Min(0.9, chance))
			if in.Rand() >= chance {
				continue
			}
		}
		// A part that wants a better tool than this one (a trophy pelt wants
		// a steel edge) stays on the carcass. A rare part is checked only
		// once it was there to be noticed, so the miss is real.
		if e.MinTool != items.ToolTierNone && tier < e.MinTool {
			missed = append(missed, e)
			continue
		}
		qty := species.ScaleHarvestQty(e.Qty, in.Size)
		if !e.Rare && !bonusGiven {
			qty += in.BonusUnits
			bonusGiven = true
		}
		g := grade
		if maxG := tier.MaxGrade(); g > maxG {
			g = maxG
		}
		takes = append(takes, HarvestTake{Entry: e, Qty: qty, Grade: g.Clamp()})
	}
	return takes, missed
}

// StatBonusUnits is the extra yield from a strong (or deft) gatherer: one
// unit per Balance.GatherStatPerBonusUnit points above 100.
func StatBonusUnits(stat int) int {
	per := int(configs.GetBalanceConfig().GatherStatPerBonusUnit)
	if per <= 0 || stat <= 100 {
		return 0
	}
	return (stat - 100) / per
}

// ResolveHarvest completes a carcass job for actor. It finds the corpse the
// job started on, rolls, creates and stores the materials, marks the carcass
// and narrates.
func ResolveHarvest(actor Actor, opts HarvestOptions) HarvestResult {
	res := HarvestResult{}
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		res.Reason = `no character or room`
		return res
	}

	idx := -1
	for i, c := range room.Corpses {
		if !c.Prunable && c.MobId == opts.MobId && c.RoundCreated == opts.RoundCreated {
			idx = i
			break
		}
	}
	if idx < 0 {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">The carcass you were working on is gone.</ansi>`)
		res.Reason = `no corpse`
		return res
	}
	corpse := &room.Corpses[idx]
	carcassName := corpse.Character.Name

	table, size, ok := CarcassTable(*corpse)
	if !ok {
		res.Reason = `no table`
		return res
	}

	section := opts.Section
	var entries []species.HarvestEntry
	if opts.Part != `` {
		for _, e := range SectionEntries(table, corpse, section) {
			if e.Key() == opts.Part {
				entries = []species.HarvestEntry{e}
				break
			}
		}
	} else {
		entries = SectionEntries(table, corpse, section)
	}
	if len(entries) == 0 {
		actor.SendText(messaging.CategoryError, fmt.Sprintf(
			`<ansi fg="red">There is nothing left to %s on the <ansi fg="mobname">%s corpse</ansi>.</ansi>`, section, carcassName))
		res.Reason = `nothing left`
		return res
	}

	bal := configs.GetBalanceConfig()
	job := JobForSection(section)
	statPool := 0
	if spec := mobs.GetMobSpec(mobs.MobId(corpse.MobId)); spec != nil {
		statPool = spec.StatPool
	}
	target := CarcassDifficulty(statPool, size)
	if opts.Part != `` {
		target += float64(bal.GatherTargetedDifficulty)
	}
	roll := gather.Roll(char, room, job, target)
	res.Roll = roll
	if roll.NoTool {
		actor.SendText(messaging.CategoryError, `<ansi fg="red">You need a knife for that.</ansi>`)
		res.Reason = `no tool`
		return res
	}

	now := util.GetRoundCount()
	staleness := corpse.Staleness(now, configs.GetGamePlayConfig().Death.CorpseDecayTime.String())

	var takes []HarvestTake
	var missed []species.HarvestEntry
	if roll.Success {
		grade := roll.Grade
		if opts.Part != `` && grade < items.QualityPristine {
			grade++
		}
		bonus := 0
		if section == HarvestButcher && opts.Part == `` {
			bonus = StatBonusUnits(char.GetStatValue(job.StatA))
		}
		takes, missed = planHarvest(planInputs{
			Entries:    entries,
			Size:       size,
			Grade:      grade,
			Staleness:  staleness,
			BonusUnits: bonus,
			Perception: char.GetStatValue(`perception`),
			ToolTier: func(t items.ToolType) (items.ToolTier, bool) {
				tool, ok := gather.BestTool(char, t)
				return tool.Tier, ok
			},
			Rand: func() float64 { return float64(util.Rand(10000)) / 10000.0 },
			IsPerishing: func(e species.HarvestEntry) bool {
				spec := HarvestEntrySpec(e)
				return spec != nil && spec.SpoilAfter != `` && section == HarvestButcher
			},
		})
	}

	// Mark the carcass. A failed attempt still spends what it was aimed at:
	// a botched skinning tears the hide, a botched butchering mangles the
	// meat. Missed entries (no cleaver, no bone saw) stay for someone better
	// equipped only when the job was a targeted part; a full butchering
	// leaves the carcass behind it.
	switch {
	case opts.Part != ``:
		corpse.HarvestedParts = append(corpse.HarvestedParts, opts.Part)
	case section == HarvestSkin:
		corpse.Skinned = true
	case section == HarvestButcher:
		corpse.Butchered = true
		if !corpse.Skinned && len(table.Skin) > 0 {
			corpse.Skinned = true
			res.HideRuined = true
		}
	}

	// Make and store the goods.
	for _, t := range takes {
		spec := HarvestEntrySpec(t.Entry)
		if spec == nil {
			continue
		}
		for n := 0; n < t.Qty; n++ {
			itm := items.New(spec.ItemId)
			if !itm.IsValid() {
				continue
			}
			itm.Quality = t.Grade
			itm.CraftedRound = now
			if !char.StoreItem(itm) {
				room.AddItem(itm, false)
			} else if actor.GetUserId() != 0 {
				events.AddToQueue(events.ItemOwnership{UserId: actor.GetUserId(), Item: itm, Gained: true})
			}
			res.Taken = append(res.Taken, itm)
		}
	}
	for _, e := range missed {
		name := HarvestEntryName(e)
		if e.MinTool != items.ToolTierNone {
			if t, ok := gather.BestTool(char, e.ToolOrDefault()); ok && t.Tier < e.MinTool {
				res.TooPoor = append(res.TooPoor, fmt.Sprintf(`%s (it wants a %s %s or better)`,
					name, e.MinTool, ToolName(e.ToolOrDefault())))
				continue
			}
		}
		res.Missed = append(res.Missed, name)
	}

	// One award per job, won on whether anything came off.
	actor.AwardResolved(len(res.Taken) > 0, char.CandidateFor(string(skills.Salvage)))

	if corpse.Spent() && !corpse.HasLoot() {
		room.RemoveCorpse(*corpse)
	}

	narrateHarvest(actor, room, carcassName, section, opts.Part != ``, res)

	// Wear: the job's own tool, plus each other tool a part needed.
	WearUsedTool(actor, roll.Tool, roll.HasTool)
	worn := map[items.ToolType]bool{job.Tool: true}
	for _, t := range takes {
		tt := t.Entry.ToolOrDefault()
		if worn[tt] {
			continue
		}
		worn[tt] = true
		if tool, ok := gather.BestTool(char, tt); ok {
			WearUsedTool(actor, tool, true)
		}
	}
	return res
}

// narrateHarvest tells the worker what came off and the room what they did.
func narrateHarvest(actor Actor, room *rooms.Room, carcass, section string, targeted bool, res HarvestResult) {
	verb := `skin`
	if section == HarvestButcher {
		verb = `butcher`
	}
	if targeted {
		verb = `cut a part from`
	}

	actorLine := messaging.NoLine
	if actor.IsPlayer() {
		if len(res.Taken) > 0 {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="green">You %s the <ansi fg="mobname">%s corpse</ansi> and take: %s.</ansi>`,
				verb, carcass, summarizeTaken(res.Taken)))
		} else if res.Roll.Success {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="yellow">You %s the <ansi fg="mobname">%s corpse</ansi> but get nothing worth keeping.</ansi>`, verb, carcass))
		} else {
			actorLine = messaging.Say(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="red">Your knife slips and you ruin the job on the <ansi fg="mobname">%s corpse</ansi>.</ansi>`, carcass))
		}
	}

	messaging.SendTrio(messaging.Trio{
		Actor: actorLine,
		Actee: messaging.NoLine,
		Observer: messaging.Say(messaging.CategoryMobIdle, fmt.Sprintf(
			`<ansi fg="username">%s</ansi> kneels over the <ansi fg="mobname">%s corpse</ansi>, working at it with a blade.`,
			actor.GetName(), carcass)),
	}, messaging.Audience{
		Actor:     actor,
		ActorId:   actor.GetUserId(),
		ActorName: actor.GetName(),
		ActeeName: messaging.NoName,
		Room:      room,
	})

	if actor.IsPlayer() {
		if res.HideRuined {
			actor.SendText(messaging.CategorySystem, `<ansi fg="yellow">Butchering it before skinning has ruined the hide.</ansi>`)
		}
		if len(res.Missed) > 0 {
			actor.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="yellow">Without the right tool you leave behind: %s. (<ansi fg="command">help tools</ansi>)</ansi>`,
				strings.Join(res.Missed, `, `)))
		}
		if len(res.TooPoor) > 0 {
			actor.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="yellow">There was a prize here your tools could not take whole: %s. (<ansi fg="command">help tools</ansi>)</ansi>`,
				strings.Join(res.TooPoor, `, `)))
		}
	}
}

// summarizeTaken groups identical results: "2x raw meat (fine), 1x bone".
func summarizeTaken(taken []items.Item) string {
	type key struct {
		id int
		q  items.Quality
	}
	order := []key{}
	counts := map[key]int{}
	names := map[key]string{}
	for _, it := range taken {
		k := key{it.ItemId, it.Quality}
		if _, ok := counts[k]; !ok {
			order = append(order, k)
			names[k] = it.DisplayName()
		}
		counts[k]++
	}
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, fmt.Sprintf(`%dx <ansi fg="itemname">%s</ansi>`, counts[k], names[k]))
	}
	return strings.Join(parts, `, `)
}

// SpoiledItems lists the spoiled raw goods a character carries at round now.
func SpoiledItems(c *characters.Character, now uint64) []items.Item {
	var out []items.Item
	for _, pool := range [][]items.Item{c.Items, c.ComponentItems} {
		for i := range pool {
			if pool[i].IsSpoiled(now) {
				out = append(out, pool[i])
			}
		}
	}
	return out
}

// ResolveHarvestJob is called by the round tick when a carcass job on the
// Salvaging activity finishes. key is the activity's ItemUuid with
// HarvestActivityPrefix removed ("<section>:<mobId>").
func ResolveHarvestJob(actor Actor, key string) HarvestResult {
	char := actor.GetCharacter()
	section, mobIdStr, _ := strings.Cut(key, `:`)
	var mobId int
	fmt.Sscanf(mobIdStr, `%d`, &mobId)
	roundCreated, _ := char.GetMiscData(HarvestRoundKey).(int)
	part, _ := char.GetMiscData(HarvestPartKey).(string)
	char.SetMiscData(HarvestRoundKey, nil)
	char.SetMiscData(HarvestPartKey, nil)
	return ResolveHarvest(actor, HarvestOptions{
		MobId:        mobId,
		RoundCreated: uint64(roundCreated),
		Section:      section,
		Part:         part,
	})
}
