package users

import (
	"sync"
	"testing"
)

// TestConcurrentUserAccess exercises the RWMutex. Must run under -race.
func TestConcurrentUserAccess(t *testing.T) {
	const writers = 2
	const readers = 8
	const iters = 300

	var wg sync.WaitGroup

	for w := range writers {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := range iters {
				u := &UserRecord{UserId: base*1000 + i}
				u.Username = "racer"
				storeUserForTest(u)
				removeUserForTest(u.UserId)
			}
		}(w)
	}

	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iters {
				_ = GetByUserId(i)
			}
		}()
	}

	wg.Wait()
}

func storeUserForTest(u *UserRecord) {
	userManager.mu.Lock()
	userManager.Users[u.UserId] = u
	userManager.mu.Unlock()
}

func removeUserForTest(id int) {
	userManager.mu.Lock()
	delete(userManager.Users, id)
	userManager.mu.Unlock()
}
