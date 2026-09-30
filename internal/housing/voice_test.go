package housing

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The defaults are the lines Hobb has always said: moving them into the
// voice table must not change a word of what New Plymouth hears.
func TestVoice_DefaultsSayWhatHobbAlwaysSaid(t *testing.T) {
	b := testBuilding()
	cases := map[string]string{
		b.Line(`terms.pitch`, `Tier`, `A simple room`, `tier`, `a simple room`, `price`, 500): `A simple room, 500 gold, paid once. Four walls, a window, and a door that opens for you and nobody else. Extensions and redecorating come after, for lodgers. It's all on the list.`,
		b.Line(`home.not_vouched`): `The Widow only lets to people the Common Quarter will vouch for, and nobody's vouched for you. Do some good round the Common Quarter and come back. I'll still be here. I'm always here.`,
		b.Line(`terms.extension`, `price`, 1500, `voucher`, 500): `If you want it bigger, an extension deed is 1500 gold for you. Goes up every time, that's the Widow's rule. A redecorating voucher is 500. Type list.`,
		b.Line(`home.welcome`): `Stamped. Welcome to the test lodgings. Try not to set anything on fire.`,
		b.Line(`terms.owner`):  `You've got a room already. Door's behind me, hand on the plate, same as always.`,
		b.Line(`terms.ready`):  `The Common Quarter speaks well enough of you. Type buy home and I'll get the stamp out.`,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	}
}

func TestVoice_OverrideIsSpoken(t *testing.T) {
	b := testBuilding()
	b.Voice = map[string]string{`home.welcome`: `In the book. Welcome to {name}. Go away.`}
	if got := b.Line(`home.welcome`); got != `In the book. Welcome to the test lodgings. Go away.` {
		t.Errorf("got %q", got)
	}
	if got := b.Line(`terms.full`); got != defaultLines[`terms.full`] {
		t.Errorf("a line with no override changed: %q", got)
	}
}

func TestVoice_Validation(t *testing.T) {
	for name, voice := range map[string]map[string]string{
		`unknown key`:         {`home.welcom`: `Hi.`},
		`unknown placeholder`: {`home.welcome`: `Welcome, {price}.`},
		`semicolon`:           {`home.welcome`: `Done; go away.`},
		`empty`:               {`home.welcome`: `  `},
	} {
		b := testBuilding()
		b.Voice = voice
		if err := b.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	b := testBuilding()
	b.Voice = map[string]string{`deed.sold`: `{price} gold. {Proprietor} thanks you.`}
	if err := b.Validate(); err != nil {
		t.Errorf("a good voice was rejected: %v", err)
	}
}

// Every default line uses only placeholders its key provides, and every
// key is actually said somewhere in the code.
func TestVoice_EveryLineIsConsistentAndUsed(t *testing.T) {
	b := testBuilding()
	b.Voice = map[string]string{}
	for key, text := range defaultLines {
		b.Voice[key] = text
	}
	if err := b.validateVoice(); err != nil {
		t.Fatalf("a default line fails its own rules: %v", err)
	}

	src := ``
	for _, pattern := range []string{`*.go`, `../behaviortree/*.go`} {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			if strings.HasSuffix(f, `_test.go`) || strings.HasSuffix(f, `voice.go`) {
				continue
			}
			data, _ := os.ReadFile(f)
			src += string(data)
		}
	}
	for key := range defaultLines {
		if !regexp.MustCompile("Line\\(`" + regexp.QuoteMeta(key) + "`").MatchString(src) {
			t.Errorf("voice line %q is never said", key)
		}
	}
}
