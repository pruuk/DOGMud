package crafting

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func validPools() SkillPools {
	return SkillPools{
		StartObserver:   []string{"{actor} begins {item}.", "{actor} starts {item}.", "{actor} sets to {item}."},
		SuccessObserver: []string{"{actor} finishes {item}.", "{actor} completes {item}.", "{actor} is done with {item}."},
		FailureObserver: []string{"{actor} ruins it.", "{actor} spoils it.", "{actor} gives up."},
		CancelObserver:  []string{"{actor} stops.", "{actor} sets it down.", "{actor} gives up for now."},
		StartHeard:      []string{"You hear work begin.", "You hear tools.", "You hear a clatter."},
		FinishHeard:     []string{"The work stops.", "The tools fall quiet.", "The clatter ends."},
	}
}

func TestCraftingMessagesValidate_AcceptsAWellFormedStore(t *testing.T) {
	cm := CraftingMessages{"cooking": validPools()}
	if err := cm.Validate([]string{"cooking"}); err != nil {
		t.Fatalf("valid store refused: %v", err)
	}
}

func TestCraftingMessagesValidate_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(cm CraftingMessages)
		skills []string
		want   string
	}{
		{"missing skill block", func(cm CraftingMessages) {}, []string{"cooking", "tailoring"}, `no block for skill "tailoring"`},
		{"unknown skill block", func(cm CraftingMessages) { cm["cookery"] = validPools() }, []string{"cooking"}, `skill "cookery" has no recipes`},
		{"shallow pool", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.StartHeard = p.StartHeard[:2]
			cm["cooking"] = p
		}, []string{"cooking"}, "start_heard"},
		{"seen line without actor", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.SuccessObserver = []string{"A stew is done.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "success_observer must name the crafter with {actor}"},
		{"item in a failure line", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.FailureObserver = []string{"{actor} burns {item}.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "failure_observer may not use {item}"},
		{"cancel line without actor", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.CancelObserver = []string{"The work stops.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "cancel_observer must name the crafter with {actor}"},
		{"item in a cancel line", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.CancelObserver = []string{"{actor} drops {item}.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "cancel_observer may not use {item}"},
		{"token in a heard line", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.FinishHeard = []string{"{actor} stops.", "b.", "c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "finish_heard may not use {actor}"},
		{"unknown token", func(cm CraftingMessages) {
			p := cm["cooking"]
			p.StartObserver = []string{"{actor} begins {itme}.", "{actor} b.", "{actor} c."}
			cm["cooking"] = p
		}, []string{"cooking"}, "start_observer may not use {itme}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cm := CraftingMessages{"cooking": validPools()}
			tc.mutate(cm)
			err := cm.Validate(tc.skills)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestCraftRoomLines_FullNamesTheItemVagueSaysSomething(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}

	full, vague := CraftRoomLines("cooking", PhaseStart, ctx, "Stew", narration.FirstPicker)
	if full != "Kael begins Stew." || vague != "Kael begins something." {
		t.Fatalf("got full %q vague %q", full, vague)
	}
	full, vague = CraftRoomLines("cooking", PhaseSuccess, ctx, "", narration.FirstPicker)
	if full != "Kael finishes something." || vague != "Kael finishes something." {
		t.Fatalf("no item: got full %q vague %q", full, vague)
	}
	full, _ = CraftRoomLines("cooking", PhaseFailure, ctx, "Stew", narration.FirstPicker)
	if full != "Kael ruins it." {
		t.Fatalf("failure: got %q", full)
	}
}

// One index for both renders: the vague line must be the same variant as the
// full one, or a shapes reader and a full reader would describe different
// moments.
func TestCraftRoomLines_FullAndVagueShareOneVariant(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}
	pick := narration.SequencePicker()
	for i := 0; i < 3; i++ {
		full, vague := CraftRoomLines("cooking", PhaseStart, ctx, "Stew", pick)
		if strings.Replace(full, "Stew", "something", 1) != vague {
			t.Fatalf("round %d: full %q and vague %q are different variants", i, full, vague)
		}
	}
}

func TestCraftLines_CancelPhase(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	ctx := textutil.TokenContext{ActorName: "Kael", ActorPlainName: "Kael"}
	full, vague := CraftRoomLines("cooking", PhaseCancel, ctx, "Stew", narration.FirstPicker)
	if full != "Kael stops." || vague != "Kael stops." {
		t.Fatalf("got full %q vague %q", full, vague)
	}
	// A cancel sounds like a finish, so a sound never says why the work ended.
	if got := CraftHeardLine("cooking", PhaseCancel, narration.FirstPicker); got != "The work stops." {
		t.Fatalf("heard: %q, want the finish pool", got)
	}
}

func TestCraftHeardLine_StartAndFinishPools(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()}))
	if got := CraftHeardLine("cooking", PhaseStart, narration.FirstPicker); got != "You hear work begin." {
		t.Fatalf("start: %q", got)
	}
	for _, p := range []Phase{PhaseSuccess, PhaseFailure} {
		if got := CraftHeardLine("cooking", p, narration.FirstPicker); got != "The work stops." {
			t.Fatalf("phase %d: %q, want the finish pool for success and failure alike", p, got)
		}
	}
}

func TestCraftLines_UnknownSkillOrUnloadedStoreRendersNothing(t *testing.T) {
	t.Cleanup(SeedCraftingMessagesForTest(nil))
	ctx := textutil.TokenContext{ActorName: "Kael"}
	if f, v := CraftRoomLines("cooking", PhaseStart, ctx, "Stew"); f != "" || v != "" {
		t.Fatalf("unloaded store rendered %q / %q", f, v)
	}
	if h := CraftHeardLine("cooking", PhaseStart); h != "" {
		t.Fatalf("unloaded store rendered %q", h)
	}

	// A loaded store with no block for the skill renders nothing too.
	SeedCraftingMessagesForTest(CraftingMessages{"cooking": validPools()})
	if f, v := CraftRoomLines("tailoring", PhaseStart, ctx, "Stew"); f != "" || v != "" {
		t.Fatalf("unknown skill rendered %q / %q", f, v)
	}
	if h := CraftHeardLine("tailoring", PhaseStart); h != "" {
		t.Fatalf("unknown skill rendered %q", h)
	}
}
