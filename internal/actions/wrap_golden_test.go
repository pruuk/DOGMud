package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Web client base, plan 1. Speech, Shout and Emote stopped pre-wrapping at 80
// and joined the pipeline, so a telnet reader at 80 columns now gets WrapAnsi's
// wrap where it got util.SplitStringNL's. The tests below compare what the
// reader sees, tags stripped and CR normalised, because the raw bytes differ
// on purpose: WrapAnsi closes and reopens color tags at each break.

var wrapGoldenTag = regexp.MustCompile(`</?ansi[^>]*>`)

// wrapGoldenVisible is the lines a reader sees.
func wrapGoldenVisible(s string) []string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.Split(wrapGoldenTag.ReplaceAllString(s, ""), "\n")
}

// preSweepLines is the wrap every sender applied before the sweep, pinned
// here as the reference.
func preSweepLines(line string) []string {
	return wrapGoldenVisible(util.SplitStringNL(line, 80))
}

// pipelineLines is the line as a telnet reader at 80 columns gets it now.
func pipelineLines(cat messaging.Category, channel messaging.Channel, line string) []string {
	return wrapGoldenVisible(messaging.RenderForRecipient(messaging.RenderInput{
		Category:      cat,
		Text:          line,
		Channel:       channel,
		SightDecision: messaging.SightFull,
		LineWidth:     80,
	}))
}

func wrapGoldenSay(text string) string {
	return FormatSayText("Kesh", text, "username", "saytext")
}

func wrapGoldenShout(text string) string {
	return formatShoutText("Kesh", text, "username", "yellow")
}

func wrapGoldenEmote(text string) string {
	return FormatEmoteText("Kesh", text, "username")
}

// wrapGoldenCases are lines a player types or a mob speaks, one to three
// lines long, whose breaks land away from the boundary cases listed in
// TestWrapGolden_AcceptedDifferences.
var wrapGoldenCases = []struct {
	name    string
	cat     messaging.Category
	channel messaging.Channel
	line    string
}{
	{"say short", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`Hello there.`)},
	{"say two lines", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`The pool answers first, it is older than the town and more patient than I am, so look into the water and wait.`)},
	{"say three lines", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`Old Fenn keeps the sluice, and Ayres holds the farm west of it, and neither has spoken to the other since the spring the river ran low and the barley failed on both banks of the water.`)},
	{"say player chatter", messaging.CategorySpeech, messaging.ChannelAudio, wrapGoldenSay(`anyone want to group up for the ridge? i have a healer alt and about an hour before i need to log off for dinner`)},
	{"shout short", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`HELP ME`)},
	{"shout two lines", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`THE BRIDGE IS OUT AND THE RIVER IS RISING AND THE WHOLE EAST ROAD WILL BE UNDER WATER BY MORNING`)},
	{"shout three lines", messaging.CategoryShout, messaging.ChannelAudio, wrapGoldenShout(`TO ARMS TO ARMS THE RAIDERS HAVE COME OVER THE NORTH WALL AND THEY ARE IN THE MARKET AND THE GATE CAPTAIN IS DOWN AND SOMEONE MUST RING THE BELL AT ONCE`)},
	{"emote short", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`waves.`)},
	{"emote two lines", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`rolls a trinket across his knuckles and murmurs that he buys what others will not sell to anyone.`)},
	{"emote three lines", messaging.CategoryEmote, messaging.ChannelVisual, wrapGoldenEmote(`turns slowly to examine the ceiling with apparent fascination, one hand hanging loose at his side, and hums something that was almost a song before it lost its way somewhere around the second verse.`)},
}

func TestWrapGolden_VisibleLinesMatchTheOldWrapAt80(t *testing.T) {
	for _, tc := range wrapGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, preSweepLines(tc.line), pipelineLines(tc.cat, tc.channel, tc.line))
		})
	}
}

// Measured over 10,925 shipped prose lines on 2026-11-09, 745 broke
// differently: never a lost word, always the last word or two of a line. Each
// shape is pinned here on both sides, so a change to either wrapper shows up.
func TestWrapGolden_AcceptedDifferences(t *testing.T) {
	long := strings.Repeat("a", 90)
	for _, tc := range []struct {
		name    string
		line    string
		oldWrap []string
		newWrap []string
	}{
		{
			"a word over 80 no longer jumps ahead of its own sentence",
			wrapGoldenSay(long + ` then more words`),
			[]string{long, `Kesh says, " then more words"`},
			[]string{`Kesh says,`, `"` + long, `then more words"`},
		},
		{
			"a hyphenated word moves whole instead of breaking at the hyphen",
			wrapGoldenSay(`You have heard Fenn's side. Ayres holds the farm west of the head-gate, at the ford.`),
			[]string{`Kesh says, "You have heard Fenn's side. Ayres holds the farm west of the head-`, `gate, at the ford."`},
			[]string{`Kesh says, "You have heard Fenn's side. Ayres holds the farm west of the`, `head-gate, at the ford."`},
		},
		{
			"punctuation that ends exactly at column 80 stays on the line",
			wrapGoldenSay(`I have kept the Bough nineteen years. It is the heart of the valley, if I may say so.`),
			[]string{`Kesh says, "I have kept the Bough nineteen years. It is the heart of the`, `valley, if I may say so."`},
			[]string{`Kesh says, "I have kept the Bough nineteen years. It is the heart of the valley,`, `if I may say so."`},
		},
		{
			"a closing quote is no longer stranded on a line of its own",
			wrapGoldenSay(`A tin thimble, pressed a little out of shape. The dimples on its top`),
			[]string{`Kesh says, "A tin thimble, pressed a little out of shape. The dimples on its top`, `"`},
			[]string{`Kesh says, "A tin thimble, pressed a little out of shape. The dimples on its`, `top"`},
		},
		{
			"a run of spaces collapses to one",
			wrapGoldenSay(`hello    there`),
			[]string{`Kesh says, "hello    there"`},
			[]string{`Kesh says, "hello there"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.oldWrap, preSweepLines(tc.line), "the old wrap's behaviour, pinned")
			require.Equal(t, tc.newWrap, pipelineLines(messaging.CategorySpeech, messaging.ChannelAudio, tc.line), "the pipeline's wrap")
		})
	}
}
