package rooms

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// RULES FOR SAVING AND LOADING ROOMS
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// The following are rules for reading and writing room files.
// This set of rules must be followed or behaviors will break.
// DO NOT CHANGE CODE FOR THE PROCESS unless you understand the implications.
//
// NOTE: in-memory cache contains a RoomId => Room struct map. This Room struct is TEMPLATE data updated on load with INSTANCE data.
// Once loaded into memory, the in-memory cache is the most recent source-of-truth for INSTANCE data.
//
// TEMPLATE data is stored in the /rooms/ folder
// INSTANCE data is stored in the /rooms.instances/ folder (may only be created on running the MUD)
//
// INSTANCE data can be cleared out by deleting the /rooms.instances/ folder, or running the `make clean-instances` to do it.
//
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// A. LOADING ROOMS BLINDLY                                                                                              AKA LoadRoom()
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// NOTE: This is the only way outside code should be loading rooms. The other functions are exported for very specialized purposes.
//
// 1. Look for in-memory Room cache record for RoomId. If found, return it.
// 2. Do [B. LOADING ROOMS FROM FILES]
// 3. Save result to in-memory Room cache.
//
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// B. LOADING ROOMS FROM FILES                                                                                  AKA  LoadRoomInstance()
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
//  1. Load the TEMPLATE data into a Room struct.
//  2. Load the INSTANCE data into the same struct - the result is only the field present in the INSTANCE data will overwrite the
//     TEMPLATE data.
//
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// C. SAVING ROOM TEMPLATES                                                                                      AKA SaveRoomTemplate()
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
//  1. Anytime a change is made to a TEMPLATE, first load the TEMPLATE ONLY, make any changes, then save the TEMPLATE ONLY.
//  2. There may be data from the OLD in-memory Room struct that needs to be copied over to the new one. (items, users, mobs, etc).
//     If so, do it.
//  3. Update the in-memory Room struct for the RoomId to use the NEW TEMPLATE version.
//  4. Rebuild maps for the Room's Zone.RoomId and then the room.RoomId, in that order. That ensures it's added to the zone map
//     IF it should be, and if not, will generate its own map.
//
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// D. SAVING ROOMS INSTANCES                                                                                     AKA SaveRoomInstance()
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
//  1. Load the TEMPLATE data into a variable
//  2. Use reflection to iterate through the structure.
//  3. If field isn't readable (unexported/lowercase) or has the struct tag `instance:"skip"`, skip writing it.
//  4. If the field value matches the TEMPLATE version of the field, skip writing it.
//  5. If the final result of the data to write is empty ( "{}\n" ), delete and do not write the new save file.
//
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// See: A. LOADING ROOMS BLINDLY
func LoadRoom(roomId int) *Room {

	// Room 0 aliases to start room
	if roomId == StartRoomIdAlias {
		if roomId = int(configs.GetSpecialRoomsConfig().StartRoom); roomId == 0 {
			roomId = 1
		}
	}

	if room := getRoomFromMemory(roomId); room != nil {
		return room
	}

	room := LoadRoomInstance(roomId)
	if room == nil {
		return nil
	}

	// Finding 8, second half. The cache check above and this store are not
	// one atomic step, and LoadRoomInstance does disk I/O in between, so the
	// window is wide. Two goroutines can both miss the cache and both build a
	// separate *Room for the same roomId.
	//
	// addRoomToMemory rejects the loser, but its error used to be DISCARDED
	// and the loser kept its own orphan copy. An orphan room is not in
	// roomManager.rooms, so it is invisible to maintenance and unload, and
	// anything that calls Prepare() on it spawns a SECOND copy of every mob
	// in its SpawnInfo. That is a live duplicate-NPC source.
	//
	// On collision, throw our copy away and return the one that actually got
	// cached, so every caller shares a single room object.
	if err := addRoomToMemory(room); err != nil {
		if cached := getRoomFromMemory(roomId); cached != nil {
			return cached
		}
		// Store failed for some other reason and nothing is cached. Returning
		// the uncached room would hand back an orphan, so report the miss.
		mudlog.Error("LoadRoom", "roomId", roomId, "error", err, "msg", "room not cached and no existing entry")
		return nil
	}

	return room
}

// See: B. LOADING ROOMS FROM FILES
func LoadRoomInstance(roomId int) *Room {

	room := LoadRoomTemplate(roomId)
	if room == nil {
		return nil
	}

	filename := roomManager.GetFilePath(roomId)

	if len(filename) == 0 {
		return nil
	}

	// Look for specially saved instance data
	filepath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/rooms.instances/`, filename)

	raw, readErr := util.ReadLivingState(filepath)
	if readErr != nil {
		// Absent is the ordinary case: a room nobody has changed has no
		// instance file. Anything else means the overlay exists but is
		// unusable, so quarantine it and return pure template state.
		if !errors.Is(readErr, util.ErrStateAbsent) {
			quarantineRoomOverlay(filepath, roomId, readErr)
		}
		return room
	}

	// Apply the overlay to a SCRATCH copy, not to the room we are about to
	// return. yaml.Unmarshal applies fields as it walks the document, so a
	// file that is valid for a while and then breaks leaves its target already
	// mutated when the error comes back. The old code unmarshalled straight
	// onto `room`, logged a Warn, and carried on with that half-applied
	// object — handing players a template/runtime hybrid with some fields from
	// the template and some from a damaged save (review finding 15).
	//
	// LoadRoomTemplate re-reads from disk on every call, so this is a genuinely
	// independent object and cannot share maps or slices with `room`.
	scratch := LoadRoomTemplate(roomId)
	if scratch == nil {
		return room
	}
	if err := yaml.Unmarshal(raw, scratch); err != nil {
		quarantineRoomOverlay(filepath, roomId, err)
		return room // Reject the overlay whole; template state stands.
	}

	// The whole document parsed, so the overlay is safe to adopt.
	room = scratch

	// yaml.Unmarshal only honors `yaml:` tags; the `instance:"skip"`
	// annotation is invisible to it. Reload a fresh template copy and
	// restore every skip-tagged field so template-owned state
	// (title/description/exits/nouns/zone/etc.) cannot be corrupted
	// by stale data in pre-fix instance files. See
	// docs/superpowers/specs/2026-04-21-room-instance-load-respects-skip-tag-design.md.
	if freshTemplate := LoadRoomTemplate(roomId); freshTemplate != nil {
		restoreSkipTaggedFields(room, freshTemplate)
	}
	// Exits were just restored wholesale from the template, which
	// resurrects any lock trap a player disarmed. DefusedExits is NOT
	// skip-tagged, so it survived the overlay; re-apply it now. See
	// Room.MarkExitTrapDefused for why the disarm is persisted as a name
	// list rather than by un-skipping Exits.
	room.applyDefusedExits()

	return room
}

// quarantineRoomOverlay moves an unusable instance overlay aside and reports it
// at ERROR. A Warn was not enough: the room silently reverts to template state,
// so anything a builder or player had changed about it is gone from play, and
// without the quarantine the next SaveRoomInstance would overwrite the only
// copy of what was lost.
func quarantineRoomOverlay(overlayPath string, roomId int, cause error) {
	dest, qErr := util.QuarantineCorrupt(overlayPath)
	if qErr != nil {
		mudlog.Error("LoadRoomInstance",
			"roomId", roomId,
			"filepath", overlayPath,
			"error", cause,
			"quarantine", "FAILED",
			"quarantineError", qErr,
			"impact", "room loads from template; corrupt overlay left in place and will be overwritten")
		return
	}
	mudlog.Error("LoadRoomInstance",
		"roomId", roomId,
		"filepath", overlayPath,
		"error", cause,
		"quarantinedTo", dest,
		"impact", "room loads from pure template state; its saved items, gold, signs and container contents were not recovered")
}

// restoreSkipTaggedFields copies every exported Room field tagged
// `instance:"skip"` from src onto dst. Used after an instance-file
// overlay unmarshal to ensure template-owned fields are not corrupted
// by stale data in pre-fix instance files.
func restoreSkipTaggedFields(dst, src *Room) {
	srcVal := reflect.ValueOf(*src)
	dstVal := reflect.ValueOf(dst).Elem()
	t := srcVal.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Tag.Get("instance") != "skip" {
			continue
		}
		dstVal.Field(i).Set(srcVal.Field(i))
	}
}

// Only loads the template data, ignores instance data.
func LoadRoomTemplate(roomId int) *Room {

	if roomId >= ephemeralRoomIdMinimum {
		return nil
	}

	filename := roomManager.GetFilePath(roomId)

	if len(filename) == 0 {
		return nil
	}

	filepath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/rooms/`, filename)

	retRoom, _ := loadRoomFromFile(filepath)

	return retRoom
}

// See C. UPDATING EXISTING ROOM TEMPLATES
func SaveRoomTemplate(roomTpl Room) error {

	if roomTpl.IsEphemeral() {
		return errors.New(`ephemeral rooms are not saved`)
	}

	// Do not accept a RoomId of zero.
	if roomTpl.RoomId == 0 {
		roomTpl.RoomId = GetNextRoomId()
		SetNextRoomId(roomTpl.RoomId + 1)
	}

	//
	// It is assumed that `roomTpl` contains empty room data
	// That means no players/mobs/items/gold/etc that aren't intended to be included in the default/empty room template
	//
	// marshalRoomTemplate is yaml.Marshal plus re-folding of long prose values
	// (description/nouns/hidden_nouns/idlemessages) back into wrapped folded
	// block scalars. Without it a builder save collapses authored prose into
	// single enormous lines. See internal/rooms/prose_wrap.go for why folded
	// scalars are the only style that round-trips the value unchanged.
	data, err := marshalRoomTemplate(roomTpl)
	if err != nil {
		return err
	}

	zoneFolder := ZoneToFolder(roomTpl.Zone)

	// First write the empty version to its template file
	roomFilePath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/rooms/`, fmt.Sprintf("%s%d.yaml", zoneFolder, roomTpl.RoomId))
	// Honour CarefulSaveFiles: write to <path>.new and rename, so a crash or
	// power loss mid-write cannot truncate an authored room file. A truncated
	// room YAML panics the server at startup (see the pre-push SOP), which is
	// precisely what the flag exists to prevent — but rooms ignored it while
	// items, mobs, users and alts all honoured it.
	if err = util.Save(roomFilePath, data, bool(configs.GetFilePathsConfig().CarefulSaveFiles)); err != nil {
		return err
	}

	// The template on disk just changed, so the autosave compare cache holds a
	// stale copy. Left alone, every field the builder just edited would look
	// like INSTANCE state to the diff and be written into the room's overlay --
	// baking a template edit into the instance file, where it then shadows
	// future template edits (chunk 4.6).
	InvalidateTemplateCache(roomTpl.RoomId)

	// Get zone root
	cfg := GetZoneConfig(roomTpl.Zone)

	// Queue rebuild the zone map
	events.AddToQueue(events.RebuildMap{MapRootRoomId: cfg.RoomId})
	// Also queue rebuild the map based on the room, if it's not already present in a map (AKA, SkipIfExists)
	events.AddToQueue(events.RebuildMap{MapRootRoomId: roomTpl.RoomId, SkipIfExists: true})

	//
	// Now we can make changes to the roomTpl as though it is the room (copy over stuff etc)
	//
	roomBeingReplaced := roomManager.rooms[roomTpl.RoomId]

	if roomBeingReplaced == nil {
		// New room not yet in memory — no live state to copy.
		addRoomToMemory(&roomTpl, true)
		return nil
	}

	// Copy container contents (if new vs. old room container names match)
	for containerName, container := range roomBeingReplaced.Containers {

		if newContainer, ok := roomTpl.Containers[containerName]; ok {

			if newContainer.Gold == 0 {
				newContainer.Gold = container.Gold
			}

			if len(newContainer.Items) == 0 && len(container.Items) > 0 {
				newContainer.Items = make([]items.Item, len(container.Items))
				copy(newContainer.Items, container.Items)
			}

			roomTpl.Containers[containerName] = newContainer
		}
	}

	// Copy items and stashed items
	for _, itm := range roomBeingReplaced.GetAllFloorItems(true) {
		if itm.StashedBy > 0 {
			roomTpl.AddItem(itm, true)
		} else {
			roomTpl.AddItem(itm, false)
		}
	}

	// Copy gold on floor
	roomTpl.Gold = roomBeingReplaced.Gold

	// Copy signs
	roomTpl.Signs = make([]Sign, len(roomBeingReplaced.Signs))
	copy(roomTpl.Signs, roomBeingReplaced.Signs)

	// Copy mobs in room
	roomTpl.mobs = make([]int, len(roomBeingReplaced.mobs))
	copy(roomTpl.mobs, roomBeingReplaced.mobs)

	// Copy players in room
	roomTpl.players = make([]int, len(roomBeingReplaced.players))
	copy(roomTpl.players, roomBeingReplaced.players)

	// Add to memory with the force flag true
	// This will clear out the old data and force write the new data.
	addRoomToMemory(&roomTpl, true)

	// Save whatever is in this room as the instance data
	SaveRoomInstance(roomTpl)

	return nil
}

// DeleteRoomTemplate removes a room's authored template entirely: it deletes
// the room's YAML file from disk and purges the room from the in-memory manager
// (rooms map, zone registry, and caches, via ClearRoomCache). It is the web
// builder's (1b) room-delete primitive.
//
// Callers are responsible for first removing any exits that point AT this room
// — the builder scans and cleans inbound exits before calling this. Ephemeral
// (instance) rooms have no template and are rejected.
func DeleteRoomTemplate(roomId int) error {
	if roomId >= ephemeralRoomIdMinimum {
		return errors.New(`ephemeral rooms have no template to delete`)
	}

	// The template file is about to go away; drop its cached copy (chunk 4.6).
	InvalidateTemplateCache(roomId)

	// Guard G2 (chunk 3.6b-1). This path deletes room data WITHOUT going through
	// SaveRoomInstance, so nothing else cancels a pending write for it. Left
	// queued, the commit would try to write an instance overlay for a room whose
	// template no longer exists. Cancel BEFORE the removal, while the room is
	// still resolvable.
	cancelPendingInstanceWriteById(roomId)

	if filename := roomManager.GetFilePath(roomId); filename != `` {
		roomFilePath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/rooms/`, filename)
		if err := os.Remove(roomFilePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Drop from memory/caches. ClearRoomCache returns an error only when the
	// room isn't cached, which is fine here (the file is already gone).
	ClearRoomCache(roomId)
	return nil
}

// See: D. SAVING ROOMS INSTANCES
//
// The body of this function moved to PrepareInstanceWrite + savequeue.Commit in
// chunk 3.6b-1. This remains the synchronous composition of the two, because
// unload, the builder, shutdown and copyover all legitimately want "save this
// room now, and tell me if it failed". Behaviour is unchanged: same diff, same
// bytes, same delete-when-empty, same CarefulSaveFiles handling.
//
// It also CANCELS any queued write for this room. A pending entry holds an
// older snapshot by definition, so letting it commit afterwards would clobber
// the newer state this call just wrote (guard G2).
func SaveRoomInstance(r Room) error {

	p, err := PrepareInstanceWrite(r)
	if err != nil {
		return err
	}

	CancelPendingInstanceWrite(r)

	if err := savequeue.Commit(p); err != nil {
		return fmt.Errorf("SaveRoomInstance: room %d: %w", r.RoomId, err)
	}
	return nil
}

func loadRoomFromFile(roomFilePath string) (*Room, error) {

	roomFilePath = util.FilePath(roomFilePath)

	roomPtr, err := fileloader.LoadFlatFile[*Room](roomFilePath)
	if err != nil {
		mudlog.Error("loadRoomFromFile()", "error", err.Error())
	}

	return roomPtr, err
}

// SaveAllRooms persists every loaded non-ephemeral room and returns an
// aggregate error if any of them failed.
//
// It used to count failures into a log line and then `return nil`
// unconditionally, so the autosave could not tell a clean save from one where
// every room failed — and told players "Done." either way (review finding 35).
//
// The first failing room's error is included so the log is actionable; the
// count tells you whether it was one room or all of them.
func SaveAllRooms() error {

	start := time.Now()
	saveCt := 0
	errCt := 0
	var firstErr error
	for _, r := range roomManager.rooms {

		if r.IsEphemeral() {
			continue
		}

		if err := SaveRoomInstance(*r); err != nil {
			errCt++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		saveCt++

	}

	mudlog.Info("SaveAllRooms()", "savedCount", saveCt, "expectedCt", len(roomManager.rooms), "errorCount", errCt, "Time Taken", time.Since(start))

	if errCt > 0 {
		return fmt.Errorf("SaveAllRooms: %d of %d room(s) failed to save; first error: %w", errCt, saveCt+errCt, firstErr)
	}

	return nil
}

// Overwrite file and memory for zoneconfig
func SaveZoneConfig(zoneConfig *ZoneConfig) error {

	zoneFolder := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), "/", "rooms")
	if err := fileloader.SaveFlatFile(zoneFolder, zoneConfig); err != nil {
		return err
	}

	roomManager.zones[zoneConfig.Name] = zoneConfig

	return nil
}

// Goes through all of the rooms and caches key information
func loadAllRoomZones() error {
	start := time.Now()

	nextRoomId := GetNextRoomId()
	defer func() {
		if nextRoomId != GetNextRoomId() {
			SetNextRoomId(nextRoomId)
		}
	}()

	loadedZones, err := fileloader.LoadAllFlatFiles[string, *ZoneConfig](configs.GetFilePathsConfig().DataFiles.String()+`/rooms`, "zone-config.yaml")
	if err != nil {
		return err
	}

	for zoneName, zoneConfig := range loadedZones {
		roomManager.zones[zoneName] = zoneConfig

		folderPath := util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/rooms.instances/`, ZoneNameSanitize(zoneConfig.Name))
		if _, err := os.Stat(folderPath); os.IsNotExist(err) {
			os.MkdirAll(folderPath, 0755)
		}
	}

	loadedRooms, err := fileloader.LoadAllFlatFiles[int, *Room](configs.GetFilePathsConfig().DataFiles.String()+`/rooms`, "[0-9]*.yaml")
	if err != nil {
		return err
	}

	roomsWithoutEntrances := map[int]string{}

	for _, loadedRoom := range loadedRooms {

		// configs.GetConfig().DeathRecoveryRoom is the death/shadow realm and gets a pass
		if loadedRoom.RoomId == int(configs.GetSpecialRoomsConfig().DeathRecoveryRoom) {
			continue
		}

		// If it has never been set, set it to the filepath
		if _, ok := roomsWithoutEntrances[loadedRoom.RoomId]; !ok {
			roomsWithoutEntrances[loadedRoom.RoomId] = loadedRoom.Filepath()
		}

		for _, exit := range loadedRoom.Exits {
			roomsWithoutEntrances[exit.RoomId] = ``
		}

	}

	for roomId, filePath := range roomsWithoutEntrances {

		if filePath == `` {
			delete(roomsWithoutEntrances, roomId)
			continue
		}

		mudlog.Warn("No Entrance", "roomId", roomId, "filePath", filePath)
	}

	for _, loadedRoom := range loadedRooms {
		// Keep track of the highest roomId

		if loadedRoom.RoomId >= nextRoomId {
			nextRoomId = loadedRoom.RoomId + 1
		}

		// Cache the file path for every roomId
		roomManager.setCachedFilePath(loadedRoom.RoomId, loadedRoom.Filepath())

		// Update the zone info cache
		if _, ok := roomManager.zones[loadedRoom.Zone]; !ok {
			// Form one?
			return fmt.Errorf("No zone-config.yaml was loaded for roomId: %d zone: %s", loadedRoom.RoomId, loadedRoom.Zone)
		}

		if loadedRoom.Title != "" {
			casing.AssertCanonical(loadedRoom.Title, "room", fmt.Sprintf("%d", loadedRoom.RoomId))
		}
	}

	mudlog.Info("rooms.loadAllRoomZones()", "zoneCount", len(loadedZones), "loadedCount", len(loadedRooms), "Time Taken", time.Since(start))

	return nil
}
