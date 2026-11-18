package users

import (
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/util"
	"golang.org/x/crypto/bcrypt"
)

var (
	// dummyHashOnce makes dummyHash on first use, so boot pays nothing.
	dummyHashOnce sync.Once
	// dummyHash is a bcrypt hash at the default cost that no account's
	// password is checked against; it only spends the same work.
	dummyHash []byte
	// passwordCheckSpent, when set, is called after each SpendPasswordCheck.
	// Only tests set it (CountSpentPasswordChecksForTest).
	passwordCheckSpent func()
)

// SpendPasswordCheck runs one bcrypt comparison against a fixed hash. A
// login that has no account to check (an unknown username) calls it so it
// takes as long as a wrong password for a real account, and the timing does
// not tell which usernames exist. The game login and the /admin login both
// use it.
func SpendPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte(`dogmud-unknown-user`), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
	if passwordCheckSpent != nil {
		passwordCheckSpent()
	}
}

// isBcryptHash reports whether a stored password is a bcrypt hash.
func isBcryptHash(p string) bool {
	return strings.HasPrefix(p, "$2a$") || strings.HasPrefix(p, "$2b$") || strings.HasPrefix(p, "$2y$")
}

// SavePasswordUpgrade writes the bcrypt hash that PasswordMatches put on
// checked in place of a legacy unsalted SHA-256 hash. checked is a record
// loaded only to check a password (LoadUser with skipValidation) and
// storedBefore is its Password before the check. It returns false and does
// nothing unless the check replaced the password with a bcrypt hash.
//
// The hash goes on the record the server treats as the account: the live
// user when the account is in the world (online or link-dead), saved now so
// its next save cannot write the old hash back; otherwise a validated load
// of the file, as a game login makes, saved with the new hash. Either record
// must still hold storedBefore, so a password changed in the meantime is
// never overwritten. The world lock is held throughout, so a game login
// cannot publish the user between the check and the save. The caller must
// not hold it.
func SavePasswordUpgrade(checked *UserRecord, storedBefore string) (bool, error) {
	if checked.Password == storedBefore || !isBcryptHash(checked.Password) {
		return false, nil
	}
	hash := checked.Password

	util.LockMud()
	defer util.UnlockMud()

	if live := GetByUserId(checked.UserId); live != nil {
		if live.Password != storedBefore {
			return false, nil
		}
		live.Password = hash
		return true, SaveUser(live)
	}

	onDisk, err := LoadUser(checked.Username)
	if err != nil {
		return false, err
	}
	if onDisk.UserId != checked.UserId || onDisk.Password != storedBefore {
		return false, nil
	}
	onDisk.Password = hash
	return true, SaveUser(onDisk)
}
