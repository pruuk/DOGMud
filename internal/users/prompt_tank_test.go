package users

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// promptRendererCases returns every string literal a case of
// ProcessPromptString's token switch matches, read from the source.
func promptRendererCases(t *testing.T) map[string]bool {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(here), "userrecord.prompt.go")
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	cases := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "ProcessPromptString" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						cases[s] = true
					}
				}
			}
			return true
		})
	}
	if !cases[`{hp}`] || !cases[`{target}`] {
		t.Fatalf("the source scan found no token cases (%d found); the scan is broken", len(cases))
	}
	return cases
}

// shippedTextFormats reads TextFormats from the shipped _datafiles/config.yaml.
// A test binary never loads it, so the file is read directly.
func shippedTextFormats(t *testing.T) (prompt, fightPrompt string) {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		TextFormats struct {
			Prompt      string `yaml:"Prompt"`
			FightPrompt string `yaml:"FightPrompt"`
		} `yaml:"TextFormats"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if doc.TextFormats.Prompt == `` || doc.TextFormats.FightPrompt == `` {
		t.Fatalf("config.yaml TextFormats.Prompt/FightPrompt not found")
	}
	return doc.TextFormats.Prompt, doc.TextFormats.FightPrompt
}

// TestShippedPromptTokensAllRender is #591: the shipped FightPrompt carried
// {tankpos} and {tankbar} for eight months after their renderer cases were
// deleted, and they printed nothing. Every token in every prompt the game
// ships (config.yaml, the Go defaults, and the toggle-built fight prompt)
// must have a case in ProcessPromptString.
func TestShippedPromptTokensAllRender(t *testing.T) {
	cases := promptRendererCases(t)

	yamlPrompt, yamlFight := shippedTextFormats(t)
	var defaults configs.TextFormats
	defaults.Validate()
	allOn := (&UserRecord{}).BuildFightPromptTemplate()

	prompts := map[string]string{
		"config.yaml Prompt":          yamlPrompt,
		"config.yaml FightPrompt":     yamlFight,
		"Go default Prompt":           defaults.Prompt.String(),
		"Go default FightPrompt":      defaults.FightPrompt.String(),
		"BuildFightPromptTemplate on": allOn,
	}
	sawTank := false
	for name, p := range prompts {
		tokens := promptFindTagsRegex.FindAllString(p, -1)
		if len(tokens) == 0 {
			t.Errorf("%s: found no tokens in %q", name, p)
		}
		var missing []string
		for _, tok := range tokens {
			if tok == `{tankbar}` {
				sawTank = true
			}
			if !cases[tok] {
				missing = append(missing, tok)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s uses tokens with no renderer case: %v", name, missing)
		}
	}
	if !sawTank {
		t.Error("no shipped prompt carries {tankbar}; this guard no longer covers #591")
	}
}

// tankFixture seeds a player fighting a mob, and the mob's victim.
type tankFixture struct {
	player *UserRecord
	tank   *UserRecord
	mob    *mobs.Mob
	pet    *mobs.Mob
}

func newTankFixture(t *testing.T) *tankFixture {
	t.Helper()
	player := NewTestUser(7101, "striker", "Striker", 0)
	tank := NewTestUser(7102, "tanker", "Tanker", 0)
	tank.Character.Health = 40

	mob := &mobs.Mob{InstanceId: 501, Character: characters.Character{Name: "Steppe Wolf"}}
	mob.Character.Health = 50
	mob.Character.HealthMax.Value = 50
	pet := &mobs.Mob{InstanceId: 502, Character: characters.Character{Name: "Fido", Position: position.NewMachine()}}
	pet.Character.Health = 30
	pet.Character.HealthMax.Value = 30

	t.Cleanup(SeedUsersForTest(map[int]*UserRecord{player.UserId: player, tank.UserId: tank}))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{501: mob, 502: pet}))

	player.Character.SetAggro(0, 501, characters.DefaultAttack)
	mob.Character.SetAggro(tank.UserId, 0, characters.DefaultAttack)
	return &tankFixture{player: player, tank: tank, mob: mob, pet: pet}
}

const tankBlock = `{tank}{tankpos}{tankbar}`

// TestPromptTankTokens: {tank}, {tankpos} and {tankbar} show who the
// player's target is attacking, that character's position and health
// (#591), and nothing when the target is attacking the player.
func TestPromptTankTokens(t *testing.T) {
	defer SetPromptSightCheck(nil)
	fx := newTankFixture(t)

	render := func(s string) string { return promptVisible(fx.player.ProcessPromptString(s)) }
	wantBar := promptVisible(renderVitalBar(40, 100, 0))

	// Each token on its own.
	if got := render(`{tank}`); got != ` T:Tanker` {
		t.Errorf("{tank} = %q, want %q", got, ` T:Tanker`)
	}
	if got := render(`{tankpos}`); got != `` {
		t.Errorf("{tankpos} for a standing tank = %q, want empty", got)
	}
	if got := render(`{tankbar}`); got != ` Thp:`+wantBar {
		t.Errorf("{tankbar} = %q, want %q", got, ` Thp:`+wantBar)
	}

	// The tank goes down: the position shows between name and bar.
	r := state.TransitionReason{Trigger: "test_setup"}
	if err := fx.tank.Character.Position.TransitionToProne(position.ProneData{}, r); err != nil {
		t.Fatalf("prone: %v", err)
	}
	if got, want := render(tankBlock), ` T:Tanker Prone Thp:`+wantBar; got != want {
		t.Errorf("prone tank block = %q, want %q", got, want)
	}

	// The shipped fight prompt, end to end.
	_, fight := shippedTextFormats(t)
	line := render(util.ConvertColorShortTags(fight))
	if !strings.Contains(line, `» Steppe Wolf[healthy] T:Tanker Prone Thp:`+wantBar+`:`) {
		t.Errorf("shipped FightPrompt line lacks the tank block: %q", line)
	}
	t.Logf("shipped FightPrompt renders: %s", line)

	// A companion tanking: a mob tank is named too.
	fx.mob.Character.SetAggro(0, fx.pet.InstanceId, characters.DefaultAttack)
	if got := render(tankBlock); !strings.HasPrefix(got, ` T:Fido Thp:`) {
		t.Errorf("pet tank block = %q, want prefix %q", got, ` T:Fido Thp:`)
	}

	// The target is hitting the player: nobody else is tanking.
	fx.mob.Character.SetAggro(fx.player.UserId, 0, characters.DefaultAttack)
	if got := render(tankBlock); got != `` {
		t.Errorf("target attacking the player: tank block = %q, want empty", got)
	}

	// Can't see the fight: who the target hits is unknown.
	fx.mob.Character.SetAggro(fx.tank.UserId, 0, characters.DefaultAttack)
	SetPromptSightCheck(func(*characters.Character) messaging.SightDecision { return messaging.SightNone })
	if got := render(tankBlock); got != `` {
		t.Errorf("unseen fight: tank block = %q, want empty", got)
	}
}

// TestPromptTankStaleToggleCache: a player who toggled a fight prompt
// element before #591 has a saved compiled template whose tank block is a
// bare {tank}. The prompt renders from today's template, so they get the
// position and bar too.
func TestPromptTankStaleToggleCache(t *testing.T) {
	fx := newTankFixture(t)
	fx.player.SetConfigOption(`fprompt-tog-bars`, false)
	fx.player.SetConfigOption(`fprompt-default-compiled`,
		util.ConvertColorShortTags(`{8}[{t}{8}] {255}» {target}{tank}{239}{h}{8}:`))
	if !fx.player.Character.IsInCombat() {
		t.Fatal("fixture player is not in combat")
	}
	got := promptVisible(fx.player.GetCommandPrompt())
	if !strings.Contains(got, ` T:Tanker Thp:`) {
		t.Errorf("stale toggle cache: prompt = %q, want the tank bar", got)
	}
}
