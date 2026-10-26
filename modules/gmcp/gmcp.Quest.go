package gmcp

// Build.Quest.* GMCP packages — the server side of the admin web quest
// editor (admin web-building 5c). Handlers take a questDeps so they unit-test
// against fakes; realQuestDeps wires the live quests package, registries, and
// the questengine re-index (one cheap full rebuild after each mutation, so
// the trigger index can never drift from the cache).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// The reserved template range (1000000-generic_quest.yaml) is hidden from
// the editor list; templates still load, round-trip, and are guarded like
// any other quest. The floor itself lives in the quests package, shared
// with id allocation.
const genericQuestIdFloor = quests.ReservedTemplateIdFloor

type questDeps struct {
	load       func(id int) *quests.Quest
	all        func() []quests.Quest
	save       func(q quests.Quest) error
	create     func(name string) (int, error)
	del        func(id int) error
	references func(id int) []questRefEntry
	reindex    func() // rebuild the questengine trigger index
	validators quests.QuestValidators

	// loopWorld builds the loop rules' world over the loaded registries with
	// the given reference checks (buildQuestUpdate passes validators, so they
	// are built once per save), and backlog returns the shipped loop backlog
	// (#375 phase 2 editor door). Either nil skips the loop rules.
	loopWorld func(v quests.QuestValidators) questloop.World
	backlog   func() (questloop.Backlog, error)
}

func realQuestDeps() questDeps {
	return questDeps{
		load: quests.GetQuestById,
		all:  quests.GetAllQuests,
		save: quests.SaveQuest,
		create: func(name string) (int, error) {
			return quests.CreateNewQuestFile(name)
		},
		del:        quests.DeleteQuest,
		references: scanQuestReferences,
		reindex:    questengine.LoadDataFiles,
		validators: questworld.Validators(),
		loopWorld:  questworld.RegistryWith,
		backlog:    questworld.LoadBacklog,
	}
}

// loopVerdict runs the loop rules over the world before and after the edit.
// It refuses only what the edit adds: a finding in the proposed world that
// the current world does not have and the backlog does not list. It warns
// about this quest's other findings (listed, or already there). Findings on
// other quests that the edit did not cause are not this save's business.
func loopVerdict(world questloop.World, q quests.Quest, b questloop.Backlog) (refuse, warn []string) {
	before := map[string]bool{}
	for _, f := range questloop.Validate(world) {
		before[f.Key()] = true
	}
	for _, f := range questloop.Validate(questworld.WithQuest(world, q)) {
		k := f.Key()
		switch {
		case b[k] && f.QuestId == q.QuestId:
			warn = append(warn, "known (loop backlog): "+f.String())
		case before[k] && f.QuestId == q.QuestId:
			warn = append(warn, "known (already in the world, not in the loop backlog): "+f.String())
		case !b[k] && !before[k]:
			refuse = append(refuse, f.String())
		}
	}
	return refuse, warn
}

// ---- payloads ----

type questListRow struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Secret       bool   `json:"secret"`
	Repeatable   bool   `json:"repeatable"`
	StepCount    int    `json:"stepCount"`
	TriggerCount int    `json:"triggerCount"`
}

type questGetReq struct {
	QuestId int    `json:"questId"`
	Name    string `json:"name"`
}

type vocabEntry struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type idName struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type strIdName struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// questTokenEntry is one quest step token ("<questId>-<stepId>") with its
// quest's name, for the quest editor's pickers.
type questTokenEntry struct {
	Token     string `json:"token"`
	QuestName string `json:"questName"`
}

type questEnums struct {
	QuestTokens      []questTokenEntry   `json:"questTokens"`
	FlagKeys         map[string][]string `json:"flagKeys"` // full key -> allowed values
	Events           []vocabEntry        `json:"events"`
	Conditions       []vocabEntry        `json:"conditions"`
	Actions          []vocabEntry        `json:"actions"`
	StatusConditions []idName            `json:"statusConditions"`
	Spells           []strIdName         `json:"spells"`
	Recipes          []string            `json:"recipes"`
	Factions         []strIdName         `json:"factions"`
	Skills           []string            `json:"skills"`
	Stats            []string            `json:"stats"`
}

type questDetail struct {
	Quest quests.Quest `json:"quest"`
	Found bool         `json:"found"`
	Enums questEnums   `json:"enums"`
}

// ---- vocabulary tables (source of truth for the client's pickers) ----

var questEventVocab = []vocabEntry{
	{"room_enter", "player walks into a room (filter: room)"},
	{"room_gate", "player tries to walk into a room; a match REFUSES the step before the move, the actions narrate why (filter: room; in_room is the side the player stands on)"},
	{"item_give", "player gives an item to a mob: fires AFTER the transfer (filters: mob, item)"},
	{"skill_use", "player uses a skill (filter: skill)"},
	{"mob_death", "a mob dies with the player involved (filters: mob, room)"},
	{"command", "a command SUCCEEDED (~27 instrumented commands; filters: command, room)"},
	{"command_issued", "player TYPED a command, success or not: fires from TryCommand for any command (filters: command, room)"},
	{"item_gain", "an item enters the player's inventory (filter: item)"},
	{"quest_granted", "a quest token was just granted: the chaining event (filter: quest_token)"},
	{"room_interact", "player looked at or searched a room feature (filters: room, noun, verb: look or search; empty means look)"},
}

var questConditionVocab = []vocabEntry{
	{"has", "player holds ALL these quest tokens"},
	{"missing", "player holds NONE of these tokens"},
	{"in_room", "player is in this room"},
	{"has_item", "player carries this item"},
	{"missing_item", "player does NOT carry this item"},
	{"has_flag", "every flag key=value matches the player"},
	{"missing_flag", "no flag key=value matches the player"},
	{"has_gold", "player carries at least this much gold"},
	{"has_masterwork", "player carries an own-crafted item at this craft skill or higher"},
}

// The two narrating actions are named `actor` and `observer`, the canonical
// role keys M4b-1 put on internal/quests/triggers.go ActionDef. They are the
// key an author sees in the quest file, so they are the key this editor offers;
// they were `send_text` and `room_text` before the rename.
var questActionVocab = []vocabEntry{
	{"grant", "give the player a quest token (advances the quest)"},
	{"consume_item", "destroy the given item (item_give triggers: keeps the mob from pocketing it)"},
	{"give_item", "hand the player an item"},
	{"give_gold", "hand the player gold"},
	{"charge_gold", "take gold from the player"},
	{"npc_say", "a mob speaks scripted lines (per-line delay/speaker/emote)"},
	{"actor", "message to the player only"},
	{"observer", "message to the whole room"},
	{"spawn_mob", "spawn a mob into a room"},
	{"spawn_item", "spawn an item into a room"},
	{"lock_exits", "lock a room's exits (optionally player-scoped)"},
	{"unlock_exits", "unlock a room's exits"},
	{"teach_spell", "teach the player a spell"},
	{"train_skill", "raise a skill to a level"},
	{"train_stat", "raise a stat by an amount"},
	{"learn_recipe", "grant a crafting recipe"},
	{"apply_condition", "apply a condition to the player"},
	{"teleport", "move the player to a room"},
	{"give_mutation", "roll and grant a random mutation"},
	{"set_flag", "record a quest flag (key+value must be declared)"},
	{"sequence", "timed line sequence with optional on-complete actions and movement lock"},
	{"bump_rep", "adjust faction reputation"},
	{"declare_bounty", "post a bounty (issuer, target, condition, overrides)"},
}

// ---- handlers ----

func buildQuestList(d questDeps) []questListRow {
	rows := []questListRow{}
	for _, q := range d.all() {
		if q.QuestId >= genericQuestIdFloor {
			continue
		}
		rows = append(rows, questListRow{
			Id: q.QuestId, Name: q.Name, Secret: q.Secret, Repeatable: q.Repeatable,
			StepCount: len(q.Steps), TriggerCount: len(q.Triggers),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Id < rows[j].Id })
	return rows
}

func buildQuestGet(d questDeps, questId int) (questDetail, bool) {
	q := d.load(questId)
	if q == nil {
		return questDetail{Found: false, Enums: collectQuestEnums()}, false
	}
	return questDetail{Quest: *q, Found: true, Enums: collectQuestEnums()}, true
}

func buildQuestUpdate(d questDeps, q quests.Quest) BuildResult {
	if d.load(q.QuestId) == nil {
		return buildErr("quest %d not found", q.QuestId)
	}
	if err := q.Validate(); err != nil {
		return buildErr("quest %d refused: %s", q.QuestId, err.Error())
	}
	errs, warns := quests.ValidateQuestRefs(q, d.validators)
	if len(errs) > 0 {
		return buildErr("quest %d refused:\n%s", q.QuestId, strings.Join(errs, "\n"))
	}
	if d.loopWorld != nil && d.backlog != nil {
		b, err := d.backlog()
		if err != nil {
			return buildErr("quest %d refused: cannot read the loop backlog: %s", q.QuestId, err.Error())
		}
		refuse, known := loopVerdict(d.loopWorld(d.validators), q, b)
		if len(refuse) > 0 {
			return buildErr("quest %d refused:\n%s", q.QuestId, strings.Join(refuse, "\n"))
		}
		warns = append(warns, known...)
	}
	if err := d.save(q); err != nil {
		return buildErr("could not save quest %d: %s", q.QuestId, err.Error())
	}
	d.reindex()
	return BuildResult{Ok: true, QuestId: q.QuestId,
		Message: fmt.Sprintf("quest %d saved: live immediately", q.QuestId), Warnings: warns}
}

func buildQuestCreate(d questDeps, name string) BuildResult {
	id, err := d.create(name)
	if err != nil {
		return buildErr("%s", err.Error())
	}
	d.reindex()
	return BuildResult{Ok: true, QuestId: id, Message: fmt.Sprintf("quest %d created", id)}
}

func buildQuestDelete(d questDeps, questId int) BuildResult {
	if d.load(questId) == nil {
		return buildErr("quest %d not found", questId)
	}
	if refs := d.references(questId); len(refs) > 0 {
		lines := make([]string, 0, len(refs))
		for _, r := range refs {
			lines = append(lines, fmt.Sprintf("%s: %s: %s", r.Kind, r.Where, r.Detail))
		}
		return BuildResult{Ok: false, QuestId: questId, QuestRefs: refs,
			Error: fmt.Sprintf("quest %d is still referenced:\n%s", questId, strings.Join(lines, "\n"))}
	}
	if err := d.del(questId); err != nil {
		return buildErr("could not delete quest %d: %s", questId, err.Error())
	}
	d.reindex()
	return BuildResult{Ok: true, QuestId: questId,
		Message: fmt.Sprintf("quest %d deleted", questId)}
}

// ---- enums ----

func collectQuestEnums() questEnums {
	e := questEnums{
		FlagKeys:   quests.GetFlagRegistry(),
		Events:     questEventVocab,
		Conditions: questConditionVocab,
		Actions:    questActionVocab,
		Stats:      []string{"strength", "dexterity", "perception", "vitality", "willpower", "charisma"},
	}
	for _, q := range quests.GetAllQuests() {
		for _, step := range q.Steps {
			e.QuestTokens = append(e.QuestTokens, questTokenEntry{
				Token:     fmt.Sprintf("%d-%s", q.QuestId, step.Id),
				QuestName: q.Name,
			})
		}
	}
	sort.Slice(e.QuestTokens, func(i, j int) bool { return e.QuestTokens[i].Token < e.QuestTokens[j].Token })
	for _, id := range conditions.GetAllConditionIds() {
		if spec := conditions.GetConditionSpec(id); spec != nil {
			e.StatusConditions = append(e.StatusConditions, idName{Id: id, Name: spec.Name})
		}
	}
	sort.Slice(e.StatusConditions, func(i, j int) bool { return e.StatusConditions[i].Id < e.StatusConditions[j].Id })
	for id, sd := range spells.GetAllSpells() {
		e.Spells = append(e.Spells, strIdName{Id: id, Name: sd.Name})
	}
	sort.Slice(e.Spells, func(i, j int) bool { return e.Spells[i].Id < e.Spells[j].Id })
	for id := range crafting.GetAll() {
		e.Recipes = append(e.Recipes, id)
	}
	sort.Strings(e.Recipes)
	for _, def := range factions.AllDefinitions() {
		e.Factions = append(e.Factions, strIdName{Id: def.FactionId, Name: def.DisplayName})
	}
	sort.Slice(e.Factions, func(i, j int) bool { return e.Factions[i].Id < e.Factions[j].Id })
	for _, s := range skills.GetAllSkillNames() {
		e.Skills = append(e.Skills, string(s))
	}
	sort.Strings(e.Skills)
	return e
}
