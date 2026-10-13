package items

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

const roleFirstDoc = `
optionid: test
options:
  weak:
    together:
      actor:
        beginner: ['A0', 'A1']
        expert: ['AE']
        master: ['A2']
      actee:
        beginner: ['D0', 'D1']
        expert: ['DE']
        master: ['D2']
      observer:
        beginner: ['R0', 'R1']
        expert: ['RE']
        master: ['R2']
    separate:
      actor: {beginner: ['SA0']}
      actee: {beginner: ['SD0']}
      observer: {beginner: ['SR0']}
      remote_observer: {beginner: ['SX0']}
`

const recordDoc = `
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A0', actee: 'D0', observer: 'R0'}
      - {actor: 'A1', actee: 'D1', observer: 'R1'}
      expert:
      - {actor: 'AE', actee: 'DE', observer: 'RE'}
      master:
      - {actor: 'A2', actee: 'D2', observer: 'R2'}
    separate:
      beginner:
      - {actor: 'SA0', actee: 'SD0', observer: 'SR0', remote_observer: 'SX0'}
`

func decodeGroup(t *testing.T, doc string) (WeaponAttackMessageGroup, error) {
	t.Helper()
	var g WeaponAttackMessageGroup
	err := yaml.Unmarshal([]byte(doc), &g)
	return g, err
}

func TestRecordLayoutDecodesToTheSamePoolsAsRoleFirst(t *testing.T) {
	a, err := decodeGroup(t, roleFirstDoc)
	if err != nil {
		t.Fatalf("role-first: %v", err)
	}
	b, err := decodeGroup(t, recordDoc)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	for _, lvl := range []int{0, 34, 67} {
		at, bt := a.Options[Weak].Together, b.Options[Weak].Together
		for name, pair := range map[string][2][]string{
			"actor":    {at.ToAttacker.PoolFor(lvl), bt.ToAttacker.PoolFor(lvl)},
			"actee":    {at.ToDefender.PoolFor(lvl), bt.ToDefender.PoolFor(lvl)},
			"observer": {at.ToRoom.PoolFor(lvl), bt.ToRoom.PoolFor(lvl)},
		} {
			if strings.Join(pair[0], "|") != strings.Join(pair[1], "|") {
				t.Errorf("level %d %s: role-first %v, records %v", lvl, name, pair[0], pair[1])
			}
		}
		as, bs := a.Options[Weak].Separate, b.Options[Weak].Separate
		for name, pair := range map[string][2][]string{
			"sep actor":           {as.ToAttacker.PoolFor(lvl), bs.ToAttacker.PoolFor(lvl)},
			"sep actee":           {as.ToDefender.PoolFor(lvl), bs.ToDefender.PoolFor(lvl)},
			"sep observer":        {as.ToAttackerRoom.PoolFor(lvl), bs.ToAttackerRoom.PoolFor(lvl)},
			"sep remote_observer": {as.ToDefenderRoom.PoolFor(lvl), bs.ToDefenderRoom.PoolFor(lvl)},
		} {
			if len(pair[0]) == 0 || strings.Join(pair[0], "|") != strings.Join(pair[1], "|") {
				t.Errorf("level %d %s: role-first %v, records %v", lvl, name, pair[0], pair[1])
			}
		}
	}
}

func TestRecordLayoutRejectsBadRecords(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"missing role": {`
optionid: test
options:
  weak:
    together:
      master:
      - {actor: 'A', actee: 'D'}
`, "option[`weak`]: together: master record 0: missing role \"observer\""},
		"empty line": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A', actee: '  ', observer: 'R'}
`, "option[`weak`]: together: beginner record 0: empty \"actee\" line"},
		"unknown key": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A', acter: 'D', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: beginner record 0: unknown key \"acter\""},
		"separate needs remote_observer": {`
optionid: test
options:
  weak:
    separate:
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: separate: beginner record 0: missing role \"remote_observer\""},
		"mixed layout": {`
optionid: test
options:
  weak:
    together:
      actor: {beginner: ['A']}
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: mixes role keys and tier keys"},
		"unknown tier": {`
optionid: test
options:
  weak:
    together:
      grandmaster:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: unknown key \"grandmaster\""},
		"nil record": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - ~
`, "option[`weak`]: together: beginner record 0: missing role \"actor\""},
		"legacy remote_observer under together": {`
optionid: test
options:
  weak:
    together:
      actor: {beginner: ['A']}
      remote_observer: {beginner: ['X']}
`, "option[`weak`]: together: unknown key \"remote_observer\""},
		"first unknown key in sorted order": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R', zzz: 'x', aaa: 'y'}
`, "option[`weak`]: together: beginner record 0: unknown key \"aaa\""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeGroup(t, c.doc)
			if err == nil {
				t.Fatalf("decoded without error, want %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}

func TestStrictDecodingStillSeesBelowOptions(t *testing.T) {
	cases := map[string]string{
		"typo split key": `
optionid: test
options:
  weak:
    togther:
      actor: {beginner: ['A']}
`,
		"typo tier key": `
optionid: test
options:
  weak:
    together:
      actor: {begginer: ['A']}
      actee: {beginner: ['D']}
      observer: {beginner: ['R']}
`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			var g WeaponAttackMessageGroup
			if err := yaml.UnmarshalStrict([]byte(doc), &g); err == nil {
				t.Fatal("strict decode accepted a typo'd key")
			}
		})
	}
}

func TestUnquotedScalarsStayExact(t *testing.T) {
	roleFirst := `
optionid: test
options:
  weak:
    together:
      actor: {beginner: [yes, 0x1F]}
      actee: {beginner: [yes, 0x1F]}
      observer: {beginner: [yes, 0x1F]}
`
	records := `
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: yes, actee: yes, observer: yes}
      - {actor: 0x1F, actee: 0x1F, observer: 0x1F}
`
	for name, doc := range map[string]string{"role-first": roleFirst, "records": records} {
		t.Run(name, func(t *testing.T) {
			g, err := decodeGroup(t, doc)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(g.Options[Weak].Together.ToAttacker.PoolFor(0), "|")
			if got != "yes|0x1F" {
				t.Errorf("got %q, want %q", got, "yes|0x1F")
			}
		})
	}
}
