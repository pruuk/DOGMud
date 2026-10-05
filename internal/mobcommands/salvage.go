package mobcommands

import (
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// Salvage runs a single-tick corpse salvage for the mob. Thin
// wrapper over actions.Salvage with TargetCorpse=true. The action
// handles corpse-finding, yield rolling, material storage, and
// room flavor (when players are present).
//
// Activity transitions are managed here (not in actions.Salvage) to
// maintain the Salvaging state for combat-entry veto logic. Guard
// nil Activity — test mobs constructed outside New() may not have
// the machine initialized yet.
func Salvage(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	// Transition to Salvaging before action.
	if mob.Character.Activity != nil {
		_ = mob.Character.Activity.TransitionToSalvaging(
			activity.SalvagingData{
				ItemUuid:    "corpse-salvage", // placeholder; not used by mob path
				RoundsTotal: 1,
			},
			state.TransitionReason{
				Trigger: activity.TriggerSalvageBegin,
				Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
			},
		)
	}

	// Run the action.
	// "salvage <mobId>:<roundCreated>" names one corpse (a bonded companion
	// butchering the kill it was asked to, not whichever is first); a bare
	// "salvage" takes the first eligible, as it always has.
	opts := actions.SalvageOptions{TargetCorpse: true}
	if id, round, ok := strings.Cut(strings.TrimSpace(rest), `:`); ok {
		mobId, err1 := strconv.Atoi(id)
		created, err2 := strconv.ParseUint(round, 10, 64)
		if err1 == nil && err2 == nil && mobId > 0 {
			opts.TargetCorpseMobId, opts.TargetCorpseRoundCreated = mobId, created
		}
	}
	actor := actions.NewMobActorInRoom(mob, room)
	_ = actions.Salvage(actor, opts)

	// Return Activity machine to Free — single-tick resolution complete.
	if mob.Character.Activity != nil {
		_ = mob.Character.Activity.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerSalvageComplete,
			Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
		})
	}

	return true, nil
}
