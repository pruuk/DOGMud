package conditions

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func stackingSpec() *ConditionSpec {
	return &ConditionSpec{ConditionId: 930, Name: "Gash", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 4,
		Flags: []Flag{Bleeding, Stacking}, TickPool: "health", TickFromMagnitude: true}
}

func heldOne(t *testing.T, bs *Conditions, id int) *Condition {
	t.Helper()
	held := bs.GetConditions(id)
	if len(held) != 1 {
		t.Fatalf("want exactly one held record %d, got %d", id, len(held))
	}
	return held[0]
}

func TestStackingAddAppendsAStackInsteadOfOverwriting(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	if !bs.AddConditionMagnitude(930, 3, -2) || !bs.AddConditionMagnitude(930, 5, -3) {
		t.Fatal("both adds must land")
	}
	b := heldOne(t, &bs, 930)
	if want := []Stack{{RoundsLeft: 3, Amount: -2}, {RoundsLeft: 5, Amount: -3}}; !reflect.DeepEqual(b.Stacks, want) {
		t.Fatalf("Stacks = %+v, want %+v", b.Stacks, want)
	}
	if b.TriggersLeft != 5 {
		t.Fatalf("TriggersLeft = %d, want 5 (the longest stack)", b.TriggersLeft)
	}
	if b.TickAmount != -5 || b.Magnitude != -5 {
		t.Fatalf("TickAmount %d / Magnitude %v, want -5 / -5 (the sum of the stacks)", b.TickAmount, b.Magnitude)
	}
}

func TestStackingTriggerSumsDecrementsAndDropsStacks(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 2, -2)
	bs.AddConditionMagnitude(930, 4, -3)

	type round struct {
		tick         int
		stacks       []Stack
		triggersLeft int
	}
	want := []round{
		{-5, []Stack{{1, -2}, {3, -3}}, 3},
		{-5, []Stack{{2, -3}}, 2},
		{-3, []Stack{{1, -3}}, 1},
		{-3, []Stack{}, 0},
	}
	for i, w := range want {
		fired := bs.Trigger()
		if len(fired) != 1 {
			t.Fatalf("round %d: want the record to fire once, got %d", i+1, len(fired))
		}
		b := fired[0]
		if b.TickAmount != w.tick {
			t.Fatalf("round %d: TickAmount = %d, want %d (every live stack summed)", i+1, b.TickAmount, w.tick)
		}
		if len(b.Stacks) != len(w.stacks) || (len(w.stacks) > 0 && !reflect.DeepEqual(b.Stacks, w.stacks)) {
			t.Fatalf("round %d: Stacks = %+v, want %+v", i+1, b.Stacks, w.stacks)
		}
		if b.TriggersLeft != w.triggersLeft {
			t.Fatalf("round %d: TriggersLeft = %d, want %d", i+1, b.TriggersLeft, w.triggersLeft)
		}
	}
	if !bs.List[0].Expired() {
		t.Fatal("the record must be expired once its last stack ends")
	}
	if fired := bs.Trigger(); len(fired) != 0 {
		t.Fatalf("an expired stacking record must not fire again, got %d", len(fired))
	}
}

func TestStackingZeroTriggersUsesTheSpecCount(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 0, -1)
	if got := heldOne(t, &bs, 930).Stacks[0].RoundsLeft; got != 4 {
		t.Fatalf("RoundsLeft = %d, want the spec's triggercount 4", got)
	}
}

func TestStackingAmountFloorsToOneInSign(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 2, -0.5)
	if got := heldOne(t, &bs, 930).Stacks[0].Amount; got != -1 {
		t.Fatalf("Amount = %d, want -1: a non-zero magnitude never snapshots to zero", got)
	}
}

// A plain AddCondition/AddConditionScaled is refused for a stacking spec (see
// TestAddConditionRefusesAStackingSpec) and addStack refuses a zero magnitude (see
// TestAddStackRefusesZeroMagnitude), so only a hand-built or legacy
// (pre-slice-1b) record can hold no stacks. If one exists it must not reach
// the tick path: a zero TickAmount there falls back to tick_percent.
func TestStackingRecordWithNoStacksExpiresWithoutFiring(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.List = append(bs.List, &Condition{ConditionId: 930, TriggersLeft: 3, TickAmount: -5})
	bs.Validate(true)
	if fired := bs.Trigger(); len(fired) != 0 {
		t.Fatalf("a stacking record with no stacks must not fire, got %d", len(fired))
	}
	if !bs.List[0].Expired() {
		t.Fatal("and it must expire")
	}
}

func TestRemoveConditionClearsStacks(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 3, -2)
	bs.RemoveCondition(930)
	if len(bs.List[0].Stacks) != 0 {
		t.Fatalf("RemoveCondition must clear the stacks, got %+v", bs.List[0].Stacks)
	}
}

// A cancel path (HasFlag with expire, which CancelConditionsWithFlag calls) expires
// a record through Condition.expire(), which already clears Stacks (see
// TestHasFlagExpireClearsStacks). This test covers the same shape from
// addStack's side: a new stack landing on an expired, unpruned record must
// not resurrect the old ones, whether or not something upstream already
// cleared them.
func TestStackingAddOnAnExpiredRecordStartsFresh(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 3, -2)
	bs.ExpireConditionsWithFlag(Bleeding)
	bs.AddConditionMagnitude(930, 5, -3)
	if want := []Stack{{RoundsLeft: 5, Amount: -3}}; !reflect.DeepEqual(bs.List[0].Stacks, want) {
		t.Fatalf("Stacks = %+v, want %+v", bs.List[0].Stacks, want)
	}
}

func TestNonStackingRecordStillOverwrites(t *testing.T) {
	withSpecs(t, &ConditionSpec{ConditionId: 931, Name: "Sting", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 4,
		TickPool: "health", TickFromMagnitude: true})
	bs := New()
	bs.AddConditionMagnitude(931, 3, -2)
	bs.AddConditionMagnitude(931, 5, -3)
	b := heldOne(t, &bs, 931)
	if b.TriggersLeft != 5 || b.TickAmount != -3 || len(b.Stacks) != 0 {
		t.Fatalf("a non-stacking record overwrites: TriggersLeft %d TickAmount %d Stacks %+v, want 5 -3 []", b.TriggersLeft, b.TickAmount, b.Stacks)
	}
}

func TestValidateRefusesStackingWithoutTickFromMagnitude(t *testing.T) {
	s := &ConditionSpec{ConditionId: 932, Name: "Bad", TriggerRate: "1 round", TriggerCount: 1, Flags: []Flag{Stacking}, TickPool: "health", TickPercent: -1}
	if err := s.Validate(); err == nil {
		t.Fatal("a stacking record without tick_from_magnitude must be refused")
	}
}

func TestValidateRefusesStackingSlowerThanOneRound(t *testing.T) {
	s := &ConditionSpec{ConditionId: 933, Name: "Bad", TriggerRate: "3 rounds", TriggerCount: 1, Flags: []Flag{Stacking}, TickPool: "health", TickFromMagnitude: true}
	if err := s.Validate(); err == nil {
		t.Fatal("a stacking record must tick every round: a stack counts rounds")
	}
}

func TestValidateAcceptsAWellFormedStackingRecord(t *testing.T) {
	if err := stackingSpec().Validate(); err != nil {
		t.Fatalf("a one-round tick_from_magnitude stacking record is legal: %v", err)
	}
}

func TestStacksRoundTripThroughYaml(t *testing.T) {
	in := []*Condition{{ConditionId: 930, TriggersLeft: 5, TickAmount: -5, Magnitude: -5,
		Stacks: []Stack{{RoundsLeft: 3, Amount: -2}, {RoundsLeft: 5, Amount: -3}}}}
	out, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var back []*Condition
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || !reflect.DeepEqual(back[0].Stacks, in[0].Stacks) {
		t.Fatalf("stacks must survive a save: got %+v from\n%s", back, out)
	}
}

// The equilibrium the owner asked for: one stack every 4 rounds (the shipped
// SpecialMoveCooldown), ticked every round. After warm-up the live count is
// exactly Rounds/4 when that divides evenly, and moves between floor and ceil
// when it does not.
//
// It also pins the tick's arithmetic: TickAmount on the round's fired record
// must be -2 times however many stacks were live going INTO that tick. That
// count is read off bs.List before calling Trigger, not recomputed, because
// nothing else touches the record between one iteration's tick+add and the
// next iteration's tick.
func TestStackEquilibriumAtTheShippedCooldown(t *testing.T) {
	cases := []struct{ rounds, lo, hi int }{{8, 2, 2}, {10, 2, 3}, {12, 3, 3}}
	for _, c := range cases {
		withSpecs(t, stackingSpec())
		bs := New()
		for r := range 40 {
			stacksBefore := 0
			if len(bs.List) > 0 {
				stacksBefore = len(bs.List[0].Stacks)
			}

			fired := bs.Trigger()
			if stacksBefore == 0 {
				if len(fired) != 0 {
					t.Fatalf("stack length %d, round %d: fired %d records with no stacks live going in", c.rounds, r, len(fired))
				}
			} else {
				if len(fired) != 1 {
					t.Fatalf("stack length %d, round %d: want 1 fired record, got %d", c.rounds, r, len(fired))
				}
				if want := -2 * stacksBefore; fired[0].TickAmount != want {
					t.Fatalf("stack length %d, round %d: TickAmount = %d, want %d (%d stacks live going into the tick)", c.rounds, r, fired[0].TickAmount, want, stacksBefore)
				}
			}

			if r%4 == 0 {
				bs.AddConditionMagnitude(930, c.rounds, -2)
			}
			if r < 12 {
				continue
			}
			if n := len(bs.List[0].Stacks); n < c.lo || n > c.hi {
				t.Fatalf("stack length %d, round %d: %d live stacks, want %d to %d", c.rounds, r, n, c.lo, c.hi)
			}
		}
	}
}

// HasFlag's expire branch backs CancelConditionsWithFlag, including
// CancelConditionsWithFlag(conditions.All) on death (Life_Cascades.go). It must clear
// Stacks along with TriggersLeft: AddCondition, AddConditionScaled and RefreshCondition can
// all revive an expired, unpruned record, and without this a dead bleed's
// stale stacks would come back to life on the next add.
func TestHasFlagExpireClearsStacks(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	bs.AddConditionMagnitude(930, 3, -2)
	if !bs.ExpireConditionsWithFlag(Bleeding) {
		t.Fatal("ExpireConditionsWithFlag must find the bleeding record")
	}
	b := bs.List[0]
	if len(b.Stacks) != 0 {
		t.Fatalf("Stacks = %+v, want none: the expire branch must clear them", b.Stacks)
	}
	if !b.Expired() {
		t.Fatal("the record must be expired")
	}
}

// A stacking record can only be added through AddConditionMagnitude, because
// only that door supplies a stack's rounds and amount. AddCondition and
// AddConditionScaled are also public (admin condition command,
// UserRecord.AddCondition, Mob.AddCondition, worn-item permanent conditions),
// and Condition_ApplyConditions.go routes a zero-magnitude, zero-trigger
// event to AddCondition, so both must refuse a stacking spec instead of
// creating a live record with no stacks that later prints a phantom end
// line.
func TestAddConditionRefusesAStackingSpec(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	if bs.AddCondition(930, false) {
		t.Fatal("AddCondition must refuse a stacking spec")
	}
	if bs.HasCondition(930) {
		t.Fatal("a refused add must hold nothing")
	}
}

func TestAddConditionScaledRefusesAStackingSpec(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	if bs.AddConditionScaled(930, 1.0) {
		t.Fatal("AddConditionScaled must refuse a stacking spec")
	}
	if bs.HasCondition(930) {
		t.Fatal("a refused add must hold nothing")
	}
}

// A zero-magnitude stack would still lengthen the record (a new RoundsLeft
// entry) and print a bleed line on its own tick, for zero harm. addStack
// refuses it outright rather than snapshot a Amount of 0.
func TestAddStackRefusesZeroMagnitude(t *testing.T) {
	withSpecs(t, stackingSpec())
	bs := New()
	if bs.AddConditionMagnitude(930, 3, 0) {
		t.Fatal("a zero-magnitude stack must be refused")
	}
	if bs.HasCondition(930) {
		t.Fatal("a refused add must hold nothing")
	}
}

func TestDisplayName(t *testing.T) {
	spec := stackingSpec()
	one := &Condition{ConditionId: 930, Stacks: []Stack{{RoundsLeft: 2, Amount: -1}}}
	three := &Condition{ConditionId: 930, Stacks: []Stack{{2, -1}, {3, -1}, {4, -1}}}
	plain := &Condition{ConditionId: 930}
	if got := DisplayName(plain, spec); got != "Gash" {
		t.Fatalf("no stacks: %q, want %q", got, "Gash")
	}
	if got := DisplayName(one, spec); got != "Gash" {
		t.Fatalf("one stack: %q, want %q (a count of one says nothing)", got, "Gash")
	}
	if got := DisplayName(three, spec); got != "Gash (3)" {
		t.Fatalf("three stacks: %q, want %q", got, "Gash (3)")
	}
}
