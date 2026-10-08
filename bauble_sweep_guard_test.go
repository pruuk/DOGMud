package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// What this guards, precisely (plan decision 13): every struct that names
// items.Item directly is reached from a sweep root or listed transient, and
// every package-level variable whose type or initializer names a struct
// that holds items at ANY depth is walked by a live source or listed in
// itemStoreVars. It does not check every type that holds items indirectly
// (84 do, mostly actors and results holding a pointer to a walked record):
// a store needs an anchor that outlives a call, and a package-level
// variable is that anchor. Known exception: the auction module keeps its
// state in a local captured by its init closures, not a package variable,
// so it is registered by hand in modules/auctions and checked by
// TestBaubleSweepSourcesMatchTheGuardedRoots instead.

// sweepModulePath is go.mod's module line.
const sweepModulePath = `github.com/GoMudEngine/GoMud`

// transientItemHolders are structs that hold an items.Item only for the
// length of one call or one tick, so the bauble sweep has nothing to find
// in them: the item they carry lives in a store the sweep walks, or is on
// its way there within the same lock hold. Keyed "<dir>.<Type>".
var transientItemHolders = map[string]string{
	`internal/actions.DropItemResult`:            `an action's result, alive for one call`,
	`internal/actions.EquipItemResult`:           `an action's result, alive for one call`,
	`internal/actions.GetItemResult`:             `an action's result, alive for one call`,
	`internal/actions.GiveItemResult`:            `an action's result, alive for one call`,
	`internal/actions.LookResolution`:            `a look's result, alive for one call (TouchItem, #218)`,
	`internal/actions.RemoveAllResult`:           `an action's result, alive for one call`,
	`internal/actions.RemoveEquipResult`:         `an action's result, alive for one call`,
	`internal/actions.StealOptions`:              `a steal's arguments, alive for one call`,
	`internal/characters.HandSlot`:               `a view: a pointer into Worn, which Character.WalkItems walks`,
	`internal/characters.SlotChoice`:             `a view: pointers into Worn plus the items one equip displaces, alive for one call`,
	`internal/characters.WornSlot`:               `a view: a pointer into Worn (AllSlots), which Worn.WalkItems walks`,
	`internal/characters.slotCandidate`:          `one equip's candidate slot, alive for one call`,
	`internal/combat.DisarmResult`:               `an attack's result, alive for one call`,
	`internal/combat.weaponSetup`:                `one attack's weapon, alive for one call`,
	`internal/events.EquipmentChange`:            `an event carrying a copy of an item that lives in a store, handled within the tick`,
	`internal/events.ItemOwnership`:              `an event carrying a copy of an item that lives in a store, handled within the tick`,
	`internal/events.StorageItemSeized`:          `an event moving a seized bank item to the auction queue within the tick`,
	`internal/hooks.WeaponBreakResult`:           `a weapon break's result, alive for one call`,
	`internal/hooks.plannedSeizure`:              `a storage fee's plan, alive for one call`,
	`internal/itemvalue.SwapDelta`:               `an item comparison, alive for one call`,
	`internal/parser.Match`:                      `a parsed command's target, alive for one command`,
	`internal/sealedcrate.cratePayload`:          `the crate file's on-disk shape; the sweep reads crates/ from disk`,
	`internal/usercommands.enchantSlotCandidate`: `one command's choice, alive for one call`,
	`modules/aicompanion.thing`:                  `one prompt's description of the room, alive for one call`,
}

// itemStoreVars are the package-level variables that hold items at some
// depth, each with the live source that walks it or why nothing in it
// needs walking. Keyed "<dir>.<name>".
var itemStoreVars = map[string]string{
	`internal/guilds.byTag`:           `the guilds live source (guilds.All)`,
	`internal/mobs.mobInstances`:      `the mobs live source (GetAllMobInstanceIds, GetInstance)`,
	`internal/rooms.roomManager`:      `the rooms live source (LoadedRooms)`,
	`internal/shops.shopCache`:        `the shops live source (AllShops)`,
	`internal/users.userManager`:      `the users live source (GetAllLoadedUsers)`,
	`internal/items.ItemDisabledSlot`: `a sentinel value (ItemId -1), never a real item`,
	`internal/mobs.mobs`:              `authored mob templates; no runtime item is ever put in one`,
	`internal/pets.petTypes`:          `authored pet templates; no runtime item is ever put in one`,
	`internal/rooms.templateCache`:    `authored room templates, cached read-only; no runtime item is ever put in one`,
	`modules/auctions.npcBuyers`:      `NPC bidders; a shopkeeper's bound shop is in the shops registry, walked by the shops live source`,
}

// itemScan is what scanItemHolders finds in a source tree.
type itemScan struct {
	direct []string        // structs with a field naming items.Item, "<dir>.<Type>"
	all    map[string]bool // every type that holds an items.Item at any depth
	vars   []string        // package-level variables naming such a type, "<dir>.<name>"
}

type scanFile struct {
	dir     string
	imports map[string]string // import name -> repo dir
	f       *ast.File
}

// scanItemHolders parses every non-test Go file under root. It skips the
// trees identifierGuardSkipDir (identifier_word_guard_test.go) names, every
// dot directory included: an agent worktree under .claude/worktrees/ is a
// second copy of internal/ and must not be read as part of this repo.
func scanItemHolders(root string) (itemScan, error) {
	files := []scanFile{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && identifierGuardSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, `.go`) || strings.HasSuffix(p, `_test.go`) {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, filepath.Dir(p))
		sf := scanFile{dir: filepath.ToSlash(rel), imports: map[string]string{}, f: f}
		for _, im := range f.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			if !strings.HasPrefix(path, sweepModulePath+`/`) {
				continue
			}
			dir := strings.TrimPrefix(path, sweepModulePath+`/`)
			name := dir[strings.LastIndex(dir, `/`)+1:]
			if im.Name != nil {
				name = im.Name.Name
			}
			sf.imports[name] = dir
		}
		files = append(files, sf)
		return nil
	})
	if err != nil {
		return itemScan{}, err
	}

	const item = `internal/items.Item`
	type typeDecl struct {
		key      string
		isStruct bool
		refs     []string
	}
	decls := []typeDecl{}
	for _, sf := range files {
		for _, d := range sf.f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				_, isStruct := ts.Type.(*ast.StructType)
				decls = append(decls, typeDecl{sf.dir + `.` + ts.Name.Name, isStruct, typeRefs(ts.Type, sf)})
			}
		}
	}

	scan := itemScan{all: map[string]bool{item: true}}
	for _, d := range decls {
		if d.isStruct && d.key != item && slices.Contains(d.refs, item) {
			scan.direct = append(scan.direct, d.key)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, d := range decls {
			if scan.all[d.key] {
				continue
			}
			for _, r := range d.refs {
				if scan.all[r] {
					scan.all[d.key] = true
					changed = true
					break
				}
			}
		}
	}
	for _, sf := range files {
		for _, d := range sf.f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				refs := []string{}
				if vs.Type != nil {
					refs = typeRefs(vs.Type, sf)
				}
				for _, v := range vs.Values {
					refs = append(refs, typeRefs(v, sf)...)
				}
				if !slices.ContainsFunc(refs, func(r string) bool { return scan.all[r] }) {
					continue
				}
				for _, n := range vs.Names {
					if n.Name != `_` {
						scan.vars = append(scan.vars, sf.dir+`.`+n.Name)
					}
				}
			}
		}
	}
	sort.Strings(scan.direct)
	sort.Strings(scan.vars)
	return scan, nil
}

// typeRefs lists "<dir>.<Name>" for every type name n mentions, resolving a
// package selector through the file's imports and a bare name to the
// file's own package. Field names, composite-literal keys, function types,
// function literals and interface types are not followed.
func typeRefs(n ast.Node, sf scanFile) []string {
	out := []string{}
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				if dir, ok := sf.imports[id.Name]; ok {
					out = append(out, dir+`.`+x.Sel.Name)
				}
			}
			return false
		case *ast.Ident:
			out = append(out, sf.dir+`.`+x.Name)
		case *ast.Field:
			out = append(out, typeRefs(x.Type, sf)...)
			return false
		case *ast.KeyValueExpr:
			out = append(out, typeRefs(x.Value, sf)...)
			return false
		case *ast.FuncType, *ast.FuncLit, *ast.InterfaceType:
			return false
		}
		return true
	})
	return out
}

// sweepReachableTypes lists "<dir>.<Type>" for every named type reachable
// from a sweep root through fields, pointers, slices, arrays and maps.
func sweepReachableTypes() map[string]bool {
	out := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var visit func(t reflect.Type)
	visit = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		if t.Name() != `` && strings.HasPrefix(t.PkgPath(), sweepModulePath+`/`) {
			out[strings.TrimPrefix(t.PkgPath(), sweepModulePath+`/`)+`.`+t.Name()] = true
		}
		switch t.Kind() {
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				visit(t.Field(i).Type)
			}
		case reflect.Pointer, reflect.Slice, reflect.Array:
			visit(t.Elem())
		case reflect.Map:
			visit(t.Key())
			visit(t.Elem())
		}
	}
	for _, r := range sweepRoots() {
		visit(r.typ)
	}
	return out
}

// Every struct that holds an items.Item is either reached from a bauble
// sweep root (and so walked: TestItemWalkersVisitEveryItemField) or listed
// as transient with its reason. A new store of items that is neither would
// have its baubles' records pruned while they still exist.
func TestEveryItemHolderIsASweepRootOrTransient(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find the repo root")
	}
	scan, err := scanItemHolders(filepath.Dir(here))
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.direct) < 20 || len(scan.vars) < 5 {
		t.Fatalf("found only %d item-holding types and %d store variables: the scan is broken, not the repo", len(scan.direct), len(scan.vars))
	}

	isVar := map[string]bool{}
	for _, v := range scan.vars {
		isVar[v] = true
		if _, ok := itemStoreVars[v]; !ok {
			t.Errorf("package-level variable %s holds items at some depth and is not in itemStoreVars. If it is a store, register a live source that walks it (bauble_sweep.go); otherwise list it with the reason nothing in it needs walking", v)
		}
	}
	for v := range itemStoreVars {
		if !isVar[v] {
			t.Errorf("itemStoreVars lists %s, which no longer holds items; remove it", v)
		}
	}

	holders := scan.direct
	reach := sweepReachableTypes()
	isHolder := map[string]bool{}
	for _, h := range holders {
		isHolder[h] = true
		_, transient := transientItemHolders[h]
		switch {
		case reach[h] && transient:
			t.Errorf("%s is reached from a bauble sweep root; take it off transientItemHolders", h)
		case !reach[h] && !transient:
			t.Errorf("%s holds an items.Item but no bauble sweep root reaches it. If it outlives one call, give it a WalkItems and a live source (bauble_sweep.go) and a root in item_walker_guard_test.go; if it is transient, add it to transientItemHolders with the reason", h)
		}
	}
	for h := range transientItemHolders {
		if !isHolder[h] {
			t.Errorf("transientItemHolders lists %s, which no longer holds an item; remove it", h)
		}
	}
}

// The scan skips dot directories: an agent worktree under
// .claude/worktrees/ is a second copy of the repo, and a struct planted
// there must not be read as part of it.
func TestItemHolderScanSkipsDotDirectories(t *testing.T) {
	root := t.TempDir()
	src := "package x\n\nimport \"github.com/GoMudEngine/GoMud/internal/items\"\n\ntype Holder struct{ It items.Item }\n\nvar store = map[int]*Holder{}\n"
	for _, dir := range []string{`internal/x`, `.claude/worktrees/w/internal/y`, `.hidden/z`} {
		p := filepath.Join(root, filepath.FromSlash(dir), `x.go`)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scan, err := scanItemHolders(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scan.direct, []string{`internal/x.Holder`}) || !reflect.DeepEqual(scan.vars, []string{`internal/x.store`}) {
		t.Fatalf("direct %v vars %v, want only internal/x's Holder and store", scan.direct, scan.vars)
	}
}
