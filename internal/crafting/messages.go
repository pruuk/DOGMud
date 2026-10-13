package crafting

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// TokenItem names the thing being made in a crafting-messages line. It is
// this store's own event token, like movenarration's {item}: the store puts it
// in the token map it hands narration.Render, and the shared vocabulary never
// sees it.
const TokenItem = "{item}"

// unknownItem is what {item} reads when the reader cannot make the item out,
// or the recipe makes no item (an enchant).
const unknownItem = "something"

// minCraftingVariants is the floor for every pool, casting's floor.
const minCraftingVariants = 3

// SkillPools is one skill's room lines (messaging M6 row 7, owner rulings
// R1 to R3). The *_observer pools are seen lines naming the crafter with
// {actor}; the *_heard pools are what a reader who cannot see the crafter
// hears, and name nobody. finish_heard serves success and failure alike so
// a sound never gives the outcome away.
type SkillPools struct {
	StartObserver   []string `yaml:"start_observer"`
	SuccessObserver []string `yaml:"success_observer"`
	FailureObserver []string `yaml:"failure_observer"`
	StartHeard      []string `yaml:"start_heard"`
	FinishHeard     []string `yaml:"finish_heard"`
}

// CraftingMessages is the store: recipe skill to that skill's pools.
type CraftingMessages map[string]SkillPools

var craftingMessages CraftingMessages

var craftTokenRE = regexp.MustCompile(`\{[a-z_]+\}`)

// craftPool is one pool with the rules its lines obey.
type craftPool struct {
	key     string
	lines   []string
	seen    bool            // validates as Observer and must name {actor}
	allowed map[string]bool // tokens a line may use
}

func (p SkillPools) pools() []craftPool {
	actorOnly := map[string]bool{narration.TokenActor: true, narration.TokenActorPlain: true}
	withItem := map[string]bool{narration.TokenActor: true, narration.TokenActorPlain: true, TokenItem: true}
	return []craftPool{
		{"start_observer", p.StartObserver, true, withItem},
		{"success_observer", p.SuccessObserver, true, withItem},
		{"failure_observer", p.FailureObserver, true, actorOnly},
		{"start_heard", p.StartHeard, false, nil},
		{"finish_heard", p.FinishHeard, false, nil},
	}
}

// Validate checks every pool of every block, and that the blocks match the
// recipe skills exactly: a recipe skill with no block would craft in
// silence, and a block no recipe uses is most likely a misspelling.
func (cm CraftingMessages) Validate(recipeSkills []string) error {
	want := map[string]bool{}
	for _, s := range recipeSkills {
		want[s] = true
		if _, ok := cm[s]; !ok {
			return fmt.Errorf("crafting-messages: no block for skill %q", s)
		}
	}
	names := make([]string, 0, len(cm))
	for s := range cm {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, skill := range names {
		if !want[skill] {
			return fmt.Errorf("crafting-messages: skill %q has no recipes (misspelled?)", skill)
		}
		for _, p := range cm[skill].pools() {
			v, role := narration.Variants{Actor: p.lines}, narration.RoleActor
			if p.seen {
				v, role = narration.Variants{Observer: p.lines}, narration.RoleObserver
			}
			if err := narration.ValidateVariants(v, minCraftingVariants, role); err != nil {
				return fmt.Errorf("crafting-messages %s %s: %w", skill, p.key, err)
			}
			for i, line := range p.lines {
				if p.seen && !strings.Contains(line, narration.TokenActor) {
					return fmt.Errorf("crafting-messages %s: %s must name the crafter with %s (variant %d)", skill, p.key, narration.TokenActor, i)
				}
				for _, tok := range craftTokenRE.FindAllString(line, -1) {
					if !p.allowed[tok] {
						return fmt.Errorf("crafting-messages %s: %s may not use %s (variant %d)", skill, p.key, tok, i)
					}
				}
			}
		}
	}
	return nil
}

// LoadCraftingMessages reads and validates the store against the loaded
// recipes. Called once at startup from main.go, after LoadRecipeFiles. It
// panics on a missing, unparseable or invalid file, as LoadCastingMessages
// does: no Go fallback text shadows shipped data.
func LoadCraftingMessages() {
	start := time.Now()

	path := string(configs.GetFilePathsConfig().DataFiles) + `/crafting-messages.yaml`
	data, err := os.ReadFile(path)
	if err != nil {
		panic(errors.Wrap(err, "reading "+path))
	}
	var cm CraftingMessages
	if err := yaml.Unmarshal(data, &cm); err != nil {
		panic(errors.Wrap(err, "parsing "+path))
	}
	if err := cm.Validate(RecipeSkills()); err != nil {
		panic(errors.Wrap(err, "validating "+path))
	}
	craftingMessages = cm
	mudlog.Info("crafting.LoadCraftingMessages()", "skills", len(cm), "Time Taken", time.Since(start))
}

// RecipeSkills lists the distinct skills of the loaded recipes, sorted.
func RecipeSkills() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range allRecipes {
		if !seen[r.Skill] {
			seen[r.Skill] = true
			out = append(out, r.Skill)
		}
	}
	sort.Strings(out)
	return out
}

// SeedCraftingMessagesForTest installs cm (nil unloads the store) and
// returns the restore func for t.Cleanup. It does not validate.
func SeedCraftingMessagesForTest(cm CraftingMessages) func() {
	prev := craftingMessages
	craftingMessages = cm
	return func() { craftingMessages = prev }
}

func pickerOf(picker []narration.Picker) narration.Picker {
	if len(picker) > 0 && picker[0] != nil {
		return picker[0]
	}
	return narration.DefaultPicker
}

// CraftRoomLines renders the seen line for one moment of a skill's craft,
// twice from ONE variant: full names the item (item, already formatted;
// "" for none), vague says "something" for a reader who makes out only
// shapes. Both are "" when the store has no pool for skill.
func CraftRoomLines(skill string, p Phase, ctx textutil.TokenContext, item string, picker ...narration.Picker) (full, vague string) {
	pools, ok := craftingMessages[skill]
	if !ok {
		return "", ""
	}
	var pool []string
	switch p {
	case PhaseStart:
		pool = pools.StartObserver
	case PhaseSuccess:
		pool = pools.SuccessObserver
	case PhaseFailure:
		pool = pools.FailureObserver
	}
	if len(pool) == 0 {
		return "", ""
	}
	index := pickerOf(picker)(len(pool))
	v := narration.Variants{Observer: pool}
	tokens := ctx.Tokens()
	if item == "" {
		item = unknownItem
	}
	// The index is already drawn; FirstPicker keeps Render from drawing a
	// second, unused random number before the override applies.
	tokens[TokenItem] = item
	full = narration.Render(v, tokens, narration.FirstPicker, index).Observer
	tokens[TokenItem] = unknownItem
	vague = narration.Render(v, tokens, narration.FirstPicker, index).Observer
	return full, vague
}

// CraftHeardLine renders what a reader who cannot see the crafter hears:
// the start pool at PhaseStart, the finish pool at success and at failure.
// "" when the store has no pool for skill.
func CraftHeardLine(skill string, p Phase, picker ...narration.Picker) string {
	pools, ok := craftingMessages[skill]
	if !ok {
		return ""
	}
	pool := pools.FinishHeard
	if p == PhaseStart {
		pool = pools.StartHeard
	}
	if len(pool) == 0 {
		return ""
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pickerOf(picker)).Actor
}

// SeedRecipesForTest installs the recipe registry (nil unloads it), clears
// the output-id index, and returns the restore func for t.Cleanup. It does
// not validate.
func SeedRecipesForTest(recipes map[string]*RecipeSpec) func() {
	prev, prevIdx := allRecipes, recipeByOutputId
	allRecipes, recipeByOutputId = recipes, nil
	return func() { allRecipes, recipeByOutputId = prev, prevIdx }
}
