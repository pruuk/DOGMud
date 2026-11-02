package questengine

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// A player buying one of Veyra's secrets may already know some of what it
// teaches (learned before the sub-recipes became learn_only, or by an admin).
// The price is charged once by the sale, and an action that failed would
// abandon the rest of the list after they paid. So re-teaching a known recipe
// must not be an error: every recipe the buyer lacked is learned, the known
// one is not announced again, the sale charges its price once, and the loop
// runs to the end.
//
// Since #375 phase 7 tier 3b the secrets are sale topics on Veyra's dialogue
// (they were nine quests with a quest_granted trigger), and the teaching goes
// through LearnRecipeWithNotice, the line GameBridge.LearnRecipe prints too.
func TestSecretPurchase_WithAKnownSubRecipe(t *testing.T) {
	b, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml")
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(b, &df))

	var sales []dialogue.Topic
	for _, tp := range df.Topics {
		if tp.Kind == dialogue.KindSale {
			sales = append(sales, tp)
		}
	}
	require.Len(t, sales, 9, "expected Veyra's nine secrets")

	for _, sale := range sales {
		t.Run(sale.Key, func(t *testing.T) {
			recipes := sale.TeachesRecipes
			require.GreaterOrEqual(t, len(recipes), 2, "need a known recipe and at least one unknown")

			const gold = 777
			u, cleanup := seedChargeGoldUser(t, gold)
			defer cleanup()
			known := recipes[0]
			u.Character.LearnRecipe(known)

			var sent []string
			send := func(_ messaging.Category, text string) { sent = append(sent, text) }
			paid := 0
			ps := &dialogue.PlayerState{
				ChargeGold: func(n int) bool { paid += n; return true },
				LearnRecipe: func(id string) {
					LearnRecipeWithNotice(u.Character, id, send)
				},
			}
			require.True(t, df.Apply(&sale, ps))

			require.Equal(t, sale.ChargesGold, paid, "the sale charges its price once")
			require.Equal(t, gold, u.Character.Gold, "the test charge hook moves no real gold")
			for _, r := range recipes {
				require.True(t, u.Character.HasRecipe(r), "recipe %s not learned; the loop was cut short", r)
			}

			msgs := strings.Join(sent, "\n")
			require.NotContains(t, msgs, "itemname\">"+known+"<",
				"the already-known recipe was announced again")
			for _, r := range recipes[1:] {
				require.Equal(t, 1, strings.Count(msgs, "itemname\">"+r+"<"),
					"recipe %s should be announced exactly once:\n%s", r, msgs)
			}
		})
	}
}
