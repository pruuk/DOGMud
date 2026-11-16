// Package movenarration holds the shipped wording for special-move narration:
// the player-side verbs (kick, bash, trip, ...) and their mob twins.
//
// Go decides WHICH event fires and names it by key. Go holds no wording. The
// three audiences of one event are variant-paired by index, which is why
// Validate refuses pools of unequal length: variant N of each role describes
// the same moment.
package movenarration

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// This store's event tokens, beyond the four canonical name tokens every store
// shares. Damage always arrives pre-formatted as prose from
// combat.GetDamageDescription; there is no numeric verb anywhere in the
// special-move surface.
const (
	TokenDamage   = "{damage}"   // a damage description, already prose
	TokenLabel    = "{label}"    // bash's species-varying noun
	TokenWith     = "{with}"     // bash's species-varying instrument
	TokenVerb     = "{verb}"     // bash's species-varying verb
	TokenWeapon   = "{weapon}"   // the ranged weapon being fired
	TokenExitName = "{exitname}" // an exit, for ranged shots across rooms
	TokenPosition = "{position}" // grapple's position description
	TokenItem     = "{item}"     // throw's thrown item name, already prose
	TokenHeal     = "{heal}"     // a heal description, already prose (drain's lifesteal)
)

// tokenPattern and allowedTokens exist because textutil.ValidateTokens knows
// only the four canonical NAME tokens, so every event token in every store
// ships unvalidated today. That is a real hole: a value reading {exit_name}
// where the caller fills {exitname} renders the token itself to the player, and
// nothing catches it.
//
// This store declares its own vocabulary and rejects anything outside it, so a
// typo fails the boot instead of reaching a player. Widening the set is a
// deliberate edit here, which is the point.
var tokenPattern = regexp.MustCompile(`\{[a-z_]+\}`)

var allowedTokens = map[string]bool{
	narration.TokenActor:      true,
	narration.TokenActee:      true,
	narration.TokenActorPlain: true,
	narration.TokenActeePlain: true,
	TokenDamage:               true,
	TokenLabel:                true,
	TokenWith:                 true,
	TokenVerb:                 true,
	TokenWeapon:               true,
	TokenExitName:             true,
	TokenPosition:             true,
	TokenItem:                 true,
	TokenHeal:                 true,
}

// EventKey names one outcome branch of one verb, such as "standard_hit".
//
// Where a verb has an orthogonal variant axis (kick resolves as stomp, knee or
// standard; trip as tailsweep or trip), the key is <variant>_<outcome>, because
// an outcome key alone would make three different moves collide on one pool.
type EventKey string

// EventMessages is one event as each audience is told it. The yaml keys are the
// arc's canonical role keys.
type EventMessages struct {
	Actor          []string `yaml:"actor"`
	Actee          []string `yaml:"actee"`
	Observer       []string `yaml:"observer"`
	RemoteObserver []string `yaml:"remote_observer"`
}

// MoveNarrationGroup is one verb's file.
type MoveNarrationGroup struct {
	MoveId string                      `yaml:"moveid"`
	Events map[EventKey]*EventMessages `yaml:"events"`
}

func (g *MoveNarrationGroup) Id() string { return g.MoveId }

func (g *MoveNarrationGroup) Filepath() string {
	return fmt.Sprintf("%s.yaml", g.MoveId)
}

// Validate fails the boot on any malformed event. This store is EVENT tier
// under the arc's two-tier loader policy: silence here means a move lands with
// no narration at all, so it must never be survivable.
func (g *MoveNarrationGroup) Validate() error {
	if g.MoveId == "" {
		return errors.New("moveid is empty")
	}
	if len(g.Events) == 0 {
		return fmt.Errorf("move %q declares no events", g.MoveId)
	}
	for key, ev := range g.Events {
		if ev == nil {
			return fmt.Errorf("move %q event %q is empty", g.MoveId, key)
		}
		if err := narration.ValidateVariants(ev.variants(), 1); err != nil {
			return fmt.Errorf("move %q event %q: %w", g.MoveId, key, err)
		}
		if err := validateEventTokens(g.MoveId, key, ev); err != nil {
			return err
		}
	}
	return nil
}

// validateEventTokens rejects any {token} this store does not fill. An unknown
// token is not a warning: nothing downstream would replace it, so it would
// render its own braces to the player.
func validateEventTokens(moveId string, key EventKey, ev *EventMessages) error {
	for _, pool := range [][]string{ev.Actor, ev.Actee, ev.Observer, ev.RemoteObserver} {
		for i, text := range pool {
			for _, tok := range tokenPattern.FindAllString(text, -1) {
				if !allowedTokens[tok] {
					return fmt.Errorf("move %q event %q variant %d uses unknown token %s", moveId, key, i, tok)
				}
			}
		}
	}
	return nil
}

func (e *EventMessages) variants() narration.Variants {
	return narration.Variants{
		Actor:         e.Actor,
		Actee:         e.Actee,
		Observer:      e.Observer,
		ActeeObserver: e.RemoteObserver,
	}
}

// Variants returns the event's pools, or ok=false if the verb does not declare
// that event.
func (g *MoveNarrationGroup) Variants(key EventKey) (narration.Variants, bool) {
	ev, ok := g.Events[key]
	if !ok || ev == nil {
		return narration.Variants{}, false
	}
	return ev.variants(), true
}

var loadedMoves map[string]*MoveNarrationGroup

// LoadFrom loads the store from an explicit directory, returning the error
// rather than panicking. It exists because a TEST binary never reads
// config.yaml, so configs.GetFilePathsConfig would hand back
// _datafiles/world/default, which does not carry this store; a test that
// needs real prose loads it explicitly from the shipped dogmud world dir.
func LoadFrom(dir string) error {
	loaded, err := fileloader.LoadAllFlatFiles[string, *MoveNarrationGroup](dir)
	if err != nil {
		return fmt.Errorf("loading special-move narration: %w", err)
	}
	loadedMoves = loaded
	return nil
}

// LoadMoveNarrationFiles loads the store at boot. It panics on any failure,
// matching combat.LoadTauntMessageFiles: this is event narration, not ambient.
func LoadMoveNarrationFiles() {
	dir := string(configs.GetFilePathsConfig().DataFiles) + `/narration/special-moves`
	if err := LoadFrom(dir); err != nil {
		panic(err)
	}
}

// GetMove returns one verb's group, or nil if the store is unloaded or the verb
// is absent.
func GetMove(moveId string) *MoveNarrationGroup {
	if loadedMoves == nil {
		return nil
	}
	return loadedMoves[moveId]
}
