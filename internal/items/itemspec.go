package items

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/pkg/errors"
)

type ItemType string
type ItemSubType string
type Element string
type Intensity string
type TokenName string

type WeaponHands = int

var (
	items map[int]*ItemSpec = make(map[int]*ItemSpec)
)

type ItemTypeInfo struct {
	Type        string
	Description string
	Count       int
	MinItemId   int
	MaxItemId   int
}

// Returns key=type and value=description
func ItemTypes() []ItemTypeInfo {
	return []ItemTypeInfo{
		// Equipment
		// Equipment - Weapons
		{string(Weapon), `This can be wielded as a weapon.`, 0, 10000, 19999},
		// Equipment - Armor
		{string(Offhand), `This can be worn in the offhand.`, 0, 20000, 29999},
		{string(Head), `This can be worn in the players head equipment slot.`, 0, 20000, 29999},
		{string(Neck), `This can be worn in the players neck equipment slot.`, 0, 20000, 29999},
		{string(Body), `This can be worn in the players body equipment slot.`, 0, 20000, 29999},
		{string(Belt), `This can be worn in the players belt equipment slot.`, 0, 20000, 29999},
		{string(Gloves), `This can be worn in the players gloves equipment slot.`, 0, 20000, 29999},
		{string(Ring), `This can be worn in the players ring equipment slot.`, 0, 20000, 29999},
		{string(Legs), `This can be worn in the players legs equipment slot.`, 0, 20000, 29999},
		{string(Feet), `This can be worn in the players feet equipment slot.`, 0, 20000, 29999},
		{string(Tail), `Worn on the tail.`, 0, 20000, 29999},
		{string(Light), `A light you carry to see by.`, 0, 20000, 29999},
		{string(Wrist), `Worn on the wrist.`, 0, 20000, 29999},
		{string(Back), `Worn on the back.`, 0, 20000, 29999},
		{string(Shoulders), `Worn on the shoulders.`, 0, 20000, 29999},
		{string(ComponentBag), `A bag for crafting materials.`, 0, 30000, 39999},
		// Consumables
		{string(Potion), `This is a magic potion.`, 0, 30000, 39999},
		{string(Food), `This is food.`, 0, 30000, 39999},
		{string(Drink), `This is a drink.`, 0, 30000, 39999},
		{string(Scroll), `This is a scroll.`, 0, 0, 9999},
		{string(Grenade), `This is an explosive object.`, 0, 0, 9999},
		{string(Junk), `This is garbage.`, 0, 0, 9999},
		// Other
		{string(Readable), `This can be read.`, 0, 0, 9999},
		{string(Key), `This is a key that opens a locked container or door.`, 0, 0, 9999},
		{string(Object), `This is a catch-all generic object without pre-defined special behaviors.`, 0, 0, 9999},
		{string(Gemstone), `This is a gemstone.`, 0, 0, 9999},
		{string(Lockpicks), `This allows use of the picklock skill.`, 0, 0, 9999},
		{string(Botanical), `This is an herb.`, 0, 30000, 39999},
		{string(Ammo), `This is ranged ammo (arrows, bolts, etc.).`, 0, 0, 9999},
	}
}

// Returns key=subtype and value=description
func ItemSubtypes() []ItemTypeInfo {
	return []ItemTypeInfo{
		// Miscellaneous
		{string(Wearable), `Can be targetted with the equip/wear/wield command.`, 0, 0, 0},
		{string(Drinkable), `Can be targetted with the drink command.`, 0, 0, 0},
		{string(Edible), `Can be targetted with the eat command.`, 0, 0, 0},
		{string(Usable), `Can be targetted with the use command.`, 0, 0, 0},
		{string(Throwable), `Can be targetted with the throw command.`, 0, 0, 0},
		{string(Mundane), `No special behavior built in.`, 0, 0, 0},
		// Weapons
		{string(Generic), `Any weapon that doesn't get assigned an actual weapon subcategory.`, 0, 0, 0},
		{string(Bludgeoning), `A blunt weapon.`, 0, 0, 0},
		{string(Cleaving), `A hacking/chopping weapon.`, 0, 0, 0},
		{string(Stabbing), `A piercing weapon.`, 0, 0, 0},
		{string(Slashing), `A slicing and slashing weapon.`, 0, 0, 0},
		{string(Shooting), `A ranged weapon.`, 0, 0, 0},
		{string(Claws), `A slashing weapon worn on the hands.`, 0, 0, 0},
		{string(Fist), `A weapon that enhances unarmed strikes.`, 0, 0, 0},
		{string(Whipping), `A whipping weapon.`, 0, 0, 0},
		{string(Wand), `A caster weapon that boosts spell damage.`, 0, 0, 0},
		{string(Sceptre), `A caster weapon that boosts spell damage.`, 0, 0, 0},
		{string(Staff), `A two-handed caster weapon with high spell damage boost.`, 0, 0, 0},
		// Miscellaneous data
		{string(BlobContent), `Can store blob content in the item data.`, 0, 0, 0},
	}
}

const (
	Unknown ItemType = ""

	// Equipment
	Weapon       ItemType = "weapon"
	Offhand      ItemType = "offhand"
	Head         ItemType = "head"
	Neck         ItemType = "neck"
	Body         ItemType = "body"
	Belt         ItemType = "belt"
	Gloves       ItemType = "gloves"
	Ring         ItemType = "ring"
	Wrist        ItemType = "wrist"        // Bracelets, bracers
	Back         ItemType = "back"         // Cloaks, backpacks
	Shoulders    ItemType = "shoulders"    // Pauldrons, mantles
	ComponentBag ItemType = "componentbag" // Crafting material bags
	Legs         ItemType = "legs"
	Feet         ItemType = "feet"
	Tail         ItemType = "tail"  // Tail attachments (tail mutation slot)
	Light        ItemType = "light" // A carried light: candle, lantern, torch (lighting plan 5a)
	// Consumables
	Potion  ItemType = "potion"
	Food    ItemType = "food"
	Drink   ItemType = "drink"
	Scroll  ItemType = "scroll"
	Grenade ItemType = "grenade" // Expected to be thrown
	Junk    ItemType = "junk"

	// Other
	Readable  ItemType = "readable"  // Something with writing to reveal when read
	Key       ItemType = "key"       // A key for a door
	Object    ItemType = "object"    // A mundane object
	Gemstone  ItemType = "gemstone"  // A gem
	Lockpicks ItemType = "lockpicks" // Used for lockpicking
	Botanical ItemType = "botanical" // A plant, herb, etc.
	Service   ItemType = "service"   // Possibly a ticket,action, or favor being purchased
	Ammo      ItemType = "ammo"      // Ranged ammo (arrows, bolts) — forward-declared for ranged combat

	// Subtypes for wearables
	Wearable  ItemSubType = "wearable"
	Drinkable ItemSubType = "drinkable"
	Edible    ItemSubType = "edible"
	Usable    ItemSubType = "usable"
	Throwable ItemSubType = "throwable" // If dropped/thrown, triggers condition effects on room and is lost
	Mundane   ItemSubType = "mundane"

	// Subtypes for weapons, chooses attack messages.
	Unarmed     ItemSubType = "unarmed"
	Generic     ItemSubType = "generic"
	Bludgeoning ItemSubType = "bludgeoning"
	Cleaving    ItemSubType = "cleaving"
	Stabbing    ItemSubType = "stabbing"
	Slashing    ItemSubType = "slashing"
	Shooting    ItemSubType = "shooting" // bows, crossbows, guns, etc.
	Claws       ItemSubType = "claws"
	Fist        ItemSubType = "fist"
	Bite        ItemSubType = "bite"
	Sting       ItemSubType = "sting"
	Gore        ItemSubType = "gore"
	Slam        ItemSubType = "slam"
	Whipping    ItemSubType = "whipping"
	Wand        ItemSubType = "wand"
	Sceptre     ItemSubType = "sceptre"
	Staff       ItemSubType = "staff"

	BlobContent ItemSubType = "blobcontent"

	OneHanded WeaponHands = 1
	TwoHanded WeaponHands = 2

	Fire        Element = "fire"
	Water       Element = "water"
	Ice         Element = "ice"
	Electricity Element = "electricity"
	Acid        Element = "acid"
	Life        Element = "life"
	Death       Element = "death"

	// Intensity of the attack
	Prepare  Intensity = "prepare"
	Wait     Intensity = "wait"
	Miss     Intensity = "miss"
	Weak     Intensity = "weak"
	Normal   Intensity = "normal"
	Heavy    Intensity = "heavy"
	Critical Intensity = "critical"
	Fumble   Intensity = "fumble"
	// CoupDeGrace is selected when the target is already dying: it has taken
	// its lethal blow and the attributed death is queued but not yet resolved.
	// Later hits that round still land and still count, but they read as
	// finishing an opponent rather than as ordinary swings. See U5c.
	CoupDeGrace Intensity = "coupdegrace"

	// Tokens
	TokenItemName TokenName = "{itemname}"
	// The four canonical NAME tokens (messaging arc M4a). One vocabulary for
	// every store: the attacker is the Actor and the defender is the Actee,
	// whether the line comes from combat-messages, defense-messages or
	// taunt-messages. Before M4a the same two people were spelled
	// {source}/{target} here and {attacker}/{defender} next door, and
	// internal/combat carried an alias map to reconcile them.
	//
	// The two name tokens are DEFINED FROM the core's constants rather than
	// respelled here, so the two vocabularies cannot drift apart again. The
	// type tokens have no core counterpart: they carry an ansi colour class,
	// not a name.
	TokenActor        TokenName = narration.TokenActor
	TokenActorType    TokenName = "{actortype}" // will be 'user' or 'mob'
	TokenActee        TokenName = narration.TokenActee
	TokenActeeType    TokenName = "{acteetype}" // will be 'user' or 'mob'
	TokenUsesLeft     TokenName = "{usesleft}"
	TokenDamage       TokenName = "{damage}"
	TokenEntranceName TokenName = "{entrancename}"
	TokenExitName     TokenName = "{exitname}"
	TokenWeapon       TokenName = "{weapon}"   // Stage 9.3: defensive action messages
	TokenAttack       TokenName = "{attack}"   // Generic "strike"/"blow" for unarmed, weapon name for armed
	TokenStance       TokenName = "{stance}"   // Stage 9.4: combat stance (aggressive/defensive/balanced/reckless)
	TokenPosition     TokenName = "{position}" // Stage 9.4: combat position (standing/prone/clinched/grounded)
	TokenMomentum     TokenName = "{momentum}" // Stage 9.4: combat momentum (offensive/defensive/pressured/in control)
	TokenBodyPart     TokenName = "{bodypart}" // Stage 42.4: random body part for hit location flavor

	POVUser  = 0
	POVOther = 1
)

type Damage struct {
	Attacks          int    `yaml:"attacks,omitempty"`          // How many attacks this weapon gets (usually 1)
	DiceRoll         string `yaml:"diceroll,omitempty"`         // legacy: 1d6, etc.
	CritConditionIds []int  `yaml:"critconditionids,omitempty"` // If this damage is a crit, what conditions does it apply?
	DiceCount        int    `yaml:"dicecount,omitempty"`        // how many dice to roll for this weapons damage
	SideCount        int    `yaml:"sidecount,omitempty"`        // how many sides per dice roll
	BonusDamage      int    `yaml:"bonusdamage,omitempty"`      // flat damage bonus, so for example 1d6+1
	BaseDamage       int    `yaml:"basedamage,omitempty"`       // distribution mode: mean damage
	Variance         int    `yaml:"variance,omitempty"`         // distribution mode: standard deviation
}

type ItemMessage string

// Attack messages
type AttackMessageOptions []ItemMessage
type AttackEffects map[Intensity]AttackMessageOptions
type AttackMessages map[ItemSubType]AttackEffects

// The blueprint for an item
type ItemSpec struct {
	ItemId           int
	Value            int
	Uses             int   `yaml:"uses,omitempty"`             // How many uses it starts with
	ConditionIds     []int `yaml:"conditionids,omitempty"`     // What conditions it can apply (if used)
	WornConditionIds []int `yaml:"wornconditionids,omitempty"` // ConditionId's that are applied while worn, and expired when removed.
	// Nouns are details of the item a player can look at by name, highlighted
	// in the item's description the way a room's nouns are (lighting plan 5a:
	// the hooded lantern's hood).
	Nouns map[string]string `yaml:"nouns,omitempty"`
	// ── Pinnacle Stage 1: reserves, bandolier, mutation drip, hunger. Procs
	// and voices live in the item's behaviour tree (item behaviour slices 2, 3).
	ReserveHealthPct      float64 `yaml:"reserve_health_pct,omitempty"`      // 0-1 fraction of HealthMax reserved while equipped
	ReserveStaminaPct     float64 `yaml:"reserve_stamina_pct,omitempty"`     // 0-1 fraction of StaminaMax reserved while equipped
	ReserveConvictionPct  float64 `yaml:"reserve_conviction_pct,omitempty"`  // 0-1 fraction of ConvictionMax reserved while equipped
	PreservesContents     bool    `yaml:"preserves_contents,omitempty"`      // bandolier: contents never age
	AmbientPotions        bool    `yaml:"ambient_potions,omitempty"`         // bandolier: slotted potion conditions always-on at Peak
	MutationTickInterval  int     `yaml:"mutation_tick_interval,omitempty"`  // rounds between mutation rolls while worn (0 = never)
	MutationTickChance    int     `yaml:"mutation_tick_chance,omitempty"`    // percent chance per roll
	MutationRarityFloor   int     `yaml:"mutation_rarity_floor,omitempty"`   // min mutation rarity in the pool (0 = no floor)
	HungerRounds          int     `yaml:"hunger_rounds,omitempty"`           // rounds without a kill before the item feeds on the wielder (0 = never)
	HungerDrainPct        float64 `yaml:"hunger_drain_pct,omitempty"`        // fraction of HealthMax drained per hungry round
	PhysicalMitigation    int     `yaml:"physical_mitigation,omitempty"`     // % physical damage reduction (Stage 34)
	MagicalMitigation     int     `yaml:"magical_mitigation,omitempty"`      // % magical damage reduction (Stage 34)
	ConvictionMitigation  int     `yaml:"conviction_mitigation,omitempty"`   // % conviction damage reduction (Stage 34)
	DamageMultiplier      float64 `yaml:"damage_multiplier,omitempty"`       // Weapon damage multiplier for new pipeline (Stage 34)
	SpellDamageMultiplier float64 `yaml:"spell_damage_multiplier,omitempty"` // Spell damage multiplier for caster weapons (wand/sceptre/staff)
	ParryRating           int     `yaml:"parryrating,omitempty"`             // Weapon parry bonus (Stage 7.1)
	BlockRating           int     `yaml:"blockrating,omitempty"`             // Shield block bonus (Stage 7.1)
	AmmoTag               string  `yaml:"ammo_tag,omitempty"`                // Ranged weapons: ammo type required (arrows/bolts/shot). Ammo items: type provided.
	MinStrength           int     `yaml:"min_strength,omitempty"`            // Minimum Strength to wield (heavy bows/arbalest)
	WaitRounds            int     `yaml:"waitrounds,omitempty"`              // How many extra rounds each combat requires
	// StaminaCost is DEPRECATED and no longer read for cost. U7 Task 7 replaced
	// the per-weapon attack charge with a config base (AttackBaseStaminaCost)
	// times the encumbrance multiplier, charged per swing. A heavy weapon already
	// costs more through its WEIGHT feeding that encumbrance multiplier, so
	// reading this field as well would charge for the same heaviness twice.
	//
	// The field is KEPT rather than deleted because thirty item files author the
	// staminacost key. The production loader is non-strict and would ignore an
	// unknown key, but internal/fileloader carries a probeStrict hook that the
	// boot smoke test enables, and deleting the Go field would make all thirty
	// fail that probe. Removing it needs a content pass over those files first.
	StaminaCost     int     `yaml:"staminacost,omitempty"`     // Deprecated (U7 Task 7): unread; see above
	SpeedMultiplier float64 `yaml:"speedmultiplier,omitempty"` // Attack speed modifier (1.0 = unarmed baseline, <1.0 slower, >1.0 faster)
	Weight          float64 `yaml:"weight,omitempty"`          // Weight in pounds (affects encumbrance)
	GrappleModifier float64 `yaml:"grapplemodifier,omitempty"` // Grapple bonus/penalty (Stage 8.2)
	EscapeModifier  float64 `yaml:"escapemodifier,omitempty"`  // Armor escape modifier for Grounded position (Stage 8.7)
	// Reach is the weapon's operational reach in meters. Combat
	// consults reach in grapple positions: weapons whose reach
	// exceeds the position's effective radius are penalized (see
	// internal/combat/reach.go). Zero is a sentinel meaning "use
	// DefaultReachForSubtype lookup based on Subtype"; authors set
	// an explicit non-zero value only for outliers (a particularly
	// short dagger, an oversized hilt, etc.).
	//
	// Reach is weapon-only — arm length / species reach is
	// intentionally out of scope for chunk 4c per the design spec.
	Reach                 float64     `yaml:"reach,omitempty"`
	Hands                 WeaponHands `yaml:"hands"` // How many hands it takes to wield
	Name                  string
	DisplayName           string `yaml:"displayname,omitempty"` // Name that is typically displayed to the user
	NameSimple            string // A simpler name for the item, for example "Golden Battleaxe" should be "Battleaxe" or "Axe" for simple
	Description           string
	QuestToken            string `yaml:"questtoken,omitempty"` // Grants this quest if given/picked up
	Type                  ItemType
	Subtype               ItemSubType
	Damage                Damage
	Element               Element           `yaml:"element,omitempty"`
	StatMods              statmods.StatMods `yaml:"statmods,omitempty"`                // What stats it modifies when equipped
	BreakChance           uint8             `yaml:"breakchance,omitempty"`             // Chance in 100 that the item will break when used, or when the character is hit with it equipped, or if it is in the characters inventory during an explosion, etc.
	Cursed                bool              `yaml:"cursed,omitempty"`                  // Can't be removed once equipped
	KeyLockId             string            `yaml:"keylockid,omitempty"`               // Example: `778-north` - If it's a key, what lock does it open? roomid-exitname etc.
	ComponentTag          string            `yaml:"component_tag,omitempty"`           // Spell component tag (e.g. "stone" for throw-stone)
	IsComponent           bool              `yaml:"is_component,omitempty"`            // Auto-routes to component bag on pickup
	WeightReduction       float64           `yaml:"weight_reduction,omitempty"`        // 0.0-1.0, fraction of contents weight reduced
	BagCapacity           int               `yaml:"bag_capacity,omitempty"`            // Max items storable in component bag
	Aging                 AgingThresholds   `yaml:"aging,omitempty"`                   // Potion aging phase thresholds
	BottleAgingMultiplier float64           `yaml:"bottle_aging_multiplier,omitempty"` // Bottle aging speed (clay=3.0, glass=1.0, phial=0.5, decanter=0.25)
	Toxicity              int               `yaml:"toxicity,omitempty"`                // Toxicity cost when consumed
	// Magnitude is the base strength for a condition in ConditionIds that
	// reads its magnitude (conditions.ScaledKinds, lighting plan 5c). The
	// drink path scales it by the same potency multiplier as duration. A
	// potion carrying such a condition must declare it (repo-root guard
	// potion_conditions_guard_test.go).
	Magnitude         float64         `yaml:"magnitude,omitempty"`
	IsBandolier       bool            `yaml:"is_bandolier,omitempty"`       // Belt item that holds potions
	BandolierCapacity int             `yaml:"bandolier_capacity,omitempty"` // Max potions storable in bandolier
	SalvageReturns    []SalvageReturn `yaml:"salvage_returns,omitempty"`    // Custom salvage returns for non-crafted items
	RarityTier        int             `yaml:"rarity_tier,omitempty"`        // Vendor stock cap tier (50/40/30/20/10). Used by shops.EffectiveMaxStock with mob.StockMultiplier. 0 = untiered (quest items, defer-to-3.0e items). NOT a difficulty signal — see MaterialTier.
	// MaterialTier is how RARE/DEMANDING a crafting material is, 1 (common) to
	// 5 (rarest). It scales craft difficulty via items.MaterialTierMultiplier.
	// 0 means untiered and is NEUTRAL (multiplier 1.0), not "cheapest" — so
	// partial coverage during the backfill cannot silently make a recipe easy.
	//
	// ⚠️ DO NOT CONFUSE WITH RarityTier ABOVE. RarityTier is a vendor stock cap
	// where a HIGHER value means MORE common, inverting its own name, and its
	// doc comment claims five values while the data holds seventeen. The two
	// fields are deliberately separate: renormalising RarityTier would move
	// vendor stock levels world-wide at the same time as craft difficulty.
	MaterialTier     int      `yaml:"material_tier,omitempty"`
	VendorCategories []string `yaml:"vendor_categories,omitempty"` // Disciplines that buy/sell this item; mirrors shops.ValidCraftSupports minus "general"
	NotSalable       bool     `yaml:"not_salable,omitempty"`       // True for lore / flavor / legacy items excluded from vendor economy validation
	NeverDrops       bool     `yaml:"never_drops,omitempty"`       // Equipped-only: this item is skipped entirely by mob death-loot drops (boss-only gear that must never reach players). Does not affect carried Character.Items — use a separate mechanism (loot_pool / character.items) for guaranteed loot on the same mob.
	Restricted       bool     `yaml:"restricted,omitempty"`        // Contraband: bid on by the auction Official (The Crown Assessor, econ #2.5). Interest tag only — no other mechanics.

	// Behavior names the item's behaviour tree,
	// behaviors/items/<name>.yaml (lighting 5e, item behaviour slice 1,
	// Rule 1). Several items may share one tree. The tree belongs to the
	// template: read it through GetItemSpec or Item.HasBehavior, never an
	// instance's override spec. The boot panics on a name that does not
	// resolve (behaviortree.ValidateItemBehaviors).
	Behavior string `yaml:"behavior,omitempty"`
	// Fixture marks an item fixed to a room's floor: FixtureLight or
	// FixtureDarkness (Rule 10). No path takes it off the floor, "On the
	// Ground" leaves it out, and its tree's set_light / pulse_light output
	// lights (or darkens) the room through internal/itemlight.
	Fixture string `yaml:"fixture,omitempty"`

	// YAML-driven use effects — replaces JS onUse/onCommand_use
	OnUseTrainSkill  string `yaml:"on_use_train_skill,omitempty"`
	OnUseTrainAmount int    `yaml:"on_use_train_amount,omitempty"`
	OnUseUserText    string `yaml:"on_use_user_text,omitempty"`
	OnUseRoomText    string `yaml:"on_use_room_text,omitempty"`
}

// SalvageReturn defines a material recovered when salvaging a tagged item.
type SalvageReturn struct {
	ItemTag  string `yaml:"item_tag"`
	Quantity int    `yaml:"quantity"`
}

func (i Element) String() string {
	return string(i)
}

func (i ItemType) String() string {
	return string(i)
}

func (i ItemSubType) String() string {
	return string(i)
}

func (d *Damage) String() string {
	if d.BaseDamage > 0 {
		return fmt.Sprintf("~%d ±%d", d.BaseDamage, d.Variance)
	}
	if d.DiceRoll == "" {
		return "N/A"
	}
	return d.DiceRoll
}

func (d *Damage) FormatDiceRoll() string {

	d.DiceRoll = util.FormatDiceRoll(d.Attacks, d.DiceCount, d.SideCount, d.BonusDamage, d.CritConditionIds)

	return d.DiceRoll
}

func (d *Damage) InitDiceRoll(dRoll string) {
	// If diceroll is specified, it overrides whatever stats are already there
	if len(dRoll) < 1 {
		return
	}

	d.Attacks, d.DiceCount, d.SideCount, d.BonusDamage, _ = util.ParseDiceRoll(dRoll)
}

// GetAttackStaminaCost returns the stamina cost for attacking with this weapon.
// Returns a default based on weapon type if not explicitly set.
//
// Deprecated: U7 Task 7 removed its only production caller
// (Character.GetAttackStaminaCost). Attacks are priced per swing by
// combat.ChargeAttackCost -- a config base times the encumbrance multiplier the
// weapon's WEIGHT already feeds -- so nothing reads a per-weapon cost any more.
// Kept alongside the StaminaCost field it reads; both go together in the content
// pass that clears staminacost out of the thirty item files that set it.
func (is *ItemSpec) GetAttackStaminaCost() int {
	if is.StaminaCost > 0 {
		return is.StaminaCost
	}

	// Default costs based on weapon type if not specified
	// Unarmed (no weapon) will be handled separately
	switch is.Type {
	case Weapon:
		// Default to medium weapon cost
		return 8
	default:
		return 0 // Non-weapons don't cost stamina
	}
}

// GetSpeedMultiplier returns the attack speed modifier for this weapon.
// 1.0 = unarmed baseline, <1.0 = slower, >1.0 = faster
// Most weapons should be <1.0 (fewer attacks than unarmed)
func (is ItemSpec) GetSpeedMultiplier() float64 {
	if is.SpeedMultiplier > 0 {
		return is.SpeedMultiplier
	}

	// Default to 1.0 (unarmed baseline)
	// Most weapons will explicitly set values <1.0
	return 1.0
}

// GetWeight returns the weight of this item in pounds.
// Returns 0 for weightless items or if not specified.
func (is ItemSpec) GetWeight() float64 {
	return is.Weight
}

func FindItem(nameOrId string) int {
	if itemId, err := strconv.Atoi(nameOrId); err == nil {
		if itm := New(itemId); itm.ItemId != 0 {
			return itm.ItemId
		}
	}

	return FindItemByName(nameOrId)
}

func FindKeyByLockId(lockId string) int {

	for _, item := range items {
		if item.Type != Key {
			continue
		}
		if item.KeyLockId == lockId {
			return item.ItemId
		}
	}

	return 0
}

func FindItemByName(name string) int {
	name = strings.ToLower(name)

	for _, item := range items {
		if strings.ToLower(item.Name) == name {
			return item.ItemId
		}
	}

	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item.Name), name) {
			return item.ItemId
		}
	}

	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), name) {
			return item.ItemId
		}
	}

	return 0
}

func GetAllItemSpecs() []ItemSpec {

	itemSpecs := []ItemSpec{}
	for _, item := range items {
		itemSpecs = append(itemSpecs, *item)
	}
	return itemSpecs
}

// GetAllItemSpecsMap returns all item specs as a map keyed by item ID.
// The returned pointers reference cached specs — callers must not mutate.
func GetAllItemSpecsMap() map[int]*ItemSpec {
	out := make(map[int]*ItemSpec, len(items))
	for id, s := range items {
		out[id] = s
	}
	return out
}

// authored is what loaded, authored items answer to: every keyword and word
// of a name (words), and every whole name, normalised (names). It is
// rebuilt by whatever writes the items map (a load, an item saved, created
// or deleted), on the game loop, as ONE snapshot so the two sets always
// agree. AuthoredKeyword and AuthoredName are read from goroutines off the
// mud lock (a bauble being named), and a range over the live items map
// there, racing one of those writes, is a fatal "concurrent map iteration
// and map write"; this snapshot is read instead.
type authoredSnapshot struct {
	words map[string]bool
	names map[string]bool
}

var authored atomic.Pointer[authoredSnapshot]

// rebuildAuthoredKeywords takes the snapshot. Call after every change to
// the items map, where the change is made.
func rebuildAuthoredKeywords() {
	snap := &authoredSnapshot{
		words: make(map[string]bool, len(items)*2),
		names: make(map[string]bool, len(items)),
	}
	for id, spec := range items {
		if id == BaubleItemId || spec == nil {
			continue
		}
		if w := strings.ToLower(strings.TrimSpace(spec.NameSimple)); w != `` {
			snap.words[w] = true
		}
		// Every word of the name, not only its head noun: a bauble keyed
		// "silver" would fully match `get silver` and take it over a real
		// "Silver Dagger", which that word only partly matches.
		for _, f := range strings.Fields(spec.Name) {
			if w := headNoun(f); w != `` {
				snap.words[w] = true
			}
		}
		if n := normalizeItemName(spec.Name); n != `` {
			snap.names[n] = true
		}
	}
	authored.Store(snap)
}

// AuthoredKeyword reports whether word is what a real, authored item
// answers to: its keyword (NameSimple), or any word of its name ("lantern"
// and "hooded" for "Hooded Lantern"). The bauble carrier is not counted. Bauble keywords are kept off these so `get lantern` never picks
// up a model-named trinket instead of the lantern. Safe from any goroutine:
// it reads the snapshot (authored), never the items map.
func AuthoredKeyword(word string) bool {
	snap := authored.Load()
	if snap == nil {
		return false
	}
	return snap.words[strings.ToLower(strings.TrimSpace(word))]
}

// AuthoredName reports whether name is a real, authored item's whole name,
// compared after normalizeItemName. The bauble carrier is not counted. A
// bauble may not take one (baubles.CleanReply). Safe from any goroutine.
func AuthoredName(name string) bool {
	snap := authored.Load()
	if snap == nil {
		return false
	}
	return snap.names[normalizeItemName(name)]
}

// normalizeItemName is a name as AuthoredName compares it: NFKC (fullwidth
// and other compatibility letters folded), lower case, every run of
// Unicode space one plain space, trimmed; a hyphen counted as a space; one
// leading article ("a", "an", "the", whole word) stripped; and trailing
// punctuation (. ! ? , ; :) stripped. Applied to an authored name when the
// snapshot is built, and to a candidate in AuthoredName, so "The Hooded
// Lantern", "Hooded Lantern." and "A Beeswax Votive Candle" all normalise to
// the same form as their plain "Hooded Lantern" / "Beeswax Votive Candle"
// counterparts (review: the exact-match gap let those near-misses pass as
// not-authored).
func normalizeItemName(s string) string {
	s = strings.ReplaceAll(strings.ToLower(norm.NFKC.String(s)), `-`, ` `)
	fields := strings.Fields(s)
	if len(fields) > 1 {
		switch fields[0] {
		case `a`, `an`, `the`:
			fields = fields[1:]
		}
	}
	return strings.TrimRight(strings.Join(fields, ` `), `.!?,;:`)
}

// headNoun is a name's last word as a keyword: lower case, with a
// possessive and any punctuation around it dropped.
func headNoun(w string) string {
	w = strings.ToLower(strings.Trim(w, `.,;:!?"()[]`))
	w = strings.TrimSuffix(strings.TrimSuffix(w, `'s`), `'`)
	return w
}

func GetAllItemNames() []string {

	itemNames := []string{}
	for _, item := range items {
		itemNames = append(itemNames, item.Name)
	}
	return itemNames
}

// Presumably to ensure the datafile hasn't messed something up.
func (i *ItemSpec) Id() int {
	return i.ItemId
}

func CanBackstab(iSubType ItemSubType) bool {
	if iSubType == Cleaving || iSubType == Stabbing || iSubType == Slashing || iSubType == Claws {
		return true
	}
	return false
}

func (i *ItemSpec) AutoCalculateValue() {

	val := 5 // base value of 5

	// Weapon based damage valuation
	if i.Damage.BaseDamage > 0 {
		val += i.Damage.BaseDamage * i.Damage.BaseDamage * 2
	} else {
		val += (i.Damage.DiceCount * i.Damage.DiceCount) * (i.Damage.SideCount * i.Damage.SideCount * 2)
		val += i.Damage.BonusDamage * 25
	}
	// Armor valuation: priced by real mitigation per channel. Replaces the
	// legacy DamageReduction² term (2026-08-03 migration) — every item that
	// set damagereduction carried the identical physical_mitigation value, so
	// legacy items reprice identically; magical/conviction armor is now
	// finally worth something here too. Only affects enchant re-pricing and
	// admin-created items; authored `value:` fields always win.
	val += (i.PhysicalMitigation*i.PhysicalMitigation +
		i.MagicalMitigation*i.MagicalMitigation +
		i.ConvictionMitigation*i.ConvictionMitigation) * 17

	// Get the value of any condition it applies
	for _, conditionId := range i.ConditionIds {
		if conditionSpec := conditions.GetConditionSpec(conditionId); conditionSpec != nil {
			val += conditionSpec.GetValue()
		}
	}

	for _, statMod := range i.StatMods {
		val += statMod * 11
	}

	// Special considerations
	if i.Uses > 1 {
		val *= i.Uses
	}

	if i.Type == Lockpicks {
		val *= 2
	}

	if i.Hands > 1 {
		val = int(math.Ceil(float64(val) * 1.25))
	}

	if i.Type == Ring {
		// rings are atomatically worth more, since they are jewelry
		val *= 2
	}

	i.Value = val
}

func (i *ItemSpec) ItemFolder(baseonly ...bool) string {
	folderName := ``
	if i.ItemId >= 40000 {
		folderName = `materials-40000` // Stage 13.1: crafting material items
	} else if i.ItemId >= 30000 {
		folderName = `consumables-30000`
	} else if i.ItemId >= 20000 {

		if len(baseonly) > 0 && baseonly[0] {
			folderName = `armor-20000`
		} else {
			folderName = `armor-20000/` + string(i.Type)
		}

	} else if i.ItemId >= 10000 {
		folderName = `weapons-10000`
	} else {
		folderName = `other-0`
	}

	return folderName
}

// Presumably to ensure the datafile hasn't messed something up.
func (i *ItemSpec) Validate() error {

	if i.Type == Weapon {
		if i.Hands == 0 {
			i.Hands = 1
		}
		if i.Damage.Attacks < 1 {
			i.Damage.Attacks = 1
		}
	}

	if i.NameSimple == `` {
		i.NameSimple = i.Name
	}

	if i.DisplayName != `` {
		i.DisplayName = util.ConvertColorShortTags(i.DisplayName)
	}

	if i.Damage.BaseDamage == 0 {
		i.Damage.InitDiceRoll(i.Damage.DiceRoll)
		i.Damage.FormatDiceRoll()
	}

	if i.Value < 1 {
		i.AutoCalculateValue()
	}

	for name, v := range map[string]float64{
		"reserve_health_pct": i.ReserveHealthPct, "reserve_stamina_pct": i.ReserveStaminaPct, "reserve_conviction_pct": i.ReserveConvictionPct,
		"hunger_drain_pct": i.HungerDrainPct,
	} {
		if v < 0 || v >= 1 {
			return fmt.Errorf("item %d: %s must be in [0,1), got %v", i.ItemId, name, v)
		}
	}
	if i.HungerRounds < 0 {
		return fmt.Errorf("item %d: hunger_rounds must not be negative, got %d", i.ItemId, i.HungerRounds)
	}
	if i.MutationTickInterval < 0 {
		return fmt.Errorf("item %d: mutation_tick_interval must not be negative, got %d", i.ItemId, i.MutationTickInterval)
	}
	if i.MutationTickChance < 0 || i.MutationTickChance > 100 {
		return fmt.Errorf("item %d: mutation_tick_chance must be 0-100, got %d", i.ItemId, i.MutationTickChance)
	}
	if i.MutationTickInterval > 0 && (i.MutationTickChance < 1 || i.MutationTickChance > 100) {
		return fmt.Errorf("item %d: mutation_tick_chance must be 1-100 when mutation_tick_interval is set, got %d", i.ItemId, i.MutationTickChance)
	}
	if i.MutationRarityFloor < 0 || i.MutationRarityFloor > 10 {
		return fmt.Errorf("item %d: mutation_rarity_floor must be 0-10, got %d", i.ItemId, i.MutationRarityFloor)
	}
	switch i.Fixture {
	case ``, FixtureLight, FixtureDarkness:
	default:
		return fmt.Errorf("item %d: fixture must be %q or %q, got %q", i.ItemId, FixtureLight, FixtureDarkness, i.Fixture)
	}

	return nil
}

func (i *ItemSpec) Filename() string {

	filename := util.ConvertForFilename(i.Name)
	return fmt.Sprintf("%d-%s.yaml", i.ItemId, filename)
}

func (i *ItemSpec) Filepath() string {
	return i.ItemFolder() + `/` + i.Filename()
}

func GetItemSpec(itemId int) *ItemSpec {
	if itemId > 0 {
		spec, ok := items[itemId]
		if ok {
			return spec
		}
	}
	return nil
}

// FindSpecByComponentTag returns the CHEAPEST ItemSpec carrying a matching
// ComponentTag, or nil if none is found. Lowest MaterialTier wins; ties break
// on lowest ItemId.
//
// 🔴 IT USED TO RETURN THE FIRST MATCH FROM A MAP RANGE, and that was a BUG
// FOUND IN PLAYTEST on 2026-08-29. Go randomises map iteration, and FOUR items
// share component_tag "bottle" — Clay Flask and Glass Vial (tier 1), Sealed
// Phial (tier 3), Crystalline Decanter (tier 4). So this function had no
// defined answer, and its callers include actions.storeRecovered, which CREATES
// the item a player receives from salvage.
//
// The consequence was a material UPGRADE LOOP: craft a potion with the cheapest
// flask, salvage it, and the bottle handed back could be any of the four. A
// Crystalline Decanter is the most valuable bottle in the game (0.25x potion
// aging) and could be farmed out of Clay Flasks by repeating a craft-salvage
// cycle. Nothing about that was visible to a player or a reviewer; it only
// surfaced when someone actually ran the loop.
//
// THE FIX IS HERE RATHER THAN AT THE CALL SITES on purpose. There are seven
// callers across salvage, mob crafting, idle mobs, planners and shop pricing;
// converting them one at a time is how the last audit MISSED this one, having
// removed the same resolver from craft difficulty and from shop stock in the
// same slice. Nothing wants an arbitrary answer, so the primitive is the right
// place to make it defined.
//
// CHEAPEST, not merely deterministic: a tag must never be redeemable for
// something better than the commonest form of that material, or the loop above
// reopens with a fixed destination instead of a random one.
//
// ⚠️ Shop stock has its own resolver, shops.FindStockedIngredient, which
// additionally restricts to what a shop actually carries. Use that on any path
// that deducts from or checks a shop's inventory.
func FindSpecByComponentTag(tag string) *ItemSpec {
	var best *ItemSpec
	for _, spec := range items {
		if spec.ComponentTag != tag {
			continue
		}
		if best == nil ||
			spec.MaterialTier < best.MaterialTier ||
			(spec.MaterialTier == best.MaterialTier && spec.ItemId < best.ItemId) {
			best = spec
		}
	}
	return best
}

// LoadDataFiles loads every item spec and the combat and defense message
// sets at boot. It panics on a load error: the server cannot run without
// them. A live reload calls LoadDataFilesE instead.
func LoadDataFiles() {
	if err := LoadDataFilesE(); err != nil {
		panic(err)
	}
}

// LoadDataFilesE is LoadDataFiles for `reload items`: it returns the error
// instead of panicking (#271). The items, attack messages and defense
// messages all load into temporaries first and swap in together only after
// every one has loaded, so a failure leaves the previous data fully live.
func LoadDataFilesE() (err error) {
	// casing.AssertCanonical panics on a non-canonical name; a reload
	// reports that like any other load error.
	defer func() {
		if r := recover(); r != nil {
			if re, ok := r.(error); ok {
				err = fmt.Errorf("%w", re)
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	start := time.Now()

	dataPath := string(configs.GetFilePathsConfig().DataFiles)
	tmpItems, err := fileloader.LoadAllFlatFiles[int, *ItemSpec](dataPath + `/items`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/items`)
	}

	for id, spec := range tmpItems {
		if spec.Name != "" {
			casing.AssertCanonical(spec.Name, "item", fmt.Sprintf("%d", id))
		}
		if spec.DisplayName != "" {
			casing.AssertCanonical(spec.DisplayName, "item displayname", fmt.Sprintf("%d", id))
		}
	}

	tmpAttackMessages, err := fileloader.LoadAllFlatFiles[ItemSubType, *WeaponAttackMessageGroup](dataPath + `/combat-messages`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/combat-messages`)
	}

	tmpDefenseMessages, err := fileloader.LoadAllFlatFiles[DefencePool, *DefenseMessageGroup](dataPath + `/defense-messages`)
	if err != nil {
		return errors.Wrap(err, `filepath: `+dataPath+`/defense-messages`)
	}

	// Every set loaded; swap them in together. rebuildAuthoredKeywords reads
	// the package items map, so it follows the items swap.
	items = tmpItems
	rebuildAuthoredKeywords()
	attackMessages = tmpAttackMessages
	defenseMessages = tmpDefenseMessages

	mudlog.Info("itemspec.LoadDataFiles()", "itemLoadedCount", len(items), "attackMessageCount", len(attackMessages), "defenseMessageCount", len(defenseMessages), "Time Taken", time.Since(start))

	return nil
}

// RegisterTestItemSpec is a test-only helper that registers an ItemSpec
// in the global items registry. Used for unit tests that need GetItemSpec()
// to return a spec. The spec is persisted across tests in the same run.
func RegisterTestItemSpec(spec *ItemSpec) {
	if spec.ItemId > 0 {
		items[spec.ItemId] = spec
		rebuildAuthoredKeywords()
	}
}
