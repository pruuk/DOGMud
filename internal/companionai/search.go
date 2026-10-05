package companionai

// Search seams. A bonded companion's `search` is the engine's own search,
// but two things about it are the module's to decide or to hear about:
// whether this particular search may also roll for a bauble on its owner's
// behalf, and what the search turned up, so the companion can say so.

// BaubleSearchFunc reports, at the moment a companion searches, the account
// a bauble roll would be made for, or 0 for no roll. The module decides
// (a share of its searches, and only for an owner with their own key).
type BaubleSearchFunc func(mobInstanceId int) (ownerUserId int)

// SearchedFunc hears what a companion's search turned up, in plain words
// ("a hidden way out to the north", "a stashed iron dagger"). found is empty
// when it turned up nothing.
type SearchedFunc func(mobInstanceId int, found []string)

// BaubleFoundFunc hears that a bauble a companion turned up has been worked
// free: into her pack (pocketed), or left on the ground when she could not
// carry it. name is the MODEL-SAFE name (items.Item.ModelName): it goes into
// a model's prompt and may be spoken aloud, so never the finder's own view,
// which may be text a player's key wrote.
type BaubleFoundFunc func(mobInstanceId int, name string, pocketed bool)

var (
	baubleSearchFunc BaubleSearchFunc
	searchedFunc     SearchedFunc
	baubleFoundFunc  BaubleFoundFunc
)

// SetBaubleSearcher installs the bauble-roll decision. Called by the
// aicompanion module.
func SetBaubleSearcher(f BaubleSearchFunc) { baubleSearchFunc = f }

// BaubleSearchFor is the account a companion's search rolls a bauble for,
// or 0. Nil-safe: with no module installed, a mob's search never does.
func BaubleSearchFor(mobInstanceId int) int {
	if baubleSearchFunc == nil {
		return 0
	}
	return baubleSearchFunc(mobInstanceId)
}

// SetSearchedHandler installs the search-result listener.
func SetSearchedHandler(f SearchedFunc) { searchedFunc = f }

// RouteSearched tells the module what a mob's search turned up. Nil-safe.
func RouteSearched(mobInstanceId int, found []string) {
	if searchedFunc != nil {
		searchedFunc(mobInstanceId, found)
	}
}

// SetBaubleFoundHandler installs the bauble-arrival listener.
func SetBaubleFoundHandler(f BaubleFoundFunc) { baubleFoundFunc = f }

// RouteBaubleFound tells the module a companion's find has been worked
// free. Nil-safe.
func RouteBaubleFound(mobInstanceId int, name string, pocketed bool) {
	if baubleFoundFunc != nil {
		baubleFoundFunc(mobInstanceId, name, pocketed)
	}
}
