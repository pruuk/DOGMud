package questengine

import "github.com/GoMudEngine/GoMud/internal/quests"

// The definition types live in internal/quests — the single owner of the
// quest file parse (5c-pre unification). These aliases keep questengine's
// API and its evaluation tests source-compatible. Evaluation logic stays in
// this package; only the data shapes moved.
type (
	QuestFlagDef       = quests.QuestFlagDef
	QuestDef           = quests.Quest
	QuestStep          = quests.QuestStep
	QuestRewards       = quests.QuestReward
	TriggerDef         = quests.TriggerDef
	Conditions         = quests.Conditions
	ActionDef          = quests.ActionDef
	BumpRepDef         = quests.BumpRepDef
	DeclareBountyDef   = quests.DeclareBountyDef
	QuestFlagAction    = quests.QuestFlagAction
	NpcSayDef          = quests.NpcSayDef
	SayLineDef         = quests.SayLineDef
	SpawnDef           = quests.SpawnDef
	ExitLock           = quests.ExitLock
	SkillDef           = quests.SkillDef
	StatDef            = quests.StatDef
	RecipeDef          = quests.RecipeDef
	StatusConditionDef = quests.StatusConditionDef
	SequenceDef        = quests.SequenceDef
)

// EventDetails carries context about the event that triggered evaluation.
// It describes an engine INVOCATION, not a quest definition, so it stays here.
//
// Noun is what the player typed, lowercased. RoomNoun is the room's own noun
// key that Noun names (an alias, a word of a multi-word name or a
// singular/plural spelling resolve to it; rooms.Room.NounNamedBy), or "" when
// the typed words name no room noun. A trigger noun matches RoomNoun only when
// no trigger matches the typed text (see evaluate).
type EventDetails struct {
	UserId     int
	RoomId     int
	MobId      int
	ItemId     int
	Skill      string
	Command    string
	Noun       string
	RoomNoun   string
	Verb       string
	QuestToken string
	TargetKind string // command events: what the command was aimed at, a quests.Target* kind (#375 3b)
	Effect     string // spell_landed: the spell's effect_type (#375 3b)
}

// NotifyResult is returned by Notify to inform the caller how the event was handled.
type NotifyResult struct {
	Handled     bool // at least one trigger matched and executed
	ConsumeItem bool // the item should be consumed (not transferred to mob)
}
