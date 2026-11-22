package quests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestActionDef_RemovedKeysAreRefused(t *testing.T) {
	for _, key := range []string{"lock_exits", "unlock_exits"} {
		var a ActionDef
		err := yaml.Unmarshal([]byte(key+":\n  room: 113\n"), &a)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s: want a refusal naming the key, got %v", key, err)
		}
	}
	// Nested in a trigger's action list, as a real quest file carries it.
	var td TriggerDef
	err := yaml.Unmarshal([]byte("event: room_enter\nactions:\n  - lock_exits:\n      room: 1\n"), &td)
	if err == nil {
		t.Error("nested lock_exits was accepted")
	}
	// A live action still decodes.
	var ok ActionDef
	if err := yaml.Unmarshal([]byte("teleport: 200\n"), &ok); err != nil || ok.Teleport != 200 {
		t.Errorf("teleport action: %v %+v", err, ok)
	}
}
