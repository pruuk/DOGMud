package users

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// openMenu is the game menu a player has open and the session it came with.
//
// Lock discipline: activeMenu and activePrompt change only under sessionMu,
// and the open/closed state is read before and after each change inside the
// same critical section. Callbacks (OnChoose, OnClose), SendText and the
// MenuChanged event all run after sessionMu is released, so a callback may
// open another menu or read the user without deadlocking.
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

// notifyMenuState queues MenuChanged when a change flipped the open state.
// Call it with sessionMu released.
func (u *UserRecord) notifyMenuState(wasOpen, nowOpen bool) {
	if wasOpen != nowOpen {
		events.AddToQueue(events.MenuChanged{UserId: u.UserId, Open: nowOpen})
	}
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
	u.sessionMu.Unlock()

	if old != nil && old.s.OnClose != nil {
		old.s.OnClose(menu.CloseReplaced)
	}
	u.SendText(messaging.CategoryScreen, menu.Render(m))
	u.notifyMenuState(wasOpen, true)
	return menu.CloseNone, true
}

// takeMenu removes om if it is still the open menu and reports whether it
// was, with the open state before and after.
func (u *UserRecord) takeMenu(om *openMenu) (taken, wasOpen, nowOpen bool) {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if om == nil || u.activeMenu != om {
		return false, false, false
	}
	wasOpen = u.menuOrPromptOpenLocked()
	u.activeMenu = nil
	return true, wasOpen, u.menuOrPromptOpenLocked()
}

// closeMenuWith closes om if it is still the open menu, then speaks the
// interrupt line (when announce), runs OnClose and queues MenuChanged, all
// with sessionMu released. It reports whether om was closed by this call.
func (u *UserRecord) closeMenuWith(om *openMenu, r menu.CloseReason, announce bool) bool {
	taken, wasOpen, nowOpen := u.takeMenu(om)
	if !taken {
		return false
	}
	if announce {
		if line := menu.InterruptLine(r, om.s.Topic); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
	}
	if om.s.OnClose != nil {
		om.s.OnClose(r)
	}
	u.notifyMenuState(wasOpen, nowOpen)
	return true
}

func (u *UserRecord) currentMenu() *openMenu {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.activeMenu
}

// clearNonModalPrompt clears the open prompt only if it is non-modal, in one
// critical section so a prompt opened in between is never the one cleared.
func (u *UserRecord) clearNonModalPrompt() bool {
	u.sessionMu.Lock()
	if u.activePrompt == nil || !u.activePrompt.NonModal {
		u.sessionMu.Unlock()
		return false
	}
	wasOpen := u.menuOrPromptOpenLocked()
	u.activePrompt = nil
	nowOpen := u.menuOrPromptOpenLocked()
	u.sessionMu.Unlock()
	u.notifyMenuState(wasOpen, nowOpen)
	return true
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
	if om := u.currentMenu(); om != nil && u.closeMenuWith(om, menu.CloseFollow, false) {
		return om.s.Topic, om.s.TopicNames, true
	}
	if u.clearNonModalPrompt() {
		return "", nil, true
	}
	return "", nil, false
}

// CheckMenuInterrupts closes the open game menu and any non-modal prompt when
// an interrupt rule holds. Pushed by the busy observers, the condition hook,
// and called on every choice.
func (u *UserRecord) CheckMenuInterrupts() {
	r := menuGate(u.Character)
	if r == menu.CloseNone {
		return
	}
	u.closeForRule(r)
}

// CloseMenusForRoomChange is the room-change rule: any move closes the open
// game menu and any non-modal prompt.
func (u *UserRecord) CloseMenusForRoomChange() {
	u.closeForRule(menu.CloseRoomChange)
}

func (u *UserRecord) closeForRule(r menu.CloseReason) {
	if om := u.currentMenu(); om != nil {
		u.closeMenuWith(om, r, true)
	}
	if u.clearNonModalPrompt() {
		if line := menu.InterruptLine(r, ""); line != "" {
			u.SendText(messaging.CategoryRefusal, line)
		}
	}
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
		if !u.closeMenuWith(om, menu.CloseChosen, false) {
			// Closed or replaced since we looked: the choice is stale.
			return true
		}
		if om.s.OnChoose != nil {
			om.s.OnChoose(*res.Entry)
		}
		return true
	case menu.Leave:
		u.closeMenuWith(om, menu.CloseLeft, false)
		return true
	case menu.NoSuchNumber:
		u.SendText(messaging.CategoryRefusal, fmt.Sprintf("There is no choice %s.", strings.TrimSpace(line)))
		return true
	case menu.Ambiguous:
		labels := make([]string, 0, len(res.Matches))
		for _, e := range res.Matches {
			labels = append(labels, e.Label)
		}
		u.SendText(messaging.CategoryRefusal, "Which one? "+strings.Join(labels, ", ")+".")
		return true
	case menu.Empty:
		u.SendText(messaging.CategoryScreen, menu.Render(om.m))
		return true
	default:
		u.closeMenuWith(om, menu.CloseOtherCommand, false)
		return false
	}
}
