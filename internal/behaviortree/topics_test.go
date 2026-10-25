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

// QuestGrantsInFile reads every grant_quest action with the event its branch
// answers and the topic_is keys checked before it on the way down (Hadwen's
// rite shape), so the walkthrough can treat a tree grant as a Way in.
func TestQuestGrantsInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "9100-cleric_hadwen.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`tree:
  type: selector
  children:
    - type: sequence
      event: player_topic
      children:
        - type: condition
          check: topic_is
          topics: [begin, rite_wait]
        - type: condition
          check: player_has_quest
          quest: "30-start"
        - type: action
          do: grant_quest
          quest: "30-rite"
        - type: selector
          children:
            - type: action
              do: say
              text: "Hush."
        - type: action
          do: grant_quest_to_user
          quest: "30-end"
    - type: sequence
      event: room_enter
      children:
        - type: action
          do: grant_quest
          quest: "4-investigate"
    - type: decorator
      mod: cooldown
      child:
        type: sequence
        event: player_topic
        children:
          - type: action
            do: grant_quest
            quest: "7-start"
          - type: condition
            check: topic_is
            topic: late
`), 0644))
	got, err := QuestGrantsInFile(path)
	require.NoError(t, err)
	require.Equal(t, []TreeQuestGrant{
		{Token: "30-rite", Event: "player_topic", Topics: []string{"begin", "rite_wait"}},
		{Token: "30-end", Event: "player_topic", Topics: []string{"begin", "rite_wait"}},
		{Token: "4-investigate", Event: "room_enter"},
		{Token: "7-start", Event: "player_topic"},
	}, got, "a topic_is after the grant does not gate it")
}
