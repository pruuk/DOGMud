package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #289: `quests all+` lists every quest, secret ones included. Owner call
// 2026-10-10: admins only; anyone else gets the plain `all` view.
func TestQuestsShowHidden_AdminOnly(t *testing.T) {
	player := &users.UserRecord{Role: users.RoleUser}
	admin := &users.UserRecord{Role: users.RoleAdmin}

	assert.False(t, questsShowHidden(`all+`, player))
	assert.True(t, questsShowHidden(`all+`, admin))
	assert.False(t, questsShowHidden(`all`, admin))
	assert.False(t, questsShowHidden(``, admin))
}
