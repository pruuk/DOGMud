package guards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// TestVeyraSecretsDialogue walks Veyra's shipped dialogue for each of her
// nine secret recipes and pins who gets which answer. Since #375 each secret
// is one offer topic (the old price_, teach_, refuse_ and owned_ nodes folded
// together; see the file's header comment):
//   - the offer's pitch and the secrets_list topic both state the exact
//     price, and hearing the list never buys or charges;
//   - a known player who can pay sees the offer, and choosing it charges the
//     exact price once and grants the secret;
//   - a player short of gold does not see the offer (goldRequired), and an
//     offer forced through anyway charges and grants nothing;
//   - an owner, and a player Veyra does not know, never see the offer.
func TestVeyraSecretsDialogue(t *testing.T) {
	b, err := os.ReadFile("_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var df dialogue.DialogueFile
	if err := yaml.Unmarshal(b, &df); err != nil {
		t.Fatal(err)
	}

	type secret struct {
		key, token string
		price      int
	}
	var secrets []secret
	for _, tp := range df.Topics {
		if strings.HasPrefix(tp.Key, "teach_") {
			secrets = append(secrets, secret{tp.Key, tp.GrantsQuest, tp.ChargesGold})
		}
	}
	if len(secrets) != 9 {
		t.Fatalf("found %d teach_ topics; want Veyra's nine", len(secrets))
	}
	list := df.FindTopic("secrets_list")
	if list == nil {
		t.Fatal("Veyra has no secrets_list topic")
	}

	// Each secret is sold once: no secret's quest may be repeatable, or its
	// completion would clear the token the offer's questExcluded relies on.
	for _, s := range secrets {
		id := strings.TrimSuffix(s.token, "-start")
		matches, _ := filepath.Glob("_datafiles/world/dogmud/quests/" + id + "-*.yaml")
		if len(matches) != 1 {
			t.Fatalf("%s: found %d quest files", s.token, len(matches))
		}
		qb, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		var q struct {
			Repeatable bool `yaml:"repeatable"`
		}
		if err := yaml.Unmarshal(qb, &q); err != nil {
			t.Fatal(err)
		}
		if q.Repeatable {
			t.Errorf("%s is repeatable; a secret is sold once", matches[0])
		}
	}

	squash := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, s := range secrets {
		tp := df.FindTopic(s.key)
		end := strings.TrimSuffix(s.token, "-start") + "-end"
		priceText := util.FormatNumber(s.price) + " gold"
		if s.price <= 0 || tp.Kind != dialogue.KindOffer || tp.GoldRequired != s.price {
			t.Errorf("%s: want an offer charging and gated at one price, got kind %q, charges %d, goldRequired %d",
				s.key, tp.Kind, s.price, tp.GoldRequired)
		}
		if !strings.Contains(squash(tp.Text), priceText) {
			t.Errorf("%s: the pitch must state the price %q, got %q", s.key, priceText, tp.Text)
		}
		if !strings.Contains(squash(list.Text), util.FormatNumber(s.price)) {
			t.Errorf("secrets_list must name %s's price %s", s.key, util.FormatNumber(s.price))
		}

		player := func(gold int, held ...string) (*dialogue.PlayerState, *[]string, *int) {
			var granted []string
			paid := 0
			ps := &dialogue.PlayerState{
				QuestOpen: func(int) bool { return true },
				HasQuest: func(tok string) bool {
					for _, h := range held {
						if h == tok {
							return true
						}
					}
					return false
				},
				GiveQuest: func(tok string) { granted = append(granted, tok) },
				HasGold:   func(amount int) bool { return gold >= amount },
				ChargeGold: func(amount int) bool {
					if gold < amount {
						return false
					}
					gold -= amount
					paid += amount
					return true
				},
				HasHeard:  func(string) bool { return false },
				MarkHeard: func(string) {},
			}
			return ps, &granted, &paid
		}

		// Known and able to pay: on the menu, and choosing it buys once.
		ps, granted, paid := player(s.price, "78-end")
		if !df.Eligible(tp, ps) {
			t.Errorf("%s: a known player carrying %s must see the offer", s.key, priceText)
		} else if !df.Apply(tp, ps) || len(*granted) != 1 || (*granted)[0] != s.token || *paid != s.price {
			t.Errorf("%s, can pay: granted %v, paid %d; want [%s], %d", s.key, *granted, *paid, s.token, s.price)
		}

		// Hearing the price list buys and charges nothing.
		ps, granted, paid = player(1000000, "78-end")
		if !df.Eligible(list, ps) || !df.Apply(list, ps) || len(*granted) != 0 || *paid != 0 {
			t.Errorf("secrets_list for %s: granted %v and charged %d", s.key, *granted, *paid)
		}

		// Short of gold: hidden, and forcing it takes nothing.
		ps, granted, paid = player(s.price-1, "78-end")
		if df.Eligible(tp, ps) {
			t.Errorf("%s: a player short of %s must not see the offer", s.key, priceText)
		}
		if df.Apply(tp, ps) || len(*granted) != 0 || *paid != 0 {
			t.Errorf("%s, short: granted %v, paid %d; want nothing taken", s.key, *granted, *paid)
		}

		// Character.HasQuest reports an earlier step as held once a later one
		// is, so a finished secret answers true for both tokens.
		for _, held := range [][]string{{"78-end", s.token}, {"78-end", s.token, end}, {}} {
			ps, _, _ := player(1000000, held...)
			if df.Eligible(tp, ps) {
				t.Errorf("%s, holding %v: the offer must not show", s.key, held)
			}
		}
	}
}
