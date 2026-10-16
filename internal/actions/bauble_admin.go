package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Admin support for baubles (docs/baubles, Phase 5): the generation request
// for an existing record, and regenerating a record's text in the
// background. Used by the admin `bauble` command.

// runBaubleJob runs a background bauble job. Production runs it on its own
// goroutine, which takes the mud lock for its last step; tests replace it
// to run in line without the lock.
var runBaubleJob = func(job func(lock bool)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`baubles`, `action`, `job`, `panic`, r)
			}
		}()
		job(true)
	}()
}

// tellBaubleAdmin sends a line to an admin if they are online. A variable
// so tests can capture it.
var tellBaubleAdmin = func(userId int, text string) {
	if u := users.GetByUserId(userId); u != nil {
		u.SendText(messaging.CategoryAdmin, text)
	}
}

// BaubleRequestForRecord is the request that would name this record now:
// built from the room it was found in when that room still exists, and from
// the recorded provenance alone when it does not. The record's own name is
// added to the names to avoid, so a regeneration asks for something new,
// unless a player's own key wrote it. Call under the mud lock.
func BaubleRequestForRecord(rec baubles.Record) baubles.GenRequest {
	var req baubles.GenRequest
	if room := rooms.LoadRoom(rec.RoomId); room != nil {
		req = BaubleRequest(room, rec.Tier, rec.Source, ``)
	} else {
		req = baubles.GenRequest{
			Tier:      rec.Tier,
			Source:    rec.Source,
			TimeOfDay: `day`,
		}
	}
	// Keep the recorded place: the room may have been re-zoned since.
	req.Place = baubles.Place{RoomId: rec.RoomId, Zone: rec.Zone, Region: rec.Region, Biome: rec.Biome}
	if req.Source == `` {
		req.Source = baubles.SourceSearch
	}
	// A name a player's own key wrote is never sent to a model (spec S3).
	if !rec.PlayerKey {
		req.RecentNames = append(req.RecentNames, rec.Name)
	}
	return req
}

// RegenerateBauble asks the model to name an existing record again, in the
// background, and tells the admin how it went. Every item pointing at the
// record takes the new text at once. Nothing changes if the model does not
// answer (no key, over budget, failure). Call under the mud lock.
func RegenerateBauble(id string, adminUserId int, adminName string) error {
	rec, ok := baubles.Get(id)
	if !ok {
		return baubles.ErrNoRecord
	}
	req := BaubleRequestForRecord(rec)

	runBaubleJob(func(lock bool) {
		res := baubles.Generate(context.Background(), req, util.Rand)
		if lock {
			util.LockMud()
			defer util.UnlockMud()
		}
		updated, removed, err := baubles.ApplyRegenerated(id, res, adminName, util.Rand)
		if err != nil && !errors.Is(err, baubles.ErrCorpusCleanup) {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s was not regenerated: %s.`, id, err))
			return
		}
		tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s is now <ansi fg="itemname">%s</ansi> (%d gold, %.1f lb): %s`,
			id, updated.Name, updated.Value, updated.WeightLbs, updated.Description))
		if removed > 0 {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`Its old text left the fallback corpus (promoted entries removed: %d).`, removed))
		}
		if err != nil {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`<ansi fg="red">%s.</ansi>`, err))
		}
	})
	return nil
}
