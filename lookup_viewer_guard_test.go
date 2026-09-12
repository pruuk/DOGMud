package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// lookupEntry records how one function looks creatures up by name: calls that
// pass the looker as a viewer (so a creature they do not perceive cannot be
// named), and calls that do not, with the reason that is right.
type lookupEntry struct {
	viewer int
	plain  int
	why    string
}

const (
	whyMob      = "a mob caller; mobs perceiving hidden creatures is slice F"
	whyStaff    = "a staff tool; it must reach every character, hidden or not"
	whySelf     = "only asks whether the typed name is the looker themselves, whom they always perceive"
	whyUnfilter = "the unfiltered form itself, kept for staff tools and mob callers"
)

// lookupRegistry is every non-test creature lookup in internal/ and modules/,
// keyed "path|function". Follow-up slice A, owner ruling 9: every player
// command that names a creature passes the player as viewer.
var lookupRegistry = map[string]lookupEntry{
	"internal/actions/buy.go|Buy":                                      {viewer: 1, plain: 1, why: whySelf},
	"internal/actions/cast.go|InitiateCast":                            {viewer: 4},
	"internal/actions/combat_attack.go|FindAttackTarget":               {viewer: 1},
	"internal/actions/combat_fire.go|ExecuteFire":                      {viewer: 2},
	"internal/actions/melee_target.go|StageMeleeTarget":                {viewer: 1, plain: 1, why: whySelf},
	"internal/actions/target_resolution.go|ResolveTargetActor":         {viewer: 1},
	"internal/actions/track.go|Track":                                  {viewer: 1},
	"internal/mobcommands/aid.go|Aid":                                  {plain: 1, why: whyMob},
	"internal/mobcommands/attack.go|Attack":                            {viewer: 1},
	"internal/mobcommands/befriend.go|Befriend":                        {plain: 1, why: whyMob},
	"internal/mobcommands/consider.go|Consider":                        {plain: 1, why: whyMob},
	"internal/mobcommands/give.go|Give":                                {plain: 1, why: whyMob},
	"internal/mobcommands/givequest.go|GiveQuest":                      {plain: 1, why: whyMob},
	"internal/mobcommands/look.go|Look":                                {plain: 1, why: whyMob},
	"internal/mobcommands/plant.go|Plant":                              {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|ReplyTo":                            {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|SayTo":                              {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|SayToOnly":                          {plain: 1, why: whyMob},
	"internal/mobcommands/shadow.go|Shadow":                            {plain: 1, why: whyMob},
	"internal/mobcommands/show.go|Show":                                {plain: 1, why: whyMob},
	"internal/mobcommands/steal.go|parseMobStealArgs":                  {plain: 1, why: whyMob},
	"internal/parser/adapters.go|mobAdapter":                           {viewer: 1},
	"internal/parser/adapters.go|playerAdapter":                        {viewer: 1},
	"internal/rooms/rooms.go|Room.FindByName":                          {plain: 1, why: whyUnfilter},
	"internal/usercommands/admin.ai.go|AiFlag":                         {plain: 1, why: whyStaff},
	"internal/usercommands/admin.buff.go|Buff":                         {plain: 1, why: whyStaff},
	"internal/usercommands/admin.command.go|Command":                   {plain: 1, why: whyStaff},
	"internal/usercommands/admin.paz.go|Paz":                           {plain: 1, why: whyStaff},
	"internal/usercommands/admin.skillset.go|Skillset":                 {plain: 1, why: whyStaff},
	"internal/usercommands/admin.zap.go|Zap":                           {plain: 1, why: whyStaff},
	"internal/usercommands/ask.go|Ask":                                 {viewer: 1},
	"internal/usercommands/attack.go|Attack":                           {viewer: 1},
	"internal/usercommands/consider.go|Consider":                       {viewer: 1},
	"internal/usercommands/give.go|Give":                               {viewer: 1},
	"internal/usercommands/give.go|giveTargetResolves":                 {viewer: 1},
	"internal/usercommands/look.go|Look":                               {viewer: 1},
	"internal/usercommands/moderation_target.go|resolveModTarget":      {plain: 1, why: whyStaff},
	"internal/usercommands/party.go|cmdPartyInvite":                    {viewer: 1},
	"internal/usercommands/report.go|Report":                           {viewer: 1},
	"internal/usercommands/sell.go|resolveSellItem":                    {viewer: 1},
	"internal/usercommands/shoot.go|resolveShootTarget":                {viewer: 2},
	"internal/usercommands/show.go|Show":                               {viewer: 1},
	"internal/usercommands/skill.skullduggery.plant.go|parsePlantArgs": {viewer: 1},
	"internal/usercommands/skill.skullduggery.shadow.go|Shadow":        {viewer: 1, plain: 1, why: whySelf},
	"internal/usercommands/skill.skullduggery.steal.go|parseStealArgs": {viewer: 1},
	"internal/usercommands/talk.go|Talk":                               {viewer: 1},
	"internal/usercommands/target.go|Target":                           {viewer: 1, plain: 1, why: whySelf},
	"modules/follow/follow.go|FollowModule.followMobCommand":           {plain: 1, why: whyMob},
	"modules/follow/follow.go|FollowModule.followUserCommand":          {viewer: 1},
}

func lookupIsNil(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

// lookupPassesViewer classifies one lookup call. A ResolveTargetActor call
// passes a viewer when its options literal names Viewer, or when the function
// sets `.Viewer =` on an options variable (buy.go does).
func lookupPassesViewer(call *ast.CallExpr, callee string, body *ast.BlockStmt) bool {
	switch callee {
	case "FindByNameSeenBy", "findPresentTargetByNoun":
		return len(call.Args) > 0 && !lookupIsNil(call.Args[0])
	case "FindAttackTarget":
		return len(call.Args) == 5 && !lookupIsNil(call.Args[4])
	case "ResolveTargetActor":
		for _, a := range call.Args {
			if cl, ok := a.(*ast.CompositeLit); ok {
				for _, elt := range cl.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						// The VALUE has to be a real viewer. Accepting the key
						// alone let `Viewer: nil` register as filtered, which
						// ships a command with the filter switched off.
						if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Viewer" && !lookupIsNil(kv.Value) {
							return true
						}
					}
				}
			}
		}
		assigns := false
		ast.Inspect(body, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok {
				for _, lhs := range as.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Viewer" {
						assigns = true
					}
				}
			}
			return true
		})
		return assigns
	}
	return false
}

func lookupFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name + "." + fd.Name.Name
		}
	case *ast.Ident:
		return t.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// TestEveryCreatureLookupDeclaresItsViewer fails when a function looks a
// creature up by name and is not in lookupRegistry, when its viewer or plain
// call counts differ from the registry, or when a registry entry is stale.
//
// The mistake it exists for is the one this slice makes easy: adding a player
// command that names a creature and forgetting the viewer, so `look <hidden
// creature>` gives the hider away again. If you are here for a new player
// command, pass the player as Viewer (ResolveTargetOptions.Viewer or
// FindByNameSeenBy). If it is a staff tool or a mob caller, register it as
// plain with the reason.
func TestEveryCreatureLookupDeclaresItsViewer(t *testing.T) {
	found := map[string]lookupEntry{}
	for _, root := range []string{"internal", "modules"} {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			rel := filepath.ToSlash(path)
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				key := rel + "|" + lookupFuncName(fd)
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := ""
					switch f := call.Fun.(type) {
					case *ast.SelectorExpr:
						callee = f.Sel.Name
					case *ast.Ident:
						callee = f.Name
					}
					switch callee {
					case "FindByName", "FindByNameSeenBy", "ResolveTargetActor", "FindAttackTarget", "findPresentTargetByNoun":
					default:
						return true
					}
					e := found[key]
					if lookupPassesViewer(call, callee, fd.Body) {
						e.viewer++
					} else {
						e.plain++
					}
					found[key] = e
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s (test must run from the repo root): %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no lookups found at all: the walk is broken, not the code")
	}

	var problems []string
	for key, got := range found {
		want, ok := lookupRegistry[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: not in lookupRegistry (viewer %d, plain %d)", key, got.viewer, got.plain))
		case got.viewer != want.viewer || got.plain != want.plain:
			problems = append(problems, fmt.Sprintf("%s: viewer %d plain %d, registry says viewer %d plain %d", key, got.viewer, got.plain, want.viewer, want.plain))
		}
	}
	for key, want := range lookupRegistry {
		if _, ok := found[key]; !ok {
			problems = append(problems, key+": in lookupRegistry but no lookup found there (stale)")
		}
		if want.plain > 0 && want.why == "" {
			problems = append(problems, key+": plain lookups need a reason")
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("%d creature lookup problem(s):\n  %s\n\n"+
			"A player command that names a creature must pass the player as Viewer, "+
			"or a hidden creature can be named. Staff tools and mob callers are "+
			"registered as plain, with the reason.",
			len(problems), strings.Join(problems, "\n  "))
	}
}
