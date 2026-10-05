package questengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// A player buying one of Veyra's secrets may already know some of what its
// quest_granted trigger teaches (learned before the sub-recipes became
// learn_only, or by an admin). The price is paid in the dialogue before the
// token is granted, so this trigger only teaches. Because an action error
// abandons the rest of a trigger, re-teaching a known recipe must not be an
// error: every recipe the buyer lacked is learned, the known one is not
// announced again, no gold moves here, and the trigger runs to the end.
func TestSecretPurchase_WithAKnownSubRecipe(t *testing.T) {
	files, err := filepath.Glob("../../_datafiles/world/dogmud/quests/*-secret_of_the_*.yaml")
	require.NoError(t, err)
	require.Len(t, files, 9, "expected Veyra's nine secret quests")

	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			var q QuestDef
			require.NoError(t, yaml.Unmarshal(data, &q))

			var token string
			var recipes []string
			for _, tr := range q.Triggers {
				if tr.Event != "quest_granted" {
					continue
				}
				token = tr.QuestToken
				for _, a := range tr.Actions {
					require.Zero(t, a.ChargeGold, "the price is charged in the dialogue, not here")
					if a.LearnRecipe != nil {
						recipes = append(recipes, a.LearnRecipe.Recipe)
					}
				}
			}
			require.GreaterOrEqual(t, len(recipes), 2, "need a known recipe and at least one unknown")

			const gold = 777
			u, cleanup := seedChargeGoldUser(t, gold)
			defer cleanup()
			known := recipes[0]
			u.Character.LearnRecipe(known)

			e := NewEngine()
			e.RegisterQuest(&q)
			bridge := NewGameBridge(u, 1)
			e.Notify("quest_granted", EventDetails{UserId: cgUserId, QuestToken: token}, bridge, bridge)

			require.Equal(t, gold, u.Character.Gold, "the trigger moves no gold")
			for _, r := range recipes {
				require.True(t, u.Character.HasRecipe(r), "recipe %s not learned; the trigger was cut short", r)
			}

			msgs := strings.Join(events.DrainQueuedMessagesForTest(cgUserId), "\n")
			require.NotContains(t, msgs, "You pay")
			require.NotContains(t, msgs, "itemname\">"+known+"<",
				"the already-known recipe was announced again")
			for _, r := range recipes[1:] {
				require.Equal(t, 1, strings.Count(msgs, "itemname\">"+r+"<"),
					"recipe %s should be announced exactly once:\n%s", r, msgs)
			}
		})
	}
}
