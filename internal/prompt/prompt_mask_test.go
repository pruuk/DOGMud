package prompt

import "testing"

func TestQuestionMaskInput(t *testing.T) {
	p := New(`password`, ``)
	q := p.Ask(`What is your current password?`, []string{})
	if q.Masked() {
		t.Fatal("a new question must not be masked")
	}
	if q.MaskInput() != q {
		t.Fatal("MaskInput must be chainable")
	}
	if !q.Masked() {
		t.Fatal("question should be masked")
	}
	if !p.Ask(`What is your current password?`, []string{}).Masked() {
		t.Fatal("asking again returns the same, masked question")
	}
	var none *Question
	if none.Masked() {
		t.Fatal("a nil question is not masked")
	}
}
