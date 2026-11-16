package pets

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

type Pet struct {
	Name          string            `yaml:"name,omitempty"`          // Name of the pet (player provided hopefully)
	NameStyle     string            `yaml:"namestyle,omitempty"`     // Optional color pattern to apply
	Type          string            `yaml:"type"`                    // type of pet
	Food          Food              `yaml:"food,omitempty"`          // how much food the pet has
	LastMealRound uint8             `yaml:"lastmealround,omitempty"` // When the pet was last fed
	Damage        items.Damage      `yaml:"damage,omitempty"`        // When the pet was last fed
	StatMods      statmods.StatMods `yaml:"statmods,omitempty"`      // stat mods the pet provides
	ConditionIds  []int             `yaml:"conditionids,omitempty"`  // Permanent conditions this pet affords the player
	Capacity      int               `yaml:"capacity,omitempty"`      // How many items this mob can carry
	Items         []items.Item      `yaml:"items,omitempty"`         // Items held by this pet
}

var (
	petTypes = map[string]*Pet{}
)

func (p *Pet) StatMod(statName string) int {
	return p.StatMods.Get(statName)
}

func (p *Pet) Exists() bool {
	return p.Type != ``
}

// PlainName is the pet's display name with no styling applied: p.Name if
// set, else p.Type. It's what DisplayName wraps in either an ansi identity
// tag or a per-character color pattern, and it's the bare substring
// messaging.HideNames needs to search for -- see combat's
// hideIdentitiesInPersonalLines, which hides a blind defender's attacker's
// pet by this string, not by DisplayName's decorated output.
func (p *Pet) PlainName() string {
	name := p.Name
	if name == `` {
		name = p.Type
	}
	return name
}

func (p *Pet) DisplayName() string {

	name := p.PlainName()

	if len(p.NameStyle) > 0 {
		patternName := p.NameStyle
		if patternName[0:1] == `:` {
			patternName = patternName[1:]
		}
		return colorpatterns.ApplyColorPattern(name, patternName)
	}

	return fmt.Sprintf(`<ansi fg="petname">%s</ansi>`, name)
}

func (p *Pet) StoreItem(i items.Item) bool {

	if p.Capacity < 1 {
		return false
	}

	if i.ItemId < 1 {
		return false
	}
	i.Validate()
	// A found bauble someone carries is no longer lying anywhere, exactly as
	// in Character.StoreItem (items/bauble_placement.go).
	i.ClearBaublePlacement()
	p.Items = append(p.Items, i)
	return true
}

func (p *Pet) RemoveItem(i items.Item) bool {

	for j := len(p.Items) - 1; j >= 0; j-- {
		if p.Items[j].Equals(i) {
			p.Items = append(p.Items[:j], p.Items[j+1:]...)
			return true
		}
	}
	return false
}

func (p *Pet) GetConditions() []int {
	return append([]int{}, p.ConditionIds...)
}

func (p *Pet) FindItem(itemName string) (items.Item, bool) {

	if itemName == `` {
		return items.Item{}, false
	}

	closeMatchItem, matchItem := items.FindMatchIn(itemName, p.Items...)

	if matchItem.ItemId != 0 {
		return matchItem, true
	}

	if closeMatchItem.ItemId != 0 {
		return closeMatchItem, true
	}

	return items.Item{}, false
}

func (p *Pet) GetDiceRoll() (attacks int, dCount int, dSides int, bonus int, conditionOnCrit []int) {
	return p.Damage.Attacks, p.Damage.DiceCount, p.Damage.SideCount, p.Damage.BonusDamage, p.Damage.CritConditionIds
}

func GetPetCopy(petId string) Pet {
	if petInfo, ok := petTypes[petId]; ok {
		return *petInfo
	}
	return Pet{}
}

func GetPetSpec(petId string) Pet {
	if petInfo, ok := petTypes[petId]; ok {
		return *petInfo
	}
	return Pet{}
}

func (p *Pet) Filename() string {
	filename := util.ConvertForFilename(p.Type)
	return fmt.Sprintf("%s.yaml", filename)
}

func (p *Pet) Filepath() string {
	return p.Filename()
}

func (p *Pet) Save() error {
	fileName := strings.ToLower(p.Name)

	bytes, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

	saveFilePath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/`, `pets`, `/`, fmt.Sprintf("%s.yaml", fileName))

	// Durable atomic write (chunk 2.8): a pet carries its own progression and
	// is player-owned living state.
	err = util.Save(saveFilePath, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pet) Id() string {
	return p.Type
}

func (p *Pet) Validate() error {

	if p.ConditionIds == nil {
		p.ConditionIds = []int{}
	}

	if p.Items == nil {
		p.Items = []items.Item{}
	}

	p.Damage.InitDiceRoll(p.Damage.DiceRoll)
	p.Damage.FormatDiceRoll()

	return nil
}

// file self loads due to init()
func LoadDataFiles() {

	start := time.Now()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/pets`
	tmpPetTypes, err := fileloader.LoadAllFlatFiles[string, *Pet](dataPath)
	if err != nil {
		panic(fmt.Errorf("filepath: %s: %w", dataPath, err))
	}

	petTypes = tmpPetTypes

	mudlog.Info("pets.LoadDataFiles()", "loadedCount", len(petTypes), "Time Taken", time.Since(start))
}
