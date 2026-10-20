package main

import "github.com/GoMudEngine/GoMud/internal/dialogue"

// The pre-topics dialogue format, copied from internal/dialogue/types.go
// (master 243b7fb39) so this one-time converter keeps building after Task 21
// deletes the originals. Field tags are unchanged.

// legacyPattern is a single keyword-triggered response rule in a dialogue file.
type legacyPattern struct {
	Keywords           []string               `yaml:"keywords" json:"keywords,omitempty"`
	Moods              []string               `yaml:"moods,omitempty" json:"moods,omitempty"`
	Responses          []string               `yaml:"responses" json:"responses,omitempty"`
	MoodChange         string                 `yaml:"moodChange,omitempty" json:"moodChange,omitempty"`
	QuestRequired      []string               `yaml:"questRequired,omitempty" json:"questRequired,omitempty"`
	QuestExcluded      []string               `yaml:"questExcluded,omitempty" json:"questExcluded,omitempty"`
	GrantsQuest        string                 `yaml:"grantsQuest,omitempty" json:"grantsQuest,omitempty"`
	RequiresItem       int                    `yaml:"requiresItem,omitempty" json:"requiresItem,omitempty"`
	ItemExcluded       int                    `yaml:"itemExcluded,omitempty" json:"itemExcluded,omitempty"`
	GivesItem          int                    `yaml:"givesItem,omitempty" json:"givesItem,omitempty"`
	QuestFlagRequired  map[string]string      `yaml:"questFlagRequired,omitempty" json:"questFlagRequired,omitempty"`
	QuestFlagExcluded  map[string]string      `yaml:"questFlagExcluded,omitempty" json:"questFlagExcluded,omitempty"`
	SetsQuestFlag      *dialogue.QuestFlagSet `yaml:"setsQuestFlag,omitempty" json:"setsQuestFlag,omitempty"`
	BumpsRep           []dialogue.RepBump     `yaml:"bumpsRep,omitempty" json:"bumpsRep,omitempty"`
	GivesGold          int                    `yaml:"givesGold,omitempty" json:"givesGold,omitempty"`
	ChargesGold        int                    `yaml:"chargesGold,omitempty" json:"chargesGold,omitempty"`
	MasterworkRequired int                    `yaml:"masterworkRequired,omitempty" json:"masterworkRequired,omitempty"`
	GoldRequired       int                    `yaml:"goldRequired,omitempty" json:"goldRequired,omitempty"`
}

// legacyTreeNode is a stateful conversation node gated by triggers and unlock requirements.
type legacyTreeNode struct {
	Id                 string                 `yaml:"id" json:"id,omitempty"`
	Triggers           []string               `yaml:"triggers" json:"triggers,omitempty"`
	Requires           []string               `yaml:"requires,omitempty" json:"requires,omitempty"`
	Text               string                 `yaml:"text" json:"text,omitempty"`
	Hints              string                 `yaml:"hints,omitempty" json:"hints,omitempty"`
	Unlocks            []string               `yaml:"unlocks,omitempty" json:"unlocks,omitempty"`
	MoodChange         string                 `yaml:"moodChange,omitempty" json:"moodChange,omitempty"`
	QuestRequired      []string               `yaml:"questRequired,omitempty" json:"questRequired,omitempty"`
	QuestExcluded      []string               `yaml:"questExcluded,omitempty" json:"questExcluded,omitempty"`
	GrantsQuest        string                 `yaml:"grantsQuest,omitempty" json:"grantsQuest,omitempty"`
	RequiresItem       int                    `yaml:"requiresItem,omitempty" json:"requiresItem,omitempty"`
	ItemExcluded       int                    `yaml:"itemExcluded,omitempty" json:"itemExcluded,omitempty"`
	GivesItem          int                    `yaml:"givesItem,omitempty" json:"givesItem,omitempty"`
	QuestFlagRequired  map[string]string      `yaml:"questFlagRequired,omitempty" json:"questFlagRequired,omitempty"`
	QuestFlagExcluded  map[string]string      `yaml:"questFlagExcluded,omitempty" json:"questFlagExcluded,omitempty"`
	SetsQuestFlag      *dialogue.QuestFlagSet `yaml:"setsQuestFlag,omitempty" json:"setsQuestFlag,omitempty"`
	BumpsRep           []dialogue.RepBump     `yaml:"bumpsRep,omitempty" json:"bumpsRep,omitempty"`
	GivesGold          int                    `yaml:"givesGold,omitempty" json:"givesGold,omitempty"`
	ChargesGold        int                    `yaml:"chargesGold,omitempty" json:"chargesGold,omitempty"`
	MasterworkRequired int                    `yaml:"masterworkRequired,omitempty" json:"masterworkRequired,omitempty"`
	GoldRequired       int                    `yaml:"goldRequired,omitempty" json:"goldRequired,omitempty"`
}

// legacyQuestGreeting is an alternative greeting shown when the player matches quest conditions.
type legacyQuestGreeting struct {
	QuestRequired      []string               `yaml:"questRequired,omitempty" json:"questRequired,omitempty"`
	QuestExcluded      []string               `yaml:"questExcluded,omitempty" json:"questExcluded,omitempty"`
	QuestFlagRequired  map[string]string      `yaml:"questFlagRequired,omitempty" json:"questFlagRequired,omitempty"`
	QuestFlagExcluded  map[string]string      `yaml:"questFlagExcluded,omitempty" json:"questFlagExcluded,omitempty"`
	Text               string                 `yaml:"text" json:"text,omitempty"`
	Hints              string                 `yaml:"hints,omitempty" json:"hints,omitempty"`
	GrantsQuest        string                 `yaml:"grantsQuest,omitempty" json:"grantsQuest,omitempty"`
	GivesItem          int                    `yaml:"givesItem,omitempty" json:"givesItem,omitempty"`
	RequiresItem       int                    `yaml:"requiresItem,omitempty" json:"requiresItem,omitempty"`
	ItemExcluded       int                    `yaml:"itemExcluded,omitempty" json:"itemExcluded,omitempty"`
	SetsQuestFlag      *dialogue.QuestFlagSet `yaml:"setsQuestFlag,omitempty" json:"setsQuestFlag,omitempty"`
	BumpsRep           []dialogue.RepBump     `yaml:"bumpsRep,omitempty" json:"bumpsRep,omitempty"`
	GivesGold          int                    `yaml:"givesGold,omitempty" json:"givesGold,omitempty"`
	ChargesGold        int                    `yaml:"chargesGold,omitempty" json:"chargesGold,omitempty"`
	MasterworkRequired int                    `yaml:"masterworkRequired,omitempty" json:"masterworkRequired,omitempty"`
	GoldRequired       int                    `yaml:"goldRequired,omitempty" json:"goldRequired,omitempty"`
}

// legacyTreeRoot holds the greeting delivered when a player first uses 'talk'.
type legacyTreeRoot struct {
	Text     string                `yaml:"text" json:"text,omitempty"`
	Hints    string                `yaml:"hints,omitempty" json:"hints,omitempty"`
	Variants []legacyQuestGreeting `yaml:"variants,omitempty" json:"variants,omitempty"`
}

// legacyTree holds all stateful conversation nodes for a mob.
type legacyTree struct {
	Root  legacyTreeRoot   `yaml:"root" json:"root,omitempty"`
	Nodes []legacyTreeNode `yaml:"nodes" json:"nodes,omitempty"`
}

// legacyMemoryConfig controls how long per-player memory persists between visits.
type legacyMemoryConfig struct {
	ExpiryPeriod string `yaml:"expiryPeriod" json:"expiryPeriod,omitempty"`
}

// legacyFile is a whole pre-topics dialogue file.
type legacyFile struct {
	MobId       int                 `yaml:"mobid"`
	Zone        string              `yaml:"zone"`
	DefaultMood string              `yaml:"defaultMood"`
	Greetings   []dialogue.Greeting `yaml:"greetings,omitempty"`
	Patterns    []legacyPattern     `yaml:"patterns"`
	Tree        *legacyTree         `yaml:"tree,omitempty"`
	Memory      legacyMemoryConfig  `yaml:"memory"`
}

// btNode mirrors behaviortree.NodeDef (yaml tags unchanged). The converter
// keeps its own copy so it does not import the behavior-tree package.
type btNode struct {
	Type     string         `yaml:"type"`
	Event    string         `yaml:"event,omitempty"`
	Children []btNode       `yaml:"children,omitempty"`
	Check    string         `yaml:"check,omitempty"`
	Do       string         `yaml:"do,omitempty"`
	Mod      string         `yaml:"mod,omitempty"`
	Note     string         `yaml:"note,omitempty"`
	Child    *btNode        `yaml:"child,omitempty"`
	Params   map[string]any `yaml:",inline"`
}

// btFile is the part of a behavior file the converter reads.
type btFile struct {
	Tree btNode `yaml:"tree"`
}
