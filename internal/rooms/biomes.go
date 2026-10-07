package rooms

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/pkg/errors"
)

type BiomeInfo struct {
	BiomeId        string  `yaml:"biomeid"`
	Name           string  `yaml:"name"`
	Symbol         string  `yaml:"symbol"`
	Description    string  `yaml:"description"`
	RequiredItemId int     `yaml:"requireditemid"`
	UsesItem       bool    `yaml:"usesitem"`
	Burns          bool    `yaml:"burns"`
	MovementCost   float64 `yaml:"movementcost"` // Terrain difficulty multiplier for stamina cost (1.0 = normal, 2.0 = rough)

	// SkyLight is the fraction of the open sky's light that reaches this
	// biome's floor: 1.0 for a desert dune, about 0.45 under forest canopy,
	// 0.0 at the back of a cave. It is applied as an attenuation on the
	// light scale (a multiplier on linear brightness since lighting plan 6),
	// so it is the SAME operator weather occlusion
	// uses in plan 4. Canopy, roof, drain-cap and blizzard are one idea.
	//
	// 🔑 It is a POINTER because zero is meaningful. A cave's sky fraction is
	// genuinely zero, which must be distinguishable from the field being
	// absent; an unset fraction reads as fully open sky.
	//
	// A room may override this; see Room.SkyLight.
	SkyLight *float64 `yaml:"skylight,omitempty"`

	// Lamp is a permanent light source belonging to the place itself: street
	// lanterns, a hearth, a cave's bioluminescence. It joins the room's light
	// on the same combine as the sky and any carried source, rather than
	// acting as a floor, so a lantern-lit tavern plus a carried torch does not
	// double-count.
	//
	// 🔑 Also a POINTER, for the same reason: a lamp of zero is a place that
	// has a light source producing nothing, which an unset field does not mean.
	//
	// Guidance: a value below LightDimBelow leaves a normal observer reading
	// shapes with names hidden, which is what a back lane should do; a value
	// above it means full sight all night, which is what a main street or an
	// inn should do.
	Lamp *int `yaml:"lamp,omitempty"`

	// StreetLamp makes the biome's Lamp a street lamp, lit and put out by a
	// lamplighter working by eye: it joins the room's light while
	// gametime.LampsLit() is true, which is at night OR while the clear sky
	// (the celestial light before weather), seen through the smallest sky
	// fraction among the street-lamp biomes, would read a level below
	// LightDimBelow, so a midwinter morning too dim to read a face keeps its
	// lamps and no street dips below faces for the round they go out
	// (lighting plan 6, owner ruling O4 as amended; registerStreetLampSky
	// hands gametime that fraction). The North Gate
	// arch lantern's dusk_to_dawn tree reads the same test (`time_of_day
	// period: lamplit`), and weather never enters it, so every street lamp
	// in the world lights and goes out at the same moment. Unset, the lamp
	// burns at all hours, which is right for an inn's lamps, a cave's glow
	// or the ether.
	//
	// It governs the BIOME lamp only. A room's own `lamp:` override is an
	// all-hours lamp whatever its biome says.
	StreetLamp bool `yaml:"streetlamp,omitempty"`

	// Indoor marks a room as sheltered from weather; outdoor-only mutators
	// don't render here.
	//
	// 🔑 ADDING, RENAMING OR REMOVING A BIOME? An indoor biome must also be
	// classified as a weather PROSE CLASS, in one of the two maps in
	// modules/weather/content/emotes.go: undergroundBiomes (felt through
	// stone: seepage, draughts, mineral cold) or surfaceIndoorBiomes (a built
	// structure: roofs, eaves, windows). Without that, a new indoor biome
	// silently serves prose about roofs and windowpanes inside it, which is
	// the exact defect the underground class was added to fix.
	//
	// modules/weather/content/biome_coupling_test.go fails the build if you
	// forget, but it runs in THAT package, so `go test ./internal/rooms/...`
	// alone will not tell you.
	Indoor bool `yaml:"indoor,omitempty"`

	// Private fields for runtime use
	symbolRune rune
	filepath   string
}

func (bi *BiomeInfo) GetSymbol() rune {
	if bi.symbolRune == 0 && len(bi.Symbol) > 0 {
		for _, r := range bi.Symbol {
			bi.symbolRune = r
			break
		}
	}
	return bi.symbolRune
}

func (bi *BiomeInfo) SymbolString() string {
	return bi.Symbol
}

// SkyLightFraction is the biome's sky fraction, defaulting to a fully open sky
// when unset.
func (bi *BiomeInfo) SkyLightFraction() float64 {
	if bi.SkyLight == nil {
		return 1.0
	}
	return *bi.SkyLight
}

// LampValue is the biome's own light source and whether it declares one at
// all, whatever the hour. LampAt is the lamp as it burns at a given hour.
func (bi *BiomeInfo) LampValue() (int, bool) {
	if bi.Lamp == nil {
		return 0, false
	}
	return *bi.Lamp, true
}

// LampAt is the biome's lamp as it burns when the street lamps are (or are
// not) lit, the value gametime.LampsLit reports: a StreetLamp lamp is out
// while they are out, every other lamp burns at all hours.
func (bi *BiomeInfo) LampAt(lampsLit bool) (int, bool) {
	if bi.StreetLamp && !lampsLit {
		return 0, false
	}
	return bi.LampValue()
}

// HasLamp reports whether this biome carries a light source that actually
// produces light.
//
// 🔑 It exists because a Go template cannot express the distinction. A template
// writing `{{ if .Lamp }}` tests only that the POINTER is non-nil, so a biome
// authored `lamp: 0` -- a legal value meaning "a light source producing
// nothing" -- would satisfy it and the player would be told the place is lit
// after dark when it is not. The pointer-versus-zero distinction this whole
// type is built on has to be readable from a template too, and this is the
// only way to make it so.
func (bi *BiomeInfo) HasLamp() bool {
	v, ok := bi.LampValue()
	return ok && v != 0
}

// GetMovementCost returns the terrain difficulty multiplier for stamina cost.
// Returns 1.0 (normal terrain) if not set.
func (bi *BiomeInfo) GetMovementCost() float64 {
	if bi.MovementCost <= 0 {
		return 1.0
	}
	return bi.MovementCost
}

// Implement Loadable interface
func (bi *BiomeInfo) Id() string {
	return strings.ToLower(bi.BiomeId)
}

func (bi *BiomeInfo) Validate() error {
	if bi.BiomeId == "" {
		return fmt.Errorf("biomeid cannot be empty")
	}
	if bi.Name == "" {
		return fmt.Errorf("biome name cannot be empty")
	}
	if bi.Symbol == "" || bi.Symbol == "?" {
		return fmt.Errorf("biome '%s' has invalid or missing symbol", bi.BiomeId)
	}
	if bi.SkyLight != nil && (*bi.SkyLight < 0 || *bi.SkyLight > 1) {
		return fmt.Errorf("biome '%s' skylight %v is outside 0.0 to 1.0", bi.BiomeId, *bi.SkyLight)
	}
	// A lamp is light, and light is never negative: below 0 is magical
	// darkness only (lighting plan 6, owner ruling O1), and a negative light
	// term would silently read as unlit.
	if bi.Lamp != nil && (*bi.Lamp < 0 || *bi.Lamp > 100) {
		return fmt.Errorf("biome '%s' lamp %d is outside 0 to 100 (a lamp is light; negative is magical darkness only)", bi.BiomeId, *bi.Lamp)
	}
	return nil
}

func (bi *BiomeInfo) Filepath() string {
	if bi.filepath == "" {
		bi.filepath = fmt.Sprintf("%s.yaml", bi.BiomeId)
	}
	return bi.filepath
}

var (
	biomes = map[string]*BiomeInfo{}
)

func LoadBiomeDataFiles() {

	start := time.Now()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/biomes`
	tmpBiomes, err := fileloader.LoadAllFlatFiles[string, *BiomeInfo](dataPath)
	if err != nil {
		panic(errors.Wrap(err, `filepath: `+dataPath))
	}

	biomes = tmpBiomes

	if len(biomes) == 0 {
		mudlog.Warn("No biomes loaded from files, using default fallback biome")
		// Create a single default fallback biome
		biomes[`default`] = &BiomeInfo{
			BiomeId:      `default`,
			Name:         `Default`,
			Symbol:       `•`,
			Description:  `A default biome used when no other biome is specified.`,
			MovementCost: 1.0,
		}
	} else {
		// Always ensure a default biome exists as fallback
		if _, ok := biomes[`default`]; !ok {
			biomes[`default`] = &BiomeInfo{
				BiomeId:      `default`,
				Name:         `Default`,
				Symbol:       `•`,
				Description:  `A default biome used when no other biome is specified.`,
				MovementCost: 1.0,
			}
		}
	}

	registerStreetLampSky()

	mudlog.Info("biomes.LoadBiomeDataFiles()", "loadedCount", len(biomes), "Time Taken", time.Since(start))
}

// streetLampSkyFraction is the smallest sky fraction among the biomes whose
// lamp is a street lamp (StreetLamp with a lamp declared), and false when no
// biome has one.
func streetLampSkyFraction() (float64, bool) {
	best, found := 1.0, false
	for _, b := range biomes {
		if b == nil || !b.StreetLamp || b.Lamp == nil {
			continue
		}
		if f := b.SkyLightFraction(); !found || f < best {
			best, found = f, true
		}
	}
	return best, found
}

// registerStreetLampSky hands gametime the dimmest street-lamp biome's sky
// fraction, which gametime.LampsLitAt reads so the lamps stay lit until that
// street reads faces by daylight alone (lighting plan 6, owner ruling O4 and
// the review fix after it). With no street-lamp biome it registers 1, the
// open sky. Called whenever the biome registry is replaced.
func registerStreetLampSky() {
	f, _ := streetLampSkyFraction()
	gametime.SetStreetLampSkyFraction(f)
}

func GetBiome(name string) (*BiomeInfo, bool) {
	if name == `` {
		name = `default`
	}
	b, ok := biomes[strings.ToLower(name)]
	return b, ok
}

func GetAllBiomes() []BiomeInfo {
	ret := []BiomeInfo{}
	for _, b := range biomes {
		ret = append(ret, *b)
	}
	return ret
}
