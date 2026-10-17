package mobs

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/gametime"
)

// Schedule is a daily routine attached to NPCs via Mob.ScheduleId.
// Loaded from _datafiles/world/dogmud/schedules/<zone>/<id>.yaml at startup.
type Schedule struct {
	Id          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Segments    []ScheduleSegment `yaml:"segments"`
}

// ScheduleSegment covers a contiguous hour range [Start, End). When Start > End
// the segment wraps midnight (e.g. Start=22 End=6 covers 22-23 and 0-5).
type ScheduleSegment struct {
	Start        int      `yaml:"start"`                 // 0-23 inclusive
	End          int      `yaml:"end"`                   // 1-24 inclusive
	TargetRoom   int      `yaml:"target_room,omitempty"` // room the mob should occupy; optional for activity: patrol
	Activity     string   `yaml:"activity,omitempty"`    // "" | "craft" | "sleeping" | "patrol"
	PatrolId     string   `yaml:"patrol_id,omitempty"`   // chunk 3.4: required when activity is "patrol"
	IdleCommands []string `yaml:"idlecommands,omitempty"`
	// OffShiftLine is what a shopkeeper says, in their own voice, when asked
	// to trade during this segment while off shift (see Mob.OffShift). Empty
	// uses the generic line. No ';' (validated): it is spoken text.
	OffShiftLine string `yaml:"offshift_line,omitempty"`
}

// OffShift reports whether a scheduled mob is away from the work its home
// room holds (owner ruling 2026-10-17: an off-shift shopkeeper does not
// trade). A mob's shop is keyed to its home room, so it is off shift when it
// is awake and either somewhere other than its home room or in a segment
// whose target_room is another room (the forge closes when Kerra leaves for
// the tavern, before she is out of the door). A sleeping mob is not off
// shift: sleep refuses trade on its own, with its own line. A mob with no
// schedule is never off shift.
func (m *Mob) OffShift() bool {
	return m.offShiftAt(gametime.GetDate().Hour24)
}

func (m *Mob) offShiftAt(hour24 int) bool {
	if m == nil || m.ScheduleId == "" || m.Character.HasConditionFlag(conditions.Sleeping) {
		return false
	}
	if m.Character.RoomId != m.HomeRoomId {
		return true
	}
	seg := GetSchedule(m.ScheduleId).CurrentSegment(hour24)
	return seg != nil && seg.Activity != "patrol" && seg.TargetRoom != 0 && seg.TargetRoom != m.HomeRoomId
}

// OffShiftLine is the line an off-shift mob answers a trade with: its
// current segment's offshift_line, or "" when the segment sets none.
func (m *Mob) OffShiftLine() string {
	if m == nil || m.ScheduleId == "" {
		return ""
	}
	if seg := GetSchedule(m.ScheduleId).CurrentSegment(gametime.GetDate().Hour24); seg != nil {
		return seg.OffShiftLine
	}
	return ""
}

// Package-level registry, populated by LoadSchedules at startup.
var (
	schedulesMu sync.RWMutex
	schedules   = map[string]*Schedule{}
)

// GetSchedule returns the schedule with the given id, or nil if no such id is
// loaded.
func GetSchedule(id string) *Schedule {
	if id == "" {
		return nil
	}
	schedulesMu.RLock()
	defer schedulesMu.RUnlock()
	return schedules[id]
}

// CurrentSegment returns the segment active at hour24 (0-23). Returns nil if
// no segment covers this hour. At runtime, loaded schedules are validated to
// cover all 24 hours exactly once, so nil should never be observed.
func (s *Schedule) CurrentSegment(hour24 int) *ScheduleSegment {
	if s == nil {
		return nil
	}
	for i := range s.Segments {
		seg := &s.Segments[i]
		if segContainsHour(seg, hour24) {
			return seg
		}
	}
	return nil
}

// segContainsHour reports whether [seg.Start, seg.End) contains hour24,
// handling the wrap-around case where Start > End.
//
// Inclusive at start, exclusive at end. End == 24 means "up to but not
// including midnight" (the day boundary).
func segContainsHour(seg *ScheduleSegment, hour24 int) bool {
	if seg.Start == seg.End {
		return false // empty segment — validation rejects these at load time
	}
	if seg.Start < seg.End {
		return hour24 >= seg.Start && hour24 < seg.End
	}
	// Wraps midnight: covers [Start, 24) and [0, End).
	return hour24 >= seg.Start || hour24 < seg.End
}

// applyScheduleSpawnOverride returns the room a scheduled mob should spawn
// at given the current hour. Returns homeRoomId unchanged when the mob has
// no schedule, when the schedule is unknown, or when the schedule has no
// segment covering the current hour. Called from newMobByIdInternal during
// instance creation.
func applyScheduleSpawnOverride(scheduleId string, homeRoomId int, hour24 int) int {
	if scheduleId == "" {
		return homeRoomId
	}
	s := GetSchedule(scheduleId)
	if s == nil {
		return homeRoomId
	}
	seg := s.CurrentSegment(hour24)
	if seg == nil {
		return homeRoomId
	}

	// Explicit target_room wins (works for all activities).
	if seg.TargetRoom != 0 {
		return seg.TargetRoom
	}

	// Chunk 3.4: patrol segment without explicit target_room falls back
	// to the patrol's first waypoint so guards start at the beginning of
	// their beat instead of at the barracks.
	if seg.Activity == "patrol" && seg.PatrolId != "" {
		if p := GetPatrol(seg.PatrolId); p != nil && len(p.Waypoints) > 0 {
			return p.Waypoints[0].Room
		}
	}

	// Defensive: no target, no patrol fallback → home.
	return homeRoomId
}

// registerScheduleForTest / unregisterScheduleForTest are unexported helpers
// used by tests within this package. RegisterScheduleForTest /
// UnregisterScheduleForTest are the exported variants for cross-package test
// injection (hooks tests, etc.).
func registerScheduleForTest(s *Schedule) {
	schedulesMu.Lock()
	defer schedulesMu.Unlock()
	schedules[s.Id] = s
}

func unregisterScheduleForTest(id string) {
	schedulesMu.Lock()
	defer schedulesMu.Unlock()
	delete(schedules, id)
}

// SegmentByStart returns the segment whose Start hour matches the given
// hour, or nil. Used by the schedule executor to look up the prior
// segment's activity for transition detection (chunk 3.3 wake-on-exit).
func (s *Schedule) SegmentByStart(startHour int) *ScheduleSegment {
	if s == nil {
		return nil
	}
	for i := range s.Segments {
		if s.Segments[i].Start == startHour {
			return &s.Segments[i]
		}
	}
	return nil
}

// RegisterScheduleForTest injects a Schedule into the registry so that
// tests in other packages can exercise schedule-dependent code paths
// without writing fixture YAML files. Call t.Cleanup(UnregisterScheduleForTest)
// after each call to avoid cross-test pollution.
func RegisterScheduleForTest(s *Schedule) {
	registerScheduleForTest(s)
}

// UnregisterScheduleForTest removes a previously injected schedule.
func UnregisterScheduleForTest(id string) {
	unregisterScheduleForTest(id)
}
