package behaviortree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayerTopicIsAKnownEvent(t *testing.T) {
	require.True(t, KnownBehaviorEvents["player_topic"])
}

func TestTopicIs_MatchesTheEventKeyOnly(t *testing.T) {
	ctx := &EvalContext{Event: EventContext{EventType: "player_topic", Text: "ferry"}}
	require.Equal(t, Success, condTopicIs(map[string]any{"topic": "ferry"}, ctx))
	require.Equal(t, Failure, condTopicIs(map[string]any{"topic": "portal"}, ctx))
	ctx.Event.EventType = "player_give"
	require.Equal(t, Failure, condTopicIs(map[string]any{"topic": "ferry"}, ctx), "only player_topic carries a key")
}

func TestTopicKeysInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "9574-a_weathered_deckhand.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`tree:
  type: selector
  children:
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topic: ferry
        - type: action
          do: board_ferry
          destination: plymouth
          routes:
            plymouth: stillwater_np_packet
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topic: fare
        - type: action
          do: say
          text: "Two coppers."
    - type: decorator
      mod: cooldown
      rounds: 3
      child:
        type: sequence
        event: player_topic
        children:
          - type: condition
            check: topic_is
            topic: rumor
          - type: action
            do: say
            text: "Heard a rumor."
`), 0644))
	keys, err := TopicKeysInFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"fare", "ferry", "rumor"}, keys)
}
