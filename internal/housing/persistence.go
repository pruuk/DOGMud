package housing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// Houses are living state: <DataFiles>/housing/<buildingid>/<entryroomid>.yaml.
// The directory is gitignored, kept on the production droplet, excluded from
// the Docker image, and never touched by the instance-save wipe.
//
// The file is named by the house's entry room rather than its owner so that a
// file which cannot be read still says which room it guards: that room is held
// back from sale instead of being sold to a stranger with the old owner's
// things still on its floor.

var housingDirOverride string // test hook; empty uses config

func housingDir() string {
	if housingDirOverride != `` {
		return housingDirOverride
	}
	return filepath.FromSlash(configs.GetFilePathsConfig().DataFiles.String() + `/housing`)
}

// SetDataDirForTest points house persistence at dir and returns a restore func.
func SetDataDirForTest(dir string) func() {
	prev := housingDirOverride
	housingDirOverride = dir
	return func() { housingDirOverride = prev }
}

func housePath(buildingId string, entryRoomId int) string {
	return filepath.Join(housingDir(), buildingId, strconv.Itoa(entryRoomId)+`.yaml`)
}

// saveHouse durably writes one house. It never touches the registry; callers
// publish only after it returns nil (persist before publishing).
func saveHouse(h House) error {
	if h.BuildingId == `` || h.EntryRoom() <= 0 {
		return fmt.Errorf(`housing.saveHouse: house has no building or entry room`)
	}
	if err := os.MkdirAll(filepath.Join(housingDir(), h.BuildingId), 0755); err != nil {
		return fmt.Errorf(`housing.saveHouse: mkdir: %w`, err)
	}
	b, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf(`housing.saveHouse: marshal: %w`, err)
	}
	return util.Save(housePath(h.BuildingId, h.EntryRoom()), b)
}

// loadHouses reads every house file into the registry. Buildings must already
// be registered. Returns the houses indexed and the rooms held.
func loadHouses() (loaded int, heldCount int) {
	root := housingDir()
	buildingDirs, err := os.ReadDir(root)
	if err != nil {
		// Absent is the ordinary first-run case.
		return 0, 0
	}

	// Deterministic order so a conflict between two files always resolves the
	// same way from one boot to the next.
	sort.Slice(buildingDirs, func(i, j int) bool { return buildingDirs[i].Name() < buildingDirs[j].Name() })

	for _, bd := range buildingDirs {
		if !bd.IsDir() {
			continue
		}
		entries, rerr := os.ReadDir(filepath.Join(root, bd.Name()))
		if rerr != nil {
			mudlog.Error(`housing.loadHouses`, `dir`, bd.Name(), `error`, rerr.Error())
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			name := e.Name()
			// Quarantined files keep their evidence alongside; skip them.
			if e.IsDir() || !strings.HasSuffix(name, `.yaml`) {
				continue
			}
			if loadOneHouse(bd.Name(), filepath.Join(root, bd.Name(), name), name) {
				loaded++
			}
		}
	}

	mu.RLock()
	heldCount = len(held)
	mu.RUnlock()
	return loaded, heldCount
}

// loadOneHouse reads, checks and indexes a single file. It reports whether a
// house was indexed. Any problem holds the room the file names.
func loadOneHouse(buildingDir string, path string, fileName string) bool {
	entryFromName, nameErr := strconv.Atoi(strings.TrimSuffix(fileName, `.yaml`))
	if nameErr != nil {
		mudlog.Error(`housing.loadHouses`, `file`, path, `error`, `file name is not an entry room id; ignored`)
		return false
	}

	raw, err := util.ReadLivingState(path)
	if err != nil {
		if errors.Is(err, util.ErrStateAbsent) {
			return false
		}
		quarantineAndHold(path, entryFromName, err)
		return false
	}

	var h House
	if uerr := yaml.Unmarshal(raw, &h); uerr != nil {
		quarantineAndHold(path, entryFromName, fmt.Errorf(`%w: %v`, util.ErrStateCorrupt, uerr))
		return false
	}
	// Item identities (UUIDs) are not written to disk; every loader mints
	// them, as the room and user loaders do, so a loaded item is whole.
	for ci := range h.Containers {
		for ii := range h.Containers[ci].Items {
			h.Containers[ci].Items[ii].Validate()
		}
	}
	for fi := range h.Floors {
		for ii := range h.Floors[fi].Items {
			h.Floors[fi].Items[ii].Validate()
		}
		for ii := range h.Floors[fi].Stash {
			h.Floors[fi].Stash[ii].Validate()
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// The file is readable but must agree with itself and with the world.
	// None of these are corruption, so the file stays where it is for staff
	// to repair, and its rooms are held so nobody else is sold them.
	reject := func(reason string) bool {
		mudlog.Error(`housing.loadHouses`, `file`, path, `error`, reason)
		holdLocked(entryFromName, reason)
		for _, roomId := range h.RoomIds {
			holdLocked(roomId, reason)
		}
		return false
	}

	if h.BuildingId != buildingDir {
		return reject(fmt.Sprintf(`building_id %q does not match its folder %q`, h.BuildingId, buildingDir))
	}
	b, ok := buildings[h.BuildingId]
	if !ok {
		return reject(fmt.Sprintf(`building %q is not authored`, h.BuildingId))
	}
	if h.EntryRoom() != entryFromName {
		return reject(fmt.Sprintf(`entry room %d does not match the file name`, h.EntryRoom()))
	}
	if h.OwnerUserId <= 0 {
		return reject(`owner_user_id is missing`)
	}
	for _, roomId := range h.RoomIds {
		if unitBuilding[roomId] != b.BuildingId {
			return reject(fmt.Sprintf(`room %d is not a unit of %s`, roomId, b.BuildingId))
		}
		if other, taken := roomHouse[roomId]; taken {
			return reject(fmt.Sprintf(`room %d is already owned by user %d`, roomId, other.OwnerUserId))
		}
	}
	if err := h.checkLinks(); err != nil {
		return reject(err.Error())
	}
	if other, dup := ownerHouse[ownerKey{h.BuildingId, h.OwnerUserId}]; dup {
		return reject(fmt.Sprintf(`user %d already owns room %d in %s`, h.OwnerUserId, other.EntryRoom(), h.BuildingId))
	}

	hh := h
	indexLocked(&hh)
	return true
}

// quarantineAndHold applies rule 3 of the contract: move the bad file aside
// (never delete), log at ERROR, and carry on. The room it named is held.
func quarantineAndHold(path string, entryRoomId int, cause error) {
	dest, qerr := util.QuarantineCorrupt(path)
	if qerr != nil {
		mudlog.Error(`housing.loadHouses`, `file`, path, `error`, cause.Error(), `quarantineError`, qerr.Error())
	} else {
		mudlog.Error(`housing.loadHouses`, `file`, path, `error`, cause.Error(), `quarantinedTo`, dest)
	}
	mu.Lock()
	holdLocked(entryRoomId, `house file unreadable: `+cause.Error())
	mu.Unlock()
}
