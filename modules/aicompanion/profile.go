package aicompanion

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed profiles/*.yaml
var profileFiles embed.FS

// Profile is the authored definition of one bonded companion: who they are,
// how they speak, and which mob template carries them in the world. It is
// content, not state; nothing here changes at runtime.
//
// Profiles are embedded in the module rather than kept under
// _datafiles/world/dogmud, because the messaging-surface guard inventories
// every text-bearing YAML key in that tree.
type Profile struct {
	Id          string   `yaml:"id"`
	MobId       int      `yaml:"mob_id"`
	Name        string   `yaml:"name"`
	Pronouns    string   `yaml:"pronouns"`
	Age         string   `yaml:"age"`
	Summary     string   `yaml:"summary"`
	Personality []string `yaml:"personality"`
	SpeechStyle string   `yaml:"speech_style"`
	Likes       []string `yaml:"likes"`
	Dislikes    []string `yaml:"dislikes"`
	Fears       []string `yaml:"fears"`
	Ambitions   []string `yaml:"ambitions"`
	Knowledge   string   `yaml:"knowledge"`
	Habits      []string `yaml:"habits"`
	// Backstory is revealed gradually. Trust is the minimum trust score
	// (0-100) before the companion will share the detail. Until then the
	// model is not even told it, only that there are things kept private.
	Backstory []BackstoryItem `yaml:"backstory"`
	Fallback  FallbackLines   `yaml:"fallback"`

	// Manner is her own voice for each band of the relationship
	// (disdain, cold, professional, friendly, warm, close). Anything left
	// out falls back to the generic wording in opinion.go.
	Manner map[string]string `yaml:"manner"`
	// WarmthStyle is how affection shows in this particular character, so a
	// dry one never turns gushing.
	WarmthStyle string `yaml:"warmth_style"`

	// Opinion sets where the companion starts with a new owner and how
	// strongly it reacts. See opinion.go.
	Opinion ProfileOpinion `yaml:"opinion"`
	// Talkativeness (0..1) is the chance, each time a quiet spell is
	// noticed, that the companion considers starting a conversation.
	Talkativeness float64 `yaml:"talkativeness"`
	// CuriousAbout lists things the companion would like to learn about its
	// owner. The model asks about them over time and records the answers.
	CuriousAbout []string `yaml:"curious_about"`
	// Interests are words that make an item catch the companion's eye
	// ("bow", "herb"). Matched against item names when scoring the scene.
	Interests []string `yaml:"interests"`
	// IdleEmotes are small things the companion does on its own when
	// nothing is happening, chosen locally without a model call.
	// IdleEmotesWarm and IdleEmotesDistant replace them when it is fond of
	// its owner, or has gone cold on them.
	IdleEmotes        []string `yaml:"idle_emotes"`
	IdleEmotesWarm    []string `yaml:"idle_emotes_warm"`
	IdleEmotesDistant []string `yaml:"idle_emotes_distant"`
	// ThinkingEmotes are shown while a slow reply is on its way.
	ThinkingEmotes []string `yaml:"thinking_emotes"`

	// Meeting is how the companion arrives when it first meets a new
	// character (emote text; the name is prepended).
	Meeting string `yaml:"meeting"`

	// Archetype, supplies and purse (phase 5).
	// Romance is what this character is open to, and how it shows. See
	// romance.go. Romanceable defaults to false: most companions are not.
	Romance ProfileRomance `yaml:"romance"`

	// StartingItems are what she carries the first time she is bonded to
	// someone: item ids, equipped where they can be. A bonded companion
	// otherwise begins with nothing, because a template that carries gear
	// would hand out free gear every time it died.
	StartingItems []int `yaml:"starting_items"`
	// StartingGoals are what she sets out wanting, in her own words.
	StartingGoals []string `yaml:"starting_goals"`

	// Crafts are the trades this companion works: recipe disciplines it
	// knows the beginner recipes of (cooking, tailoring, alchemy...).
	Crafts []string `yaml:"crafts"`
	// Pastimes weigh what they do with themselves when nothing is
	// happening (autonomy.go pastime): a thief searches, a herbalist
	// forages, a scout reads the ways out. Keys from pastimeDefaults;
	// anything left out keeps its default weight.
	Pastimes map[string]int `yaml:"pastimes"`

	Archetype Archetype     `yaml:"archetype"`
	Supplies  []Supply      `yaml:"supplies"`
	Purse     Purse         `yaml:"purse"`
	Combat    CombatProfile `yaml:"combat"`

	// Specialty is what this companion is good at, in a sentence or two of
	// their own: it goes into WHO YOU ARE, so the model plays to it.
	Specialty string `yaml:"specialty"`
	// StartingSpells are spell ids put into the spellbook whenever this
	// companion takes up with someone, beside the starting kit. Each must
	// name a spell the game has (checked at load, see spellsValid).
	StartingSpells []string `yaml:"starting_spells"`
	// AmmoWord is what the companion calls its ammunition in a fight
	// ("arrows", "bolts", "stones"). Empty is arrows.
	AmmoWord string `yaml:"ammo_word"`

	// Hollow is where and how this companion waits, in the Waystone
	// Hollow, while nobody has them (hollow.go).
	Hollow HollowProfile `yaml:"hollow"`
}

// HollowProfile is the authored half of a companion waiting in the
// Waystone Hollow. Everything a passer-by meets there before they have a
// key or have agreed to the model is here; what the companion says once
// they have is the model's, judged against Appreciates and WaryOf.
type HollowProfile struct {
	// RoomId is the companion's own room in the Hollow, decorated to
	// their taste. No two companions share one.
	RoomId int `yaml:"room_id"`
	// Appreciates are the things that would win this companion over: what
	// a player could bring and show them, or do and bring back evidence
	// of. They are judged by the model against what the companion can see
	// of the player, never counted.
	Appreciates []string `yaml:"appreciates"`
	// WaryOf are the things that put this companion off a would-be
	// travelling companion.
	WaryOf []string `yaml:"wary_of"`
	// Waiting is how the companion passes the time in the Hollow (emote
	// text, the name is prepended), chosen locally.
	Waiting []string `yaml:"waiting"`
	// Returns is the emote the companion makes on getting back to the
	// Hollow after parting from someone.
	Returns string `yaml:"returns"`
}

// ammoWord is what the companion calls its ammunition.
func (p *Profile) ammoWord() string {
	if w := strings.TrimSpace(p.AmmoWord); w != `` {
		return w
	}
	return `arrows`
}

// ProfileOpinion is the authored starting opinion and sensitivity.
type ProfileOpinion struct {
	Baseline    Opinion     `yaml:"baseline"`
	Sensitivity Sensitivity `yaml:"sensitivity"`
}

// Sensitivity scales model-proposed opinion changes before they are bounded.
// A proud or wounded character has Negative above 1; a forgiving one below.
type Sensitivity struct {
	Positive float64 `yaml:"positive"`
	Negative float64 `yaml:"negative"`
}

// OpinionBaseline is the opinion a new mind starts with.
func (p *Profile) OpinionBaseline() Opinion {
	return p.Opinion.Baseline.clamped()
}

// sensitivity returns the profile's multipliers with defaults and bounds.
func (p *Profile) sensitivity() Sensitivity {
	s := p.Opinion.Sensitivity
	if s.Positive <= 0 {
		s.Positive = 1
	}
	if s.Negative <= 0 {
		s.Negative = 1
	}
	if s.Positive > 2 {
		s.Positive = 2
	}
	if s.Negative > 2 {
		s.Negative = 2
	}
	return s
}

// talkativeness returns the initiative chance, defaulting to 0.4.
func (p *Profile) talkativeness() float64 {
	if p.Talkativeness <= 0 {
		return 0.4
	}
	if p.Talkativeness > 1 {
		return 1
	}
	return p.Talkativeness
}

type BackstoryItem struct {
	Trust int    `yaml:"trust"`
	Text  string `yaml:"text"`
}

// FallbackLines are what the companion does when no model is available.
// Every entry is emote text (the companion's name is prepended by the game).
type FallbackLines struct {
	Greetings []string `yaml:"greetings"`
	Replies   []string `yaml:"replies"`
	Recovered []string `yaml:"recovered"`
	Farewells []string `yaml:"farewells"`
}

// FirstName is the part of the name players most often use to address them.
func (p *Profile) FirstName() string {
	if f := strings.Fields(p.Name); len(f) > 0 {
		return f[0]
	}
	return p.Name
}

func (p *Profile) validate() error {
	if strings.TrimSpace(p.Id) == `` {
		return fmt.Errorf(`profile has no id`)
	}
	if p.MobId <= 0 {
		return fmt.Errorf(`profile %q has no mob_id`, p.Id)
	}
	if strings.TrimSpace(p.Name) == `` {
		return fmt.Errorf(`profile %q has no name`, p.Id)
	}
	if strings.TrimSpace(p.Summary) == `` {
		return fmt.Errorf(`profile %q has no summary`, p.Id)
	}
	if len(p.Fallback.Replies) == 0 || len(p.Fallback.Greetings) == 0 {
		return fmt.Errorf(`profile %q needs fallback greetings and replies`, p.Id)
	}
	if err := p.Archetype.validate(); err != nil {
		return fmt.Errorf(`profile %q: %w`, p.Id, err)
	}
	switch p.Combat.FleeAt {
	case ``, `never`, `badly_hurt`, `about_to_die`:
	default:
		return fmt.Errorf(`profile %q: combat flee_at must be never, badly_hurt or about_to_die`, p.Id)
	}
	switch p.Combat.Style {
	case ``, `melee`, `ranged`:
	default:
		return fmt.Errorf(`profile %q: combat style must be melee or ranged`, p.Id)
	}
	if !inSet(combatStanceDefaults, p.Combat.Stance) {
		return fmt.Errorf(`profile %q: combat stance must be fight, protect or hold_back`, p.Id)
	}
	for _, mv := range p.Combat.Moves {
		if !inSet(realMoves(), mv) {
			return fmt.Errorf(`profile %q: combat move %q is not one of %s`, p.Id, mv, strings.Join(realMoves(), `, `))
		}
	}
	if p.Combat.MendBelow < 0 || p.Combat.MendBelow > 95 {
		return fmt.Errorf(`profile %q: combat mend_below must be 0 to 95`, p.Id)
	}
	switch p.Combat.Opener {
	case ``, `surprise`:
	default:
		return fmt.Errorf(`profile %q: combat opener must be empty or surprise`, p.Id)
	}
	for k, v := range p.Pastimes {
		if _, ok := pastimeDefaults[k]; !ok || v < 0 || v > 10 {
			return fmt.Errorf(`profile %q: pastime %q must be one of emote, search, scan, forage, salvage, butcher, gearup, weighted 0 to 10`, p.Id, k)
		}
	}
	if (p.Combat.MendBelow > 0 || p.Combat.WardOwner) && len(p.StartingSpells) == 0 {
		return fmt.Errorf(`profile %q: combat mend_below and ward_owner need starting_spells to mend and ward with`, p.Id)
	}
	for _, s := range p.Supplies {
		if s.Name == `` || (s.Type == `` && s.Keyword == ``) || s.Min < 0 || s.Target < s.Min {
			return fmt.Errorf(`profile %q: supply %q needs a name, a type or keyword, and target >= min`, p.Id, s.Name)
		}
	}
	if p.Hollow.RoomId <= 0 {
		return fmt.Errorf(`profile %q: hollow room_id is required; every companion waits in a room of their own`, p.Id)
	}
	if len(p.Hollow.Appreciates) == 0 {
		return fmt.Errorf(`profile %q: hollow appreciates needs at least one entry`, p.Id)
	}
	for _, id := range p.StartingSpells {
		if strings.TrimSpace(id) == `` {
			return fmt.Errorf(`profile %q: starting_spells has an empty entry`, p.Id)
		}
	}
	return nil
}

// spellsValid reports the first starting spell the game does not have.
// It needs the spells loaded, so it is checked when the module loads its
// profiles, not by validate.
func (p *Profile) spellsValid(known func(string) bool) error {
	for _, id := range p.StartingSpells {
		if !known(id) {
			return fmt.Errorf(`profile %q: starting spell %q does not exist`, p.Id, id)
		}
	}
	return nil
}

// parseProfile decodes and validates one profile document.
func parseProfile(b []byte) (*Profile, error) {
	p := &Profile{}
	if err := yaml.Unmarshal(b, p); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// loadProfiles reads every embedded profile. A bad profile is reported and
// skipped; it never stops the others loading. Duplicate ids or mob ids are
// errors because the mob id is how a live companion finds its profile, and
// so is a shared Hollow room, because each companion waits in their own.
func loadProfiles() (map[string]*Profile, []error) {
	out := map[string]*Profile{}
	byMob := map[int]string{}
	byRoom := map[int]string{}
	var errs []error

	entries, err := profileFiles.ReadDir(`profiles`)
	if err != nil {
		return out, []error{err}
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), `.yaml`) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		b, err := profileFiles.ReadFile(path.Join(`profiles`, n))
		if err != nil {
			errs = append(errs, fmt.Errorf(`%s: %w`, n, err))
			continue
		}
		p, err := parseProfile(b)
		if err != nil {
			errs = append(errs, fmt.Errorf(`%s: %w`, n, err))
			continue
		}
		if _, dup := out[p.Id]; dup {
			errs = append(errs, fmt.Errorf(`%s: duplicate profile id %q`, n, p.Id))
			continue
		}
		if other, dup := byMob[p.MobId]; dup {
			errs = append(errs, fmt.Errorf(`%s: mob_id %d already used by profile %q`, n, p.MobId, other))
			continue
		}
		if other, dup := byRoom[p.Hollow.RoomId]; dup {
			errs = append(errs, fmt.Errorf(`%s: hollow room_id %d already used by profile %q`, n, p.Hollow.RoomId, other))
			continue
		}
		out[p.Id] = p
		byMob[p.MobId] = p.Id
		byRoom[p.Hollow.RoomId] = p.Id
	}
	return out, errs
}

// idlePool is the set of idle gestures that fits how the companion feels
// about its owner just now.
func (p *Profile) idlePool(o Opinion) []string {
	if o.Affection >= 40 && len(p.IdleEmotesWarm) > 0 {
		return append(append([]string{}, p.IdleEmotesWarm...), p.IdleEmotes...)
	}
	if o.Affection <= -20 && len(p.IdleEmotesDistant) > 0 {
		return append(append([]string{}, p.IdleEmotesDistant...), p.IdleEmotes...)
	}
	return p.IdleEmotes
}
