package rooms

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// Room-instance saving, split into a phase that reads live state and a phase
// that only touches bytes (roadmap chunk 3.6b-1, review finding 36).
//
// 3.6a measured one dirty room's save as:
//
//	LoadRoomTemplate  0.110ms   3.0%   disk read of immutable authored data
//	reflection diff   0.055ms   1.5%   reads LIVE state
//	yaml.Marshal      0.007ms   0.2%   reads LIVE state, emits immutable bytes
//	util.Save         3.459ms  95.3%   touches only the bytes
//
// Everything above the last line has to happen under the world lock. The last
// line does not, and it is essentially all of the cost. So PrepareInstanceWrite
// does the first three and returns bytes; committing is left to the caller,
// which either does it immediately (SaveRoomInstance, unchanged behaviour) or
// hands it to the save queue to be spread across later ticks.
//
// NOTE the phase table above is a warm-page-cache local-SSD measurement, and
// LoadRoomTemplate is an UNCACHED read and parse per room per cycle -- 64% of
// what prepare costs. Template caching is deliberately a separate slice, but it
// is the named prerequisite for the prepare figure holding on the droplet.

// autosaveQueue is the process-wide pending set.
//
// Package-level rather than injected because every caller is on MainWorker and
// there is exactly one world. See internal/savequeue/context.md for why it
// carries no mutex.
var autosaveQueue = savequeue.New()

// AutosaveQueue exposes the pending set to the autosave hook, which owns the
// prepare/drain cycle. Rooms own the queue only because something has to.
func AutosaveQueue() *savequeue.Queue { return autosaveQueue }

// instanceFilePathFor returns where a room's instance overlay lives.
func instanceFilePathFor(r Room) string {
	folderPath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(),
		`/rooms.instances/`, ZoneToFolder(r.Zone))
	return fmt.Sprintf("%s%d.yaml", folderPath, r.RoomId)
}

// PrepareInstanceWrite computes what persisting this room would write, without
// writing it. The caller must hold the world lock.
//
// A returned PendingWrite with nil Data means the overlay should be DELETED:
// the room now matches its template, and a stale overlay left on disk is
// re-applied on the next room load, resurrecting state the room no longer has.
func PrepareInstanceWrite(r Room) (savequeue.PendingWrite, error) {

	if r.IsEphemeral() {
		return savequeue.PendingWrite{}, errors.New(`ephemeral rooms are not saved`)
	}

	rTpl := LoadRoomTemplate(r.RoomId)
	if rTpl == nil {
		return savequeue.PendingWrite{}, fmt.Errorf(`could not load template for room %d`, r.RoomId)
	}

	rVal := reflect.ValueOf(r)
	tplVal := reflect.ValueOf(*rTpl)
	t := reflect.TypeOf(r)

	instanceSaveData := make(map[string]interface{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		yamlTag := field.Tag.Get("yaml")
		if yamlTag == `-` {
			continue
		}

		if field.Tag.Get("instance") == "skip" {
			continue
		}

		rVal2 := rVal.Field(i)
		tplVal2 := tplVal.Field(i)

		if reflect.DeepEqual(rVal2.Interface(), tplVal2.Interface()) {
			continue
		}

		tagParts := strings.Split(yamlTag, ",")
		fieldName := tagParts[0]
		if fieldName == `` || fieldName == `omitempty` || fieldName == `flow` {
			fieldName = field.Name
		}

		instanceSaveData[fieldName] = rVal2.Interface()
	}

	p := savequeue.PendingWrite{
		Kind:    "room",
		Id:      r.RoomId,
		Path:    instanceFilePathFor(r),
		Careful: bool(configs.GetFilePathsConfig().CarefulSaveFiles),
	}

	if len(instanceSaveData) == 0 {
		// nil Data is the delete instruction. Leaving Data nil here is load
		// bearing, not laziness.
		return p, nil
	}

	data, err := yaml.Marshal(instanceSaveData)
	if err != nil {
		return savequeue.PendingWrite{}, err
	}
	p.Data = data

	return p, nil
}

// CancelPendingInstanceWrite drops any queued write for this room.
//
// Guard G2. This must be called by everything that takes ownership of the
// room's instance file, which is NOT the same set as "everything that calls
// SaveRoomInstance": the deletion paths bypass it entirely and reach for
// os.Remove / os.RemoveAll directly. Missing them was a real hole found in
// adversarial review of the 3.6b-1 plan.
//
// Safe to call for a room with nothing queued.
func CancelPendingInstanceWrite(r Room) {
	if r.IsEphemeral() {
		return
	}
	autosaveQueue.Cancel(instanceFilePathFor(r))
}

// cancelPendingInstanceWriteById is the same guard for paths that only have an
// id, such as template deletion and cache purges. It resolves the zone from the
// in-memory room when it can, and falls back to the template otherwise.
//
// A room that cannot be resolved at all has no queued write worth worrying
// about: the write could only have been queued from a room object that existed.
func cancelPendingInstanceWriteById(roomId int) {
	if roomId >= ephemeralRoomIdMinimum {
		return
	}
	if r := getRoomFromMemory(roomId); r != nil {
		CancelPendingInstanceWrite(*r)
		return
	}
	if tpl := LoadRoomTemplate(roomId); tpl != nil {
		CancelPendingInstanceWrite(*tpl)
	}
}
