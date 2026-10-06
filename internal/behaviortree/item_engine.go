package behaviortree

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Items are the third subject of the behaviour-tree engine, beside mobs and
// rooms (lighting 5e, item behaviour slice 1, spec Shape A).

// ItemSubject is the item a tree runs for, and where it is (Rule 2).
type ItemSubject struct {
	UUID          uuid.UUID // the instance; keys its tree state
	ItemId        int       // the template; names its tree
	UserId        int       // the player holding it, 0 if none
	MobInstanceId int       // the mob holding it, 0 if none
	RoomId        int       // its room: the holder's, or the floor's
	Slot          string    // the characters.Worn AllSlots key it is worn in; "" in a backpack or on a floor
	OnFloor       bool      // lying on RoomId's floor
}

// GetItemTreePath is the path of a named item tree:
// {dataFiles}/behaviors/items/{name}.yaml. Several items may share one tree
// (the archetype pattern, Rule 1); the name is authored, so it is not
// sanitized.
func GetItemTreePath(name string) string {
	dataFiles := configs.GetFilePathsConfig().DataFiles.String()
	return util.FilePath(dataFiles, `/`, `behaviors`, `/`, `items`, `/`, name+`.yaml`)
}

// resolveItemTree returns a named item tree, loading it on first request. At
// boot ValidateItemBehaviors has already loaded every tree a template names,
// so this load path serves tests and is a safety net; a failure is logged
// once and negative-cached.
func resolveItemTree(name string) Node {
	e := GetEngine()
	if tree := e.GetItemTree(name); tree != nil {
		return tree
	}
	if e.HasNoItemTree(name) {
		return nil
	}
	if err := e.LoadItemTree(name, GetItemTreePath(name)); err != nil {
		mudlog.Error("TryItemBehavior", "tree", name, "error", err.Error())
		e.SetNoItemTree(name)
		return nil
	}
	return e.GetItemTree(name)
}

// loggedItemTreePanics keeps a node's panic to one log line per tree and
// node (the time_of_day log-once precedent).
var loggedItemTreePanics sync.Map

// TryItemBehavior runs an item's tree for an event (Rule 3), mirroring
// TryRoomBehavior: resolve the template's tree, resolve the holder (gone:
// return false, the round is skipped), build the context, evaluate. A panic
// in a node is recovered, logged once per tree and node, and returns false:
// the item does nothing that round (Rule 14). Returns true on Success.
func TryItemBehavior(event EventContext, subject ItemSubject) (handled bool) {
	spec := items.GetItemSpec(subject.ItemId)
	if spec == nil || spec.Behavior == `` {
		return false
	}
	tree := resolveItemTree(spec.Behavior)
	if tree == nil {
		return false
	}

	switch {
	case subject.UserId > 0:
		u := users.GetByUserId(subject.UserId)
		if u == nil || u.Character == nil {
			return false
		}
		subject.RoomId = u.Character.RoomId
	case subject.MobInstanceId > 0:
		m := mobs.GetInstance(subject.MobInstanceId)
		if m == nil {
			return false
		}
		subject.RoomId = m.Character.RoomId
	case subject.OnFloor:
		if !rooms.IsRoomLoaded(subject.RoomId) {
			return false
		}
	default:
		return false
	}

	event.RoomId = subject.RoomId
	ctx := &EvalContext{
		Event:    event,
		MobState: EnsureItemBTreeState(subject.UUID),
		RoomId:   subject.RoomId,
		Item:     &subject,
	}
	defer func() {
		if r := recover(); r != nil {
			key := spec.Behavior + `/` + ctx.node
			if _, already := loggedItemTreePanics.LoadOrStore(key, true); !already {
				mudlog.Error("TryItemBehavior", "tree", spec.Behavior, "node", ctx.node,
					"itemId", subject.ItemId, "error", fmt.Sprint(r))
			}
			handled = false
		}
	}()
	return tree.Evaluate(ctx) == Success
}

// ValidateItemBehaviors loads and compiles every item tree a template names
// and returns one error listing every problem, or nil. main.go panics on it
// after items load (X12, the precedent the retired voice_id check set): a
// missing or broken tree, or a malformed voice (slice 2), fails the boot
// instead of leaving an item silently inert.
func ValidateItemBehaviors() error {
	specs := items.GetAllItemSpecsMap()
	ids := make([]int, 0, len(specs))
	for id, spec := range specs {
		if spec != nil && spec.Behavior != `` {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	e := GetEngine()
	loaded := map[string]error{}
	writesLight := map[string]bool{}
	var problems []string
	for _, id := range ids {
		spec := specs[id]
		err, seen := loaded[spec.Behavior]
		if !seen {
			path := GetItemTreePath(spec.Behavior)
			err = e.LoadItemTree(spec.Behavior, path)
			loaded[spec.Behavior] = err
			if err == nil {
				if def, derr := LoadTreeDef(path); derr == nil {
					writesLight[spec.Behavior] = TreeWritesLight(def.Tree)
				}
			}
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %d (%s): behavior %q: %v", id, spec.Name, spec.Behavior, err))
			continue
		}
		// One writer per record (Rule 8, X22): the trim owns an adjustable
		// light, so no tree may schedule one.
		if writesLight[spec.Behavior] {
			for _, cid := range spec.WornConditionIds {
				cspec := conditions.GetConditionSpec(cid)
				if cspec != nil && (cspec.IsLightSource() || cspec.IsDarknessSource()) &&
					slices.Contains(cspec.Flags, conditions.Adjustable) {
					problems = append(problems, fmt.Sprintf(
						"item %d (%s): behavior %q writes light, but worn condition %d is adjustable: the trim owns it, so no tree may schedule it",
						id, spec.Name, spec.Behavior, cid))
				}
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// TreeWritesLight reports whether a tree names set_light or pulse_light
// anywhere. The repo-root content guards read it too.
func TreeWritesLight(def NodeDef) bool {
	if def.Do == `set_light` || def.Do == `pulse_light` {
		return true
	}
	for _, ch := range def.Children {
		if TreeWritesLight(ch) {
			return true
		}
	}
	return def.Child != nil && TreeWritesLight(*def.Child)
}
