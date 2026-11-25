package buttons

import "maps"

// The two dark marks (owner ruling 4): a direction tried in the dark that
// answered "You're bumping into walls." (MarkWall, drawn as a cross) or the
// locked line (MarkLocked, drawn as a padlock).
const (
	MarkWall   = `wall`
	MarkLocked = `locked`
)

type darkMarks struct {
	roomId int
	marks  map[string]string
}

// marksOf is guarded by mu (buttons.go).
var marksOf = map[int]*darkMarks{}

// RecordDarkMark notes that userId, seeing nothing in roomId, tried direction
// and was answered mark. Only the six T directions are marked: they are the
// only buttons the dark offers. A mark in a new room starts that room's set.
// It reports whether anything changed.
func RecordDarkMark(userId, roomId int, direction, mark string) bool {
	if _, ok := fixedIds[direction]; !ok {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	dm := marksOf[userId]
	if dm == nil || dm.roomId != roomId {
		dm = &darkMarks{roomId: roomId, marks: map[string]string{}}
		marksOf[userId] = dm
	}
	if dm.marks[direction] == mark {
		return false
	}
	dm.marks[direction] = mark
	return true
}

// DarkMarks returns userId's marks in roomId, direction to mark, while dark
// is true. When they stand in another room, or the room is not dark to them,
// the marks are cleared and none are returned: leaving the room or the room
// becoming lit clears them (ruling 4).
func DarkMarks(userId, roomId int, dark bool) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	dm := marksOf[userId]
	if dm == nil {
		return map[string]string{}
	}
	if !dark || dm.roomId != roomId {
		delete(marksOf, userId)
		return map[string]string{}
	}
	return maps.Clone(dm.marks)
}

// ClearDarkMarks drops userId's marks: they changed rooms.
func ClearDarkMarks(userId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(marksOf, userId)
}
