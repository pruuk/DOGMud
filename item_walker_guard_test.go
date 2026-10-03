package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rifts"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/modules/auctions"
)

// The bauble catalog sweep (internal/baubles/sweep.go) prunes a record once
// nothing in the world points at it, and it finds live items only through
// these roots and their WalkItems methods. A store it misses loses its
// baubles' names. Two guards keep that complete:
//
//   - TestItemWalkersVisitEveryItemField (here) builds each root with an
//     item planted in EVERY items.Item field reachable from it by
//     reflection, unexported ones included, and fails naming the path of
//     any planted item WalkItems does not visit.
//   - TestEveryItemHolderIsASweepRootOrTransient (bauble_sweep_guard_test.go)
//     finds every struct in the repo that holds an items.Item and fails for
//     one no root reaches, unless it is listed as transient with a reason.
//
// Known limit: an item held behind an interface (any) is invisible to both.
// None is today: Mob.BTreeState, Room.LongTermDataStore and the
// tempDataStore maps hold no items.

type sweepRoot struct {
	name string
	typ  reflect.Type
	walk func(root reflect.Value, fn func(*items.Item))
}

// sweepRoots are the live stores registerBaubleSweepSources (bauble_sweep.go)
// and modules/auctions register, one per source name.
func sweepRoots() []sweepRoot {
	return []sweepRoot{
		{`auctions`, reflect.TypeOf((*auctions.AuctionManager)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*auctions.AuctionManager).WalkItems(fn)
		}},
		{`guilds`, reflect.TypeOf((*guilds.Guild)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*guilds.Guild).WalkItems(fn)
		}},
		{`housing`, reflect.TypeOf((*housing.House)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*housing.House).WalkItems(fn)
		}},
		{`mobs`, reflect.TypeOf((*mobs.Mob)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*mobs.Mob).WalkItems(fn)
		}},
		{`rifts`, reflect.TypeOf((*rifts.LostRecord)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Interface().(rifts.LostRecord).WalkItems(fn)
		}},
		{`rooms`, reflect.TypeOf((*rooms.Room)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*rooms.Room).WalkItems(fn)
		}},
		{`shops`, reflect.TypeOf((*shops.ShopInventory)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*shops.ShopInventory).WalkItems(fn)
		}},
		{`users`, reflect.TypeOf((*users.UserRecord)(nil)).Elem(), func(v reflect.Value, fn func(*items.Item)) {
			v.Addr().Interface().(*users.UserRecord).WalkItems(fn)
		}},
	}
}

var walkGuardItemType = reflect.TypeOf(items.Item{})

var walkGuardReach = map[reflect.Type]bool{}

// walkGuardReaches reports whether a value of type t can hold an items.Item
// anywhere inside it (through struct fields, pointers, slices, arrays and
// map keys or values; not through interfaces).
func walkGuardReaches(t reflect.Type) bool {
	if r, ok := walkGuardReach[t]; ok {
		return r
	}
	r := walkGuardSearch(t, map[reflect.Type]bool{})
	walkGuardReach[t] = r
	return r
}

func walkGuardSearch(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == walkGuardItemType {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if walkGuardSearch(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return walkGuardSearch(t.Elem(), seen)
	case reflect.Map:
		return walkGuardSearch(t.Key(), seen) || walkGuardSearch(t.Elem(), seen)
	}
	return false
}

// walkGuardPlanter puts a distinct bauble in every items.Item field it can
// reach, allocating pointers, one-element slices and one-entry maps on the
// way. A type may appear at most twice on one path, so a recursive type is
// planted one level deep instead of forever.
type walkGuardPlanter struct {
	next    int
	planted map[string]string // bauble id -> field path
	onPath  map[reflect.Type]int
}

func (p *walkGuardPlanter) plant(v reflect.Value, path string) {
	t := v.Type()
	if !walkGuardReaches(t) || p.onPath[t] >= 2 {
		return
	}
	v = walkGuardSettable(v)
	if t == walkGuardItemType {
		p.next++
		id := fmt.Sprintf(`B%07d`, p.next)
		v.Set(reflect.ValueOf(items.Item{ItemId: items.BaubleItemId, Bauble: id}))
		p.planted[id] = path
		return
	}
	p.onPath[t]++
	defer func() { p.onPath[t]-- }()
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			p.plant(v.Field(i), path+`.`+t.Field(i).Name)
		}
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(t.Elem()))
		}
		p.plant(v.Elem(), path)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(t, 1, 1))
		p.plant(v.Index(0), path+`[0]`)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			p.plant(v.Index(i), fmt.Sprintf(`%s[%d]`, path, i))
		}
	case reflect.Map:
		elem := reflect.New(t.Elem()).Elem()
		p.plant(elem, path+`[k]`)
		m := reflect.MakeMap(t)
		m.SetMapIndex(reflect.New(t.Key()).Elem(), elem)
		v.Set(m)
	}
}

// walkGuardSettable makes an unexported field settable. Test-only.
func walkGuardSettable(v reflect.Value) reflect.Value {
	if v.CanSet() {
		return v
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func TestItemWalkersVisitEveryItemField(t *testing.T) {
	for _, root := range sweepRoots() {
		t.Run(root.name, func(t *testing.T) {
			p := &walkGuardPlanter{planted: map[string]string{}, onPath: map[reflect.Type]int{}}
			v := reflect.New(root.typ).Elem()
			p.plant(v, root.typ.Name())
			if len(p.planted) == 0 {
				t.Fatalf("planted nothing in %s: the guard cannot fail", root.typ)
			}
			seen := map[string]bool{}
			root.walk(v, func(it *items.Item) { seen[it.Bauble] = true })
			missing := []string{}
			for id, path := range p.planted {
				if !seen[id] {
					missing = append(missing, path)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("%s.WalkItems misses %d item field(s); walk them, or the bauble sweep prunes the records of what they hold:\n  %s",
					root.typ, len(missing), strings.Join(missing, "\n  "))
			}
		})
	}
}
