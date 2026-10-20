package dialogue

import "fmt"

// Topic kinds (quest arc #375 phase 1). Every topic runs one code path; the
// kind sets its marker and what choosing it does.
const (
	KindInfo   = "info"
	KindOffer  = "offer"
	KindHandIn = "handin"
)

// Gates is the gating family every topic and opener variant shares. The YAML
// names and the semantics are the ones patterns and tree nodes used.
type Gates struct {
	QuestRequired      []string          `yaml:"questRequired,omitempty" json:"questRequired,omitempty"`
	QuestExcluded      []string          `yaml:"questExcluded,omitempty" json:"questExcluded,omitempty"`
	QuestFlagRequired  map[string]string `yaml:"questFlagRequired,omitempty" json:"questFlagRequired,omitempty"`
	QuestFlagExcluded  map[string]string `yaml:"questFlagExcluded,omitempty" json:"questFlagExcluded,omitempty"`
	RequiresItem       int               `yaml:"requiresItem,omitempty" json:"requiresItem,omitempty"`
	ItemExcluded       int               `yaml:"itemExcluded,omitempty" json:"itemExcluded,omitempty"`
	CarriesItem        int               `yaml:"carriesItem,omitempty" json:"carriesItem,omitempty"` // carried anywhere, never taken (a reusable key)
	GoldRequired       int               `yaml:"goldRequired,omitempty" json:"goldRequired,omitempty"`
	MasterworkRequired int               `yaml:"masterworkRequired,omitempty" json:"masterworkRequired,omitempty"`
}

// pass reports whether the player clears every gate. A nil PlayerState skips
// the checks, as everywhere in this package.
func (g Gates) pass(ps *PlayerState) bool {
	return checkQuestGate(g.QuestRequired, g.QuestExcluded, g.RequiresItem,
		g.QuestFlagRequired, g.QuestFlagExcluded, g.MasterworkRequired, g.GoldRequired, ps) &&
		!itemExcludedBlocks(g.ItemExcluded, ps) && carriesItemPasses(g.CarriesItem, ps)
}

// carriesItemPasses reports whether the player carries itemId anywhere
// (CarriesItem, falling back to HasItem). A nil PlayerState skips the gate;
// no item callback at all fails it closed.
func carriesItemPasses(itemId int, ps *PlayerState) bool {
	if itemId <= 0 || ps == nil {
		return true
	}
	if ps.CarriesItem != nil {
		return ps.CarriesItem(itemId)
	}
	if ps.HasItem != nil {
		return ps.HasItem(itemId)
	}
	return false
}

// Effects is what choosing a topic changes. requiresItem (a gate) is also
// taken when the effects apply.
type Effects struct {
	GrantsQuest   string        `yaml:"grantsQuest,omitempty" json:"grantsQuest,omitempty"`
	GivesItem     int           `yaml:"givesItem,omitempty" json:"givesItem,omitempty"`
	SetsQuestFlag *QuestFlagSet `yaml:"setsQuestFlag,omitempty" json:"setsQuestFlag,omitempty"`
	BumpsRep      []RepBump     `yaml:"bumpsRep,omitempty" json:"bumpsRep,omitempty"`
	GivesGold     int           `yaml:"givesGold,omitempty" json:"givesGold,omitempty"`
	ChargesGold   int           `yaml:"chargesGold,omitempty" json:"chargesGold,omitempty"`
	MoodChange    string        `yaml:"moodChange,omitempty" json:"moodChange,omitempty"`
}

// Topic is one menu entry on an NPC. See the spec, section 3.2.
type Topic struct {
	Key           string   `yaml:"key" json:"key"`
	Kind          string   `yaml:"kind" json:"kind"`
	Quest         int      `yaml:"quest,omitempty" json:"quest,omitempty"`
	Label         string   `yaml:"label" json:"label"`
	PlayerLine    string   `yaml:"playerLine,omitempty" json:"playerLine,omitempty"`
	Text          string   `yaml:"text" json:"text"`
	Hints         string   `yaml:"hints,omitempty" json:"hints,omitempty"`
	Marked        bool     `yaml:"marked,omitempty" json:"marked,omitempty"`
	HeardRequired []string `yaml:"heardRequired,omitempty" json:"heardRequired,omitempty"`
	Breadcrumb    string   `yaml:"breadcrumb,omitempty" json:"breadcrumb,omitempty"`
	// Behavior fires the mob's behavior tree with event player_topic and
	// this topic's key once the topic has played (R12).
	Behavior bool `yaml:"behavior,omitempty" json:"behavior,omitempty"`
	Gates    `yaml:",inline"`
	Effects  `yaml:",inline"`
}

// OpenerVariant is a gated alternative opener. It carries no effects.
type OpenerVariant struct {
	Gates `yaml:",inline"`
	Text  string `yaml:"text" json:"text"`
	Hints string `yaml:"hints,omitempty" json:"hints,omitempty"`
}

// Opener is what the NPC says when talk opens, before the menu.
type Opener struct {
	Text     string          `yaml:"text" json:"text"`
	Hints    string          `yaml:"hints,omitempty" json:"hints,omitempty"`
	Variants []OpenerVariant `yaml:"variants,omitempty" json:"variants,omitempty"`
}

// HeardKey is the saved form of a heard topic: "<mob template id>.<key>".
func HeardKey(mobId int, key string) string {
	return fmt.Sprintf("%d.%s", mobId, key)
}
