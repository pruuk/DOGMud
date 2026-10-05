package items

import "github.com/GoMudEngine/GoMud/internal/configs"

// ToolType names the job a tool does. A gathering or processing job asks for
// one ToolType and is served by the best tool of that type the character has.
type ToolType string

const (
	ToolKnife        ToolType = "knife"         // skinning, cutting meat
	ToolCleaver      ToolType = "cleaver"       // butchering through joints: bone, fat
	ToolBoneSaw      ToolType = "bone_saw"      // horn, antler, tusk
	ToolAxe          ToolType = "axe"           // felling trees
	ToolSaw          ToolType = "saw"           // logs into planks, staves and shafts
	ToolScraper      ToolType = "scraper"       // fleshing hides before curing
	ToolSickle       ToolType = "sickle"        // herbs and fibre
	ToolCarvingKnife ToolType = "carving_knife" // bone, horn and fine woodwork
	ToolTrowel       ToolType = "trowel"        // roots and tubers
	ToolPick         ToolType = "pick"          // mining ore, coal and gems
)

// AllToolTypes is every ToolType, in a stable order (validation, help text).
var AllToolTypes = []ToolType{
	ToolKnife, ToolCleaver, ToolBoneSaw, ToolAxe, ToolSaw,
	ToolScraper, ToolSickle, ToolCarvingKnife, ToolTrowel, ToolPick,
}

// ToolTier is how good a tool is. It multiplies the user's stat term on a
// gathering roll (Balance.ToolMult*) and caps the best material grade the
// tool can produce (MaxGrade).
type ToolTier int

const (
	ToolTierNone       ToolTier = 0
	ToolTierCrude      ToolTier = 1 // improvised: a sword used as a knife, a stone axe
	ToolTierIron       ToolTier = 2
	ToolTierSteel      ToolTier = 3
	ToolTierMasterwork ToolTier = 4
)

var toolTierNames = map[ToolTier]string{
	ToolTierCrude:      `crude`,
	ToolTierIron:       `iron`,
	ToolTierSteel:      `steel`,
	ToolTierMasterwork: `masterwork`,
}

func (t ToolTier) String() string { return toolTierNames[t] }

// Valid reports whether t is crude through masterwork.
func (t ToolTier) Valid() bool { return t >= ToolTierCrude && t <= ToolTierMasterwork }

// MaxGrade is the best material grade a tool of this tier can produce. A crude
// tool tops out at standard; only a masterwork tool can reach pristine.
func (t ToolTier) MaxGrade() Quality {
	switch t {
	case ToolTierCrude:
		return QualityStandard
	case ToolTierIron:
		return QualityFine
	case ToolTierSteel:
		return QualitySuperb
	case ToolTierMasterwork:
		return QualityPristine
	}
	return QualityStandard
}

// ToolSpec marks an item as a tool. It is authored on the item spec:
//
//	tool:
//	  type: knife
//	  tier: 3        # 1 crude, 2 iron, 3 steel, 4 masterwork
//	  speed: 1.25    # optional; >1 finishes jobs faster (default 1.0)
//
// An item with no ToolSpec may still serve as an IMPROVISED tool: see
// ImprovisedTool.
type ToolSpec struct {
	Type  ToolType `yaml:"type"`
	Tier  ToolTier `yaml:"tier"`
	Speed float64  `yaml:"speed,omitempty"`
	// Durability is how many jobs the tool lasts before it breaks, before
	// the instance grade scales it. 0 = Balance.ToolDurability<Tier>.
	Durability int `yaml:"durability,omitempty"`
}

// DefaultToolDurability is Balance.ToolDurability<Tier>.
func DefaultToolDurability(t ToolTier) int {
	b := configs.GetBalanceConfig()
	switch t {
	case ToolTierIron:
		return int(b.ToolDurabilityIron)
	case ToolTierSteel:
		return int(b.ToolDurabilitySteel)
	case ToolTierMasterwork:
		return int(b.ToolDurabilityMasterwork)
	}
	return int(b.ToolDurabilityCrude)
}

// toolGradeDurability scales durability by the instance grade: a pristine
// tool lasts twice as long as a standard one, a crude one three quarters.
func toolGradeDurability(q Quality) float64 {
	switch q {
	case QualityCrude:
		return 0.75
	case QualityFine:
		return 1.25
	case QualitySuperb:
		return 1.5
	case QualityPristine:
		return 2.0
	}
	return 1.0
}

// Wear and durability (wilderness trades). Item.Wear counts points of wear;
// at Durability the item is broken. A tool wears one point per finished job
// (gather.WearTool); weapons and armour wear on critical hits and bows on
// shots (characters.CritWearWeapon and friends). A broken item stays in the
// inventory: a broken tool cannot be used and broken gear works badly
// (applyCondition) until it is repaired (actions.Repair).

// Durability is how much wear this item takes before it breaks: a tool's
// ToolDurability, else a weapon's or armour piece's GearDurability, else 0
// (it never wears).
func (i *Item) Durability() int {
	if d := i.ToolDurability(); d > 0 {
		return d
	}
	return i.GearDurability()
}

// ToolDurability is how many jobs this tool instance lasts in all. 0 when the
// item is not a tool (an improvised weapon never wears as a tool). Read from
// the raw spec: GetSpec itself asks for durability.
func (i *Item) ToolDurability() int {
	spec := i.GetRawSpec()
	if spec.Tool == nil {
		return 0
	}
	base := spec.Tool.Durability
	if base <= 0 {
		base = DefaultToolDurability(spec.Tool.Tier)
	}
	d := int(float64(base) * toolGradeDurability(i.Quality))
	if d < 1 {
		d = 1
	}
	return d
}

// IsWearableGear reports whether a spec wears in a fight: any weapon, and
// armour and shields (not jewelry, lights, bags or tails).
func IsWearableGear(spec ItemSpec) bool {
	switch spec.Type {
	case Weapon, Offhand, Head, Body, Belt, Gloves, Wrist, Back, Shoulders, Legs, Feet:
		return true
	}
	return false
}

// GearDurability is how much wear a weapon or armour piece takes before it
// breaks: ItemSpec.Durability when authored, else Balance
// GearDurabilityWeapon or GearDurabilityArmor, scaled by the instance grade.
// 0 for anything else.
func (i *Item) GearDurability() int {
	spec := i.GetRawSpec()
	if !IsWearableGear(spec) {
		return 0
	}
	base := spec.Durability
	if base <= 0 {
		b := configs.GetBalanceConfig()
		if spec.Type == Weapon {
			base = int(b.GearDurabilityWeapon)
		} else {
			base = int(b.GearDurabilityArmor)
		}
	}
	d := int(float64(base) * toolGradeDurability(i.Quality))
	if d < 1 {
		d = 1
	}
	return d
}

// AddWear adds n points of wear and reports whether that just broke the
// item. Items that never wear, and items already broken, report false.
func (i *Item) AddWear(n int) (justBroke bool) {
	d := i.Durability()
	if d <= 0 || n <= 0 || i.Wear >= d {
		return false
	}
	i.Wear += n
	if i.Wear >= d {
		i.Wear = d
		return true
	}
	return false
}

// AddToolWear records one finished job on a tool; see AddWear.
func (i *Item) AddToolWear(n int) (justBroke bool) {
	return i.AddWear(n)
}

// IsBroken reports whether the item has worn out and needs repairing.
func (i *Item) IsBroken() bool {
	d := i.Durability()
	return d > 0 && i.Wear >= d
}

// WearFraction is how worn the item is, 0 (new) to 1 (broken).
func (i *Item) WearFraction() float64 {
	d := i.Durability()
	if d <= 0 || i.Wear <= 0 {
		return 0
	}
	f := float64(i.Wear) / float64(d)
	if f > 1 {
		f = 1
	}
	return f
}

// Repair clears all wear.
func (i *Item) Repair() { i.Wear = 0 }

// BadlyWornFraction is the wear fraction at which gear reads "(badly worn)"
// and works worse; WornFraction is where it first reads "(worn)".
const (
	WornFraction      = 0.60
	BadlyWornFraction = 0.85
)

// ConditionMult is the multiplier wear puts on how well gear works: 1 until
// it is worn (60%), then Balance GearWornMult, GearBadlyWornMult (85%) and
// GearBrokenMult when broken.
func (i *Item) ConditionMult() float64 {
	f := i.WearFraction()
	if f <= 0 {
		return 1
	}
	b := configs.GetBalanceConfig()
	switch {
	case f >= 1:
		return float64(b.GearBrokenMult)
	case f >= BadlyWornFraction:
		return float64(b.GearBadlyWornMult)
	case f >= WornFraction:
		return float64(b.GearWornMult)
	}
	return 1
}

// wearSuffix marks an item that is wearing out: "(worn)" past 60% of its
// durability, "(badly worn)" past 85%, "(broken)" when worn out.
func (i *Item) wearSuffix() string {
	if i.Wear <= 0 {
		return ``
	}
	switch f := i.WearFraction(); {
	case f >= 1:
		return ` <ansi fg="red">(broken)</ansi>`
	case f >= BadlyWornFraction:
		return ` <ansi fg="item-quality">(badly worn)</ansi>`
	case f >= WornFraction:
		return ` <ansi fg="item-quality">(worn)</ansi>`
	}
	return ``
}

// IsKnownToolType reports whether t is one of AllToolTypes.
func IsKnownToolType(t ToolType) bool {
	for _, k := range AllToolTypes {
		if k == t {
			return true
		}
	}
	return false
}

// EffectiveToolTier is the tier of THIS tool instance: the authored tier,
// nudged by the instance's own grade when it was crafted. A pristine iron
// knife works like steel; a crude steel one works like iron. Ungraded tools
// use the authored tier unchanged.
func EffectiveToolTier(authored ToolTier, grade Quality) ToolTier {
	t := authored
	switch grade {
	case QualityPristine:
		t++
	case QualityCrude:
		t--
	}
	if t < ToolTierCrude {
		t = ToolTierCrude
	}
	if t > ToolTierMasterwork {
		t = ToolTierMasterwork
	}
	return t
}

// ImprovisedTool reports whether a spec WITHOUT a ToolSpec can stand in for a
// tool of type want, and at what tier (always crude). Only one-handed weapons
// qualify, and only for the jobs their edge suits:
//
//   - knife: a stabbing or slashing weapon (dagger, short sword)
//   - cleaver and axe: a cleaving weapon (hatchet, war axe)
//
// Two-handed weapons never qualify: nobody skins a deer with a greatsword.
func ImprovisedTool(spec ItemSpec, want ToolType) (ToolTier, bool) {
	if spec.Tool != nil || spec.Type != Weapon || spec.Hands == TwoHanded {
		return ToolTierNone, false
	}
	switch want {
	case ToolKnife:
		if spec.Subtype == Stabbing || spec.Subtype == Slashing {
			return ToolTierCrude, true
		}
	case ToolCleaver, ToolAxe:
		if spec.Subtype == Cleaving {
			return ToolTierCrude, true
		}
	}
	return ToolTierNone, false
}

// Furnishings are crafted pieces of furniture (woodwork) a lodger places in
// their own lodging with "use", the way a bought deed is placed. internal/housing
// does the placing.
const (
	FurnishingChest     = `chest`     // a container, like a container deed
	FurnishingBed       = `bed`       // a bed, like a bed deed
	FurnishingWorkbench = `workbench` // a woodworking bench station
)

// AllFurnishings is every furnishing kind, in a stable order.
var AllFurnishings = []string{FurnishingChest, FurnishingBed, FurnishingWorkbench}

// IsKnownFurnishing reports whether kind is one of AllFurnishings.
func IsKnownFurnishing(kind string) bool {
	for _, k := range AllFurnishings {
		if k == kind {
			return true
		}
	}
	return false
}

// NeverResold reports whether a merchant who buys this item must not put it
// back on the shelf: a real tool of iron or better. Good tools come only from
// a player smith (wilderness trades review); shops sell crude ones and buy
// the rest for scrap.
func NeverResold(spec ItemSpec) bool {
	return spec.Tool != nil && spec.Tool.Tier > ToolTierCrude
}
