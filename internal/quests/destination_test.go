package quests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type destHolder struct {
	Destination StepDestination `yaml:"destination,omitempty"`
}

func decodeDest(t *testing.T, src string) (StepDestination, error) {
	t.Helper()
	var h destHolder
	err := yaml.Unmarshal([]byte(src), &h)
	return h.Destination, err
}

func TestStepDestination_Decodes(t *testing.T) {
	cases := []struct {
		src  string
		want StepDestination
	}{
		{"destination: anywhere\n", StepDestination{Anywhere: true}},
		{"destination:\n  room: 473\n", StepDestination{Room: 473}},
		{"destination:\n  area: unquiet_grove\n", StepDestination{Area: "unquiet_grove"}},
		{"destination:\n  zone: Pothole Coulee\n", StepDestination{Zone: "Pothole Coulee"}},
		{"destination:\n  mob: 94\n  room: 473\n", StepDestination{Mob: 94, Room: 473}},
		{"destination:\n  mob: 94\n", StepDestination{Mob: 94}},
		{"other: 1\n", StepDestination{}},
	}
	for _, c := range cases {
		got, err := decodeDest(t, c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %+v, want %+v", c.src, got, c.want)
		}
	}
}

func TestStepDestination_RefusesBadInput(t *testing.T) {
	for _, src := range []string{
		"destination: somewhere\n",
		"destination: 473\n",
		"destination:\n  roam: 473\n",
		"destination:\n  anywhere: true\n",
	} {
		if _, err := decodeDest(t, src); err == nil {
			t.Errorf("%q decoded; want a refusal", src)
		}
	}
}

func TestStepDestination_ValidateShapes(t *testing.T) {
	good := []StepDestination{
		{}, {Anywhere: true}, {Room: 1}, {Area: "a"}, {Zone: "z"},
		{Mob: 2}, {Mob: 2, Room: 1}, {Mob: 2, Area: "a"}, {Mob: 2, Zone: "z"},
	}
	for _, d := range good {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	bad := []StepDestination{
		{Room: 1, Area: "a"}, {Area: "a", Zone: "z"}, {Mob: 2, Room: 1, Zone: "z"},
		{Anywhere: true, Room: 1}, {Anywhere: true, Mob: 2}, {Room: -1}, {Mob: -2},
	}
	for _, d := range bad {
		if err := d.Validate(); err == nil {
			t.Errorf("%+v validated; want a refusal", d)
		}
	}
}

func TestStepDestination_MarshalRoundTrips(t *testing.T) {
	for _, d := range []StepDestination{{Anywhere: true}, {Room: 473}, {Mob: 94, Room: 473}, {Area: "g"}} {
		out, err := yaml.Marshal(destHolder{Destination: d})
		if err != nil {
			t.Fatal(err)
		}
		back, err := decodeDest(t, string(out))
		if err != nil || back != d {
			t.Errorf("%+v -> %q -> %+v (%v)", d, out, back, err)
		}
	}
	out, _ := yaml.Marshal(destHolder{})
	if strings.Contains(string(out), "destination") {
		t.Errorf("an empty destination must be omitted, got %q", out)
	}
	out, _ = yaml.Marshal(destHolder{Destination: StepDestination{Anywhere: true}})
	if string(out) != "destination: anywhere\n" {
		t.Errorf("anywhere must write as the scalar, got %q", out)
	}
}

func TestQuestValidate_RefusesABadDestination(t *testing.T) {
	q := Quest{QuestId: 1, Name: "x", Steps: []QuestStep{
		{Id: "start", Destination: StepDestination{Room: 1, Area: "a"}}, {Id: "end"},
	}}
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("want a destination refusal, got %v", err)
	}
}
