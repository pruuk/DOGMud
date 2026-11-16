package apiframework

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// The OpenAI chat completions wire format, built and read in one place for
// every feature and every route (the server's key over HTTP, or a player's
// own key through their browser, which posts the same body).

// Message is one message in a chat completions request. An assistant
// message may carry tool calls instead of content; a tool message answers
// one of them by id.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallId string
}

// wireMessage is Message as the API expects it: content is null on an
// assistant message that only calls tools.
type wireMessage struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string     `json:"tool_call_id,omitempty"`
}

func toWire(msgs []Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		w := wireMessage{Role: m.Role, ToolCalls: m.ToolCalls, ToolCallId: m.ToolCallId}
		if !(m.Role == `assistant` && len(m.ToolCalls) > 0 && m.Content == ``) {
			content := m.Content
			w.Content = &content
		}
		out[i] = w
	}
	return out
}

// ToolCall is a function call the model asks for.
type ToolCall struct {
	Id       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function a ToolCall names, with its JSON arguments.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSpec offers the model one read-only question it may ask the game.
type ToolSpec struct {
	Type     string  `json:"type"`
	Function ToolDef `json:"function"`
}

// ToolDef describes one tool: its name, what it is for and its parameters.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict"`
}

type jsonSchemaFormat struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type responseFormat struct {
	Type       string           `json:"type"`
	JSONSchema jsonSchemaFormat `json:"json_schema"`
}

type chatBody struct {
	Model               string         `json:"model"`
	Messages            []wireMessage  `json:"messages"`
	Tools               []ToolSpec     `json:"tools,omitempty"`
	ToolChoice          string         `json:"tool_choice,omitempty"`
	ResponseFormat      responseFormat `json:"response_format"`
	MaxCompletionTokens int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64       `json:"temperature,omitempty"`
	ReasoningEffort     string         `json:"reasoning_effort,omitempty"`
}

// Chat is one chat completions request under a strict JSON schema.
type Chat struct {
	Model       string
	Messages    []Message
	Tools       []ToolSpec
	ToolChoice  string // auto, none; empty = not sent
	SchemaName  string
	Schema      map[string]any
	MaxTokens   int     // max_completion_tokens; 0 = not sent
	Temperature float64 // 0 = not sent (required for some reasoning models)
	Effort      string  // reasoning effort; empty = not sent
}

// Body is the request body for c.
func (c Chat) Body() ([]byte, error) {
	b := chatBody{
		Model:      c.Model,
		Messages:   toWire(c.Messages),
		Tools:      c.Tools,
		ToolChoice: c.ToolChoice,
		ResponseFormat: responseFormat{
			Type:       `json_schema`,
			JSONSchema: jsonSchemaFormat{Name: c.SchemaName, Strict: true, Schema: c.Schema},
		},
		MaxCompletionTokens: c.MaxTokens,
		ReasoningEffort:     c.Effort,
	}
	if c.Temperature > 0 {
		t := c.Temperature
		b.Temperature = &t
	}
	return json.Marshal(b)
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string     `json:"content"`
			Refusal   string     `json:"refusal"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// MaxToolCallsPerReply is the most questions one reply may put to the game.
// Each is answered under the mud lock and sent back in the next request, so
// a reply asking dozens at once would hold the game and swell the next
// prompt. The first few are answered; the rest were never asked.
const MaxToolCallsPerReply = 4

// Reply is a decoded chat completions answer.
type Reply struct {
	Content   string     // the JSON content, for the caller to parse against its schema
	Tokens    int        // total tokens the provider reported
	ToolCalls []ToolCall // the model's questions, when it asked instead of answering
	Err       error
}

// keyTextRE matches an OpenAI key in provider text: sk-..., sk-proj-...,
// and the masked sk-proj-****abcd form, from the key's start to the next
// space. \b keeps it off words that merely end in "sk" (task-, risk-).
var keyTextRE = regexp.MustCompile(`\bsk-\S*`)

// DecodeChat reads a provider's chat completions reply, whichever way it
// came back: over HTTP or through a player's browser.
func DecodeChat(status int, raw []byte) Reply {
	res := Reply{}
	if status != http.StatusOK {
		// Scrub any key BEFORE the cut, so a key the cut would split
		// leaves no fragment behind.
		snippet := strings.TrimSpace(keyTextRE.ReplaceAllString(string(raw), `sk-[redacted]`))
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		res.Err = &StatusError{Status: status, Detail: snippet}
		return res
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		res.Err = fmt.Errorf(`decode response: %w`, err)
		return res
	}
	res.Tokens = cr.Usage.TotalTokens

	if len(cr.Choices) == 0 {
		res.Err = errors.New(`model returned no choices`)
		return res
	}
	ch := cr.Choices[0]
	if ch.Message.Refusal != `` {
		res.Err = errors.New(`model refused`)
		return res
	}
	if len(ch.Message.ToolCalls) > 0 {
		res.ToolCalls = ch.Message.ToolCalls
		if len(res.ToolCalls) > MaxToolCallsPerReply {
			res.ToolCalls = res.ToolCalls[:MaxToolCallsPerReply]
		}
		return res
	}
	if ch.FinishReason == `length` {
		res.Err = errors.New(`model output truncated (raise MaxCompletionTokens)`)
		return res
	}
	if strings.TrimSpace(ch.Message.Content) == `` {
		res.Err = errors.New(`model returned empty content`)
		return res
	}
	res.Content = ch.Message.Content
	return res
}

// EstimateTokens is a rough count of a request's prompt, about four
// characters to the token. Bytes, not runes: a multi-byte character is more
// tokens, not fewer, so counting bytes errs towards over-reserving.
func EstimateTokens(msgs []Message) int {
	n := 0
	for _, msg := range msgs {
		n += len(msg.Content)/4 + 8
		for _, tc := range msg.ToolCalls {
			n += (len(tc.Function.Name) + len(tc.Function.Arguments)) / 4
		}
	}
	return n
}

// Charged is what one request is charged against a budget. reported is the
// provider's usage. A request that left (sent) but came back with no usage
// (a timeout, a dropped connection, a call given up on after it was sent)
// is charged at its prompt estimate plus maxTokens, since the provider may
// have billed a whole answer; estimated says so. A count relayed through a
// player's browser, which that player can write, is held between nothing
// and the most one request could cost.
func Charged(reported int, sent bool, status int, prompt int, maxTokens int, relayed bool) (tokens int, estimated bool) {
	tokens = max(reported, 0)
	if relayed && tokens > prompt+maxTokens {
		tokens = prompt + maxTokens
	}
	if sent && tokens == 0 && (status == 0 || status == http.StatusOK) {
		return prompt + maxTokens, true
	}
	return tokens, false
}
