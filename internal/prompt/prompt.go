package prompt

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

/*

// Usage

// At user input submission:

if cmdPrompt := prompt.Get(userId); cmdPrompt != nil {

	for {
		question := cmdPrompt.GetQuestion()

		if question == nil {
			break
		}

		question.Answer(inputString)
		// Display the current prompt, and wait for a response
		print( question.Question )

	}

}

*/

type Question struct {
	Question        string     // What's the prompt?
	Options         []string   // What options (if any) are available? None = freeform
	DefaultResponse string     // What is the default response?
	Response        string     // What was the response last submitted
	Done            bool       // Was it seen and responded to?
	Flags           int        // Mask reply etc
	Menu            *menu.Menu // set by AskMenu: the question is a menu (#375)
	// LastResult is what the last Answer meant to the menu, so the caller
	// can say why a line was refused.
	LastResult menu.ResultKind
}

type Prompt struct {
	Command   string         // Where does it call when complete?
	Rest      string         // What is the 'rest' of the command
	Questions []*Question    // All questions so far
	State     map[string]any // Optional place to store state
	// NonModal: a menu question that gets a line it does not recognise
	// closes the prompt and the line runs as a command, and game interrupts
	// close it. Player flows set it; builders stay modal (#375).
	NonModal bool
	// RoomBound: the prompt edits the room (or zone) the player stands in,
	// so a room change cancels it even though it is modal; otherwise the
	// next answer would act on the new room (#375).
	RoomBound bool
	// OpenedUnder is the interrupt rule that already held when the prompt
	// opened (menu.CloseNone when none did). A player may open a non-modal
	// prompt while sitting or fighting, so only a different rule (a new
	// interrupt) closes it; set by users.StartPrompt (#375).
	OpenedUnder menu.CloseReason
	// SetAsideLine, when set, is said to the player whenever the non-modal
	// prompt closes without being finished, so a draft never vanishes
	// silently ("You set your unsent mail aside.").
	SetAsideLine string
}

func New(command string, rest string) *Prompt {
	return &Prompt{
		Command:   command,
		Rest:      rest,
		Questions: make([]*Question, 0),
		State:     map[string]any{},
	}
}

// Returns the next pending question.
func (p *Prompt) Ask(question string, responseOptions []string, defaultOption ...string) *Question {

	if question == `` {
		question = `?`
	}

	qCt := len(p.Questions)
	for i := 0; i < qCt; i++ {
		if p.Questions[i].Question == question {
			return p.Questions[i]
		}
	}

	defOpt := ``
	if len(defaultOption) > 0 {
		defOpt = defaultOption[0]
	}

	finalResponseOptions := []string{}
	for _, opt := range responseOptions {
		if opt != `` {
			finalResponseOptions = append(finalResponseOptions, opt)
		}
	}

	q := &Question{
		Question:        question,
		Options:         finalResponseOptions,
		DefaultResponse: defOpt,
	}

	p.Questions = append(p.Questions, q)

	return q
}

// AskMenu asks a menu question. The menu's Heading identifies the question
// (as Ask's text does), and each call replaces the stored menu so a toggled
// marker shows on the next render. leaveResponse is the Response when the
// player picks the Leave entry.
func (p *Prompt) AskMenu(m menu.Menu, leaveResponse ...string) *Question {
	heading := m.Heading
	if heading == `` {
		heading = `?`
	}
	leave := ``
	if len(leaveResponse) > 0 {
		leave = leaveResponse[0]
	}
	for _, q := range p.Questions {
		if q.Question == heading {
			mm := m
			q.Menu = &mm
			q.DefaultResponse = leave
			return q
		}
	}
	mm := m
	q := &Question{Question: heading, Menu: &mm, DefaultResponse: leave}
	p.Questions = append(p.Questions, q)
	return q
}

// AskConfirm asks a two-entry yes/no menu; Response is "yes" or "no", and
// Enter picks no.
func (p *Prompt) AskConfirm(question, yesLabel, noLabel string) *Question {
	return p.AskMenu(menu.Confirm(question, yesLabel, noLabel))
}

// Resolve reports what a line would mean to this question's menu without
// answering it. A non-menu question reports menu.Text for any line.
func (q *Question) Resolve(answer string) menu.Result {
	if q.Menu == nil {
		return menu.Result{Kind: menu.Text, Text: strings.TrimSpace(answer)}
	}
	return menu.Resolve(*q.Menu, answer)
}

// Returns the next pending question.
func (p *Prompt) GetNextQuestion() *Question {

	qCt := len(p.Questions)
	for i := 0; i < qCt; i++ {
		if !p.Questions[i].Done {
			return p.Questions[len(p.Questions)-1]
		}
	}

	return nil
}

func (p *Prompt) Store(name string, val any) {
	p.State[name] = val
}

func (p *Prompt) Recall(name string) (any, bool) {
	if v, ok := p.State[name]; ok {
		return v, true
	}
	return nil, false
}

func (q *Question) Reset() {
	q.Done = false
}

/*
If fewer than 2 choices provided, answer is what they enter.
If they provide an empty string, and a default respnose is provided,
answer is default.
*/
func (q *Question) Answer(answer string) {
	if q.Menu != nil {
		res := menu.Resolve(*q.Menu, answer)
		q.LastResult = res.Kind
		switch res.Kind {
		case menu.Chosen:
			q.Response, q.Done = res.Entry.Id, true
		case menu.Leave:
			q.Response, q.Done = q.DefaultResponse, true
		case menu.Text:
			q.Response, q.Done = res.Text, true
		}
		return
	}

	// If an empty string, failover to default (if any)
	// Otherwise, just abort and wait for a valid response
	answer = strings.TrimSpace(answer)
	if len(answer) == 0 {
		if q.DefaultResponse == `` {
			return
		}
		q.Response = q.DefaultResponse
		q.Done = true
	}

	// If options were provided, find best match if any
	optLen := len(q.Options)
	if optLen > 1 {

		closestMatchIdx := -1
		closestMatchLen := 0

		testAnswer := strings.ToLower(answer)

		for i := 0; i < optLen; i++ {
			optTest := strings.ToLower(q.Options[i])

			if optTest == testAnswer {
				closestMatchIdx = i
				break
			}

			longestPossible := len(optTest)
			if len(testAnswer) < longestPossible {
				longestPossible = len(testAnswer)
			}
			for j := 0; j < longestPossible; j++ {
				if optTest[j] != testAnswer[j] {
					break
				}

				if j+1 > closestMatchLen {
					closestMatchLen = j + 1
					closestMatchIdx = i
				}
			}
		}

		if closestMatchIdx == -1 {
			return
		}

		answer = q.Options[closestMatchIdx]
	}

	if len(answer) > 0 {
		q.Response = answer
	}
	q.Done = true
}

func (q *Question) RejectResponse() {

	q.Response = `` // Clear the response
	q.Done = false  // Mark as not done
}

func (q *Question) String() string {
	if q.Menu != nil {
		// The menu itself was drawn on screen; the prompt line stays bare.
		return `<ansi fg="black-bold">.:</ansi> `
	}

	ret := strings.Builder{}
	ret.WriteString(`<ansi fg="black-bold">.:</ansi> `) // Prompt prefix
	ret.WriteString(`<ansi fg="yellow-bold">`)
	ret.WriteString(q.Question) // Actual question
	ret.WriteString(`</ansi>`)

	optLen := len(q.Options)
	if optLen > 0 {

		ret.WriteString(` <ansi fg="black-bold">[</ansi>`)
		for i := 0; i < optLen; i++ {

			if q.DefaultResponse != `` {

				if q.Options[i] == q.DefaultResponse {
					ret.WriteString(`<ansi fg="white">`)
				} else {
					ret.WriteString(`<ansi fg="black-bold">`)
				}

				ret.WriteString(q.Options[i])

				ret.WriteString(`</ansi>`)

			} else {
				ret.WriteString(q.Options[i])
			}

			if i < optLen-1 {
				ret.WriteString(`<ansi fg="black-bold">/</ansi>`)
			}
		}
		ret.WriteString(`<ansi fg="black-bold">]</ansi>`)
	}
	ret.WriteString(` `)
	return ret.String()
}
