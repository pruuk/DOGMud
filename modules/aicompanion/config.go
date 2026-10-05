package aicompanion

import (
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/plugins"
)

// Config is the resolved module configuration. Values arrive through the
// plugin config bag as `any`, so every getter below tolerates the several
// types YAML and the config overlay can produce.
type Config struct {
	Enabled                      bool
	Model                        string
	FastModel                    string
	RequestTimeoutSeconds        int
	MaxCompletionTokens          int
	Temperature                  float64
	MinSecondsBetweenCalls       int
	RecoveryRounds               int
	WorkingMemoryLines           int
	PromptMemoryLines            int
	GreetOnLogin                 bool
	MoodDecayMinutes             int
	LogDecisions                 bool
	PromptMemories               int
	MaxMemories                  int
	MaxFacts                     int
	MaxSummaries                 int
	ReflectOnLogout              bool
	MinSessionLinesForReflection int
	InitiativeMinutes            int
	NoticeThreshold              float64
	NoticeCooldownSeconds        int
	NoticeCallsPerDay            int
	AutonomyMinutes              int
	IdleEmoteMinutes             int
	IdlePastimeMinutes           int
	IdlePastimeChance            float64
	IdleTradeSeconds             int
	CompanionBaubleChance        int
	OptionsInPrompt              int
	AllowErrands                 bool
	MaxErrandSteps               int
	ErrandLingerRounds           int
	LostRounds                   int
	RescueRounds                 int
	NearbyPlacesInPrompt         int
	MaxKnownRooms                int
	ThinkingSeconds              int
	CombatReactionRounds         int
	CombatVariance               float64
	RecoverArrows                bool
	RecoverArrowsMin             float64
	RecoverArrowsMax             float64
	DeepModel                    string
	FastReasoningEffort          string
	MainReasoningEffort          string
	DeepReasoningEffort          string
	FastMaxCompletionTokens      int
	DeepMaxCompletionTokens      int
	FastTimeoutSeconds           int
	RetryTransient               bool
	BreakerErrors                int
	BreakerSeconds               int
	DailyTokensPerCompanion      int
	ModerateOutput               bool
	ModerationModel              string
	BackupEverySessions          int
	SnapshotRounds               int
	ToolRounds                   int
	LeaveConfirmSeconds          int
	RespondWhenAlone             bool
	RecordBystanderSpeech        bool
	RequireConsent               bool
	StrangerAskSeconds           int
	StrangerDailyTokens          int
	StrangerTokensPerOwner       int
	HoldWhenSneaking             bool
	FollowOnFoot                 bool
	FollowDelayMin               float64
	FollowDelayMax               float64
	RoadTalkLines                int
	ConversationSummaries        bool
	ConversationGapSeconds       int
	MinConversationExchanges     int

	// PlayerKeys lets a player run their companion on their own key, held
	// in their browser on the relay origin. Off unless the config says so.
	PlayerKeys bool
	// RelayOrigin is where the relay page is served, "https://host". Tier 2
	// is offered only when this is a valid https origin other than the
	// game's own (validRelayOrigin).
	RelayOrigin string
	// RelayTimeoutSeconds bounds how long a call waits for the browser.
	RelayTimeoutSeconds int

	// RequirePlayerKey: companions speak only to players who run them on
	// their own key (tier 2, the relay). Anyone else gets no answer at
	// all, not set lines, and the server's key is never spent on them.
	RequirePlayerKey bool
	// ReleaseAfterDays: a companion whose owner has not been seen for this
	// many days goes back to the Waystone Hollow (roster.go).
	ReleaseAfterDays int
	// AbandonBelow: a companion whose trust AND affection for their owner
	// have both fallen to this or lower leaves them and goes back to the
	// Hollow, without being told twice.
	AbandonBelow int
	// HollowSignRoom is the room whose board tells players, the first time
	// they step into it, what speaking to the travellers sends and where.
	HollowSignRoom int
}

type getter func(string) any

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// asStringList reads a YAML list (or a comma-separated string) of strings.
func asStringList(v any) []string {
	var out []string
	switch t := v.(type) {
	case []string:
		out = append(out, t...)
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		for _, s := range strings.Split(t, `,`) {
			if s = strings.TrimSpace(s); s != `` {
				out = append(out, s)
			}
		}
	}
	return out
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(t))
		return b
	}
	return false
}

func asInt(v any, def int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case uint64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}

func asFloat(v any, def float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f
		}
	}
	return def
}

// buildConfig resolves config from a getter and applies safe defaults and
// floors, so a missing or mistyped key can never produce a zero timeout or an
// unbounded memory.
func buildConfig(get getter) Config {
	if get == nil {
		get = func(string) any { return nil }
	}
	c := Config{
		Enabled:                      false,
		Model:                        strings.TrimSpace(asString(get(`Model`))),
		FastModel:                    strings.TrimSpace(asString(get(`FastModel`))),
		RequestTimeoutSeconds:        asInt(get(`RequestTimeoutSeconds`), 25),
		MaxCompletionTokens:          asInt(get(`MaxCompletionTokens`), 900),
		Temperature:                  asFloat(get(`Temperature`), 0),
		MinSecondsBetweenCalls:       asInt(get(`MinSecondsBetweenCalls`), 2),
		RecoveryRounds:               asInt(get(`RecoveryRounds`), 40),
		WorkingMemoryLines:           asInt(get(`WorkingMemoryLines`), 40),
		PromptMemoryLines:            asInt(get(`PromptMemoryLines`), 24),
		GreetOnLogin:                 true,
		MoodDecayMinutes:             asInt(get(`MoodDecayMinutes`), 20),
		LogDecisions:                 true,
		PromptMemories:               asInt(get(`PromptMemories`), 8),
		MaxMemories:                  asInt(get(`MaxMemories`), 300),
		MaxFacts:                     asInt(get(`MaxFacts`), 40),
		MaxSummaries:                 asInt(get(`MaxSummaries`), 20),
		ReflectOnLogout:              true,
		MinSessionLinesForReflection: asInt(get(`MinSessionLinesForReflection`), 4),
		InitiativeMinutes:            asInt(get(`InitiativeMinutes`), 5),
		NoticeThreshold:              asFloat(get(`NoticeThreshold`), 0.6),
		NoticeCooldownSeconds:        asInt(get(`NoticeCooldownSeconds`), 45),
		NoticeCallsPerDay:            asInt(get(`NoticeCallsPerDay`), 40),
		AutonomyMinutes:              asInt(get(`AutonomyMinutes`), 3),
		IdleEmoteMinutes:             asInt(get(`IdleEmoteMinutes`), 6),
		IdlePastimeMinutes:           asInt(get(`IdlePastimeMinutes`), 4),
		IdlePastimeChance:            asFloat(get(`IdlePastimeChance`), 0.5),
		IdleTradeSeconds:             asInt(get(`IdleTradeSeconds`), 60),
		CompanionBaubleChance:        asInt(get(`CompanionBaubleChance`), 15),
		OptionsInPrompt:              asInt(get(`OptionsInPrompt`), 10),
		AllowErrands:                 true,
		MaxErrandSteps:               asInt(get(`MaxErrandSteps`), 15),
		ErrandLingerRounds:           asInt(get(`ErrandLingerRounds`), 8),
		LostRounds:                   asInt(get(`LostRounds`), 40),
		RescueRounds:                 asInt(get(`RescueRounds`), 90),
		NearbyPlacesInPrompt:         asInt(get(`NearbyPlacesInPrompt`), 6),
		MaxKnownRooms:                asInt(get(`MaxKnownRooms`), 5000),
		ThinkingSeconds:              asInt(get(`ThinkingSeconds`), 4),
		CombatReactionRounds:         asInt(get(`CombatReactionRounds`), 1),
		CombatVariance:               asFloat(get(`CombatVariance`), 0.25),
		RecoverArrows:                true,
		RecoverArrowsMin:             asFloat(get(`RecoverArrowsMin`), 0.1),
		RecoverArrowsMax:             asFloat(get(`RecoverArrowsMax`), 0.8),
		DeepModel:                    defaultString(asString(get(`DeepModel`)), `gpt-4.1-mini`),
		FastReasoningEffort:          effort(asString(get(`FastReasoningEffort`))),
		MainReasoningEffort:          effort(asString(get(`MainReasoningEffort`))),
		DeepReasoningEffort:          effort(asString(get(`DeepReasoningEffort`))),
		FastMaxCompletionTokens:      asInt(get(`FastMaxCompletionTokens`), 500),
		DeepMaxCompletionTokens:      asInt(get(`DeepMaxCompletionTokens`), 2000),
		FastTimeoutSeconds:           asInt(get(`FastTimeoutSeconds`), 12),
		RetryTransient:               true,
		BreakerErrors:                asInt(get(`BreakerErrors`), 5),
		BreakerSeconds:               asInt(get(`BreakerSeconds`), 60),
		DailyTokensPerCompanion:      asInt(get(`DailyTokensPerCompanion`), 300000),
		ModerateOutput:               true,
		ModerationModel:              strings.TrimSpace(asString(get(`ModerationModel`))),
		BackupEverySessions:          asInt(get(`BackupEverySessions`), 1),
		SnapshotRounds:               asInt(get(`SnapshotRounds`), 25),
		ToolRounds:                   asInt(get(`ToolRounds`), 2),
		LeaveConfirmSeconds:          asInt(get(`LeaveConfirmSeconds`), 120),
		RespondWhenAlone:             true,
		RecordBystanderSpeech:        false,
		RequireConsent:               true,
		StrangerAskSeconds:           asInt(get(`StrangerAskSeconds`), 30),
		StrangerDailyTokens:          asInt(get(`StrangerDailyTokens`), 50000),
		StrangerTokensPerOwner:       asInt(get(`StrangerTokensPerOwner`), 100000),
		HoldWhenSneaking:             true,
		FollowOnFoot:                 true,
		FollowDelayMin:               asFloat(get(`FollowDelayMin`), 0.15),
		FollowDelayMax:               asFloat(get(`FollowDelayMax`), 0.45),
		RoadTalkLines:                asInt(get(`RoadTalkLines`), 3),
		ConversationSummaries:        true,
		ConversationGapSeconds:       asInt(get(`ConversationGapSeconds`), 240),
		MinConversationExchanges:     asInt(get(`MinConversationExchanges`), 3),
		RelayOrigin:                  strings.TrimRight(strings.TrimSpace(asString(get(`RelayOrigin`))), `/`),
		RelayTimeoutSeconds:          asInt(get(`RelayTimeoutSeconds`), 30),
		RequirePlayerKey:             true,
		ReleaseAfterDays:             asInt(get(`ReleaseAfterDays`), 60),
		AbandonBelow:                 asInt(get(`AbandonBelow`), -50),
		HollowSignRoom:               asInt(get(`HollowSignRoom`), 6880),
	}
	if v := get(`RequirePlayerKey`); v != nil {
		c.RequirePlayerKey = asBool(v)
	}
	if v := get(`PlayerKeys`); v != nil {
		c.PlayerKeys = asBool(v)
	}
	if v := get(`Enabled`); v != nil {
		c.Enabled = asBool(v)
	}
	if v := get(`GreetOnLogin`); v != nil {
		c.GreetOnLogin = asBool(v)
	}
	if v := get(`LogDecisions`); v != nil {
		c.LogDecisions = asBool(v)
	}
	if v := get(`ReflectOnLogout`); v != nil {
		c.ReflectOnLogout = asBool(v)
	}
	if v := get(`RespondWhenAlone`); v != nil {
		c.RespondWhenAlone = asBool(v)
	}
	if v := get(`RecordBystanderSpeech`); v != nil {
		c.RecordBystanderSpeech = asBool(v)
	}
	if v := get(`RequireConsent`); v != nil {
		c.RequireConsent = asBool(v)
	}
	if v := get(`ModerateOutput`); v != nil {
		c.ModerateOutput = asBool(v)
	}
	if v := get(`HoldWhenSneaking`); v != nil {
		c.HoldWhenSneaking = asBool(v)
	}
	if v := get(`FollowOnFoot`); v != nil {
		c.FollowOnFoot = asBool(v)
	}
	if v := get(`RecoverArrows`); v != nil {
		c.RecoverArrows = asBool(v)
	}
	if v := get(`ConversationSummaries`); v != nil {
		c.ConversationSummaries = asBool(v)
	}
	if v := get(`AllowErrands`); v != nil {
		c.AllowErrands = asBool(v)
	}
	if v := get(`RetryTransient`); v != nil {
		c.RetryTransient = asBool(v)
	}

	// The key, the endpoint (BaseURL, AllowCustomEndpoint) and the daily
	// token budget are the server's, shared with every feature: the
	// APIFramework config section (internal/apiframework).
	if c.RequestTimeoutSeconds < 5 {
		c.RequestTimeoutSeconds = 5
	}
	if c.RelayTimeoutSeconds < 5 {
		c.RelayTimeoutSeconds = 5
	}
	if c.MaxCompletionTokens < 200 {
		c.MaxCompletionTokens = 200
	}
	if c.MinSecondsBetweenCalls < 0 {
		c.MinSecondsBetweenCalls = 0
	}
	if c.RecoveryRounds < 1 {
		c.RecoveryRounds = 1
	}
	if c.WorkingMemoryLines < 10 {
		c.WorkingMemoryLines = 10
	}
	if c.PromptMemoryLines < 4 {
		c.PromptMemoryLines = 4
	}
	if c.PromptMemoryLines > c.WorkingMemoryLines {
		c.PromptMemoryLines = c.WorkingMemoryLines
	}
	if c.PromptMemories < 0 {
		c.PromptMemories = 0
	}
	if c.MaxMemories < 20 {
		c.MaxMemories = 20
	}
	if c.MaxFacts < 5 {
		c.MaxFacts = 5
	}
	if c.ConversationGapSeconds < 30 {
		c.ConversationGapSeconds = 30
	}
	if c.MinConversationExchanges < 1 {
		c.MinConversationExchanges = 1
	}
	if c.MaxSummaries < 1 {
		c.MaxSummaries = 1
	}
	if c.MinSessionLinesForReflection < 1 {
		c.MinSessionLinesForReflection = 1
	}
	if c.InitiativeMinutes < 0 {
		c.InitiativeMinutes = 0
	}
	if c.NoticeThreshold <= 0 {
		c.NoticeThreshold = 0.6
	}
	if c.NoticeCooldownSeconds < 0 {
		c.NoticeCooldownSeconds = 0
	}
	if c.NoticeCallsPerDay < 0 {
		c.NoticeCallsPerDay = 0
	}
	if c.AutonomyMinutes < 0 {
		c.AutonomyMinutes = 0
	}
	if c.IdleEmoteMinutes < 0 {
		c.IdleEmoteMinutes = 0
	}
	if c.IdlePastimeMinutes < 1 {
		c.IdlePastimeMinutes = 1
	}
	if c.CompanionBaubleChance < 0 {
		c.CompanionBaubleChance = 0
	}
	if c.CompanionBaubleChance > 100 {
		c.CompanionBaubleChance = 100
	}
	if c.IdleTradeSeconds < 10 {
		c.IdleTradeSeconds = 10
	}
	if c.IdlePastimeChance < 0 {
		c.IdlePastimeChance = 0
	}
	if c.IdlePastimeChance > 1 {
		c.IdlePastimeChance = 1
	}
	if c.OptionsInPrompt < 3 {
		c.OptionsInPrompt = 3
	}
	if c.MaxErrandSteps < 1 {
		c.MaxErrandSteps = 1
	}
	if c.ErrandLingerRounds < 1 {
		c.ErrandLingerRounds = 1
	}
	if c.LostRounds < c.ErrandLingerRounds {
		c.LostRounds = c.ErrandLingerRounds
	}
	if c.RescueRounds < c.LostRounds {
		c.RescueRounds = c.LostRounds
	}
	if c.NearbyPlacesInPrompt < 0 {
		c.NearbyPlacesInPrompt = 0
	}
	if c.MaxKnownRooms < 100 {
		c.MaxKnownRooms = 100
	}
	if c.ThinkingSeconds < 0 {
		c.ThinkingSeconds = 0
	}
	if c.CombatReactionRounds < 1 {
		c.CombatReactionRounds = 1
	}
	if c.FollowDelayMin < 0 {
		c.FollowDelayMin = 0
	}
	if c.FollowDelayMax < c.FollowDelayMin {
		c.FollowDelayMax = c.FollowDelayMin
	}
	if c.FollowDelayMax > 3 {
		c.FollowDelayMax = 3
	}
	if c.RoadTalkLines < 0 {
		c.RoadTalkLines = 0
	}
	if c.CombatVariance < 0 {
		c.CombatVariance = 0
	}
	if c.CombatVariance > 1 {
		c.CombatVariance = 1
	}
	if c.RecoverArrowsMin < 0 {
		c.RecoverArrowsMin = 0
	}
	if c.RecoverArrowsMax > 0.95 {
		c.RecoverArrowsMax = 0.95
	}
	if c.RecoverArrowsMax < c.RecoverArrowsMin {
		c.RecoverArrowsMax = c.RecoverArrowsMin
	}
	if c.FastMaxCompletionTokens < 0 {
		c.FastMaxCompletionTokens = 0
	}
	if c.DeepMaxCompletionTokens < 0 {
		c.DeepMaxCompletionTokens = 0
	}
	if c.FastTimeoutSeconds < 0 {
		c.FastTimeoutSeconds = 0
	}
	if c.BreakerErrors < 1 {
		c.BreakerErrors = 1
	}
	if c.BreakerSeconds < 5 {
		c.BreakerSeconds = 5
	}
	if c.DailyTokensPerCompanion < 0 {
		c.DailyTokensPerCompanion = 0
	}
	if c.ModerationModel == `` {
		c.ModerationModel = `omni-moderation-latest`
	}
	if c.BackupEverySessions < 0 {
		c.BackupEverySessions = 0
	}
	if c.SnapshotRounds < 1 {
		c.SnapshotRounds = 1
	}
	if c.ToolRounds < 0 {
		c.ToolRounds = 0
	}
	if c.ToolRounds > 4 {
		c.ToolRounds = 4
	}
	if c.LeaveConfirmSeconds < 30 {
		c.LeaveConfirmSeconds = 30
	}
	if c.ReleaseAfterDays < 1 {
		c.ReleaseAfterDays = 1
	}
	// Abandoning has to sit below where a relationship merely sours, and
	// above where nothing could ever reach it.
	if c.AbandonBelow > -10 {
		c.AbandonBelow = -10
	}
	if c.AbandonBelow < -95 {
		c.AbandonBelow = -95
	}
	if c.MoodDecayMinutes < 0 {
		c.MoodDecayMinutes = 0
	}
	return c
}

func loadConfig(p *plugins.Plugin) Config {
	return buildConfig(func(k string) any { return p.Config.Get(k) })
}

// defaultString is a trimmed setting, or a fallback when it is empty.
func defaultString(v string, fallback string) string {
	if v = strings.TrimSpace(v); v != `` {
		return v
	}
	return fallback
}

// effort normalises a reasoning-effort setting. Only the values the API
// accepts are passed through; anything else means "do not send".
func effort(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case `none`, `minimal`, `low`, `medium`, `high`, `xhigh`:
		return v
	}
	return ``
}
