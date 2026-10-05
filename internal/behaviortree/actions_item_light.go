package behaviortree

import (
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The light actions (lighting 5e, Rules 9 to 11). Both need an item
// subject (Rule 8).
//
// A WORN item's light lives on its holder's condition records, and these
// write the record's existing trimmed-output state, the same state
// rooms.(*Room).TrimLightFor writes on entry (Rule 9, owner ruling R3): one
// mechanism, no second value. The two writers never share a record: the
// trim owns adjustable records and these refuse them, and the boot refuses a
// light-writing tree on an item whose worn light is adjustable
// (ValidateItemBehaviors). A FIXTURE's output lives in internal/itemlight
// (Rule 10). On any other item both actions fail and change nothing.

// itemLightLevel is a light level an item tree asks for.
type itemLightLevel struct {
	full  bool    // the record's full strength (worn lights only)
	off   bool    // no light at all
	value float64 // a strength on the light scale, when neither full nor off
}

// lightLevelParam reads set_light's level: full, off, or a number at or
// above 0. YAML 1.1 reads an unquoted off as the boolean false, so false
// means off too.
func lightLevelParam(v any) (itemLightLevel, bool) {
	switch x := v.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case `full`:
			return itemLightLevel{full: true}, true
		case `off`:
			return itemLightLevel{off: true}, true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil && f >= 0 {
			return itemLightLevel{value: f}, true
		}
	case bool:
		if !x {
			return itemLightLevel{off: true}, true
		}
	case int:
		if x >= 0 {
			return itemLightLevel{value: float64(x)}, true
		}
	case float64:
		if x >= 0 {
			return itemLightLevel{value: x}, true
		}
	}
	return itemLightLevel{}, false
}

// actSetLight: `set_light` with `level: full | off | <n>`.
func actSetLight(params map[string]any, ctx *EvalContext) Result {
	lv, ok := lightLevelParam(params["level"])
	if !ok {
		return Failure
	}
	return writeItemLight(ctx, lv)
}

// actPulseLight: `pulse_light` with `min`, `max`, `period_rounds` (at
// least 2): the triangle wave of PulseLightValue at this round.
func actPulseLight(params map[string]any, ctx *EvalContext) Result {
	min := getFloatParam(params, "min", -1)
	max := getFloatParam(params, "max", -1)
	period := getIntParam(params, "period_rounds")
	if min < 0 || max < min || period < 2 {
		return Failure
	}
	return writeItemLight(ctx, itemLightLevel{value: PulseLightValue(min, max, period, util.GetRoundCount())})
}

// PulseLightValue is pulse_light's output at a round: a triangle wave from
// min at round 0 up to max at half the period and back to min at the full
// period. Deterministic, no state (Rule 11).
func PulseLightValue(min, max float64, periodRounds int, round uint64) float64 {
	if periodRounds < 2 {
		periodRounds = 2
	}
	phase := float64(round % uint64(periodRounds))
	half := float64(periodRounds) / 2
	frac := phase / half
	if phase > half {
		frac = (float64(periodRounds) - phase) / half
	}
	return min + (max-min)*frac
}

// writeItemLight applies a level to the subject: a fixture on its floor, or
// a worn item's light and darkness records.
func writeItemLight(ctx *EvalContext, lv itemLightLevel) Result {
	if ctx == nil || ctx.Item == nil {
		return Failure
	}
	tmpl := items.GetItemSpec(ctx.Item.ItemId)
	if tmpl == nil {
		return Failure
	}

	if ctx.Item.OnFloor && tmpl.Fixture != `` {
		if lv.full {
			// A fixture has no condition, so no full strength of its own:
			// its tree names a number.
			return Failure
		}
		kind := itemlight.Light
		if tmpl.Fixture == items.FixtureDarkness {
			kind = itemlight.Darkness
		}
		v := lv.value
		if lv.off {
			v = lightscale.Absent()
		}
		itemlight.Set(ctx.Item.RoomId, ctx.Item.UUID, kind, v)
		return Success
	}

	if ctx.Item.Slot == `` {
		return Failure
	}
	holder := itemHolder(ctx)
	if holder == nil {
		return Failure
	}
	worn, ok := wornItemIn(holder, ctx.Item.Slot, ctx.Item.UUID)
	if !ok {
		return Failure
	}
	wrote := false
	for _, id := range worn.GetSpec().WornConditionIds {
		cspec := conditions.GetConditionSpec(id)
		if cspec == nil || !(cspec.IsLightSource() || cspec.IsDarknessSource()) {
			continue
		}
		// The trim owns an adjustable record; this never writes one (X22).
		if slices.Contains(cspec.Flags, conditions.Adjustable) {
			continue
		}
		for _, rec := range holder.Conditions.GetConditions(id) {
			writeRecordLight(rec, lv)
			wrote = true
		}
	}
	if !wrote {
		return Failure
	}
	return Success
}

// wornItemIn returns the item in the holder's slot when it is still the
// subject instance.
func wornItemIn(c *characters.Character, slot string, id uuid.UUID) (items.Item, bool) {
	for _, s := range c.Equipment.AllSlots() {
		if s.Key == slot && s.Item.ItemId > 0 && s.Item.UUID == id {
			return *s.Item, true
		}
	}
	return items.Item{}, false
}

// writeRecordLight writes a level to one record, only when its state
// changes: full is the trim's own full branch (LightFull, output 0), off is
// SetLightOutput(Absent) (LightOff), a number is SetLightOutput(n)
// (LightTrimmed; LightNow caps it at full strength).
func writeRecordLight(rec *conditions.Condition, lv itemLightLevel) {
	switch {
	case lv.full:
		if rec.LightTrim != conditions.LightFull || rec.LightOutput != 0 {
			rec.LightTrim, rec.LightOutput = conditions.LightFull, 0
		}
	case lv.off:
		if rec.LightTrim != conditions.LightOff {
			rec.SetLightOutput(lightscale.Absent())
		}
	default:
		if rec.LightTrim != conditions.LightTrimmed || rec.LightOutput != lv.value {
			rec.SetLightOutput(lv.value)
		}
	}
}
