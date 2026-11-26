package users

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/audio"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/prompt"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/util"
	"golang.org/x/crypto/bcrypt"
)

var (
	// immutable roles
	RoleGuest string = "guest"
	RoleUser  string = "user"
	RoleAdmin string = "admin"
)

type UserRecord struct {
	UserId          int                   `yaml:"userid"`
	Role            string                `yaml:"role"` // Roles group one or more admin commands
	Username        string                `yaml:"username"`
	Password        string                `yaml:"password"`
	Joined          time.Time             `yaml:"joined"`
	LastRenameAt    time.Time             `yaml:"lastrenameat,omitempty"` // Last time the player used the rename command; cooldown uses this
	Macros          map[string]string     `yaml:"macros,omitempty"`       // Up to 10 macros, just string commands.
	Aliases         map[string]string     `yaml:"aliases,omitempty"`      // string=>string remapping of commands
	Ticks           []UserTick            `yaml:"ticks,omitempty"`        // client-run interval timers (web automation panel)
	Triggers        []UserTrigger         `yaml:"triggers,omitempty"`     // client-run text-pattern automation (web automation panel)
	Character       *characters.Character `yaml:"character,omitempty"`
	ItemStorage     Storage               `yaml:"itemstorage,omitempty"`
	ConfigOptions   map[string]any        `yaml:"configoptions,omitempty"`
	Inbox           Inbox                 `yaml:"inbox,omitempty"`
	Muted           bool                  `yaml:"muted,omitempty"`           // Cannot SEND custom communications to anyone but admin/mods
	Deafened        bool                  `yaml:"deafened,omitempty"`        // Cannot HEAR custom communications from anyone but admin/mods
	IsAI            bool                  `yaml:"isai,omitempty"`            // Flagged as an AI account
	ScreenReader    bool                  `yaml:"screenreader,omitempty"`    // Are they using a screen reader? (We should remove excess symbols)
	AsciiMode       bool                  `yaml:"asciimode,omitempty"`       // Convert UTF-8 decorative chars to ASCII for legacy clients
	LineWidth       int                   `yaml:"linewidth,omitempty"`       // Column width for line wrapping; 0 = default 80
	PageSize        int                   `yaml:"pagesize,omitempty"`        // Lines per page of a long reply (#842): 0 follows the screen, -1 (pager.SettingOff) is off
	CombatVerbosity string                `yaml:"combatverbosity,omitempty"` // Combat text level: ""/full, medium (hits only), light (round tally)
	EmailAddress    string                `yaml:"emailaddress,omitempty"`    // Email address (if provided)
	TipsComplete    map[string]bool       `yaml:"tipscomplete,omitempty"`    // Tips the user has followed/completed so they can be quiet
	EventLog        UserLog               `yaml:"-"`                         // Do not retain in user file (for now)
	LastMusic       string                `yaml:"-"`                         // Keeps track of the last music that was played
	LastWhisperFrom int                   `yaml:"-"`                         // UserId of last person who whispered to us (don't save)
	connectionId    uint64
	// sessionMu guards unsentText, suggestText, tempDataStore, activePrompt and activeMenu
	// — the only UserRecord state written from BOTH the per-connection
	// goroutine (main.go's read loop, which holds util.RLockMud around the
	// prompt read and util.LockMud only in LoginUser, CopyoverReconnectUser,
	// AddConnectionEvent and the login kick) and the
	// tick loop (world.go processInput, hooks/RedrawPrompt_SendRedraw, which
	// always does). tempDataStore in particular is a map, and a concurrent map
	// read+write is a hard runtime panic, not a benign race.
	//
	// Unexported, so yaml marshalling ignores it.
	sessionMu      sync.Mutex
	unsentText     string
	suggestText    string
	connectionTime time.Time
	lastInputRound uint64
	tempDataStore  map[string]any
	activePrompt   *prompt.Prompt
	activeMenu     *openMenu  // the open game menu (#375); guarded by sessionMu
	pager          pagerState // the reply being built and the pages waiting (#842); guarded by sessionMu
	// menuSignalHold and menuSignalBase defer MenuChanged across a menu
	// choice (userrecord.menu.go choose); guarded by sessionMu.
	menuSignalHold int
	menuSignalBase bool
	// textMaskSent and textMaskConn record the TEXTMASK state last sent to
	// the web client on connection textMaskConn (userrecord.textmask.go);
	// guarded by sessionMu.
	textMaskSent bool
	textMaskConn uint64
	// inputMasked is what InputMasked returns: the masked state SyncTextMask
	// last computed on the world goroutine, so the connection goroutine reads
	// one locked bool and never the prompt; guarded by sessionMu.
	inputMasked  bool
	isZombie     bool // are they a zombie currently?
	inputBlocked bool // Whether input is currently intentionally turned off (for a certain category of commands)
}

// UserTick is a user-defined real-time-second timer that fires commands from
// a capable client (the web automation panel). Stored per account; executed
// client-side.
type UserTick struct {
	Id          int    `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Commands    string `yaml:"commands" json:"commands"` // ";"-separated
	IntervalSec int    `yaml:"intervalsec" json:"intervalSec"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
}

// SetTick creates (Id==0 → next id, append) or updates (matching Id) a tick,
// enforcing a 1-second interval floor. Returns the stored tick, or an error
// (ErrAutomationFull, ErrAutomationTooLong) and stores nothing when it is
// past a cap (automation_limits.go).
func (u *UserRecord) SetTick(t UserTick) (UserTick, error) {
	if t.IntervalSec < 1 {
		t.IntervalSec = 1
	}
	at := -1
	if t.Id != 0 {
		for i := range u.Ticks {
			if u.Ticks[i].Id == t.Id {
				at = i
				break
			}
		}
	}
	var prev *UserTick
	if at >= 0 {
		prev = &u.Ticks[at]
	}
	if err := u.tickFits(t, prev); err != nil {
		return UserTick{}, err
	}
	if at >= 0 {
		u.Ticks[at] = t
		return t, nil
	}
	if t.Id == 0 {
		maxId := 0
		for _, ex := range u.Ticks {
			if ex.Id > maxId {
				maxId = ex.Id
			}
		}
		t.Id = maxId + 1
	}
	u.Ticks = append(u.Ticks, t)
	return t, nil
}

// RemoveTick deletes the tick with the given id, if present.
func (u *UserRecord) RemoveTick(id int) {
	for i := range u.Ticks {
		if u.Ticks[i].Id == id {
			u.Ticks = append(u.Ticks[:i], u.Ticks[i+1:]...)
			return
		}
	}
}

// TriggerCondition is the optional single if/else gate on a UserTrigger.
// Open for Tier-2 without breaking v1. SourceKind: "pool"|"status"|"capture"|"target".
// SourceKey: pool->"hp"/"sp"/"cp", capture->"$1"/"$2", else "". Op:
// below|above|equals|contains|include|exclude|oneof|notoneof. Values: single for
// pool/capture/status; list (OR) for target oneof/notoneof.
type TriggerCondition struct {
	SourceKind string   `yaml:"sourcekind" json:"sourceKind"`
	SourceKey  string   `yaml:"sourcekey,omitempty" json:"sourceKey,omitempty"`
	Op         string   `yaml:"op" json:"op"`
	Values     []string `yaml:"values" json:"values"`
}

// UserTrigger is a wildcard text pattern that fires commands (optionally gated by
// one condition) when matched against incoming game text. Executed client-side.
type UserTrigger struct {
	Id        int               `yaml:"id" json:"id"`
	Name      string            `yaml:"name" json:"name"`
	Pattern   string            `yaml:"pattern" json:"pattern"`
	Condition *TriggerCondition `yaml:"condition,omitempty" json:"condition,omitempty"`
	ThenCmds  string            `yaml:"thencmds" json:"thenCmds"`
	ElseCmds  string            `yaml:"elsecmds,omitempty" json:"elseCmds,omitempty"`
	Enabled   bool              `yaml:"enabled" json:"enabled"`
	QueueMode string            `yaml:"queuemode,omitempty" json:"queueMode,omitempty"` // ""=fire now, "back", "front"
}

// SetTrigger creates (Id==0 → next id, append) or updates (matching Id) a trigger.
// Returns the stored trigger, or an error (ErrAutomationFull,
// ErrAutomationTooLong) and stores nothing when it is past a cap
// (automation_limits.go).
func (u *UserRecord) SetTrigger(t UserTrigger) (UserTrigger, error) {
	at := -1
	if t.Id != 0 {
		for i := range u.Triggers {
			if u.Triggers[i].Id == t.Id {
				at = i
				break
			}
		}
	}
	var prev *UserTrigger
	if at >= 0 {
		prev = &u.Triggers[at]
	}
	if err := u.triggerFits(t, prev); err != nil {
		return UserTrigger{}, err
	}
	if at >= 0 {
		u.Triggers[at] = t
		return t, nil
	}
	if t.Id == 0 {
		maxId := 0
		for _, ex := range u.Triggers {
			if ex.Id > maxId {
				maxId = ex.Id
			}
		}
		t.Id = maxId + 1
	}
	u.Triggers = append(u.Triggers, t)
	return t, nil
}

// RemoveTrigger deletes the trigger with the given id, if present.
func (u *UserRecord) RemoveTrigger(id int) {
	for i := range u.Triggers {
		if u.Triggers[i].Id == id {
			u.Triggers = append(u.Triggers[:i], u.Triggers[i+1:]...)
			return
		}
	}
}

func NewUserRecord(userId int, connectionId uint64) *UserRecord {

	u := &UserRecord{
		connectionId:   connectionId,
		UserId:         userId,
		Role:           RoleUser,
		Username:       "",
		Password:       "",
		Macros:         make(map[string]string),
		Character:      characters.New(),
		ConfigOptions:  map[string]any{},
		Joined:         time.Now(),
		connectionTime: time.Now(),
		tempDataStore:  make(map[string]any),
		EventLog:       UserLog{},
	}

	return u
}

func (u *UserRecord) ClientSettings() connections.ClientSettings {
	return connections.GetClientSettings(u.connectionId)
}

// HasPlaintextPassword reports whether the stored password is neither a
// bcrypt hash nor a 64-char SHA-256 hex digest — i.e. a legacy/default
// plaintext credential the user must replace.
func (u *UserRecord) HasPlaintextPassword() bool {
	p := u.Password
	if isBcryptHash(p) {
		return false
	}
	// SHA256 hex check: 64 chars of valid hex
	if len(p) == 64 {
		if _, err := hex.DecodeString(p); err == nil {
			return false
		}
	}
	return true
}

// PasswordMatches reports whether input is this account's password. An empty
// input and an empty stored password never match. A bcrypt hash is compared
// as is. A legacy unsalted SHA-256 hash that matches is replaced on u with a
// bcrypt hash, in memory only: SavePasswordUpgrade writes it. A stored
// plain-text password matches when equal, so a game login can force the
// player to change it; the /admin login refuses such an account before it
// gets here.
func (u *UserRecord) PasswordMatches(input string) bool {

	if input == `` || u.Password == `` {
		return false
	}

	// Try bcrypt first (new format).
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(input)); err == nil {
		return true
	}

	// Migration: check if stored password is old unsalted SHA256 format.
	if u.Password == util.Hash(input) {
		// Re-hash with bcrypt so subsequent logins use the secure path.
		if hash, err := bcrypt.GenerateFromPassword([]byte(input), bcrypt.DefaultCost); err == nil {
			u.Password = string(hash)
		}
		return true
	}

	// Plaintext fallback: a legacy plain-text credential authenticates a game
	// login so the player can be forced to change it. The compare does not
	// stop at the first differing byte.
	if u.HasPlaintextPassword() && subtle.ConstantTimeCompare([]byte(u.Password), []byte(input)) == 1 {
		return true
	}

	return false
}

func (u *UserRecord) AddCommandAlias(input string, output string) (addedAlias string, deletedAlias string) {

	if u.Aliases == nil {
		u.Aliases = map[string]string{}
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == `alias` {
		return
	}

	if output == `` {
		delete(u.Aliases, input)
		return ``, input
	}

	if len(input) >= 64 {
		input = input[0:64]
	}

	if len(output) >= 64 {
		output = output[0:64]
	}

	u.Aliases[input] = strings.TrimSpace(output)

	return input, ``
}

func (u *UserRecord) TryCommandAlias(input string) string {

	if u.Aliases == nil {
		u.Aliases = map[string]string{}
		return input
	}

	if alias, ok := u.Aliases[strings.ToLower(input)]; ok {
		return alias
	}

	return input
}

func (u *UserRecord) ShorthandId() string {
	return fmt.Sprintf(`@%d`, u.UserId)
}

func (u *UserRecord) SetLastInputRound(rdNum uint64) {
	u.lastInputRound = rdNum
	// Note: the Idle/AFK → Active wake fires from usercommands.TryCommand
	// (per-command entry point) so it can branch on which command is
	// being dispatched. The `afk` command toggles its own state and must
	// not be auto-cleared before its handler runs. The wake itself is
	// WakePresence; a line a game menu consumes goes through NoteActivity.
}

func (u *UserRecord) GetLastInputRound() uint64 {
	return u.lastInputRound
}

func (u *UserRecord) HasShop() bool {
	return len(u.Character.Shop) > 0
}

// AchievementsExcluded reports whether this account is kept out of achievement
// tracking: admins and AI-flagged accounts, matching the leaderboard exclusion.
// The award poll skips these accounts and the achievements screen tells them so.
func (u *UserRecord) AchievementsExcluded() bool {
	return u.Role == RoleAdmin || u.IsAI
}

func (u *UserRecord) DidTip(tipName string, completed ...bool) bool {

	if u.TipsComplete == nil {
		u.TipsComplete = map[string]bool{}
	}

	if len(completed) > 0 {
		if completed[0] {
			u.TipsComplete[tipName] = completed[0]
		} else {
			delete(u.TipsComplete, tipName)
		}
		return completed[0]
	}

	return u.TipsComplete[tipName]
}

func (u *UserRecord) PlayMusic(musicFileOrId string) {

	v := 100
	if soundConfig := audio.GetFile(musicFileOrId); soundConfig.FilePath != `` {
		musicFileOrId = soundConfig.FilePath
		if soundConfig.Volume > 0 && soundConfig.Volume <= 100 {
			v = soundConfig.Volume
		}
	}

	events.AddToQueue(events.MSP{
		UserId:    u.UserId,
		SoundType: `MUSIC`,
		SoundFile: musicFileOrId,
		Volume:    v,
	})

}

func (u *UserRecord) PlaySound(soundId string, category string) {

	v := 100
	if soundConfig := audio.GetFile(soundId); soundConfig.FilePath != `` {
		soundId = soundConfig.FilePath
		if soundConfig.Volume > 0 && soundConfig.Volume <= 100 {
			v = soundConfig.Volume
		}
	}

	events.AddToQueue(events.MSP{
		UserId:    u.UserId,
		SoundType: `SOUND`,
		SoundFile: soundId,
		Volume:    v,
		Category:  category,
	})

}

func (u *UserRecord) Command(inputTxt string, waitSeconds ...float64) {

	events.AddToQueue(events.Input{
		UserId:    u.UserId,
		InputText: inputTxt,
		ReadyTurn: commandReadyTurn(waitSeconds),
	})

}

// commandReadyTurn is the turn a command waiting waitSeconds (none: now) is
// ready. A wait of zero or less means now: gearup and wimpy's flee pass -1
// for "at once", and a negative wait converted to uint64 wrapped, so on a
// fresh boot (turn count below one second's turns) the command never ran.
func commandReadyTurn(waitSeconds []float64) uint64 {
	readyTurn := util.GetTurnCount()
	if len(waitSeconds) > 0 && waitSeconds[0] > 0 {
		readyTurn += uint64(float64(configs.GetTimingConfig().SecondsToTurns(1)) * waitSeconds[0])
	}
	return readyTurn
}

func (u *UserRecord) BlockInput() {
	u.inputBlocked = true
}

func (u *UserRecord) UnblockInput() {
	u.inputBlocked = false
}

func (u *UserRecord) InputBlocked() bool {
	return u.inputBlocked
}

// CommandFlagged queues inputTxt as Command does, with flagData, and reports
// whether it was queued: a command carrying events.CmdFromClient is refused
// while the player's pending inputs are at the cap (events.TryAddToQueue).
func (u *UserRecord) CommandFlagged(inputTxt string, flagData events.EventFlag, waitSeconds ...float64) bool {

	readyTurn := commandReadyTurn(waitSeconds)

	if flagData&events.CmdBlockInput == events.CmdBlockInput {
		u.BlockInput()
	}

	return events.TryAddToQueue(events.Input{
		UserId:    u.UserId,
		InputText: inputTxt,
		ReadyTurn: readyTurn,
		Flags:     flagData,
	})

}

// QueueCondition queues a condition the producer has filled in, stamping this
// user as the holder and the current life epoch. It is the door a caster
// rides through (messaging M6 slice 1): a spell sets Caster and CasterCrit,
// and every other field means what it means on events.Condition. The other
// add doors here are this door with their own fields set.
func (u *UserRecord) QueueCondition(evt events.Condition) {
	evt.UserId = u.UserId
	evt.MobInstanceId = 0
	evt.LifeEpoch = u.lifeEpoch()
	events.AddToQueue(evt)
}

func (u *UserRecord) AddCondition(conditionId int, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source})
}

// AddConditionScaled queues a condition whose duration is scaled, the way potion
// potency and crafting skill scale them. It travels the same event as
// AddCondition, so the holder still reads the start notice; applying through
// Character.AddConditionScaled directly would land in silence.
func (u *UserRecord) AddConditionScaled(conditionId int, durationMult float64, source string) {

	// Normalise a non-positive multiplier to the authored duration. Conditions.AddConditionScaled
	// clamps a zero to a single trigger, but the hook reads 0 as "unscaled" and would
	// send it down the full-duration path, so the two disagree unless it is fixed here.
	if durationMult <= 0 {
		durationMult = 1.0
	}

	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, DurationMult: durationMult})
}

// AddConditionMagnitude queues a record with an exact trigger count and a
// per-instance magnitude through the event path, so the holder reads the
// start notice. Former conditions apply synchronously through
// Character.AddConditionMagnitude instead; this door is for a spell or item that
// wants the notice too. triggers is the exact trigger count, which for a
// one-round record is the rounds.
func (u *UserRecord) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, Triggers: triggers, Magnitude: magnitude})
}

// AddConditionTickScaled queues a tick_pool condition whose per-round amount
// is scaled by scale, computed where the condition lands
// (Condition_ApplyConditions). A spell passes its caster's scale; 0 means
// 1.0. The record's duration is the authored one.
func (u *UserRecord) AddConditionTickScaled(conditionId int, scale float64, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, TickScale: scale})
}

// lifeEpoch is the epoch every queued condition is stamped with, so ApplyConditions can
// refuse one aimed at a life the holder has since ended. A record with no
// character stamps zero, which matches a fresh character.
func (u *UserRecord) lifeEpoch() uint64 {
	if u.Character == nil {
		return 0
	}
	return u.Character.LifeEpoch
}

// SendText delivers an audio-channel message to this user. See
// rooms.SendText for channel semantics. CategoryDefault keeps prose
// uncolored.
func (u *UserRecord) SendText(cat messaging.Category, txt string) {
	u.sendTextAt(cat, txt, 0)
}

// sendTextAt is SendText queued at an event priority: a higher number is
// delivered after every lower-numbered line in flight, including lines
// queued later in the same event pass.
func (u *UserRecord) sendTextAt(cat messaging.Category, txt string, priority int) {
	rendered := messaging.RenderForRecipient(messaging.RenderInput{
		Category:  cat,
		Text:      txt,
		Channel:   messaging.ChannelAudio,
		LineWidth: u.WrapWidth(),
	})
	if rendered == "" {
		return
	}
	events.AddToQueue(events.Message{
		UserId: u.UserId,
		Text:   rendered + "\n",
	}, priority)
}

// SendBlankLine delivers exactly one empty line to this user (#610). It is
// the only way to send a deliberate blank line: SendText with an empty text
// delivers nothing, so a message that comes out empty at runtime stays
// silent. cat must be a screen category (messaging.BlankLineAllowed); any
// other is refused and logged, because a blank line is screen spacing and
// has no place in a combat, ambient, speech or refusal stream.
func (u *UserRecord) SendBlankLine(cat messaging.Category) {
	if !messaging.BlankLineAllowed(cat) {
		mudlog.Error("SendBlankLine", "error", "category refused for a blank line", "category", cat.String(), "userId", u.UserId)
		return
	}
	events.AddToQueue(events.Message{
		UserId: u.UserId,
		Text:   "\n",
	})
}

// GetLineWidth returns the user's configured line width, falling back
// to 80 if unset or invalid. The messaging pipeline reads this for
// per-recipient wrapping.
func (u *UserRecord) GetLineWidth() int {
	if u == nil || u.LineWidth <= 0 {
		return 80
	}
	return u.LineWidth
}

// GetCombatVerbosity returns the user's combat-text verbosity, defaulting
// to full for unset/unknown values and nil receivers. The combat round
// hook reads this when draining attack narration.
func (u *UserRecord) GetCombatVerbosity() messaging.Verbosity {
	if u == nil {
		return messaging.VerbosityFull
	}
	return messaging.ParseVerbosity(u.CombatVerbosity)
}

func (u *UserRecord) SendWebClientCommand(txt string) {

	events.AddToQueue(events.WebClientCommand{
		ConnectionId: u.connectionId,
		Text:         txt,
	})

}

func (u *UserRecord) SetTempData(key string, value any) {

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	if u.tempDataStore == nil {
		u.tempDataStore = make(map[string]any)
	}

	if value == nil {
		delete(u.tempDataStore, key)
		return
	}
	u.tempDataStore[key] = value
}

func (u *UserRecord) GetTempData(key string) any {

	// A full Lock rather than RLock: this "getter" lazily allocates the map.
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	if u.tempDataStore == nil {
		u.tempDataStore = make(map[string]any)
	}

	if value, ok := u.tempDataStore[key]; ok {
		return value
	}
	return nil
}

// TakeTempData atomically reads and deletes one transient session value. Use it
// for single-use handoffs where separate GetTempData and SetTempData(nil) calls
// would allow concurrent or reentrant consumers to observe the same value.
func (u *UserRecord) TakeTempData(key string) any {
	if u == nil {
		return nil
	}
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	if u.tempDataStore == nil {
		return nil
	}
	value, ok := u.tempDataStore[key]
	if !ok {
		return nil
	}
	delete(u.tempDataStore, key)
	return value
}

// RoleKeyCovers reports whether a granted role key permits the wanted key:
// the same key, or a key below it on a dot boundary. `room` covers
// `room.info` and `room.edit.exits`; `room.info` covers neither `room`, nor
// a sibling such as `room.edit`, nor `room.infox`. An empty granted key
// covers nothing.
func RoleKeyCovers(granted, wanted string) bool {
	if granted == `` {
		return false
	}
	return wanted == granted || strings.HasPrefix(wanted, granted+`.`)
}

// roleKeys returns the keys the user's role grants in the Roles config, or
// nil for the built-in user role and for a role the config does not define.
// The built-in admin role is not listed here; callers pass it first.
func (u *UserRecord) roleKeys() []string {
	if u.Role == RoleUser {
		return nil
	}
	return configs.GetRolesConfig()[u.Role]
}

// HasRolePermission reports whether the user's role permits permissionId.
// Admins hold every key and plain users hold none. Any other role permits a
// key when one of its granted keys covers it (RoleKeyCovers): a granted key
// permits itself and every key below it, never a parent or a sibling.
// Passing true as the optional argument only skips the audit log line; it
// does not widen the match. A command entry point that checks each of its
// sub-keys itself admits a role holding only some of them through
// HasRolePermissionBelow instead.
func (u *UserRecord) HasRolePermission(permissionId string, quiet ...bool) bool {

	if u.Role == RoleAdmin {
		return true
	}

	if len(quiet) == 0 {
		mudlog.Info("RoleCheck", "permissionId", permissionId, "userId", u.UserId, "username", u.Username, "characterName", u.Character.Name)
	}

	for _, granted := range u.roleKeys() {
		if RoleKeyCovers(granted, permissionId) {
			return true
		}
	}

	return false
}

// HasRolePermissionBelow reports whether the user's role grants some key
// strictly below permissionId on a dot boundary (`room.info` is below
// `room`). It never permits permissionId itself: it exists so a command
// whose handler checks every sub-key with HasRolePermission can be reached
// by a role that holds only some of them. Admins always pass.
func (u *UserRecord) HasRolePermissionBelow(permissionId string) bool {

	if u.Role == RoleAdmin {
		return true
	}

	for _, granted := range u.roleKeys() {
		if granted != permissionId && RoleKeyCovers(permissionId, granted) {
			return true
		}
	}

	return false
}

func (u *UserRecord) SetConfigOption(key string, value any) {
	if u.ConfigOptions == nil {
		u.ConfigOptions = make(map[string]any)
	}
	if value == nil {
		delete(u.ConfigOptions, key)
		return
	}
	u.ConfigOptions[key] = value
}

func (u *UserRecord) GetConfigOption(key string) any {
	if u.ConfigOptions == nil {
		u.ConfigOptions = make(map[string]any)
	}
	if value, ok := u.ConfigOptions[key]; ok {
		return value
	}
	return nil
}

func (u *UserRecord) GetConnectTime() time.Time {
	return u.connectionTime
}

func (u *UserRecord) RoundTick() {

}

// The purpose of SetUnsentText(), GetUnsentText() is to
// Capture what the user is typing so that when we redraw the
// "prompt" or status bar, we can redraw what they were in the middle
// of typing.
// I don't like the idea of capturing it every time they hit a key though
// There is probably a better way.
func (u *UserRecord) SetUnsentText(t string, suggest string) {

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	u.unsentText = t
	u.suggestText = suggest
}

func (u *UserRecord) GetUnsentText() (unsent string, suggestion string) {

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	return u.unsentText, u.suggestText
}

// Replace a characters information with another.
func (u *UserRecord) ReplaceCharacter(replacement *characters.Character) {
	u.Character = replacement
}

func (u *UserRecord) SetUsername(un string) error {

	if err := ValidateName(un); err != nil {
		return err
	}

	u.Username = un

	// If no character name, just set it to username for now.
	if u.Character.Name == "" {
		u.Character.Name = u.TempName()
	}

	return nil
}

func (u *UserRecord) TempName() string {
	hasher := md5.New()
	hasher.Write([]byte([]byte(u.Username)))
	hashInBytes := hasher.Sum(nil)
	number := new(big.Int).SetBytes(hashInBytes)

	mod := new(big.Int)
	mod.SetInt64(9087919)

	return fmt.Sprintf("nameless-%d", number.Mod(number, mod).Int64())
}

func (u *UserRecord) SetCharacterName(cn string) error {

	if err := ValidateName(cn); err != nil {
		return err
	}

	u.Character.Name = cn

	return nil
}

func (u *UserRecord) SetPassword(pw string) error {

	validation := configs.GetValidationConfig()

	if len(pw) < int(validation.PasswordSizeMin) || len(pw) > int(validation.PasswordSizeMax) {
		return fmt.Errorf("password must be between %d and %d characters long", validation.PasswordSizeMin, validation.PasswordSizeMax)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	u.Password = string(hash)
	return nil
}

func (u *UserRecord) ConnectionId() uint64 {
	return u.connectionId
}

// PromptDrawnTempKey is the temp-data key holding the last pending question
// the prompt redraw drew; the redraw skips a question that matches it.
const PromptDrawnTempKey = `cmdprompt`

// Prompt related functionality
func (u *UserRecord) StartPrompt(command string, rest string) (*prompt.Prompt, bool) {

	// The rule already holding as the prompt opens; read before sessionMu
	// because the gate reads the character's machines.
	openedUnder := menuGate(u.Character)

	u.sessionMu.Lock()
	if u.activePrompt != nil {
		// If it's the same prompt, return the existing one
		if u.activePrompt.Command == command && u.activePrompt.Rest == rest {
			p := u.activePrompt
			u.sessionMu.Unlock()
			return p, false
		}
	}

	// If no prompt found or it seems like a new prompt, create a new one and replace the old.
	// A prompt replaces an open game menu (#375): one signal, OnClose after unlock.
	wasOpen := u.menuOrPromptOpenLocked()
	replaced := u.activeMenu
	u.activeMenu = nil
	u.activePrompt = prompt.New(command, rest)
	u.activePrompt.OpenedUnder = openedUnder
	// A new prompt's question has not been drawn yet, even when it reads the
	// same as the last one drawn (a second stake at Sable). Forget that draw
	// so the redraw after the command's own lines puts the question last on
	// screen instead of deduping it away (#375 R14).
	delete(u.tempDataStore, PromptDrawnTempKey)
	p := u.activePrompt
	u.signalMenuStateLocked(wasOpen)
	u.sessionMu.Unlock()

	// The prompt it replaced may have held a secret question (#868).
	u.SyncTextMask()

	if replaced != nil {
		u.afterMenuClosed(replaced, menu.CloseReplaced, false)
	}
	return p, true
}

func (u *UserRecord) GetPrompt() *prompt.Prompt {

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()

	return u.activePrompt
}

func (u *UserRecord) ClearPrompt() {
	u.sessionMu.Lock()
	wasOpen := u.menuOrPromptOpenLocked()
	u.activePrompt = nil
	u.signalMenuStateLocked(wasOpen)
	u.sessionMu.Unlock()
	// A masked question may just have gone away (#868); SyncTextMask takes
	// sessionMu itself, so it runs after the unlock.
	u.SyncTextMask()
}

func (u *UserRecord) GetOnlineInfo() OnlineInfo {
	connTime := u.GetConnectTime()

	oTime := time.Since(connTime)

	h := int(math.Floor(oTime.Hours()))
	m := int(math.Floor(oTime.Minutes())) - (h * 60)
	s := int(math.Floor(oTime.Seconds())) - (h * 60 * 60) - (m * 60)

	timeStr := ``
	if h > 0 {
		timeStr = fmt.Sprintf(`%dh%dm`, h, m)
	} else if m > 0 {
		timeStr = fmt.Sprintf(`%dm`, m)
	} else {
		timeStr = fmt.Sprintf(`%ds`, s)
	}

	// Chunk 5 (Presence): IsAFK shim reads the canonical Presence state.
	isAfk := false
	if u.Character != nil && u.Character.Presence != nil {
		isAfk = u.Character.Presence.State() == presence.AFK
	}

	isAI := false
	if cd := connections.Get(u.connectionId); cd != nil {
		isAI = cd.ConnType() == connections.ConnAI
	}

	return OnlineInfo{
		Username:      u.Username,
		CharacterName: u.Character.Name,
		Title:         skills.GetTitle(u.Character.Mutations, u.Character.GetAllSkillRanks(), u.Character.Stats),
		OnlineTime:    int64(oTime.Seconds()),
		OnlineTimeStr: timeStr,
		IsAFK:         isAfk,
		IsAI:          isAI,
		Role:          u.Role,
	}
}

func (u *UserRecord) WimpyCheck() {
	// A flee may be started once every round (#951). In the round one was
	// started, wimpy waits for the next hit in a later round rather than
	// queueing a flee that is only refused, a line for every blow taken.
	if u.Character.FleeTriedThisRound() {
		return
	}
	if currentWimpy := u.GetConfigOption(`wimpy`); currentWimpy != nil {
		healthPct := int(math.Floor(float64(u.Character.Health) / float64(u.Character.HealthMax.Value) * 100))
		if healthPct < currentWimpy.(int) {
			u.Command(`flee`, -1)
		}
	}
}

func (u *UserRecord) SwapToAlt(targetAltName string) bool {

	altNames := []string{}
	nameToAlt := map[string]characters.Character{}

	for _, char := range characters.LoadAlts(u.UserId) {
		altNames = append(altNames, char.Name)
		nameToAlt[char.Name] = char
	}

	match, closeMatch := util.FindMatchIn(targetAltName, altNames...)
	if match == `` {
		match = closeMatch
	}

	if match == `` {
		return false
	}

	selectedChar, ok := nameToAlt[match]
	if !ok {
		return false
	}

	retiredCharName := u.Character.Name

	newAlts := []characters.Character{}
	for _, altChar := range nameToAlt {
		if altChar.Name != selectedChar.Name {
			newAlts = append(newAlts, altChar)
		}
	}

	// add current char to the alts
	newAlts = append(newAlts, *u.Character)
	// Write them to disc
	characters.SaveAlts(u.UserId, newAlts)

	// A quest retired whole is dropped here too, as LoadUser does: an alt that
	// comes into play this way would otherwise keep it until the next login.
	// No marker: the drop is idempotent, and each alt carries its own progress.
	for _, qid := range selectedChar.DropRetiredQuests(quests.RetiredQuests()) {
		mudlog.Info("SwapToAlt", "user", u.Username, "dropped retired quest", qid)
	}

	// Run validation on the new character
	selectedChar.Validate()

	// Set userId
	selectedChar.SetUserId(u.UserId)

	// Replace the current character (has already been written to alts)
	u.Character = &selectedChar

	SaveUser(u)

	events.AddToQueue(events.CharacterChanged{UserId: u.UserId, LastCharacterName: retiredCharName, CharacterName: u.Character.Name})

	return true
}
