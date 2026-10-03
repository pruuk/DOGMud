package apiframework

import (
	"context"
	"strings"
	"sync/atomic"
)

// A player's own key, held in their browser (the AI companion's key relay,
// opened from the web client's "Companion key" button). The companion module
// owns the relay: the page, the transport through the player's client and
// each player's own breaker. It registers itself here so another feature
// can send a request through a player's relay, but only for a purpose that
// player has allowed.
//
// Nothing here is the server's: a relayed call is never reserved against
// the server's budget and never feeds the server key's breaker.

// PurposeFinds is naming what the player finds while searching (baubles).
// The player allows it with "Also name things I find while searching" on
// the key page.
const PurposeFinds = `finds`

// PurposeLively is the player's one permission for every feature that
// makes the world livelier on their key: "Make the world livelier" on the
// key page, which starts ticked. Each such feature lends the key under its
// own purpose, LivelyPurpose(feature), so each has its own breaker on the
// player's key and one feature failing never pauses another. Every
// lively purpose is allowed or refused together, by that one box.
//
// To add a lively feature: give it a LivelyPurpose constant here, its own
// consumer and allowance dimension in budget.go, and add its reply schema
// name to LIVELY_SCHEMAS in modules/aicompanion/relayweb/relay.js (the
// browser relays a lively request only under a name listed there, and only
// while the box is ticked).
const PurposeLively = `lively`

// LivelyPurpose is the purpose a lively feature lends a player's key under.
func LivelyPurpose(feature string) string { return PurposeLively + `:` + feature }

// IsLively reports whether purpose is a lively feature's (LivelyPurpose).
func IsLively(purpose string) bool {
	feature, ok := strings.CutPrefix(purpose, PurposeLively+`:`)
	return ok && feature != ``
}

// PurposeNPCIdle is townsfolk near the player now and then coming up with an
// idle moment of their own (modules/npcidle), instead of one of their set
// lines. A lively purpose.
const PurposeNPCIdle = PurposeLively + `:npcidle`

// PurposeRoomLife is the place around the player now and then producing a
// small event of its own, seen or heard (modules/roomlife), instead of one of
// its set ambient lines. A lively purpose.
const PurposeRoomLife = PurposeLively + `:roomlife`

// PurposeLookDetail is a closer look at something a room's description
// mentions but nothing in the room answers to (modules/lookdetail). A
// lively purpose.
const PurposeLookDetail = PurposeLively + `:lookdetail`

// PurposeRifts is a new room for a rift's pools, written in the background
// as a run's rooms are built and saved to the rift's bank so the pools grow
// (modules/rifts). A lively purpose.
const PurposeRifts = PurposeLively + `:rifts`

// Relay sends requests through a player's own key.
type Relay interface {
	// Model returns the model the player's live relay uses when it may be
	// used now for purpose: the relay is up, the player allowed purpose,
	// and the player's own breaker is closed.
	Model(userId int, purpose string) (model string, ok bool)
	// Send posts a chat completions body (built with Chat.Body; the relay
	// sets the model itself) through the player's browser and returns the
	// provider's status and raw reply. carries says what the body carries,
	// for the relay's consent door. sent is the request possibly having
	// reached the provider.
	Send(ctx context.Context, userId int, body []byte, carries Carries) (status int, raw []byte, sent bool, err error)
	// Result feeds the player's own breaker for purpose with a call's
	// outcome. Each purpose has its own (each lively feature too): one
	// feature's failures never pause another, nor the companion.
	Result(userId int, purpose string, err error)
}

var relay atomic.Pointer[Relay]

// SetRelay installs the relay (the companion module, at load). nil removes
// it.
func SetRelay(r Relay) {
	if r == nil {
		relay.Store(nil)
		return
	}
	relay.Store(&r)
}

// PlayerRelay returns the installed relay, or nil.
func PlayerRelay() Relay {
	if r := relay.Load(); r != nil {
		return *r
	}
	return nil
}
