package main

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
// nine secret recipes, over EVERY trigger of each node, and pins who gets
// which answer:
//   - asking the bare name states the exact price and how to buy, and never
//     buys or charges;
//   - "teach <name>" from a known player who can pay charges the exact price
//     once and grants the secret;
//   - "teach <name>" from a player short of gold names the exact price and
//     grants and charges nothing;
//   - an owner gets "already carry that one" for any trigger, and nothing
//     is granted or charged.
func TestVeyraSecretsDialogue(t *testing.T) {
	b, err := os.ReadFile("_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var df dialogue.DialogueFile
	if err := yaml.Unmarshal(b, &df); err != nil {
		t.Fatal(err)
	}

	byId := map[string]dialogue.TreeNode{}
	type secret struct {
		slug, token string
		price       int
	}
	var secrets []secret
	for _, n := range df.Tree.Nodes {
		byId[n.Id] = n
		if strings.HasPrefix(n.Id, "teach_") {
			secrets = append(secrets, secret{strings.TrimPrefix(n.Id, "teach_"), n.GrantsQuest, n.ChargesGold})
		}
	}
	if len(secrets) != 9 {
		t.Fatalf("found %d teach_ nodes; want Veyra's nine", len(secrets))
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

	// A name trigger of one secret must not appear inside another secret's
	// triggers, or asking about one would answer for the other.
	for _, a := range secrets {
		for _, c := range secrets {
			if a.slug == c.slug {
				continue
			}
			for _, ta := range byId["price_"+a.slug].Triggers {
				for _, tc := range append(byId["price_"+c.slug].Triggers, byId["teach_"+c.slug].Triggers...) {
					if strings.Contains(tc, ta) {
						t.Errorf("trigger %q of %s is inside %q of %s", ta, a.slug, tc, c.slug)
					}
				}
			}
		}
	}

	const mobInst = 958400
	userId := 9000
	for _, s := range secrets {
		end := strings.TrimSuffix(s.token, "-start") + "-end"
		priceText := util.FormatNumber(s.price) + " gold"
		ask := func(topic string, gold int, held ...string) (string, string, []string, int) {
			userId++
			var granted []string
			paid := 0
			ps := &dialogue.PlayerState{
				HasQuest: func(tok string) bool {
					for _, h := range append([]string{"78-end"}, held...) {
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
			}
			text, hints, _, ok := dialogue.TreeAdvance(&df, mobInst, userId, topic, ps)
			dialogue.ResetMemory(mobInst, userId)
			if !ok {
				t.Fatalf("%s: no tree node answered %q", s.slug, topic)
			}
			return text, hints, granted, paid
		}

		for _, topic := range byId["teach_"+s.slug].Triggers {
			if _, _, g, paid := ask(topic, s.price); len(g) != 1 || g[0] != s.token || paid != s.price {
				t.Errorf("%s %q, can pay: granted %v, paid %d; want [%s], %d", s.slug, topic, g, paid, s.token, s.price)
			}
			if text, _, g, paid := ask(topic, s.price-1); len(g) != 0 || paid != 0 || !strings.Contains(text, priceText) {
				t.Errorf("%s %q, short: got %q, granted %v, paid %d; want the price %q and nothing taken", s.slug, topic, text, g, paid, priceText)
			}
		}
		for _, topic := range byId["price_"+s.slug].Triggers {
			text, hints, g, paid := ask(topic, 1000000)
			if len(g) != 0 || paid != 0 {
				t.Errorf("%s %q: asking the price granted %v and charged %d", s.slug, topic, g, paid)
			}
			if !strings.Contains(text, priceText) || !strings.Contains(hints, "ask veyra teach ") {
				t.Errorf("%s %q: want the price %q and a teach hint, got %q / %q", s.slug, topic, priceText, text, hints)
			}
		}
		// Character.HasQuest reports an earlier step as held once a later
		// one is, so a finished secret answers true for both tokens.
		for _, held := range [][]string{{s.token}, {s.token, end}} {
			for _, topic := range append(byId["price_"+s.slug].Triggers, byId["teach_"+s.slug].Triggers...) {
				text, _, g, paid := ask(topic, 1000000, held...)
				if len(g) != 0 || paid != 0 || !strings.Contains(text, "already") {
					t.Errorf("%s %q, owner %v: got %q, granted %v, paid %d", s.slug, topic, held, text, g, paid)
				}
			}
		}
	}
}
