package conditions

import "testing"

// ExpiresOnNextTrigger must agree with Trigger itself on every round of a
// record's life: the round ticks use it to snapshot a room just before a
// light or darkness runs out (#220), so a disagreement either misses the
// snapshot or takes one for a record that lives on.
func TestExpiresOnNextTriggerAgreesWithTrigger(t *testing.T) {
	timed := func(id, interval, count int) *ConditionSpec {
		return &ConditionSpec{ConditionId: id, Name: "Timed", RoundInterval: interval, TriggerCount: count}
	}
	withSpecs(t,
		timed(931, 1, 3),
		timed(932, 2, 2),
		timed(933, 3, 1),
		&ConditionSpec{ConditionId: 934, Name: "Flag only"},
		stackingSpec(),
	)

	cases := []struct {
		name string
		add  func(bs *Conditions)
		id   int
	}{
		{"every round, three triggers", func(bs *Conditions) { bs.AddCondition(931, false) }, 931},
		{"every second round, two triggers", func(bs *Conditions) { bs.AddCondition(932, false) }, 932},
		{"every third round, one trigger", func(bs *Conditions) { bs.AddCondition(933, false) }, 933},
		{"unlimited never expires", func(bs *Conditions) { bs.AddCondition(931, true) }, 931},
		{"a flag record never ticks", func(bs *Conditions) { bs.AddCondition(934, false) }, 934},
		{"stacking, outlived by its longest stack", func(bs *Conditions) {
			bs.AddConditionMagnitude(930, 2, -2)
			bs.AddConditionMagnitude(930, 4, -3)
		}, 930},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bs := New()
			c.add(&bs)
			rec := bs.List[0]
			spec := GetConditionSpec(c.id)
			sawExpiry := false
			for round := 1; round <= 8; round++ {
				predicted := rec.ExpiresOnNextTrigger(spec)
				wasLive := !rec.Expired()
				bs.Trigger()
				expiredNow := wasLive && rec.Expired()
				if predicted != expiredNow {
					t.Fatalf("round %d: ExpiresOnNextTrigger said %v, Trigger expired it: %v", round, predicted, expiredNow)
				}
				sawExpiry = sawExpiry || expiredNow
			}
			wantExpiry := c.id != 934 && c.name != "unlimited never expires"
			if sawExpiry != wantExpiry {
				t.Fatalf("expired within eight rounds = %v, want %v: the case does not test what it says", sawExpiry, wantExpiry)
			}
		})
	}

	t.Run("nil spec", func(t *testing.T) {
		if (&Condition{TriggersLeft: 1}).ExpiresOnNextTrigger(nil) {
			t.Fatal("a record with no spec is never ticked by Trigger")
		}
	})
}
