package behaviortree

import (
	"math"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The instance-zone amount prompt (#375 R14). A talk topic that opens an
// instance zone (open_instance_portal with a zone param) cannot carry the
// stake in its text, so the action asks for it privately:
//
//	<guidance, authored per zone>
//	.: How much gold will you stake?
//
// The prompt runs through users.StartPrompt with PortalStakeCommand: the
// input loop gives the next typed line to the question, then re-runs the
// command, whose handler (usercommands.PortalStake) calls AnswerStake. The
// prompt is non-modal, so moving, a fight or a follow pull closes it.

// createZoneInstance clones an instance zone; a variable so tests can
// record the clone without zone templates on disk.
var createZoneInstance = rooms.CreateZoneInstance

const (
	// PortalStakeCommand is the hidden user command the amount prompt
	// re-runs with each answer.
	PortalStakeCommand = `portalstake`
	// StakeQuestion is the amount prompt's question.
	StakeQuestion = `How much gold will you stake?`

	stakeNotAmountLine = `You name no amount of gold, so the way stays closed.`
	stakeMobGoneLine   = `No one is here to take your stake now.`
	stakeSetAsideLine  = `You name no stake, so the way stays closed.`

	stakeStateMob  = `mobInstanceId`
	stakeStateRoom = `roomId`
	stakeStateReq  = `portalRequest`
)

// StakeOutcome is what AnswerStake did with the prompt.
type StakeOutcome int

const (
	StakeNoPrompt  StakeOutcome = iota // no amount prompt is open
	StakePending                       // still waiting for an answer (Enter on an empty line)
	StakeHandled                       // a stake was named; the mob answered, or the player was told why not
	StakeNotAmount                     // the answer was not a whole number; the prompt closed with one line
)

// startStakePrompt opens the private amount prompt for req, showing the
// zone's guidance first. A stake prompt already open is replaced, so the
// new one carries this mob, room and zone.
func startStakePrompt(user *users.UserRecord, mob *mobs.Mob, roomId int, req portalRequest, guidance string) {
	if cur := user.GetPrompt(); cur != nil && cur.Command == PortalStakeCommand {
		user.ClearPrompt()
	}
	p, _ := user.StartPrompt(PortalStakeCommand, ``)
	p.NonModal = true
	p.SetAsideLine = stakeSetAsideLine
	p.Store(stakeStateMob, mob.InstanceId)
	p.Store(stakeStateRoom, roomId)
	p.Store(stakeStateReq, req)

	if guidance = strings.TrimSpace(guidance); guidance != `` {
		user.SendText(messaging.CategoryScreen, guidance)
	}
	p.Ask(StakeQuestion, []string{})
}

// AnswerStake reads the answer to the open amount prompt. A whole number
// opens the portal through openInstancePortal (which refuses a stake under
// the minimum) once it has checked the mob is still with the player; any
// other answer closes the prompt with one line and is returned, so the
// caller can run it as a command.
func AnswerStake(user *users.UserRecord) (StakeOutcome, string) {
	p := user.GetPrompt()
	if p == nil || p.Command != PortalStakeCommand {
		return StakeNoPrompt, ``
	}
	q := p.Ask(StakeQuestion, []string{})
	if !q.Done {
		return StakePending, ``
	}
	answer := strings.TrimSpace(q.Response)
	user.ClearPrompt()

	mobAny, _ := p.Recall(stakeStateMob)
	roomAny, _ := p.Recall(stakeStateRoom)
	reqAny, _ := p.Recall(stakeStateReq)
	mobInstanceId, okMob := mobAny.(int)
	roomId, okRoom := roomAny.(int)
	req, okReq := reqAny.(portalRequest)
	if !okMob || !okRoom || !okReq {
		mudlog.Error("AnswerStake", "error", "stake prompt state missing", "userId", user.UserId)
		return StakeHandled, ``
	}

	amount, isAmount := parseStake(answer)
	if !isAmount {
		user.SendText(messaging.CategoryRefusal, stakeNotAmountLine)
		return StakeNotAmount, answer
	}

	mob := mobs.GetInstance(mobInstanceId)
	if mob == nil || mob.Character.RoomId != roomId || user.Character.RoomId != roomId {
		user.SendText(messaging.CategoryRefusal, stakeMobGoneLine)
		return StakeHandled, ``
	}

	openInstancePortal(user, mob, roomId, req, amount)
	return StakeHandled, ``
}

// parseStake reads a whole number of gold: digits, with optional thousands
// commas ("1,000"). A number too large to hold reads as the largest stake,
// which the purse check then refuses.
func parseStake(s string) (int, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), `,`, ``)
	if s == `` {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return math.MaxInt, true
	}
	return n, true
}
