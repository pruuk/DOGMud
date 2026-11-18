package users

import (
	"fmt"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

// After a copyover every web client reconnects at about the same moment, each
// on its own websocket goroutine. CopyoverReconnectUser writes the user maps,
// so without userManager.mu those writes collide and the runtime aborts with
// "concurrent map writes", which recover cannot catch. Many reconnects at once
// must all land.
func TestCopyoverReconnectUser_ConcurrentReconnects(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			u := &UserRecord{
				UserId:    5000 + i,
				Username:  fmt.Sprintf("copyover%d", i),
				Role:      RoleUser,
				Character: &characters.Character{Name: fmt.Sprintf("Copy%d", i)},
			}
			_, _, err := CopyoverReconnectUser(u, connections.ConnectionId(70000+i))
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	userManager.mu.RLock()
	defer userManager.mu.RUnlock()
	for i := range n {
		require.Equal(t, 5000+i, userManager.Connections[connections.ConnectionId(70000+i)])
	}
}
