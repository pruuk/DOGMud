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

// Asking about buying is not buying: only a plain request sells anything.
func TestIsQuestion(t *testing.T) {
	questions := []string{
		`how much to buy a room?`, `do you buy furniture?`, `can I buy a room`,
		`to tell me how to buy a deed?`, `if I can buy a home`, `what does a deed cost`,
		`will you buy back my key`,
	}
	for _, q := range questions {
		if !isQuestion(q) {
			t.Errorf("%q read as a request to buy", q)
		}
	}
	requests := []string{`buy a room`, `to buy a room`, `I want to buy a room`, `buy deed`, `buy a strongbox please`}
	for _, r := range requests {
		if isQuestion(r) {
			t.Errorf("%q read as a question", r)
		}
	}
}
