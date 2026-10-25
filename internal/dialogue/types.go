package dialogue

// PlayerState provides quest and inventory callbacks so the dialogue engine
// can gate options on player progress without importing characters/users.
// When nil is passed the engine skips all quest/item checks (backward compat).
type PlayerState struct {
	HasQuest func(token string) bool
	HasItem  func(itemId int) bool
	// CarriesItem reports whether the player holds the item anywhere: the
	// backpack, worn or wielded, the component pouch or the bandolier. It
	// backs itemExcluded. Nil falls back to HasItem.
	CarriesItem func(itemId int) bool
	RemoveItem  func(itemId int) bool
	GiveQuest   func(token string)
	// GiveItem reports whether the item actually reached the player.
	// false (e.g. over carry capacity) makes the engine abort the node's
	// OTHER effects too — grantsQuest, requiresItem removal, flags, rep,
	// gold — so a givesItem+grantsQuest node can never burn its quest
	// token on an undelivered item (the soft-lock fixed 2026-08-03; the
	// questengine bridge got the same treatment in Tier 2). The node
	// stays re-triggerable once the player makes room.
	GiveItem         func(itemId int) bool
	GetQuestFlag     func(key string) string
	SetQuestFlag     func(key, value string)
	BumpRep          func(faction string, delta int)
	GiveGold         func(amount int)
	HasOwnMasterwork func(skillMin int) bool
	// HasGold reports whether the player carries at least amount gold. It
	// backs goldRequired, which hides a priced node (typically one that
	// also carries chargesGold at the same price) from a player who cannot
	// pay. Nil fails a goldRequired gate closed.
	HasGold func(amount int) bool
	// ChargeGold takes amount gold from the player and reports success. It
	// must refuse, taking nothing, when the player holds less. It backs
	// chargesGold, which is paid in the same step that matches the entry
	// and before any of its other effects. Nil fails the charge closed.
	ChargeGold func(amount int) bool
	// HasHeard reports whether the player has heard a topic, by HeardKey.
	// It backs heardRequired and the Dim / [*] state. Nil fails
	// heardRequired closed and treats every topic as unheard.
	HasHeard func(key string) bool
	// MarkHeard records a heard info topic on the character (saved).
	MarkHeard func(key string)
	// QuestOpen reports whether a quest can be offered now: false for a
	// repeatable on its cooldown or at its repeat_limit (#375 P3). Eligible
	// asks it for every offer topic with a quest; nil fails those closed,
	// like every interrogative callback here.
	QuestOpen func(questId int) bool
}

// QuestFlagSet describes a single key/value flag to write on the player's character.
type QuestFlagSet struct {
	Key   string `yaml:"key" json:"key,omitempty"`
	Value string `yaml:"value" json:"value,omitempty"`
}

// RepBump describes a single faction-reputation change applied when a dialogue
// node matches. Lets an ask-path delivery node replicate a give-path's
// item_give bump_rep actions so both paths are equivalent.
type RepBump struct {
	Faction string `yaml:"faction" json:"faction,omitempty"`
	Delta   int    `yaml:"delta" json:"delta,omitempty"`
}

// Mood represents the current emotional state of an NPC instance.
type Mood string

const (
	MoodFriendly Mood = "friendly"
	MoodNeutral  Mood = "neutral"
	MoodHostile  Mood = "hostile"
	MoodAfraid   Mood = "afraid"
	MoodGrateful Mood = "grateful"
)

// Greeting is one ambient line an NPC offers when a player arrives in its
// room. Authored in 186 dialogue files since long before the engine read
// them; the struct is shaped to the existing YAML, not the other way round.
type Greeting struct {
	Text  string   `yaml:"text" json:"text,omitempty"`
	Moods []string `yaml:"moods,omitempty" json:"moods,omitempty"`
}

// DialogueFile is the top-level structure of a mob's dialogue YAML: the
// ambient greetings, the opener talk says first, and the topic menu.
type DialogueFile struct {
	MobId       int        `yaml:"mobid" json:"mobid,omitempty"`
	Zone        string     `yaml:"zone" json:"zone,omitempty"`
	DefaultMood string     `yaml:"defaultMood" json:"defaultMood,omitempty"`
	Greetings   []Greeting `yaml:"greetings,omitempty" json:"greetings,omitempty"`
	Opener      *Opener    `yaml:"opener,omitempty" json:"opener,omitempty"`
	Topics      []Topic    `yaml:"topics,omitempty" json:"topics,omitempty"`
}
