package quests

import (
	"reflect"
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
		{"destination:\n  mob: 94\n  room: 473\n", StepDestination{Mobs: []int{94}, Room: 473}},
		{"destination:\n  mob: 94\n", StepDestination{Mobs: []int{94}}},
		{"other: 1\n", StepDestination{}},
	}
	for _, c := range cases {
		got, err := decodeDest(t, c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
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
		{Mobs: []int{2}}, {Mobs: []int{2}, Room: 1}, {Mobs: []int{2}, Area: "a"}, {Mobs: []int{2}, Zone: "z"},
	}
	for _, d := range good {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	bad := []StepDestination{
		{Room: 1, Area: "a"}, {Area: "a", Zone: "z"}, {Mobs: []int{2}, Room: 1, Zone: "z"},
		{Anywhere: true, Room: 1}, {Anywhere: true, Mobs: []int{2}}, {Room: -1}, {Mobs: []int{-2}},
	}
	for _, d := range bad {
		if err := d.Validate(); err == nil {
			t.Errorf("%+v validated; want a refusal", d)
		}
	}
}

func TestStepDestination_MarshalRoundTrips(t *testing.T) {
	for _, d := range []StepDestination{{Anywhere: true}, {Room: 473}, {Mobs: []int{94}, Room: 473}, {Area: "g"}} {
		out, err := yaml.Marshal(destHolder{Destination: d})
		if err != nil {
			t.Fatal(err)
		}
		back, err := decodeDest(t, string(out))
		if err != nil || !reflect.DeepEqual(back, d) {
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

// Owner ruling 16: mob takes one id or a list of ids, any of which will do.
func TestStepDestination_MobList(t *testing.T) {
	got, err := decodeDest(t, "destination:\n  mob: [9316, 9300]\n")
	if err != nil || !reflect.DeepEqual(got, StepDestination{Mobs: []int{9316, 9300}}) {
		t.Errorf("a list of mobs: %+v (%v)", got, err)
	}
	got, err = decodeDest(t, "destination:\n  mob: 94\n")
	if err != nil || !reflect.DeepEqual(got, StepDestination{Mobs: []int{94}}) {
		t.Errorf("a single mob: %+v (%v)", got, err)
	}
	for _, src := range []string{"destination:\n  mob: []\n", "destination:\n  mob: [-1]\n", "destination:\n  mob: [9316, -1]\n"} {
		d, err := decodeDest(t, src)
		if err == nil {
			err = d.Validate()
		}
		if err == nil {
			t.Errorf("%q decoded and validated as %+v; want a refusal", src, d)
		}
	}
	for _, c := range []struct {
		d    StepDestination
		want string
	}{
		{StepDestination{Mobs: []int{94}}, "destination:\n  mob: 94\n"},
		{StepDestination{Mobs: []int{9316, 9300}, Room: 7}, "destination:\n  room: 7\n  mob:\n  - 9316\n  - 9300\n"},
	} {
		out, err := yaml.Marshal(destHolder{Destination: c.d})
		if err != nil || string(out) != c.want {
			t.Errorf("%+v wrote %q (%v), want %q", c.d, out, err, c.want)
			continue
		}
		back, err := decodeDest(t, string(out))
		if err != nil || !reflect.DeepEqual(back, c.d) {
			t.Errorf("%+v -> %q -> %+v (%v)", c.d, out, back, err)
		}
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
