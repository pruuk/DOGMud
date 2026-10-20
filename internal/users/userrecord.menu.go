package users

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/prompt"
)

// openMenu is the game menu a player has open and the session it came with.
//
// Lock discipline: activeMenu and activePrompt change only under sessionMu,
// and MenuChanged is queued inside the same critical section as the change
// (sessionMu then the events queue lock; queue listeners run with neither
// held), so two goroutines can never queue the signals out of order.
// Callbacks (OnChoose, OnClose) and SendText run after sessionMu is
// released, so a callback may open another menu or read the user without
// deadlocking.
type openMenu struct {
	m menu.Menu
	s menu.Session
}

// The characters push hook lands on the user's interrupt check: a busy-state
// transition closes an open menu at once (#375).
func init() {
	characters.BusyStateChanged = func(userId int) {
		if u := GetByUserId(userId); u != nil {
			u.CheckMenuInterrupts()
		}
	}
}

// menuOrPromptOpenLocked is MenuOrPromptOpen for a caller holding sessionMu.
func (u *UserRecord) menuOrPromptOpenLocked() bool {
	return u.activeMenu != nil || u.activePrompt != nil
}

// MenuOrPromptOpen reports whether the player has a game menu or any prompt
// open: what Char.Menu tells the web client.
func (u *UserRecord) MenuOrPromptOpen() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.menuOrPromptOpenLocked()
}

// HasOpenMenu reports whether a game menu is open.
func (u *UserRecord) HasOpenMenu() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu != nil
}

// signalMenuStateLocked queues MenuChanged when the open state now differs
// from wasOpen. While a choice holds the signal (holdMenuSignalLocked) it
// does nothing; the release compares against the state before the choice.
// The caller holds sessionMu.
func (u *UserRecord) signalMenuStateLocked(wasOpen bool) {
	if u.menuSignalHold > 0 {
		return
	}
	if now := u.menuOrPromptOpenLocked(); now != wasOpen {
		events.AddToQueue(events.MenuChanged{UserId: u.UserId, Open: now})
	}
}

// holdMenuSignalLocked defers MenuChanged until the matching release, so a
// choice whose OnChoose opens the next menu never tells the client the menu
// closed in between. wasOpen is the state before the choice. The caller
// holds sessionMu.
func (u *UserRecord) holdMenuSignalLocked(wasOpen bool) {
	if u.menuSignalHold == 0 {
		u.menuSignalBase = wasOpen
	}
	u.menuSignalHold++
}

// releaseMenuSignal ends a hold and, at the outermost release, signals any
// change against the state before the hold began.
func (u *UserRecord) releaseMenuSignal() {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	u.menuSignalHold--
	if u.menuSignalHold == 0 {
		u.signalMenuStateLocked(u.menuSignalBase)
	}
}

// MenuBlockedReason reports what would stop OpenMenu right now: the
// interrupt rule that holds (menu.CloseNone when none does), and whether a
// prompt is open. A caller checks it before speaking any lines that lead
// into a menu, so a refused menu never leaves half a conversation behind.
func (u *UserRecord) MenuBlockedReason() (reason menu.CloseReason, promptOpen bool) {
	reason = menuGate(u.Character)
	u.sessionMu.Lock()
	promptOpen = u.activePrompt != nil
	u.sessionMu.Unlock()
	return reason, promptOpen
}

// OpenMenu opens a game menu and sends it. It refuses, returning the gate's
// reason, while an interrupt rule holds, and refuses (menu.CloseNone, false)
// while a prompt is open. An open game menu is replaced.
func (u *UserRecord) OpenMenu(m menu.Menu, s menu.Session) (menu.CloseReason, bool) {
	if r := menuGate(u.Character); r != menu.CloseNone {
		return r, false
	}
	u.sessionMu.Lock()
	if u.activePrompt != nil {
		u.sessionMu.Unlock()
		return menu.CloseNone, false
	}
	wasOpen := u.menuOrPromptOpenLocked()
	old := u.activeMenu
	u.activeMenu = &openMenu{m: m, s: s}
	u.signalMenuStateLocked(wasOpen)
	u.sessionMu.Unlock()

	if old != nil && old.s.OnClose != nil {
		old.s.OnClose(menu.CloseReplaced)
	}
	u.SendText(messaging.CategoryScreen, menu.Render(m))
	return menu.CloseNone, true
}

// closeMenuWith closes om if it is still the open menu (signalling under the
// lock), then speaks the interrupt line (when announce) and runs OnClose with
// sessionMu released. It reports whether om was closed by this call.
func (u *UserRecord) closeMenuWith(om *openMenu, r menu.CloseReason, announce bool) bool {
	u.sessionMu.Lock()
	if om == nil || u.activeMenu != om {
		u.sessionMu.Unlock()
		return false
	}
	wasOpen := u.menuOrPromptOpenLocked()
	u.activeMenu = nil
	u.signalMenuStateLocked(wasOpen)
	u.sessionMu.Unlock()

	u.afterMenuClosed(om, r, announce)
	return true
}

// afterMenuClosed speaks the interrupt line (when announce) and runs OnClose
// for a menu already taken off the user. Call it with sessionMu released.
func (u *UserRecord) afterMenuClosed(om *openMenu, r menu.CloseReason, announce bool) {
	if announce {
		if line := menu.InterruptLine(r, om.s.Topic); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
	}
	if om.s.OnClose != nil {
		om.s.OnClose(r)
	}
}

func (u *UserRecord) currentMenu() *openMenu {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu
}

// clearNonModalPromptLocked clears the open prompt only if it is non-modal
// and r is a new interrupt for it, and returns the prompt it cleared (nil
// when none). A rule that already held when the prompt opened (OpenedUnder)
// is not new: mail opened while resting survives the next push while the
// player still rests. The caller holds sessionMu and signals.
func (u *UserRecord) clearNonModalPromptLocked(r menu.CloseReason) *prompt.Prompt {
	p := u.activePrompt
	if p == nil || !p.NonModal {
		return nil
	}
	if r != menu.CloseNone && r == p.OpenedUnder {
		return nil
	}
	u.activePrompt = nil
	return p
}

// takeMenuAndNonModalPrompt removes the open game menu and any non-modal
// prompt r closes, in one critical section, signalling once, and returns
// what it took.
func (u *UserRecord) takeMenuAndNonModalPrompt(r menu.CloseReason) (om *openMenu, cleared *prompt.Prompt) {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	wasOpen := u.menuOrPromptOpenLocked()
	om = u.activeMenu
	u.activeMenu = nil
	cleared = u.clearNonModalPromptLocked(r)
	u.signalMenuStateLocked(wasOpen)
	return om, cleared
}

// sayPromptSetAside speaks a closed prompt's SetAsideLine, if it has one.
func (u *UserRecord) sayPromptSetAside(p *prompt.Prompt) {
	if p != nil && p.SetAsideLine != "" {
		u.SendText(messaging.CategorySystem, p.SetAsideLine)
	}
}

// lowerPromptOpenedUnder forgets the rule a prompt opened under once that
// rule no longer holds, so the same rule coming back is a new interrupt
// (sit, stand, sit again closes it).
func (u *UserRecord) lowerPromptOpenedUnder() {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if u.activePrompt != nil {
		u.activePrompt.OpenedUnder = menu.CloseNone
	}
}

// CloseMenu closes the open game menu, if any, announcing an interrupt reason.
func (u *UserRecord) CloseMenu(r menu.CloseReason) {
	u.closeMenuWith(u.currentMenu(), r, true)
}

// CloseMenuForFollow closes the open game menu or non-modal prompt silently
// because a leader is pulling the player away, and returns the session's
// topic, its names, and whether anything was open. The caller speaks the
// three follow lines (actions.BreakOffMenuToFollow).
func (u *UserRecord) CloseMenuForFollow() (topic string, names []string, had bool) {
	om, cleared := u.takeMenuAndNonModalPrompt(menu.CloseFollow)
	u.sayPromptSetAside(cleared)
	if om != nil {
		u.afterMenuClosed(om, menu.CloseFollow, false)
		return om.s.Topic, om.s.TopicNames, true
	}
	return "", nil, cleared != nil
}

// CheckMenuInterrupts closes the open game menu when an interrupt rule
// holds, and any non-modal prompt when the rule is new to it (see
// clearNonModalPromptLocked). Pushed by the busy observers, the condition
// hook, and called on every choice.
func (u *UserRecord) CheckMenuInterrupts() {
	// Hot path: nothing open means nothing to close or lower.
	if !u.MenuOrPromptOpen() {
		return
	}
	r := menuGate(u.Character)
	if r == menu.CloseNone {
		u.lowerPromptOpenedUnder()
		return
	}
	u.closeForRule(r)
}

// CloseMenusForRoomChange is the room-change rule: any move closes the open
// game menu, any non-modal prompt, and a RoomBound builder prompt (its next
// answer would otherwise edit the new room).
func (u *UserRecord) CloseMenusForRoomChange() {
	if u.takeRoomBoundPrompt() {
		u.SendText(messaging.CategoryAdmin, "You left the room, so the edit is cancelled.")
	}
	u.closeForRule(menu.CloseRoomChange)
}

// takeRoomBoundPrompt clears the open prompt only if it is RoomBound,
// signalling the change, and reports whether it did.
func (u *UserRecord) takeRoomBoundPrompt() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if u.activePrompt == nil || !u.activePrompt.RoomBound {
		return false
	}
	wasOpen := u.menuOrPromptOpenLocked()
	u.activePrompt = nil
	u.signalMenuStateLocked(wasOpen)
	return true
}

func (u *UserRecord) closeForRule(r menu.CloseReason) {
	om, cleared := u.takeMenuAndNonModalPrompt(r)
	if om != nil {
		u.afterMenuClosed(om, r, true)
	}
	if cleared != nil {
		if line := menu.InterruptLine(r, ""); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
		u.sayPromptSetAside(cleared)
	}
}

// choose closes om as chosen and runs its callbacks, holding MenuChanged
// until OnChoose returns so a choice that opens the next menu never reads as
// closed in between. It reports false when om was no longer the open menu.
func (u *UserRecord) choose(om *openMenu, e menu.Entry) bool {
	u.sessionMu.Lock()
	if u.activeMenu != om {
		u.sessionMu.Unlock()
		return false
	}
	u.holdMenuSignalLocked(u.menuOrPromptOpenLocked())
	u.activeMenu = nil
	u.sessionMu.Unlock()
	defer u.releaseMenuSignal()

	if om.s.OnClose != nil {
		om.s.OnClose(menu.CloseChosen)
	}
	if om.s.OnChoose != nil {
		om.s.OnChoose(e)
	}
	return true
}

// RouteMenuInput offers a line to the open game menu. It returns true when
// the menu consumed the line. A line the menu does not recognise closes it
// (menu.CloseOtherCommand) and returns false so the caller runs it as a
// command. Only typed lines may answer.
func (u *UserRecord) RouteMenuInput(line string, typed bool) bool {
	if !typed {
		return false
	}
	om := u.currentMenu()
	if om == nil {
		return false
	}
	res := menu.Resolve(om.m, line)
	switch res.Kind {
	case menu.Chosen:
		if r := menuGate(u.Character); r != menu.CloseNone {
			u.closeMenuWith(om, r, true)
			return true
		}
		if om.s.StillValid != nil && !om.s.StillValid() {
			u.closeMenuWith(om, menu.CloseContextLost, true)
			return true
		}
		// A false return means the menu closed or was replaced since we
		// looked: the choice is stale and is dropped.
		u.choose(om, *res.Entry)
		return true
	case menu.Leave:
		u.closeMenuWith(om, menu.CloseLeft, false)
		return true
	case menu.NoSuchNumber:
		u.SendText(messaging.CategoryRefusal, noSuchChoiceLine(line))
		return true
	case menu.Ambiguous:
		u.SendText(messaging.CategoryRefusal, ambiguousLine(res.Matches))
		return true
	case menu.Empty:
		u.SendText(messaging.CategoryScreen, menu.Render(om.m))
		return true
	default:
		u.closeMenuWith(om, menu.CloseOtherCommand, false)
		return false
	}
}
