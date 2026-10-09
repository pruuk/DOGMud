package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// finderViewSites is every function allowed to read a bauble's text as one
// viewer sees it (items.Item GetSpecFor, DisplayNameFor, NameFor,
// LongDescriptionFor; baubles.Record MaterialFor), keyed "path|function"
// as lookupFuncName names it, each with why its output reaches that viewer
// alone. A finder-only bauble's own text (owner ruling 2026-09-29) is read
// through these and nowhere else: every viewer-agnostic accessor shows the
// generic trinket (internal/items
// TestFinderOnlyBaubleIsGenericToEveryoneButItsFinder), so a new render
// path that uses them cannot leak it, and this guard stops a new caller of
// the finder's view from reaching anyone else.
//
// Two rules, so neither a new function nor a new line inside a listed one
// slips past (review finding c):
//   - Each listed function makes exactly `calls` finder-view REFERENCES: a
//     call (x.NameFor(id)) and a method value (v := x.NameFor) count alike.
//     A new reference in Look (which also talks to another player) changes
//     the count and fails until someone reads it and updates the row; a
//     reference in an unlisted function is reported.
//   - No finder-view reference, nor a call through a local alias of one
//     (v := x.NameFor; ...v(id)), may sit inside anything that sends
//     beyond its one reader: a send in beyondReaderCalls (the room sends,
//     SendCounterTrio, any SendTo...Room method, discord's SendMessage, a
//     mob's Command, merchantSay), an events literal in
//     beyondReaderEventTypes (Message, Broadcast, Communication,
//     ChannelMessage), or a SendText whose receiver fails the receiver
//     rule. The receiver rule only compares ROOT IDENTIFIERS: the SendText
//     receiver and the accessor's viewer argument must hang from the same
//     identifier (user.SendText(..., x.NameFor(user.UserId))). That is not
//     a proof that the receiver is the viewer:
//     actor.Target().SendText(..., x.NameFor(actor.GetUserId())) sends to
//     someone else yet passes it, and is stopped only by the count rule.
//
// KNOWN LIMITS (each is a way a real leak could pass):
//   - A helper that wraps an accessor (func label(i items.Item, id int)
//     string { return i.NameFor(id) }) is flagged only by the count rule AT
//     THE HELPER; a caller that sends label(...) to a room is not reported
//     as a leak.
//   - A template rendered with a finder-view value and then sent to a room
//     is not traced; the value's reference is counted where it is read.
//   - A finder-view value kept in a variable (s := x.NameFor(id)) and sent
//     later is not traced; the count makes every such line a reviewed one.
//     Only a method value bound by := or = to a local identifier is
//     followed as an alias, and only within the function that binds it; an
//     alias of an alias, a struct field or a parameter holding one is not.
//   - The receiver rule compares root identifiers only (above).
//   - The five method names are matched by name, not by type, and only
//     inside function bodies (not package-level initialisers).
//   - internal/items and internal/baubles define the accessors and are not
//     scanned; tools/ and _test.go files are not scanned either.
type finderViewSite struct {
	calls int
	why   string
}

var finderViewSites = map[string]finderViewSite{
	"internal/actions/buy.go|tryPurchaseFromInventory":         {2, "the buyer's own view of a shelf bauble: a match key compared with what the buyer typed, and the buyer's own purchase line (buyer.SendText)"},
	"internal/actions/search_bauble.go|BaubleDelivery.deliver": {1, "the find's own lines, sent to the finder alone (who.send); the room line names no item"},
	"internal/actions/steal.go|takeFromMob":                    {3, "the thief's own success line (actor.SendText); the room is not told what was taken"},
	"internal/usercommands/appraise.go|appraiseBauble":         {4, "the appraisal, sent to the player who asked for it (user.SendText); the room line names no item"},
	"internal/usercommands/inventory.go|Inventory":             {2, "the player's own inventory listing"},
	"internal/usercommands/list.go|buildShelfRows":             {1, "the lister's own shop listing; renderShopTable sends it to that user alone"},
	"internal/usercommands/look.go|Look":                       {3, "what the looker reads about an item they carry, by sight or by touch in the dark (#218); the room lines beside them keep DisplayName"},
	"internal/usercommands/look.go|lookAtFloorItem":            {2, "what the looker reads about an item on the floor (moved out of Look in lighting 5e so a fixture named in full beats a room noun); the room lines beside it keep DisplayName"},
	"internal/usercommands/look.go|lookRoom":                   {2, "the looker's own view of the room's floor and their own stash"},
	"modules/gmcp/gmcp.Char.go|GMCPCharModule.GetCharNode":     {1, "the player's own Char.Inventory backpack"},
	"modules/gmcp/gmcp.Char.go|buildBandolierContainer":        {1, "the player's own bandolier payload (GetCharNode passes user.UserId)"},
	"modules/gmcp/gmcp.Char.go|buildComponentBagContainer":     {1, "the player's own component bag payload (GetCharNode passes user.UserId)"},
	"modules/gmcp/gmcp.Room.go|GMCPRoomModule.GetRoomNode":     {1, "Room.Info.Contents.Items, built for one user and sent to that user"},
}

// finderViewSelectors are the viewer-aware accessors.
var finderViewSelectors = map[string]bool{
	"GetSpecFor": true, "DisplayNameFor": true, "NameFor": true, "LongDescriptionFor": true, "MaterialFor": true,
}

// beyondReaderCalls send text beyond one reader, by callee name, whatever
// the receiver: the room sends, actions.SendCounterTrio, discord's
// SendMessage (the only SendMessage in internal/ and modules/), a mob's
// Command (a `say`), merchantSay. isSendToRoom adds every SendTo...Room
// method (combat.AttackResult SendToSourceRoom, SendToTargetRoom).
var beyondReaderCalls = map[string]bool{
	"SendTextCommunication": true, "SendTextVisual": true, "SendTextVisualHidingNames": true,
	"SendTextVisualWithAudio": true, "SendTextVisualToSnapshot": true, "SendTextVisualWithAudioToSnapshot": true,
	"SendTextHidingNames": true, "SendCommunicationHidingNames": true, "SendVisualCommunicationHidingNames": true,
	"SendTextToExits": true, "SendTrio": true, "SendCounterTrio": true,
	"SendMessage": true, "Command": true, "merchantSay": true,
	"SendHeard": true, "SendSeen": true,
}

// isSendToRoom matches any SendTo...Room callee.
func isSendToRoom(name string) bool {
	return strings.HasPrefix(name, "SendTo") && strings.HasSuffix(name, "Room")
}

// beyondReaderEventTypes are the events literals that reach someone other
// than the viewer: Message is queued to some user or room, Broadcast to
// everyone, Communication and ChannelMessage to chat audiences.
var beyondReaderEventTypes = map[string]bool{
	"Message": true, "Broadcast": true, "Communication": true, "ChannelMessage": true,
}

// rootIdent is the identifier an expression hangs from: user for
// user.UserId, actor for actor.GetUserId(), "" when there is none.
func rootIdent(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			e = x.X
		case *ast.CallExpr:
			e = x.Fun
		case *ast.ParenExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return ""
		}
	}
}

// calleeName is a call's function name: Sel for a selector, the identifier
// for a plain call.
func calleeName(call *ast.CallExpr) (string, *ast.SelectorExpr) {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name, f
	case *ast.Ident:
		return f.Name, nil
	}
	return "", nil
}

// finderViewUse is one finder-view reference, or one call through a local
// alias of one. viewer is the call's first argument, nil for a bare
// method value or alias that is not called on the spot.
type finderViewUse struct {
	pos    token.Pos
	name   string
	viewer ast.Expr
}

// finderViewUsesIn returns every finder-view reference within n (a call
// x.NameFor(id), a method value x.NameFor, a plain NameFor), plus every use
// of an identifier in aliases. Each reference is returned once.
func finderViewUsesIn(n ast.Node, aliases map[string]bool) []finderViewUse {
	var out []finderViewUse
	skip := map[ast.Node]bool{}
	firstArg := func(c *ast.CallExpr) ast.Expr {
		if len(c.Args) == 0 {
			return nil
		}
		return c.Args[0]
	}
	ast.Inspect(n, func(m ast.Node) bool {
		switch x := m.(type) {
		case *ast.CallExpr:
			switch f := x.Fun.(type) {
			case *ast.SelectorExpr:
				if finderViewSelectors[f.Sel.Name] {
					out = append(out, finderViewUse{x.Pos(), f.Sel.Name, firstArg(x)})
					skip[f] = true
				}
			case *ast.Ident:
				if finderViewSelectors[f.Name] || aliases[f.Name] {
					out = append(out, finderViewUse{x.Pos(), f.Name, firstArg(x)})
					skip[f] = true
				}
			}
		case *ast.SelectorExpr:
			skip[x.Sel] = true // a field or method name is never an alias
			if !skip[x] && finderViewSelectors[x.Sel.Name] {
				out = append(out, finderViewUse{x.Pos(), x.Sel.Name, nil})
			}
		case *ast.Ident:
			if !skip[x] && (finderViewSelectors[x.Name] || aliases[x.Name]) {
				out = append(out, finderViewUse{x.Pos(), x.Name, nil})
			}
		}
		return true
	})
	return out
}

// finderViewAliases returns every local identifier bound, by := or = or
// var, to a finder-view method value (v := itm.NameFor) in body.
func finderViewAliases(body *ast.BlockStmt) map[string]bool {
	aliases := map[string]bool{}
	bind := func(lhs ast.Expr, rhs ast.Expr) {
		for {
			p, ok := rhs.(*ast.ParenExpr)
			if !ok {
				break
			}
			rhs = p.X
		}
		sel, ok := rhs.(*ast.SelectorExpr)
		if !ok || !finderViewSelectors[sel.Sel.Name] {
			return
		}
		if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
			aliases[id.Name] = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i := range x.Lhs {
					bind(x.Lhs[i], x.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i := range x.Names {
					bind(x.Names[i], x.Values[i])
				}
			}
		}
		return true
	})
	return aliases
}

type finderViewFile struct {
	rel  string
	file *ast.File
}

// scanFinderView returns how many finder-view references each function
// makes, and every reference (or alias call) that sits inside a send
// beyond its one reader.
func scanFinderView(fset *token.FileSet, files []finderViewFile) (calls map[string]int, leaks []string) {
	calls = map[string]int{}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			key := f.rel + "|" + lookupFuncName(fd)
			if n := len(finderViewUsesIn(fd.Body, nil)); n > 0 {
				calls[key] += n
			}
			aliases := finderViewAliases(fd.Body)
			report := func(u finderViewUse, via string) {
				leaks = append(leaks, fmt.Sprintf("%s:%d in %s: %s inside %s",
					f.rel, fset.Position(u.pos).Line, key, u.name, via))
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CompositeLit:
					// events.Message{UserId: ..., Text: ...} and its kin:
					// queued to someone, not provably the viewer.
					if sel, ok := x.Type.(*ast.SelectorExpr); ok && beyondReaderEventTypes[sel.Sel.Name] {
						for _, u := range finderViewUsesIn(x, aliases) {
							report(u, "events."+sel.Sel.Name)
						}
					}
				case *ast.CallExpr:
					name, sel := calleeName(x)
					switch {
					case beyondReaderCalls[name] || isSendToRoom(name):
						for _, arg := range x.Args {
							for _, u := range finderViewUsesIn(arg, aliases) {
								report(u, name)
							}
						}
					case name == "SendText" && sel != nil:
						receiver := rootIdent(sel.X)
						if inner, ok := sel.X.(*ast.CallExpr); ok {
							if n, _ := calleeName(inner); n == "GetRoom" {
								receiver = "" // a room reached through its reader is still a room
							}
						}
						if s, ok := sel.X.(*ast.SelectorExpr); ok && strings.HasSuffix(strings.ToLower(s.Sel.Name), "room") {
							receiver = "" // who.room.SendText, any xxxRoom field
						}
						if id, ok := sel.X.(*ast.Ident); ok && strings.HasSuffix(strings.ToLower(id.Name), "room") {
							receiver = "" // room, fromRoom, markRoom: a room variable
						}
						for _, arg := range x.Args {
							for _, u := range finderViewUsesIn(arg, aliases) {
								if u.viewer == nil || receiver == "" || rootIdent(u.viewer) != receiver {
									report(u, receiver+".SendText")
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	return calls, leaks
}

// loadFinderViewFiles parses every non-test .go file under base/roots,
// skipping internal/items and internal/baubles, with rel paths relative to
// base.
func loadFinderViewFiles(fset *token.FileSet, base string, roots []string) ([]finderViewFile, error) {
	var files []finderViewFile
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join(base, root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			r, rerr := filepath.Rel(base, path)
			if rerr != nil {
				return rerr
			}
			rel := filepath.ToSlash(r)
			if d.IsDir() {
				if rel == "internal/items" || rel == "internal/baubles" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				// A file the guard cannot read is a file it cannot clear.
				return fmt.Errorf("%s does not parse, so the finder-view guard cannot check it: %w", rel, perr)
			}
			files = append(files, finderViewFile{rel: rel, file: file})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}
	}
	return files, nil
}

// finderViewProblems compares the per-function counts to sites and appends
// every leak, sorted.
func finderViewProblems(calls map[string]int, leaks []string, sites map[string]finderViewSite) []string {
	var problems []string
	for key, got := range calls {
		want, ok := sites[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: makes %d finder-view reference(s), and is not in finderViewSites", key, got))
		case got != want.calls:
			problems = append(problems, fmt.Sprintf("%s: makes %d finder-view reference(s), finderViewSites says %d: read each one before changing the row", key, got, want.calls))
		}
	}
	for key := range sites {
		if calls[key] == 0 {
			problems = append(problems, key+": in finderViewSites but reads no finder view (stale)")
		}
	}
	problems = append(problems, leaks...)
	sort.Strings(problems)
	return problems
}

// TestFinderViewReachesOnlyItsReader fails when a function not in
// finderViewSites reads a bauble as one viewer sees it, when a listed
// function's reference count changes or it reads none (stale), when any
// finder-view reference sits inside a send beyond its one reader, or when
// a scanned Go file does not parse. If you are here for a new single-reader site
// (the output goes to one player, and only them), add it with the reason.
// If the output reaches anyone else, use the viewer-agnostic accessor,
// which shows a finder-only bauble as the generic trinket.
func TestFinderViewReachesOnlyItsReader(t *testing.T) {
	fset := token.NewFileSet()
	files, err := loadFinderViewFiles(fset, ".", []string{"internal", "modules"})
	if err != nil {
		t.Fatalf("(test must run from the repo root) %v", err)
	}
	scanned := map[string]bool{}
	for _, f := range files {
		scanned[f.rel] = true
	}
	for _, must := range []string{"internal/usercommands/look.go", "internal/usercommands/get.go", "modules/gmcp/gmcp.Char.go", "modules/auctions/auctions.go"} {
		if !scanned[must] {
			t.Fatalf("the walk never read %s: it cannot see what it guards", must)
		}
	}

	calls, leaks := scanFinderView(fset, files)
	problems := finderViewProblems(calls, leaks, finderViewSites)
	if len(problems) > 0 {
		t.Errorf("%d finder-view problem(s):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

// TestFinderViewGuardCatchesALeak proves the scan can fail on every shape
// it claims to catch, and passes the one private shape.
func TestFinderViewGuardCatchesALeak(t *testing.T) {
	fset := token.NewFileSet()
	src := `package probe

func Shout(room *rooms.Room, itm items.Item, uid int) {
	room.SendTextVisual(1, fmt.Sprintf("%s", itm.DisplayNameFor(uid)))
}

func Mine(user *users.UserRecord, itm items.Item) { user.SendText(1, itm.NameFor(user.UserId)) }

func ToAnother(user *users.UserRecord, u *users.UserRecord, itm items.Item) {
	u.SendText(1, itm.NameFor(user.UserId))
}

func Chained(user *users.UserRecord, itm items.Item) { user.GetRoom().SendText(1, itm.NameFor(user.UserId)) }

func Says(room *rooms.Room, mob *mobs.Mob, itm items.Item, uid int) {
	merchantSay(room, mob, itm.NameFor(uid))
	mob.Command("say " + itm.NameFor(uid))
}

func Queued(itm items.Item, uid int) {
	events.AddToQueue(events.Message{UserId: 2, Text: itm.NameFor(uid)})
}

func Alias(room *rooms.Room, itm items.Item, uid int) {
	v := itm.NameFor
	room.SendTextVisual(1, v(uid))
}

func Targeted(actor *users.UserRecord, itm items.Item) {
	actor.Target().SendText(1, itm.NameFor(actor.GetUserId()))
}

func Announced(itm items.Item, uid int) {
	events.AddToQueue(events.Broadcast{Text: itm.NameFor(uid)})
}

func Chatted(itm items.Item, uid int) {
	events.AddToQueue(events.Communication{CommType: "say", Message: itm.NameFor(uid)})
	events.AddToQueue(events.ChannelMessage{Text: itm.NameFor(uid)})
}

func Countered(room *rooms.Room, itm items.Item, uid int) {
	SendCounterTrio(room, itm.NameFor(uid), nil, uid)
}

func Relayed(ar *combat.AttackResult, itm items.Item, uid int) {
	ar.SendToSourceRoom(1, itm.NameFor(uid))
	ar.SendToTargetRoom(1, itm.NameFor(uid))
}

func Posted(itm items.Item, uid int) { discord.SendMessage(itm.NameFor(uid)) }
`
	file, err := parser.ParseFile(fset, "probe.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls, leaks := scanFinderView(fset, []finderViewFile{{rel: "probe.go", file: file}})

	wantCalls := map[string]int{
		"Shout": 1, "Mine": 1, "ToAnother": 1, "Chained": 1, "Says": 2, "Queued": 1,
		"Alias": 1, "Targeted": 1, "Announced": 1, "Chatted": 2, "Countered": 1, "Relayed": 2, "Posted": 1,
	}
	if len(calls) != len(wantCalls) {
		t.Errorf("every reference counted, per function: want %v, got %v", wantCalls, calls)
	}
	for fn, n := range wantCalls {
		if calls["probe.go|"+fn] != n {
			t.Errorf("probe.go|%s: want %d finder-view reference(s), got %d (all: %v)", fn, n, calls["probe.go|"+fn], calls)
		}
	}

	// Targeted is the named limit of the receiver rule: actor.Target() hangs
	// from the same root identifier as actor.GetUserId(), so it is not a
	// leak here, and only its count stands between it and a real leak.
	want := []string{"Alias", "Announced", "Chained", "Chatted", "Chatted", "Countered", "Posted",
		"Queued", "Relayed", "Relayed", "Says", "Says", "Shout", "ToAnother"}
	leakFn := regexp.MustCompile(` in probe\.go\|(\w+):`)
	var got []string
	for _, l := range leaks {
		m := leakFn.FindStringSubmatch(l)
		if m == nil {
			t.Fatalf("leak line names no function: %q", l)
		}
		got = append(got, m[1])
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("leaks by function:\n  want %v\n  got  %v\n  %s", want, got, strings.Join(leaks, "\n  "))
	}

	// Through the real problem check, with only Mine listed, the alias is
	// reported as an unlisted reference and Mine not at all.
	problems := finderViewProblems(calls, leaks, map[string]finderViewSite{"probe.go|Mine": {1, "the player's own line"}})
	var sawAlias bool
	for _, p := range problems {
		if strings.Contains(p, "probe.go|Mine") {
			t.Errorf("the player's own SendText is private: %v", p)
		}
		if strings.HasPrefix(p, "probe.go|Alias: makes 1 finder-view reference(s), and is not in finderViewSites") {
			sawAlias = true
		}
	}
	if !sawAlias {
		t.Errorf("the method value in Alias is not reported as an unlisted reference:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestFinderViewGuardFailsOnAnUnparsableFile proves a file the parser
// rejects fails the guard by name instead of vanishing from the scan.
func TestFinderViewGuardFailsOnAnUnparsableFile(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "internal", "probe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.go"), []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package probe\n\nfunc {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadFinderViewFiles(token.NewFileSet(), base, []string{"internal"})
	if err == nil || !strings.Contains(err.Error(), "internal/probe/broken.go") {
		t.Fatalf("want an error naming internal/probe/broken.go, got %v", err)
	}
}
