package baubles

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// schemaOverhead is the reply schema, sent with every call and not in the
// messages, in round numbers.
const schemaOverhead = 300

var (
	errNoRoute     = errors.New(`no key to name it with`)
	errBreakerOpen = errors.New(`the server key's breaker is open`)
	errSlotsBusy   = errors.New(`all generation slots busy`)
)

// finderCharges is the finder's own allowance, when there is a finder (an
// admin's regeneration has none and counts only under the baubles share).
func finderCharges(cfg Config, finderId int) []apiframework.Charge {
	if finderId <= 0 {
		return nil
	}
	return []apiframework.Charge{{Dim: apiframework.DimBaublesFinder, UserId: finderId, Limit: cfg.DailyTokensPerUser}}
}

// generate is the baubles.GeneratorFunc this module installs. It runs on a
// delivery goroutine WITHOUT the mud lock (see actions/search_bauble.go), so
// it touches no game state. Any error sends the find down the fallback path
// (the corpus, or a generic trinket), and the player never sees it.
//
// The route: the finder's own key first, when they allowed it on the key
// page (apiframework.PurposeFinds); then the server's key, reserved against
// the one daily budget every feature shares; else no name. Player-key text
// the server cannot moderate is kept to its finder (moderate).
func (m *BaublesModule) generate(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
	cfg := m.snapshot()
	msgs := buildMessages(req)
	chat := apiframework.Chat{
		Model:       cfg.Model,
		Messages:    msgs,
		SchemaName:  baubles.ReplySchemaName,
		Schema:      baubles.ReplySchema(),
		MaxTokens:   cfg.MaxCompletionTokens,
		Temperature: cfg.Temperature,
		Effort:      cfg.ReasoningEffort,
	}

	content, tokens, model, playerKey, report, err := m.name(ctx, cfg, req, chat)
	if errors.Is(err, errSlotsBusy) {
		// Refused at the door, not a call that failed: it counts in no
		// statistic (m.count) and feeds no breaker, as before this slice.
		return baubles.GenResult{}, err
	}
	// A refused reservation (a spent day, share or allowance) made no call:
	// it is neither a naming nor a failure in `bauble status`, and it feeds
	// no breaker (its report is a no-op). baubles.Generate logs the refusal,
	// naming the counter (apiframework.RefusedBy), and unlike errSlotsBusy it
	// still gets its model call line below when LogRequests is on.
	if apiframework.RefusedBy(err) == `` {
		m.count(playerKey, err != nil)
	}
	if cfg.LogRequests {
		mudlog.Info(`baubles`, `action`, `model call`, `zone`, req.Place.Zone, `tier`, string(req.Tier),
			`model`, model, `playerKey`, playerKey, `tokens`, tokens, `error`, errString(err))
	}
	if err != nil {
		report(err)
		return baubles.GenResult{}, fmt.Errorf(`model call: %w`, err)
	}

	// The find's outcome for the breakers is whether its answer can be
	// used, not only whether one came back: a model that keeps ignoring the
	// schema pauses baubles' own breaker (never the provider's, which it
	// answered) instead of being paid for again and again.
	reply, err := baubles.ParseReply(content)
	if err == nil {
		// Keep the cleaned text: it is what is moderated and what the
		// world shows. A player-key reply already passed the allowlist in
		// name (refusedByAllowlist); it is not checked again here, where a
		// refusal would feed the player's breaker. baubles.Generate holds
		// every generator to it all the same.
		reply, err = baubles.CleanReply(reply)
	}
	report(err)
	if err != nil {
		return baubles.GenResult{}, err
	}

	moderated, finderOnly, err := m.moderate(cfg, reply, playerKey)
	if err != nil {
		return baubles.GenResult{}, err
	}

	return baubles.GenResult{
		Reply:         reply,
		Generator:     baubles.GeneratorOpenAI,
		Model:         model,
		PromptVersion: PromptVersion,
		Tokens:        tokens,
		Moderated:     moderated,
		PlayerKey:     playerKey,
		FinderOnly:    finderOnly,
	}, nil
}

// moderationBreaker is the moderation check's OWN consumer breaker, apart
// from ConsumerBaubles (review finding d): a naming call's success on the
// server's key reports to ConsumerBaubles before moderation runs
// (generate), and a breaker shared by both would have every good naming
// call reset the run of failed checks, so it could never open; and failed
// naming calls would count as moderation failures. It is fed only by
// moderate, on both routes (apiframework.RecordConsumer), and read by
// moderationPossible and moderate.
const moderationBreaker = `baubles-moderation`

// moderationPossible reports whether the server can moderate a reply now:
// ModerateOutput on, a server key, and neither the provider breaker nor
// the moderation breaker open. s and blocked (apiframework.Blocked read
// once by the caller, moderate, against a single now) are passed in, so
// the breakers are consulted exactly once per find: a moderate that
// re-read them after this call could see the breaker open between the two
// reads and refuse a player-key find that this decision already allowed,
// rather than keeping it to its finder (finderOnly). Player-key text named
// while moderation is not possible is kept to its finder (owner ruling
// 2026-09-29), never shown to anyone else unmoderated.
func moderationPossible(cfg Config, s apiframework.ServerSettings, blocked bool) bool {
	return cfg.ModerateOutput && s.HasKey() && !blocked
}

// moderateReadForTest runs, when set, immediately after moderate takes its
// one breaker read for a find, before that read is used. Nil outside
// tests; it is how a test lands a breaker state change inside the window a
// second, redundant read once raced with (fixed by this commit: the
// breaker is now read exactly once per find, into blocked, in moderate).
var moderateReadForTest = func() {}

// refusedByAllowlist reports whether content, from a finder's own key, is
// a usable answer (it parses and passes CleanReply) whose cleaned text
// baubles.CheckPlayerKeyText refuses (ruling 15). That is not the key
// failing: the caller tells its breaker nothing and goes on to the server's
// route. An answer that does not parse or clean is not this; it goes on to
// generate, which reports it to the player's breaker as before.
func refusedByAllowlist(content string) bool {
	reply, err := baubles.ParseReply(content)
	if err != nil {
		return false
	}
	if reply, err = baubles.CleanReply(reply); err != nil {
		return false
	}
	return baubles.CheckPlayerKeyText(reply) != nil
}

// name makes the call on the first route that is open and returns the
// reply's content, and report, which takes the find's final outcome (its
// answer parsed and checked, or the failure) to that route's breaker,
// exactly once. A failure on the finder's own key is reported there and
// falls back to the server's key.
//
// A refusal on the finder's own key (their baubles.finder allowance, the
// only counter that route reserves against) also goes on to the server's
// key, where the same allowance refuses it again. When the server's route
// cannot even get that far (errNoRoute, errBreakerOpen: no reservation, no
// call), the first refusal is the find's answer, so generate's refusal
// guard sees it: no failure in `bauble status`, and the refusal is what is
// logged. errSlotsBusy keeps its place ahead of it, as in generate, where
// it is turned away before the refusal guard is reached; neither counts.
func (m *BaublesModule) name(ctx context.Context, cfg Config, req baubles.GenRequest, chat apiframework.Chat) (content string, tokens int, model string, playerKey bool, report func(error), err error) {
	var refusal error // the finder's own key's reservation, refused
	refusedModel := ``
	// A pickpocket's find takes this route too, on the thief's own key. Its
	// naming starts at the moment the roll succeeds, so a thief watching
	// their browser's network traffic can learn the outcome before the
	// reveal; the owner accepted that so every find follows one order.
	if cfg.UsePlayerKeys && req.FinderUserId > 0 {
		if r := apiframework.PlayerRelay(); r != nil {
			if relayModel, ok := r.Model(req.FinderUserId, apiframework.PurposeFinds); ok {
				// The finder's own slot, never one of the server's: a busy
				// one (their last find still naming) goes to the server.
				if release, free := m.takeFinderSlot(req.FinderUserId); free {
					content, tokens, report, err = viaPlayer(ctx, cfg, r, req.FinderUserId, relayModel, chat)
					release()
					switch {
					case err == nil && refusedByAllowlist(content):
						// Plain enough for the model, not for other players
						// (ruling 15): not the key's failure, so its breaker
						// hears nothing, and the server's key names the find.
						// The tokens the finder's key spent stay charged to
						// their allowance, and the server's key charges it
						// again: the finder's key did the work, and the owner
						// intends both to count.
					case err == nil:
						return content, tokens, relayModel, true, report, nil
					default:
						report(err)
						if ctx.Err() != nil {
							return ``, 0, relayModel, true, func(error) {}, err
						}
						if apiframework.RefusedBy(err) != `` {
							refusal, refusedModel = err, relayModel
						}
					}
				}
			}
		}
	}
	// The server key's model calls, MaxConcurrent at once. A slot covers
	// the call only; moderation afterwards is free and not a model call.
	release, free := m.takeServerSlot()
	if !free {
		return ``, 0, cfg.Model, false, func(error) {}, errSlotsBusy
	}
	defer release()
	content, tokens, report, err = viaServer(ctx, cfg, chat, req.FinderUserId)
	if refusal != nil && (errors.Is(err, errNoRoute) || errors.Is(err, errBreakerOpen)) {
		return ``, 0, refusedModel, true, func(error) {}, refusal
	}
	return content, tokens, cfg.Model, false, report, err
}

// canceled is a find given up on (a copyover's flush): nobody's failure.
// Its own time running out is not: that is the provider not answering.
func canceled(ctx context.Context) bool { return errors.Is(ctx.Err(), context.Canceled) }

// viaPlayer names the find through the finder's own key, in their browser.
// The player's provider may not accept a reasoning effort, and the relay
// sets its own model, so neither is the server's. Nothing of the server's
// is reserved; the finder's own allowance is, and the player's finds
// breaker is fed.
func viaPlayer(ctx context.Context, cfg Config, r apiframework.Relay, userId int, model string, chat apiframework.Chat) (string, int, func(error), error) {
	none := func(error) {}
	// The outcome is held against the finder's key for finds only (the
	// relay keeps a breaker per purpose, and their companion's is never
	// touched), so a provider that cannot serve finds stops being asked.
	report := func(err error) {
		if !canceled(ctx) {
			r.Result(userId, apiframework.PurposeFinds, err)
		}
	}
	chat.Model, chat.Effort = model, ``
	body, err := chat.Body()
	if err != nil {
		return ``, 0, none, err
	}
	prompt := apiframework.EstimateTokens(chat.Messages) + schemaOverhead
	// Held against the finder's allowance only. A refusal is nobody's
	// failure, so the relay is neither asked nor fed.
	hold, err := apiframework.Reserve(apiframework.ConsumerBaubles, prompt+chat.MaxTokens, false, finderCharges(cfg, userId)...)
	if err != nil {
		return ``, 0, none, err
	}
	status, raw, sent, err := r.Send(ctx, userId, body, apiframework.CarriesNoPlayerData)
	if err == nil && status != http.StatusOK {
		// The body is the provider's own text about the player's own
		// account: neither kept nor logged. The status says enough.
		err = &apiframework.StatusError{Status: status}
	}
	if err != nil {
		// A request that may have left is charged as the provider may have
		// billed it; one that never left costs nothing.
		n, _ := apiframework.Charged(0, sent, status, prompt, chat.MaxTokens, true)
		apiframework.Settle(hold, n, false)
		return ``, 0, report, err
	}
	reply := apiframework.DecodeChat(status, raw)
	// The count came through the player's browser, which they can write:
	// held to what one request could cost before it is recorded (spec S3).
	tokens, _ := apiframework.Charged(reply.Tokens, true, status, prompt, chat.MaxTokens, true)
	apiframework.Settle(hold, tokens, false)
	return reply.Content, tokens, report, reply.Err
}

// viaServer names the find on the server's key: leave from the breakers
// (the baubles' own and the provider's, apiframework.Allow), the worst case
// reserved against the one daily budget, the call (retried once on a
// transient failure when RetryTransient), then the settlement. It returns
// report, through which the caller gives ONE outcome for the whole find,
// retries included, once its answer has been parsed and checked. Only a failure that says the provider or key is unwell reaches
// the provider breaker the companion shares (apiframework.ProviderFailure);
// a bauble model or schema the provider refuses pauses baubles only.
func viaServer(ctx context.Context, cfg Config, chat apiframework.Chat, finderId int) (string, int, func(error), error) {
	none := func(error) {}
	s := apiframework.Server()
	if !s.HasKey() {
		return ``, 0, none, errNoRoute
	}
	body, err := chat.Body()
	if err != nil {
		return ``, 0, none, err
	}
	prompt := apiframework.EstimateTokens(chat.Messages) + schemaOverhead
	reserve := prompt + chat.MaxTokens
	if cfg.RetryTransient {
		reserve *= 2
	}
	ticket, ok := apiframework.Allow(apiframework.ConsumerBaubles, time.Now())
	if !ok {
		return ``, 0, none, errBreakerOpen
	}
	hold, err := apiframework.Reserve(apiframework.ConsumerBaubles, reserve, true, finderCharges(cfg, finderId)...)
	if err != nil {
		apiframework.Release(apiframework.ConsumerBaubles, ticket)
		return ``, 0, none, err
	}

	charged := 0
	var reply apiframework.Reply
	for attempt := 0; ; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
		ex := apiframework.Post(callCtx, s.Endpoint, `/chat/completions`, body, apiframework.CarriesNoPlayerData, nil)
		cancel()
		if ex.Err == nil {
			reply = apiframework.DecodeChat(ex.Status, ex.Raw)
		} else {
			reply = apiframework.Reply{Err: ex.Err}
		}
		n, _ := apiframework.Charged(reply.Tokens, ex.Sent, ex.Status, prompt, chat.MaxTokens, false)
		charged += n
		if reply.Err == nil || ctx.Err() != nil || attempt > 0 || !cfg.RetryTransient || !transient(ex) {
			break
		}
		time.Sleep(1500 * time.Millisecond)
	}
	apiframework.Settle(hold, charged, reply.Err != nil)
	// ONE outcome for the find, given by the caller once the answer has
	// been parsed and checked (report); a find given up on is handed back
	// unjudged. A transport failure here is already its outcome.
	report := func(err error) {
		if canceled(ctx) {
			apiframework.Release(apiframework.ConsumerBaubles, ticket)
			return
		}
		apiframework.Record(apiframework.ConsumerBaubles, ticket, err, time.Now())
	}
	return reply.Content, charged, report, reply.Err
}

// transient reports a failure worth one retry: a rate limit, a server
// error, or no answer at all.
func transient(ex apiframework.Exchange) bool {
	return ex.Status == 0 || ex.Status == http.StatusTooManyRequests || ex.Status >= 500
}

// moderate checks the name, keyword (NameSimple), description and material
// through the server's key (a player's key page reaches no moderation
// endpoint). The policy, decided and pinned by test (spec S3, ruling 15,
// owner ruling 2026-09-29):
//
//   - A flag always keeps the text out of the world: a corpus fallback.
//   - Server-key text: checked when ModerateOutput is on, and kept out when
//     the check cannot be made or fails; not checked when it is off.
//   - Player-key text: checked whenever the server can
//     (moderationPossible), and then a failed check keeps it out too.
//     When the server cannot check it, no call is made and it is kept to
//     its finder (finderOnly: everyone else reads the generic trinket).
//   - Every check made, on either route, is recorded on the moderation
//     breaker alone (apiframework.RecordConsumer), never the provider's
//     and never the naming breaker. A flag is the check working.
//
// The check is free and is not a model call, so it reserves nothing. The
// server settings, the clock and the breakers are read once here (blocked)
// and passed on; nothing below re-reads apiframework.Blocked, so a breaker
// that opens mid-decision cannot turn an already-decided player-key find
// into a refusal instead of finderOnly.
func (m *BaublesModule) moderate(cfg Config, reply baubles.Reply, playerKey bool) (moderated bool, finderOnly bool, err error) {
	now := time.Now()
	s := apiframework.Server()
	blocked := apiframework.Blocked(moderationBreaker, now)
	moderateReadForTest()
	possible := moderationPossible(cfg, s, blocked)
	if playerKey && !possible {
		return false, true, nil
	}
	if !cfg.ModerateOutput {
		return false, false, nil
	}
	if !s.HasKey() {
		return false, false, errNoRoute
	}
	if blocked {
		return false, false, errBreakerOpen
	}
	// Every field a player reads or types. CleanReply always leaves a
	// keyword (the model's, a word of the name, or "trinket"); a material
	// it found too long is empty and not sent.
	texts := []string{reply.Name, reply.NameSimple, reply.Description}
	if reply.Material != `` {
		texts = append(texts, reply.Material)
	}
	flags, err := apiframework.Moderate(s.Endpoint, cfg.ModerationModel, time.Duration(cfg.TimeoutSeconds)*time.Second,
		texts, apiframework.CarriesNoPlayerData, nil)
	// Enough failed checks in a row, on either route, open the moderation
	// breaker: later player-key finds are then kept to their finders, and
	// server-key finds refused, until it closes. A flag is the check working.
	apiframework.RecordConsumer(moderationBreaker, err, now)
	if err != nil {
		return false, false, fmt.Errorf(`moderation: %w`, err)
	}
	for _, f := range flags {
		if f {
			return false, false, errors.New(`moderation flagged the reply`)
		}
	}
	return true, false, nil
}

func errString(err error) string {
	if err == nil {
		return ``
	}
	return err.Error()
}
