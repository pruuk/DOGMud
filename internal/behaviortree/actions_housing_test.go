package behaviortree

import "testing"

// A player asks in sentences, so a keyword followed by punctuation must still
// match ("do you have a room?"). Inner punctuation is kept.
func TestCondKeywordMatch_IgnoresEdgePunctuation(t *testing.T) {
	fn := LookupCondition("keyword_match")
	cases := []struct {
		text string
		want Result
	}{
		{`do you have a room?`, Success},
		{`"room", please.`, Success},
		{`(room)`, Success},
		{`roommate`, Failure},
		{`bedroom!`, Failure},
	}
	for _, c := range cases {
		ctx := &EvalContext{Event: EventContext{Text: c.text}}
		got := fn(map[string]any{"keywords": []any{"room"}}, ctx)
		if got != c.want {
			t.Errorf("text %q: got %v, want %v", c.text, got, c.want)
		}
	}

	ctx := &EvalContext{Event: EventContext{Text: `the smith's-mark, please`}}
	if got := fn(map[string]any{"keywords": []any{"smith's-mark"}}, ctx); got != Success {
		t.Errorf("inner punctuation should be kept, got %v", got)
	}
}

func TestBuyHousingActionRegistered(t *testing.T) {
	for _, name := range []string{"buy_housing", "housing_terms"} {
		if LookupAction(name) == nil {
			t.Fatalf("%s action not registered", name)
		}
	}
}

// Misconfigured params and missing actors fall through (Failure) so a later
// tree branch can still answer.
func TestBuyHousingFailsWithoutActors(t *testing.T) {
	for _, name := range []string{"buy_housing", "housing_terms"} {
		fn := LookupAction(name)
		ctx := &EvalContext{InstanceId: -1, Event: EventContext{UserId: -1}}
		if got := fn(map[string]any{"building": "x", "tier": "simple"}, ctx); got != Failure {
			t.Errorf("%s: got %v, want Failure", name, got)
		}
	}
}
