package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/assert"
)

func TestSplashTemplatePath(t *testing.T) {
	assert.Equal(t, "generic/splash/sunrise", splashTemplatePath("sunrise"))
	assert.Equal(t, "generic/splash/weather_storm", splashTemplatePath("weather_storm"))
}

type notASplashEvent struct{}

func (notASplashEvent) Type() string { return "dummy" }

func TestSplashArtOnlyWhereItFits(t *testing.T) {
	const art, caption = "art", "The sun rises."
	for _, tc := range []struct {
		name          string
		artOK, sr     bool
		width         int
		want          string
		wantSomething bool
	}{
		{"a full-width screen gets the art", true, false, 80, art, true},
		{"art exactly as wide as the screen", true, false, 76, art, true},
		{"a narrow screen gets the words", true, false, 75, caption, true},
		{"a screen reader gets the words", true, true, 80, caption, true},
		{"failed art falls back to the words", false, false, 80, caption, true},
	} {
		got, ok := splashFor(art, 76, tc.artOK, tc.sr, tc.width, caption)
		assert.Equal(t, tc.want, got, tc.name)
		assert.Equal(t, tc.wantSomething, ok, tc.name)
	}
	_, ok := splashFor(art, 76, true, false, 40, "")
	assert.False(t, ok, "a narrow screen with no caption is sent nothing")

	long := "A shimmer passes over your skin as the change settles in. You feel the mutation take hold."
	got, ok := splashFor(art, 76, true, false, 30, long)
	assert.True(t, ok)
	for _, line := range strings.Split(got, "\n") {
		assert.LessOrEqual(t, templates.WidestLine(line), 30, "the caption is wrapped to the screen: %q", line)
	}
	got, _ = splashFor(art, 76, true, true, 30, long)
	assert.Equal(t, long, got, "a screen reader gets the caption as it is")
}

func TestSplashDeliverWrongEventType(t *testing.T) {
	// A non-Splash event is ignored (Continue), no panic.
	assert.Equal(t, events.Continue, Splash_Deliver(notASplashEvent{}))
}
