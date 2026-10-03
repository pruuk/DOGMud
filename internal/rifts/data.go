package rifts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Pool is the kind of room a door leads to.
type Pool string

const (
	PoolPassage Pool = `A` // generic passage
	PoolFeature Pool = `B` // larger room with decorative features
	PoolPuzzle  Pool = `C` // puzzle or trap
	PoolMonster Pool = `D` // ordinary monsters
	PoolBoss    Pool = `E` // a major boss
	PoolExit    Pool = `F` // a way back to the world
)

// AllPools is every pool, in order.
var AllPools = []Pool{PoolPassage, PoolFeature, PoolPuzzle, PoolMonster, PoolBoss, PoolExit}

func (p Pool) valid() bool {
	for _, q := range AllPools {
		if p == q {
			return true
		}
	}
	return false
}

// IntRange is an inclusive [Min, Max] range.
type IntRange struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// MobTiers lists the mob ids a profile spawns, by tier.
type MobTiers struct {
	Trash []int `yaml:"trash"`
	Elite []int `yaml:"elite"`
	Boss  []int `yaml:"boss"`
	// Hunter is sent after a player who flees a fight (hunter.go). 0 = the
	// profile has no hunter, and fleeing costs nothing extra.
	Hunter int `yaml:"hunter"`
}

// LurkerSpec puts a lone creature in rooms that are otherwise quiet (a Glint
// Stalker waiting in a passage), so no pool is always safe. It is placed with
// a trash stat pool and does not seal the room's doors.
type LurkerSpec struct {
	Chance map[Pool]int `yaml:"chance"` // percent, by the room's pool
	Mobs   []int        `yaml:"mobs"`   // one is chosen
}

// TierStatPools is the base stat pool for each tier; depth adds to it.
type TierStatPools struct {
	Trash  int `yaml:"trash"`
	Elite  int `yaml:"elite"`
	Boss   int `yaml:"boss"`
	Hunter int `yaml:"hunter"`
}

// Profile is one rift biome/theme: everything the engine needs to build rooms
// of that kind except the rooms themselves, which are templates.
type Profile struct {
	Id       string `yaml:"id"`
	Name     string `yaml:"name"`      // player-facing name, e.g. "the Obelisk"
	Zone     string `yaml:"zone"`      // zone name stamped on every room built
	Biome    string `yaml:"biome"`     // engine biome for every room built
	Lamp     *int   `yaml:"lamp"`      // optional light override for every room
	ExitName string `yaml:"exit_name"` // exit name of the way out in an exit (F) room

	Weights   map[Pool]int      `yaml:"weights"`    // pool roll weights per door
	MinDepth  map[Pool]int      `yaml:"min_depth"`  // a pool is not rolled shallower than this
	Doors     map[Pool]IntRange `yaml:"doors"`      // door count per room, by the room's pool
	OreChance map[Pool]int      `yaml:"ore_chance"` // percent chance a room of the pool holds ore
	OreItems  []int             `yaml:"ore_items"`  // ore item ids; one is chosen per ore room
	OreWeight int               `yaml:"ore_weight"` // copies of the ore added to the room's forage pool

	Mobs             MobTiers      `yaml:"mobs"`
	Lurkers          LurkerSpec    `yaml:"lurkers"`
	StatPools        TierStatPools `yaml:"statpools"`
	StatPoolPerDepth int           `yaml:"statpool_per_depth"` // stat pool points added per room of depth
	// HunterDelaySeconds is how long after a flee (and after a hunter that
	// lost its quarry withdraws) the hunter appears where the player is.
	HunterDelaySeconds IntRange `yaml:"hunter_delay_seconds"`
	KeyItemId          int      `yaml:"key_item_id"`
	TrapConditionIds   []int    `yaml:"trap_condition_ids"` // default conditions for a trap with none of its own
	StartPool          Pool     `yaml:"start_pool"`         // pool of the first room
	DefaultOreNoun     string   `yaml:"default_ore_noun"`
	DefaultOreLook     string   `yaml:"default_ore_look"`
	IdleMessages       []string `yaml:"idlemessages"` // shared ambience added to every room's own lines

	// Generated ambience (internal/roomlife, the same events the world
	// gets): the percent of a rift room's ambient lines, once one is due,
	// that a model writes fresh instead (replacing Modules.roomlife.Chance
	// in the rift), and the setting the model is told it is writing for.
	// Rift rooms have no time of day.
	AmbientGeneratedChance int      `yaml:"ambient_generated_chance"`
	AmbientSetting         string   `yaml:"ambient_setting"`
	Messages               Messages `yaml:"messages"`

	// Memory: the lens table, the memory puzzle (memory.go). Required when
	// any template has a puzzle of kind memory.
	Memory *MemorySpec `yaml:"memory"`

	// Losses: what death and logging out inside cost, and the lost-items
	// record rubble draws on (lost.go). Nil: nothing is lost.
	Losses *LossSpec `yaml:"losses"`

	// Generation: rooms a model writes in the background as rooms are
	// built, saved beside the authored ones so the pools grow (gen.go).
	Generation GenerationSpec `yaml:"generation"`

	// Light: a light item left on the floor of a room, by the room's pool.
	LightItemId int          `yaml:"light_item_id"`
	LightChance map[Pool]int `yaml:"light_chance"` // percent

	// Portal sites: where the profile's way in appears in the world. Each
	// real-world day every region in PortalRegions gets PortalSites sites, in
	// rooms of PortalBiomes (never cities or roads: leave their biomes out).
	PortalBiomes  []string `yaml:"portal_biomes"`
	PortalRegions []string `yaml:"portal_regions"`          // zone-config regions; empty = every region
	PortalSites   IntRange `yaml:"portal_sites_per_region"` // sites per region per day
	// Zones never to place a site in: any zone whose default biome is one of
	// PortalSkipZoneBiomes (cities), or whose name contains one of
	// PortalSkipZoneWords (case-insensitive: "Road").
	PortalSkipZoneBiomes []string `yaml:"portal_skip_zone_biomes"`
	PortalSkipZoneWords  []string `yaml:"portal_skip_zone_words"`
	PortalExit           string   `yaml:"portal_exit"`         // the exit (and noun) players use: `enter crystal`
	PortalTitle          string   `yaml:"portal_title"`        // the temporary exit's title
	PortalLook           string   `yaml:"portal_look"`         // what `look <portal_exit>` shows
	PortalMutator        string   `yaml:"portal_mutator"`      // mutator id that shows the portal in the room description
	JoinWindowSecs       int      `yaml:"join_window_seconds"` // after the first player goes in, how long the rest of a party has to follow

	// Lore and writings. Lore objects build understanding; a player who has
	// studied Lore.Needed distinct ones can read the writings, which tell the
	// profile's story in fragments (Writings, in order).
	Lore          LoreSpec      `yaml:"lore"`
	Writings      []WritingSpec `yaml:"writings"`
	WritingChance map[Pool]int  `yaml:"writing_chance"` // percent

	// Rubble: a searchable pile, the rift's equivalent of a dungeon chest.
	Rubble RubbleSpec `yaml:"rubble"`

	templates map[Pool][]*Template
}

// LoreSpec is the profile's lore object: something a player studies to come
// to understand the place. Each distinct one studied says `lore_progress`;
// the Needed-th says `lore_unlocked` instead and makes the player a reader.
type LoreSpec struct {
	Chance  map[Pool]int `yaml:"chance"`  // percent, by the room's pool
	Noun    string       `yaml:"noun"`    // the noun it is looked at by
	Look    string       `yaml:"look"`    // what looking at it shows anyone
	Mention string       `yaml:"mention"` // sentence added to the room's description
	Needed  int          `yaml:"needed"`  // distinct ones to study before writings can be read
}

// RubbleSpec is the profile's searchable pile. `search <noun>` turns up one
// find per pile, for whoever searches it first; for now that find is a bauble
// of the room pool's tier (cheap, average or rare), delivered by the bauble
// system.
type RubbleSpec struct {
	Chance  map[Pool]int    `yaml:"chance"`  // percent, by the room's pool
	Noun    string          `yaml:"noun"`    // what it is searched and looked at by
	Look    string          `yaml:"look"`    // what looking at it shows
	Mention string          `yaml:"mention"` // sentence added to the room's description
	Tier    map[Pool]string `yaml:"tier"`    // bauble value tier by the room's pool
}

// GenerationSpec is what a model is told when it writes a new room for a
// profile, and which pools it may write for. Empty Pools: none.
type GenerationSpec struct {
	Pools   []Pool          `yaml:"pools"`   // pools a model may add rooms to
	Setting string          `yaml:"setting"` // the canon, for the model
	Guides  map[Pool]string `yaml:"guides"`  // what a room of each pool is
	Effects map[string]int  `yaml:"effects"` // effect word -> condition id (traps, wrong answers)
	Seeds   []string        `yaml:"seeds"`   // a few are offered per request, for variety
}

// allows reports whether a model may write rooms for pool.
func (g GenerationSpec) allows(pool Pool) bool {
	for _, p := range g.Pools {
		if p == pool {
			return true
		}
	}
	return false
}

// WritingSpec is one writing object; its place in Profile.Writings is the
// fragment of the story it holds.
type WritingSpec struct {
	Noun     string `yaml:"noun"`
	Look     string `yaml:"look"`     // what anyone sees: the writing, unread
	Mention  string `yaml:"mention"`  // sentence added to the room's description
	Fragment string `yaml:"fragment"` // what a reader understands
}

// Messages are the profile's narration lines, by role. Each is required.
type Messages map[string]string

// RequiredMessages are the keys every profile must author.
var RequiredMessages = []string{
	`portal_open`,        // overworld room: a portal site appears (daily rotation)
	`portal_close`,       // overworld room: a portal site fades (daily rotation)
	`portal_dormant`,     // refusal at the portal: it is not ready to be entered
	`enter`,              // to a player: arriving in the rift for the first time
	`sealed_hostile`,     // refusal: something hostile still stands in the room
	`sealed_puzzle`,      // refusal: this door waits on the room's puzzle
	`locked`,             // refusal: the door needs a key the player lacks
	`unsettled`,          // refusal: the room beyond is not built yet
	`key_used`,           // to the player who spends a key; %s is the door
	`key_used_room`,      // to the room; %s is the player, %s the door
	`doors_open`,         // room: the hostiles are gone and the doors open
	`boss_key`,           // room: the boss falls and a key is left behind
	`puzzle_unsealed`,    // room: a puzzle was solved and its door opens
	`puzzle_key`,         // to the solver of a hard puzzle: a key is theirs
	`touch_progress`,     // to a player who touched the right thing next; %s is the thing
	`leave`,              // to a player stepping back out into the world
	`pick_refusal`,       // picklock on a rift door; %s is the door
	`closed`,             // to a player refused entry to a rift they are not part of
	`hunted`,             // to a player who fled a fight: something has noticed (only if the profile has a hunter)
	`hunter_arrives`,     // room: the hunter steps out of the walls
	`hunter_holds`,       // refusal: the hunted player cannot leave while the hunter stands here
	`hunter_withdraws`,   // room: the hunter sinks back into the walls (its quarry is gone)
	`hunter_destroyed`,   // room: the hunter is destroyed and the hunt is over
	`lore_progress`,      // to a player studying a new lore object
	`lore_unlocked`,      // to a player whose study is complete: writings can be read now
	`writing_unreadable`, // to a player who cannot read the writing yet
	`rubble_found`,       // to the first player to search a rubble pile: something is in it
	`rubble_empty`,       // to a player searching a pile already picked over
}

// Msg returns a profile message, formatted with args and wrapped at 80
// columns like the rest of the game's prose.
func (p *Profile) Msg(key string, args ...any) string {
	s := p.Messages[key]
	if len(args) > 0 {
		s = fmt.Sprintf(s, args...)
	}
	return wrap(s)
}

// wrap folds authored prose to the game's 80-column width. YAML folded
// scalars arrive as one long line.
func wrap(s string) string {
	return util.SplitStringNL(strings.TrimSpace(s), 80)
}

// Templates returns the profile's templates for a pool.
func (p *Profile) Templates(pool Pool) []*Template {
	return p.templates[pool]
}

// DoorSpec is one door a template offers: the exit name the player types and
// what they see when they look at it.
type DoorSpec struct {
	Exit        string `yaml:"exit"`
	Description string `yaml:"description"`
}

// PuzzleSpec is a puzzle the engine can check by itself. Kinds:
//
//	riddle   - `answer <words>`; any of Answers solves it.
//	sequence - `touch <noun>` for each of Sequence, in order; a wrong touch
//	           resets progress and applies WrongConditionIds.
type PuzzleSpec struct {
	Kind              string   `yaml:"kind"`
	SealedDoor        string   `yaml:"sealed_door"` // the door the puzzle holds shut; always among the room's doors
	Answers           []string `yaml:"answers"`
	Sequence          []string `yaml:"sequence"`
	Reward            string   `yaml:"reward"` // none | key | ore
	Hard              bool     `yaml:"hard"`   // a hard puzzle may reward a key
	Solved            string   `yaml:"solved"` // to the solver
	Wrong             string   `yaml:"wrong"`  // to the player who got it wrong
	WrongConditionIds []int    `yaml:"wrong_condition_ids"`
}

// TrapSpec is a trap sprung on a player's first entry into the room unless a
// Perception roll beats Difficulty.
type TrapSpec struct {
	Difficulty   int    `yaml:"difficulty"`
	ConditionIds []int  `yaml:"condition_ids"`
	Triggered    string `yaml:"triggered"` // to the player caught
	Avoided      string `yaml:"avoided"`   // to the player who saw it coming
}

// EncounterSpec is how many monsters of which tier a monster (D) room holds.
type EncounterSpec struct {
	Tier  string   `yaml:"tier"` // trash | elite
	Count IntRange `yaml:"count"`
}

// OreSpec is how an ore room shows its ore.
type OreSpec struct {
	Noun string `yaml:"noun"`
	Look string `yaml:"look"`
}

// Template is one authored or generated room of a pool.
type Template struct {
	Id           string            `yaml:"id"`
	Pool         Pool              `yaml:"pool"`
	Title        string            `yaml:"title"`
	Description  string            `yaml:"description"`
	Nouns        map[string]string `yaml:"nouns"`
	IdleMessages []string          `yaml:"idlemessages"`
	Doors        []DoorSpec        `yaml:"doors"`
	Puzzle       *PuzzleSpec       `yaml:"puzzle,omitempty"`
	Trap         *TrapSpec         `yaml:"trap,omitempty"`
	Encounter    *EncounterSpec    `yaml:"encounter,omitempty"`
	Ore          *OreSpec          `yaml:"ore,omitempty"`
	Source       string            `yaml:"source"` // authored | generated

	// Provenance of a generated room (gen.go): the model that wrote it, the
	// prompt version, and when (RFC 3339, UTC).
	Model         string `yaml:"model,omitempty"`
	PromptVersion int    `yaml:"prompt_version,omitempty"`
	Created       string `yaml:"created,omitempty"`

	profileId string
}

var profiles = map[string]*Profile{}

// exitNameRE is what a door's exit name may be: a word a player can type.
var exitNameRE = regexp.MustCompile(`^[a-z]+$`)

// GetProfile returns a loaded profile by id.
func GetProfile(id string) *Profile {
	return profiles[strings.ToLower(id)]
}

// ProfileIds lists the loaded profiles, sorted.
func ProfileIds() []string {
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// DataDir is where rift data lives: <DataFiles>/rifts.
func DataDir() string {
	return filepath.Join(configs.GetFilePathsConfig().DataFiles.String(), `rifts`)
}

// LoadDataFiles loads every profile under rifts/profiles and its templates
// under rifts/rooms/<profile id>, and validates them against the loaded mobs,
// items and conditions. Like the other boot loaders it panics on bad data, so
// it must run after mobs, items and conditions are loaded. A missing rifts
// directory is not an error: the world simply has no rifts.
func LoadDataFiles() {
	loaded, err := loadFrom(DataDir())
	if err != nil {
		panic(fmt.Sprintf(`rifts.LoadDataFiles: %v`, err))
	}
	for _, p := range loaded {
		if err := p.validate(true); err != nil {
			panic(fmt.Sprintf(`rifts.LoadDataFiles: %v`, err))
		}
	}
	profiles = loaded
	mudlog.Info(`rifts.LoadDataFiles`, `profiles`, len(profiles))
}

func loadFrom(dir string) (map[string]*Profile, error) {
	out := map[string]*Profile{}
	profileFiles, err := filepath.Glob(filepath.Join(dir, `profiles`, `*.yaml`))
	if err != nil {
		return nil, err
	}
	for _, f := range profileFiles {
		p := &Profile{}
		if err := readStrict(f, p); err != nil {
			return nil, err
		}
		p.Id = strings.ToLower(p.Id)
		if want := strings.TrimSuffix(filepath.Base(f), `.yaml`); p.Id != want {
			return nil, fmt.Errorf(`%s: id %q must match the file name %q`, f, p.Id, want)
		}
		p.templates = map[Pool][]*Template{}
		roomFiles, err := filepath.Glob(filepath.Join(dir, `rooms`, p.Id, `*.yaml`))
		if err != nil {
			return nil, err
		}
		for _, rf := range roomFiles {
			t := &Template{}
			err := readStrict(rf, t)
			if want := strings.TrimSuffix(filepath.Base(rf), `.yaml`); err == nil && t.Id != want {
				err = fmt.Errorf(`%s: id %q must match the file name %q`, rf, t.Id, want)
			}
			if err != nil {
				if isGeneratedFile(rf) {
					// A machine-written room never stops the server: it is
					// left out, and said so.
					mudlog.Warn(`rifts.LoadDataFiles`, `skipped generated room`, rf, `error`, err)
					continue
				}
				return nil, err
			}
			t.profileId = p.Id
			p.templates[t.Pool] = append(p.templates[t.Pool], t)
		}
		out[p.Id] = p
	}
	return out, nil
}

// isGeneratedFile is a room file a model wrote (gen.go names them gen-*).
func isGeneratedFile(path string) bool { return strings.HasPrefix(filepath.Base(path), `gen-`) }

func readStrict(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.UnmarshalStrict(b, out); err != nil {
		return fmt.Errorf(`%s: %w`, path, err)
	}
	return nil
}

// validate checks a profile and its templates. withWorld also checks mob,
// item and condition ids against the loaded world (off in unit tests).
func (p *Profile) validate(withWorld bool) error {
	errf := func(format string, args ...any) error {
		return fmt.Errorf(`rift profile %q: %s`, p.Id, fmt.Sprintf(format, args...))
	}
	p.dropBadGenerated(withWorld)
	if p.Name == `` || p.Zone == `` || p.Biome == `` || p.ExitName == `` || p.PortalExit == `` {
		return errf(`name, zone, biome, exit_name and portal_exit are required`)
	}
	if _, ok := rooms.GetBiome(p.Biome); !ok && withWorld {
		return errf(`unknown biome %q`, p.Biome)
	}
	if !p.StartPool.valid() || p.StartPool == PoolExit || p.StartPool == PoolBoss {
		return errf(`start_pool must be one of A-D`)
	}
	total := 0
	for pool, w := range p.Weights {
		if !pool.valid() || w < 0 {
			return errf(`bad weight %q: %d`, pool, w)
		}
		total += w
	}
	if total <= 0 || p.Weights[PoolPassage]+p.Weights[PoolFeature] <= 0 {
		return errf(`weights must allow at least one passage or feature room`)
	}
	for _, pool := range AllPools {
		d, ok := p.Doors[pool]
		if !ok {
			return errf(`doors: no range for pool %s`, pool)
		}
		min := 2
		if pool == PoolExit {
			min = 0 // the way out is its own exit
		}
		if d.Min < min || d.Max < d.Min {
			return errf(`doors[%s]: need %d <= min <= max, got %d..%d`, pool, min, d.Min, d.Max)
		}
		if p.Weights[pool] > 0 && len(p.templates[pool]) == 0 {
			return errf(`pool %s can be rolled but has no room templates`, pool)
		}
	}
	if len(p.templates[p.StartPool]) == 0 {
		return errf(`start pool %s has no room templates`, p.StartPool)
	}
	for _, pool := range AllPools {
		if c := p.OreChance[pool]; c < 0 || c > 100 {
			return errf(`ore_chance[%s] must be 0-100`, pool)
		}
	}
	for _, key := range RequiredMessages {
		if strings.TrimSpace(p.Messages[key]) == `` {
			return errf(`messages: %q is required`, key)
		}
	}
	if p.AmbientGeneratedChance < 0 || p.AmbientGeneratedChance > 100 {
		return errf(`ambient_generated_chance must be 0-100`)
	}
	if p.AmbientGeneratedChance > 0 && strings.TrimSpace(p.AmbientSetting) == `` {
		return errf(`ambient_setting is required when ambient_generated_chance is above 0`)
	}
	if len(p.IdleMessages) == 0 {
		return errf(`idlemessages: the rift's shared ambience is required`)
	}
	if p.JoinWindowSecs <= 0 {
		return errf(`join_window_seconds must be positive`)
	}
	if !exitNameRE.MatchString(p.PortalExit) || strings.TrimSpace(p.PortalLook) == `` || p.PortalMutator == `` {
		return errf(`portal_exit (one lowercase word), portal_look and portal_mutator are required`)
	}
	if p.PortalSites.Min < 1 || p.PortalSites.Max < p.PortalSites.Min {
		return errf(`portal_sites_per_region must be 1 <= min <= max`)
	}
	if len(p.PortalBiomes) == 0 {
		return errf(`portal_biomes is required`)
	}
	for _, chances := range []map[Pool]int{p.LightChance, p.WritingChance, p.Lore.Chance, p.Rubble.Chance} {
		for pool, c := range chances {
			if !pool.valid() || c < 0 || c > 100 {
				return errf(`chance %s=%d must be a pool A-F and 0-100`, pool, c)
			}
		}
	}
	if anyChance(p.LightChance) && p.LightItemId == 0 {
		return errf(`light_chance is set but light_item_id is not`)
	}
	if anyChance(p.Lore.Chance) || len(p.Writings) > 0 {
		l := p.Lore
		if l.Noun == `` || strings.TrimSpace(l.Look) == `` || strings.TrimSpace(l.Mention) == `` {
			return errf(`lore needs noun, look and mention`)
		}
		if l.Needed < 1 {
			return errf(`lore needs needed >= 1`)
		}
	}
	if anyChance(p.Rubble.Chance) {
		r := p.Rubble
		if r.Noun == `` || strings.TrimSpace(r.Look) == `` || strings.TrimSpace(r.Mention) == `` {
			return errf(`rubble needs noun, look and mention`)
		}
		for pool, c := range r.Chance {
			if c <= 0 {
				continue
			}
			switch r.Tier[pool] {
			case `cheap`, `average`, `rare`:
			default:
				return errf(`rubble tier for pool %s must be cheap, average or rare, got %q`, pool, r.Tier[pool])
			}
		}
	}
	if anyChance(p.WritingChance) && len(p.Writings) == 0 {
		return errf(`writing_chance is set but there are no writings`)
	}
	nouns := map[string]bool{p.Lore.Noun: true}
	if p.Rubble.Noun != `` {
		if nouns[p.Rubble.Noun] {
			return errf(`rubble noun %q is already the lore noun`, p.Rubble.Noun)
		}
		nouns[p.Rubble.Noun] = true
	}
	for i, w := range p.Writings {
		if w.Noun == `` || strings.TrimSpace(w.Look) == `` || strings.TrimSpace(w.Mention) == `` || strings.TrimSpace(w.Fragment) == `` {
			return errf(`writing %d needs noun, look, mention and fragment`, i+1)
		}
		if nouns[w.Noun] {
			return errf(`writing %d: noun %q is already used by the lore, the rubble or another writing`, i+1, w.Noun)
		}
		nouns[w.Noun] = true
	}
	if p.Losses != nil {
		if err := p.Losses.validate(); err != nil {
			return errf(`losses: %v`, err)
		}
	}
	if p.Memory != nil {
		if err := p.Memory.validate(); err != nil {
			return errf(`memory: %v`, err)
		}
		if nouns[p.Memory.Noun] {
			return errf(`memory noun %q is already used by the lore, the rubble or a writing`, p.Memory.Noun)
		}
	}
	if p.Weights[PoolMonster] > 0 && len(p.Mobs.Trash)+len(p.Mobs.Elite) == 0 {
		return errf(`monster rooms can be rolled but there are no trash or elite mobs`)
	}
	if p.Weights[PoolBoss] > 0 && len(p.Mobs.Boss) == 0 {
		return errf(`boss rooms can be rolled but there are no boss mobs`)
	}
	for pool, c := range p.Lurkers.Chance {
		if !pool.valid() || c < 0 || c > 100 {
			return errf(`lurkers: bad chance %q: %d`, pool, c)
		}
		if c > 0 && len(p.Lurkers.Mobs) == 0 {
			return errf(`lurkers: a chance with no mobs`)
		}
	}
	if p.StatPoolPerDepth < 0 {
		return errf(`statpool_per_depth must not be negative`)
	}
	if p.Mobs.Hunter != 0 {
		d := p.HunterDelaySeconds
		if d.Min < 1 || d.Max < d.Min {
			return errf(`hunter_delay_seconds needs 1 <= min <= max`)
		}
		if p.StatPools.Hunter < 1 {
			return errf(`statpools.hunter is required with a hunter`)
		}
	}

	if withWorld {
		ids := append(append(append(append([]int{}, p.Mobs.Trash...), p.Mobs.Elite...), p.Mobs.Boss...), p.Lurkers.Mobs...)
		if p.Mobs.Hunter != 0 {
			ids = append(ids, p.Mobs.Hunter)
		}
		for _, id := range ids {
			spec := mobs.GetMobSpec(mobs.MobId(id))
			if spec == nil {
				return errf(`unknown mob %d`, id)
			}
			// A mob that subdues instead of killing sends its victim home
			// with no death and so no loss: a free way out of the rift.
			policy := characters.DefaultSubmissionPolicyForArchetype(spec.BehaviorArchetype)
			if spec.SubmissionPolicy != `` {
				if p, ok := characters.ParseSubmissionPolicy(spec.SubmissionPolicy); ok {
					policy = p
				}
			}
			if policy != characters.PolicyLethal {
				return errf(`mob %d must have submission_policy: lethal (a rift mob that subdues lets its victim out for free)`, id)
			}
		}
		spec := items.GetItemSpec(p.KeyItemId)
		if spec == nil || spec.Type != items.Key {
			return errf(`key_item_id %d must be an item of type key`, p.KeyItemId)
		}
		if spec.KeyLockId != `` {
			return errf(`key_item_id %d must not have a keylockid: it is a generic rift key`, p.KeyItemId)
		}
		for _, id := range p.OreItems {
			if items.GetItemSpec(id) == nil {
				return errf(`unknown ore item %d`, id)
			}
		}
		for _, id := range p.TrapConditionIds {
			if !conditions.HasSpec(id) {
				return errf(`unknown trap condition %d`, id)
			}
		}
		if p.LightItemId != 0 {
			if ls := items.GetItemSpec(p.LightItemId); ls == nil || ls.Type != items.Light {
				return errf(`light_item_id %d must be an item of type light`, p.LightItemId)
			}
		}
	}
	if err := p.Generation.validate(withWorld); err != nil {
		return errf(`generation: %v`, err)
	}
	if anyChance(p.OreChance) && (len(p.OreItems) == 0 || p.OreWeight <= 0) {
		return errf(`ore_chance is set but ore_items or ore_weight is missing`)
	}

	for _, pool := range AllPools {
		for _, t := range p.templates[pool] {
			if err := t.validate(p, withWorld); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Template) validate(p *Profile, withWorld bool) error {
	errf := func(format string, args ...any) error {
		return fmt.Errorf(`rift room %q (profile %q): %s`, t.Id, p.Id, fmt.Sprintf(format, args...))
	}
	if !t.Pool.valid() {
		return errf(`unknown pool %q`, t.Pool)
	}
	if strings.TrimSpace(t.Title) == `` || strings.TrimSpace(t.Description) == `` {
		return errf(`title and description are required`)
	}
	if want := casing.Title(t.Title); want != t.Title {
		return errf(`title %q is not canonical title case (expected %q)`, t.Title, want)
	}
	switch t.Source {
	case ``, `authored`, `generated`:
	default:
		return errf(`source must be authored or generated, not %q`, t.Source)
	}
	if need := p.Doors[t.Pool].Max; len(t.Doors) < need {
		return errf(`needs at least %d doors for pool %s, has %d`, need, t.Pool, len(t.Doors))
	}
	seen := map[string]bool{p.ExitName: true}
	for _, d := range t.Doors {
		if !exitNameRE.MatchString(d.Exit) {
			return errf(`door exit %q must be one lowercase word (letters only)`, d.Exit)
		}
		if cmd := shadowsCommand(d.Exit); cmd != `` {
			return errf(`door exit %q would take over the command %q (exits are matched before commands)`, d.Exit, cmd)
		}
		if seen[d.Exit] {
			return errf(`door exit %q is used twice (or clashes with exit_name)`, d.Exit)
		}
		seen[d.Exit] = true
		if strings.TrimSpace(d.Description) == `` {
			return errf(`door %q needs a description`, d.Exit)
		}
	}
	switch t.Pool {
	case PoolPuzzle:
		if t.Puzzle == nil && t.Trap == nil {
			return errf(`a puzzle room needs a puzzle, a trap, or both`)
		}
	case PoolMonster:
		if t.Encounter == nil {
			return errf(`a monster room needs an encounter`)
		}
	}
	if t.Puzzle != nil {
		if err := t.Puzzle.validate(t, withWorld); err != nil {
			return errf(`puzzle: %v`, err)
		}
		if t.Puzzle.Kind == `memory` {
			if p.Memory == nil {
				return errf(`puzzle: a memory puzzle needs the profile's memory block`)
			}
			if _, clash := t.Nouns[p.Memory.Noun]; clash {
				return errf(`puzzle: the room has its own %q noun; the lens table needs it`, p.Memory.Noun)
			}
			for _, d := range t.Doors {
				if d.Exit == p.Memory.Noun {
					return errf(`puzzle: a door is named %q, the lens table's noun`, d.Exit)
				}
			}
		}
	}
	if t.Trap != nil {
		if t.Trap.Difficulty <= 0 || t.Trap.Triggered == `` || t.Trap.Avoided == `` {
			return errf(`trap needs difficulty, triggered and avoided`)
		}
		if withWorld {
			for _, id := range t.Trap.ConditionIds {
				if !conditions.HasSpec(id) {
					return errf(`trap: unknown condition %d`, id)
				}
			}
		}
	}
	if t.Encounter != nil {
		if t.Encounter.Tier != `trash` && t.Encounter.Tier != `elite` {
			return errf(`encounter tier must be trash or elite`)
		}
		if t.Encounter.Count.Min < 1 || t.Encounter.Count.Max < t.Encounter.Count.Min {
			return errf(`encounter count must be 1 <= min <= max`)
		}
	}
	return nil
}

func (ps *PuzzleSpec) validate(t *Template, withWorld bool) error {
	switch ps.Kind {
	case `riddle`:
		if len(ps.Answers) == 0 {
			return fmt.Errorf(`a riddle needs answers`)
		}
	case `memory`:
		// The table is the profile's; the board is laid out when the room
		// is built. A memory puzzle brings only its door and its reward.
		if len(ps.Answers) > 0 || len(ps.Sequence) > 0 {
			return fmt.Errorf(`a memory puzzle has no answers or sequence`)
		}
	case `sequence`:
		if len(ps.Sequence) < 2 {
			return fmt.Errorf(`a sequence needs at least two nouns`)
		}
		for _, n := range ps.Sequence {
			if _, ok := t.Nouns[n]; !ok {
				return fmt.Errorf(`sequence noun %q is not one of the room's nouns`, n)
			}
		}
	default:
		return fmt.Errorf(`unknown kind %q (memory, riddle or sequence)`, ps.Kind)
	}
	if ps.SealedDoor == `` {
		return fmt.Errorf(`sealed_door is required: name the door the puzzle holds shut`)
	}
	found := false
	for _, d := range t.Doors {
		found = found || d.Exit == ps.SealedDoor
	}
	if !found {
		return fmt.Errorf(`sealed_door %q is not one of the room's doors`, ps.SealedDoor)
	}
	switch ps.Reward {
	case ``, `none`, `ore`:
		if ps.Kind == `memory` {
			// Solving a lens table is how a key is earned (besides a boss).
			return fmt.Errorf(`a memory puzzle rewards a key`)
		}
	case `key`:
		if ps.Kind != `memory` {
			return fmt.Errorf(`only a memory puzzle (the lens table) may reward a key`)
		}
		if !ps.Hard {
			return fmt.Errorf(`only a hard puzzle may reward a key`)
		}
	default:
		return fmt.Errorf(`unknown reward %q`, ps.Reward)
	}
	if ps.Solved == `` {
		return fmt.Errorf(`solved is required`)
	}
	if ps.Kind != `memory` && ps.Wrong == `` {
		// A lens table's mistakes are told by the profile's memory block.
		return fmt.Errorf(`wrong is required`)
	}
	if withWorld {
		for _, id := range ps.WrongConditionIds {
			if !conditions.HasSpec(id) {
				return fmt.Errorf(`unknown condition %d`, id)
			}
		}
	}
	return nil
}

// anyChance reports whether any pool has a chance above zero.
func anyChance(chances map[Pool]int) bool {
	for _, c := range chances {
		if c > 0 {
			return true
		}
	}
	return false
}

// ValidateMutators checks every profile's portal_mutator against the loaded
// mutators. Mutators load late in boot (after LoadDataFiles), so main.go calls
// this right after mutators.LoadDataFiles. It panics like the loader.
func ValidateMutators() {
	for _, id := range ProfileIds() {
		p := profiles[id]
		if !mutators.IsMutator(p.PortalMutator) {
			panic(fmt.Sprintf(`rifts.ValidateMutators: rift profile %q: unknown portal_mutator %q`, p.Id, p.PortalMutator))
		}
	}
}

// ownsMob reports whether mobId is one of the profile's own creatures (any
// tier, or its hunter).
func (p *Profile) ownsMob(mobId int) bool {
	if mobId == 0 {
		return false
	}
	if mobId == p.Mobs.Hunter {
		return true
	}
	for _, list := range [][]int{p.Mobs.Trash, p.Mobs.Elite, p.Mobs.Boss, p.Lurkers.Mobs} {
		for _, id := range list {
			if id == mobId {
				return true
			}
		}
	}
	return false
}

func (g GenerationSpec) validate(withWorld bool) error {
	if len(g.Pools) == 0 {
		return nil
	}
	if strings.TrimSpace(g.Setting) == `` {
		return fmt.Errorf(`setting is required when pools are set`)
	}
	seen := map[Pool]bool{}
	for _, pool := range g.Pools {
		if !pool.valid() || seen[pool] {
			return fmt.Errorf(`bad or repeated pool %q`, pool)
		}
		seen[pool] = true
		if strings.TrimSpace(g.Guides[pool]) == `` {
			return fmt.Errorf(`guides: pool %s needs a guide`, pool)
		}
	}
	if len(g.Effects) == 0 {
		return fmt.Errorf(`effects: at least one is required`)
	}
	for word, id := range g.Effects {
		if !exitNameRE.MatchString(word) {
			return fmt.Errorf(`effect %q must be one lowercase word`, word)
		}
		if withWorld && !conditions.HasSpec(id) {
			return fmt.Errorf(`effect %q: unknown condition %d`, word, id)
		}
	}
	if len(g.Seeds) == 0 {
		return fmt.Errorf(`seeds: at least one is required`)
	}
	return nil
}

// dropBadGenerated leaves out every generated room that no longer passes
// validation (a rule tightened since it was written, a condition removed, a
// hand edit gone wrong), with a warning: a machine-written room never stops
// the server. Authored rooms are left for validate to judge.
func (p *Profile) dropBadGenerated(withWorld bool) {
	for pool, list := range p.templates {
		kept := list[:0]
		for _, t := range list {
			if t.Source == `generated` || strings.HasPrefix(t.Id, `gen-`) {
				if err := t.validate(p, withWorld); err != nil {
					mudlog.Warn(`rifts.LoadDataFiles`, `skipped generated room`, t.Id, `error`, err)
					continue
				}
			}
			kept = append(kept, t)
		}
		p.templates[pool] = kept
	}
}
