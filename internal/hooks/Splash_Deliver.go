package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/splash"
	"github.com/GoMudEngine/GoMud/internal/templates"
)

func splashTemplatePath(sceneId string) string { return "generic/splash/" + sceneId }

// Splash_Deliver renders a splash scene to its recipients: the refined ASCII art
// for normal clients (the web client is xterm.js and renders the same ANSI art),
// or the plain caption for screen-reader users.
func Splash_Deliver(e events.Event) events.ListenerReturn {
	evt, ok := e.(splash.Splash)
	if !ok {
		return events.Continue
	}
	if !bool(configs.GetGamePlayConfig().SplashesEnabled) {
		return events.Continue
	}

	// Render the art once — the template output is identical for every recipient
	// (per-recipient line wrapping happens later in SendText).
	text, terr := templates.Process(splashTemplatePath(evt.SceneId), evt.Data)
	artOK := terr == nil && text != ""
	artWidth := templates.WidestLine(text)

	for _, u := range splash.Recipients(evt) {
		if out, ok := splashFor(text, artWidth, artOK, u.ScreenReader, u.WrapWidth(), evt.Caption); ok {
			u.SendText(messaging.CategorySplash, out)
		}
	}
	return events.Continue
}

// splashFor is what one reader is sent: the art when it fits their screen,
// otherwise the caption, words only (web client base spec, section 2: there
// is no small art). CategorySplash is not wrapped by the pipeline, so the
// caption is wrapped here; screen-reader users get it as it is. ok is false
// when there is nothing to send.
func splashFor(art string, artWidth int, artOK, screenReader bool, width int, caption string) (string, bool) {
	if artOK && !screenReader && artWidth <= width {
		return art, true
	}
	if caption == "" {
		return "", false
	}
	if screenReader {
		return caption, true
	}
	return messaging.WrapAnsi(caption, width), true
}
