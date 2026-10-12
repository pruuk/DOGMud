package combat

import (
	"strings"
	"testing"
)

// A wordless taunt (a beast's snarl, any howl) hurts conviction without words,
// so its damage text must not name an insult, a rebuke, a verbal assault or a
// tirade (M6 slice 3). The bands that name resolve, confidence and will read
// right for any taunt and are shared.
func TestWordlessConvictionDamageNamesNoWords(t *testing.T) {
	verbal := []string{"insult", "rebuke", "verbal", "tirade", "words"}
	for _, max := range []int{0, 100} {
		for dmg := 0; dmg <= 100; dmg++ {
			for _, desc := range []string{
				GetWordlessConvictionDamageDescription(dmg, max),
				GetWordlessConvictionDamageDescriptionToTarget(dmg, max),
			} {
				for _, w := range verbal {
					if strings.Contains(desc, w) {
						t.Fatalf("wordless damage %d of %d reads %q, which names %q", dmg, max, desc, w)
					}
				}
			}
		}
	}
	if got := GetWordlessConvictionDamageDescriptionToTarget(60, 100); got != "a devastating attack on your will" {
		t.Fatalf("shared band changed: %q", got)
	}
	if got := GetConvictionDamageDescription(10, 100); got != "a stinging insult" {
		t.Fatalf("the spoken table must keep its words: %q", got)
	}
}
