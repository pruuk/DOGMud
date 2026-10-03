package rifts

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/casing"
)

func shippedObelisk(t *testing.T) *Profile {
	t.Helper()
	loaded, err := loadFrom(shippedDir)
	require.NoError(t, err)
	p := loaded[`obelisk`]
	require.NotNil(t, p)
	require.NoError(t, p.validate(false))
	return p
}

// asReply renders a template as a model would answer, with a new title and
// opening (so it is not a copy) and its doors trimmed to the pool's exact
// count, keeping a puzzle's sealed door.
func asReply(t *testing.T, tp *Template, req GenRequest) RoomReply {
	t.Helper()
	r, err := ParseRoomReply(exampleJSON(tp, req.profile.Generation.Effects))
	require.NoError(t, err, tp.Id)
	r.Title = `new ` + tp.Title
	r.Description = `Freshly made, ` + r.Description
	var doors []DoorReply
	for _, d := range r.Doors {
		if r.Puzzle != nil && d.Exit == r.Puzzle.SealedDoor {
			doors = append([]DoorReply{d}, doors...)
		} else {
			doors = append(doors, d)
		}
	}
	r.Doors = doors[:req.Doors]
	return r
}

// The bank's rules fit the house style: every shipped room of a pool the
// model may write for, answered back as a reply, passes them.
func TestBuildGenerated_ShippedRoomsPass(t *testing.T) {
	p := shippedObelisk(t)
	checked := 0
	for _, pool := range p.Generation.Pools {
		req := p.genRequest(pool)
		req.UsedTitles, req.Openings = nil, nil
		for _, tp := range p.templates[pool] {
			got, err := BuildGenerated(req, asReply(t, tp, req))
			require.NoError(t, err, tp.Id)
			assert.Equal(t, `generated`, got.Source)
			assert.True(t, strings.HasPrefix(got.Id, `gen-`+strings.ToLower(string(pool))+`-new-`), got.Id)
			assert.Equal(t, casing.Title(`New `+tp.Title), got.Title, `title cased`)
			if got.Puzzle != nil {
				assert.Equal(t, `key`, got.Puzzle.Reward, `a lens table always gives a key`)
				assert.True(t, got.Puzzle.Hard)
			}
			checked++
		}
	}
	assert.Greater(t, checked, 40)
}

func TestBuildGenerated_Refusals(t *testing.T) {
	p := shippedObelisk(t)
	var cRoom *Template
	for _, tp := range p.templates[PoolPuzzle] {
		if tp.Id == `c-the-room-of-one-hand` {
			cRoom = tp
		}
	}
	require.NotNil(t, cRoom)
	req := p.genRequest(PoolPuzzle)
	req.UsedTitles, req.Openings = nil, nil

	cases := map[string]func(r *RoomReply){
		`a used title`:         func(r *RoomReply) { r.Title = `the room of one hand` },
		`too few doors`:        func(r *RoomReply) { r.Doors = r.Doors[:2] },
		`a reserved noun`:      func(r *RoomReply) { r.Nouns[0].Name = `prism` },
		`a writing's noun`:     func(r *RoomReply) { r.Nouns[0].Name = `orb` },
		`a noun named as exit`: func(r *RoomReply) { r.Nouns[0].Name = r.Doors[1].Exit },
		`two words for a noun`: func(r *RoomReply) { r.Nouns[0].Name = `old slab` },
		`a short description`:  func(r *RoomReply) { r.Description = `A small room. It is sealed.` },
		`markup`:               func(r *RoomReply) { r.Nouns[0].Look += ` {lens}` },
		`the table's noun`:     func(r *RoomReply) { r.Nouns[0].Name = `table` },
		`an unknown effect`: func(r *RoomReply) {
			r.Trap = &TrapReply{Difficulty: 100, Effect: `fire`, Triggered: `Something bites at you hard.`, Avoided: `You step around the thing.`}
		},
		`neither puzzle nor trap`: func(r *RoomReply) { r.Puzzle, r.Trap = nil, nil },
		`a sealed door elsewhere`: func(r *RoomReply) { r.Puzzle.SealedDoor = `nowhere` },
		`too many idle lines`: func(r *RoomReply) {
			r.IdleMessages = []string{`One thing happens.`, `Another thing happens.`, `A third thing happens.`}
		},
	}
	for name, mutate := range cases {
		r := asReply(t, cRoom, req)
		if name == `a used title` {
			req.UsedTitles = []string{`The Room of One Hand`}
		}
		mutate(&r)
		_, err := BuildGenerated(req, r)
		assert.ErrorIs(t, err, ErrUnusableRoom, name)
		req.UsedTitles = nil
	}

	// A D room needs an encounter; an F room names the way out.
	var dRoom, fRoom *Template
	dRoom, fRoom = p.templates[PoolMonster][0], p.templates[PoolExit][0]
	dReq, fReq := p.genRequest(PoolMonster), p.genRequest(PoolExit)
	dReq.UsedTitles, dReq.Openings, fReq.UsedTitles, fReq.Openings = nil, nil, nil, nil
	r := asReply(t, dRoom, dReq)
	r.Encounter = nil
	_, err := BuildGenerated(dReq, r)
	assert.ErrorIs(t, err, ErrUnusableRoom)
	r = asReply(t, fRoom, fReq)
	r.Description = strings.ReplaceAll(strings.ReplaceAll(r.Description, `breach`, `crack`), `Breach`, `Crack`)
	_, err = BuildGenerated(fReq, r)
	assert.ErrorIs(t, err, ErrUnusableRoom)

	// Not JSON, or JSON that is not the schema, is the schema ignored.
	_, err = ParseRoomReply(`a room`)
	assert.Error(t, err)
	_, err = ParseRoomReply(`{"title":"x","colour":"red"}`)
	assert.Error(t, err)
	_, err = ParseRoomReply(`{"title":"x"} and more`)
	assert.Error(t, err)

	// Mentions are words, not fragments.
	assert.True(t, mentions(`Two arches lean`, `arch`))
	assert.True(t, mentions(`a shallow dish`, `dishes`))
	assert.False(t, mentions(`a long search`, `arch`))
}

// The request carries what the prompt needs, and the schema fits the pool.
func TestGenRequest_AndSchema(t *testing.T) {
	p := shippedObelisk(t)
	req := p.genRequest(PoolPuzzle)
	assert.Equal(t, 3, req.Doors)
	assert.Len(t, req.Examples, examplesPerCall)
	assert.Len(t, req.Seeds, seedsPerCall)
	assert.Contains(t, req.Setting, `Watchers`)
	assert.Contains(t, req.Guide, `puzzle`)
	assert.Len(t, req.UsedTitles, len(p.templates[PoolPuzzle]))
	for _, n := range []string{`prism`, `rubble`, `seam`, `crystal`, `breach`, `stele`, `orb`, `frieze`} {
		assert.Contains(t, req.ReservedNouns, n)
	}
	for _, ex := range req.Examples {
		var r RoomReply
		require.NoError(t, json.Unmarshal([]byte(ex), &r))
		assert.NotEmpty(t, r.Title)
	}
	assert.Equal(t, `breach`, p.genRequest(PoolExit).ExitName)

	props := func(pool Pool) map[string]any {
		return RoomSchema(p.genRequest(pool))[`properties`].(map[string]any)
	}
	assert.Contains(t, props(PoolPuzzle), `puzzle`)
	assert.Contains(t, props(PoolPuzzle), `trap`)
	assert.Contains(t, props(PoolMonster), `encounter`)
	assert.NotContains(t, props(PoolPassage), `puzzle`)
	assert.NotContains(t, props(PoolPassage), `encounter`)
	// Strict: every property is required.
	s := RoomSchema(req)
	assert.Len(t, s[`required`], len(s[`properties`].(map[string]any)))
	assert.Equal(t, false, s[`additionalProperties`])
}

// fakeGen writes a fixed room on userId 1's key.
type fakeGen struct {
	chance, most int
	room         func(req GenRequest) (*Template, error)
	reserved     []int
	asked        []GenRequest
}

func (g *fakeGen) Chance() int     { return g.chance }
func (g *fakeGen) MaxPerPool() int { return g.most }
func (g *fakeGen) Reserve(userId int) bool {
	g.reserved = append(g.reserved, userId)
	return userId == testUser
}
func (g *fakeGen) Generate(ctx context.Context, userId int, req GenRequest) (*Template, error) {
	g.asked = append(g.asked, req)
	return g.room(req)
}

// As rooms are built with a player inside, a new room is written, saved to
// the bank's folder and added to its pool; it loads back like an authored
// one. Nothing is asked with nobody inside, at chance 0, past the cap, or
// for a pool the profile does not generate.
func TestGenerate_SavesAndGrowsThePool(t *testing.T) {
	u := setupRuntime(t)
	dir := t.TempDir()
	origDir, origStart := genDir, startGen
	genDir = func(*Profile) string { return dir }
	startGen = func(job func(lock bool)) { job(false) }
	t.Cleanup(func() { genDir, startGen = origDir, origStart; SetGenerator(nil) })

	g := &fakeGen{chance: 100}
	n := 0
	g.room = func(req GenRequest) (*Template, error) {
		n++
		var src *Template
		for _, tp := range GetProfile(req.ProfileId).templates[req.Pool] {
			if tp.Source != `generated` {
				src = tp
				break
			}
		}
		r := asReply(t, src, req)
		r.Title = `Generated Room ` + string(rune('A'+n))
		r.Description = strings.Repeat(string(rune('a'+n)), 1) + ` ` + r.Description
		tp, err := BuildGenerated(req, r)
		require.NoError(t, err, `pool %s from %s`, req.Pool, src.Id)
		return tp, nil
	}
	SetGenerator(g)

	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	p := run.Profile
	before := map[Pool]int{}
	for _, pool := range AllPools {
		before[pool] = len(p.templates[pool])
	}
	assert.Empty(t, g.asked, `the entry room is built with nobody inside`)

	walk(t, u, run.EntryRoomId) // the rooms behind its doors are built
	require.NotEmpty(t, g.asked)
	files, _ := filepath.Glob(filepath.Join(dir, `gen-*.yaml`))
	require.Len(t, files, len(g.asked), `one file per room written`)
	grown := 0
	for _, pool := range AllPools {
		grown += len(p.templates[pool]) - before[pool]
	}
	assert.Equal(t, len(g.asked), grown, `each joined its pool`)
	for _, f := range files {
		tp := &Template{}
		require.NoError(t, readStrict(f, tp))
		assert.Equal(t, strings.TrimSuffix(filepath.Base(f), `.yaml`), tp.Id)
		assert.Equal(t, `generated`, tp.Source)
		assert.NotEmpty(t, tp.Created)
		require.NoError(t, tp.validate(p, false), f)
	}
	assert.Equal(t, 0, len(genBusy), `nothing left in flight`)

	// The cap: no more once a pool holds that many generated rooms.
	asked := len(g.asked)
	g.most = 1
	for _, pool := range p.Generation.Pools {
		for p.generatedCount(pool) < 1 {
			require.NoError(t, p.addGenerated(mustGen(t, p, pool, `Capped Filler `+string(pool))))
		}
	}
	run.maybeGenerate(PoolPassage)
	assert.Equal(t, asked, len(g.asked), `the pool is full`)
	filled, _ := filepath.Glob(filepath.Join(dir, `gen-*.yaml`))

	g.most, g.chance = 0, 0
	run.maybeGenerate(PoolPassage)
	assert.Equal(t, asked, len(g.asked), `chance 0`)

	g.chance = 100
	run.maybeGenerate(PoolBoss)
	assert.Equal(t, asked, len(g.asked), `the boss pool is not generated`)

	genBusy[p.Id+`/A`] = true
	run.maybeGenerate(PoolPassage)
	delete(genBusy, p.Id+`/A`)
	assert.Equal(t, asked, len(g.asked), `one at a time per pool`)

	// A refused or failed generation saves nothing.
	g.room = func(GenRequest) (*Template, error) { return nil, errors.New(`the key failed`) }
	run.maybeGenerate(PoolPassage)
	files2, _ := filepath.Glob(filepath.Join(dir, `gen-*.yaml`))
	assert.Equal(t, len(filled), len(files2))

	// A title another room took meanwhile is refused at the bank.
	dup := mustGen(t, p, PoolPassage, `Late Twin`)
	require.NoError(t, p.addGenerated(dup))
	twin := mustGen(t, p, PoolPassage, `Late Twin`)
	assert.Error(t, p.addGenerated(twin))
	_, err = os.Stat(filepath.Join(dir, `gen-a-late-twin-2.yaml`))
	assert.True(t, os.IsNotExist(err))
}

func mustGen(t *testing.T, p *Profile, pool Pool, title string) *Template {
	t.Helper()
	req := p.genRequest(pool)
	req.UsedTitles, req.Openings = nil, nil
	var src *Template
	for _, tp := range p.templates[pool] {
		if tp.Source != `generated` {
			src = tp
			break
		}
	}
	r := asReply(t, src, req)
	r.Title = title
	tp, err := BuildGenerated(req, r)
	require.NoError(t, err)
	return tp
}

// A generated room that no longer passes is left out at load, never fatal;
// a broken authored room still is.
func TestLoad_SkipsBadGeneratedRooms(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(shippedDir)))
	rooms := filepath.Join(dir, `rooms`, `obelisk`)
	require.NoError(t, os.WriteFile(filepath.Join(rooms, `gen-a-broken.yaml`), []byte("id: gen-a-broken\npool: A\nsource: generated\ntitle: broken lowercase\ndescription: x\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(rooms, `gen-a-garbled.yaml`), []byte("id: [unclosed\n"), 0o644))
	loaded, err := ReadProfiles(dir)
	require.NoError(t, err)
	for _, tp := range loaded[`obelisk`].templates[PoolPassage] {
		assert.NotContains(t, tp.Id, `gen-a-`)
	}
	require.NoError(t, os.WriteFile(filepath.Join(rooms, `a-broken.yaml`), []byte("id: a-broken\npool: A\ntitle: broken lowercase\ndescription: x\n"), 0o644))
	_, err = ReadProfiles(dir)
	assert.Error(t, err)
}

// A door may not be named so that a command typed in its room walks through
// it instead: exits are matched first, by prefix.
func TestShadowsCommand(t *testing.T) {
	saved := commandNames
	SetCommandNames(func() []string { return []string{`drop`, `go`, `look`, `party chat`, `x`} })
	t.Cleanup(func() { commandNames = saved })

	for exit, cmd := range map[string]string{`drop`: `drop`, `dropway`: `drop`, `gorge`: `go`, `lookout`: `look`} {
		assert.Equal(t, cmd, shadowsCommand(exit), exit)
	}
	for _, exit := range []string{`plunge`, `fissure`, `stair`, `ledge`, `gap`, `arch`} {
		assert.Empty(t, shadowsCommand(exit), exit)
	}
}
