package baubles

import (
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Bauble weight, in pounds (the unit of items.ItemSpec.Weight and so of
// encumbrance). The MODEL decides the weight, from what the object is: a
// child's wooden toy weighs a fraction of a pound, a large glazed vase
// several pounds. Code only keeps the answer sane.
const (
	MinWeightLbs     = 0.1  // anything lighter is rounded up to this
	MaxWeightLbs     = 25.0 // a bauble is carried loot, never furniture
	DefaultWeightLbs = 0.5  // used when the model's answer is unusable
)

// ClampWeight makes a proposed weight safe to put on an item: a missing,
// zero, negative or non-finite weight becomes DefaultWeightLbs, anything
// else is clamped to [MinWeightLbs, MaxWeightLbs] and rounded to a tenth of a
// pound.
func ClampWeight(w float64) float64 {
	if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
		return DefaultWeightLbs
	}
	w = math.Round(w*10) / 10
	if w < MinWeightLbs {
		return MinWeightLbs
	}
	if w > MaxWeightLbs {
		return MaxWeightLbs
	}
	return w
}

// MaxWeightFor is the heaviest a bauble from source may be: a pickpocketed
// one is pocket-sized (Balance.BaublePickpocketMaxWeight, default 1 lb);
// anything else MaxWeightLbs.
func MaxWeightFor(s Source) float64 {
	if s == SourcePickpocket {
		if w := float64(configs.GetBalanceConfig().BaublePickpocketMaxWeight); w > 0 && w < MaxWeightLbs {
			return w
		}
	}
	return MaxWeightLbs
}

// TooBigFor reports a reply that describes something too big for source.
// For a pickpocketed find: a weight the model proposed over the pocket
// limit (MaxWeightFor), or a name whose words are things no pocket holds
// (notPocketSized). Its text would name such a thing, so Generate refuses
// it (a fallback from the corpus's pocket pool) rather than only clamping
// the number.
func TooBigFor(r Reply, s Source) bool {
	if s != SourcePickpocket {
		return false
	}
	if r.WeightLbs > MaxWeightFor(s) {
		return true
	}
	for w := range strings.FieldsSeq(strings.ToLower(r.Name)) {
		w = strings.TrimSuffix(strings.TrimSuffix(strings.Trim(w, `.,;:!?"()`), `'s`), `'`)
		if notPocketSized[w] {
			return true
		}
	}
	return false
}

// notPocketSized are objects no pocket or purse holds, however light the
// model says they are ("a bronze urn, 0.4 lb").
var notPocketSized = map[string]bool{
	`urn`: true, `vase`: true, `statue`: true, `statuette`: true, `bust`: true, `idol`: true,
	`chest`: true, `strongbox`: true, `crate`: true, `barrel`: true, `cask`: true, `keg`: true,
	`candlestick`: true, `candelabra`: true, `lantern`: true, `lamp`: true, `jug`: true,
	`pitcher`: true, `ewer`: true, `flagon`: true, `jar`: true, `pot`: true, `kettle`: true,
	`cauldron`: true, `basin`: true, `bowl`: true, `platter`: true, `plate`: true, `tray`: true,
	`bucket`: true, `pail`: true, `basket`: true, `shield`: true, `helm`: true, `helmet`: true,
	`sword`: true, `axe`: true, `mace`: true, `spear`: true, `staff`: true, `bow`: true,
	`book`: true, `tome`: true, `painting`: true, `tapestry`: true, `rug`: true, `blanket`: true,
	`cloak`: true, `boots`: true, `saddle`: true, `anvil`: true, `mirror`: true, `clock`: true,
}

// ClampWeightFor is ClampWeight, then no heavier than source allows.
func ClampWeightFor(w float64, s Source) float64 {
	w = ClampWeight(w)
	if max := MaxWeightFor(s); w > max {
		w = math.Floor(max*10) / 10
		if w < MinWeightLbs {
			w = MinWeightLbs
		}
	}
	return w
}

// WeightGuidance is the scale the generation prompt gives the model, so its
// weights are consistent with each other and with the game's other items
// (a dagger is about 1 lb, a longsword about 3 lb).
const WeightGuidance = `Give weight_lbs as the object's real weight in pounds, judged from what it is and what it is made of. ` +
	`Tiny (0.1 to 0.5): a coin, ring, button, bead, thimble, small charm. ` +
	`Small (0.5 to 2): a child's wooden toy, a carved figurine, a small cup, a cloth doll, a pipe. ` +
	`Medium (2 to 8): a candlestick, a clay jug, a small box, a metal plate, a book. ` +
	`Large (8 to 25): a large vase, a heavy statuette, a small chest, a bronze urn. ` +
	`Never more than 25.`
