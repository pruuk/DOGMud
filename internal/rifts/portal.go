package rifts

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// portal.go: portal sites, the places in the world where a profile's way in
// (the Obelisk's floating crystal) appears, and the runs they open.
//
// Every real-world day the sites move: each region in a profile's
// portal_regions gets portal_sites_per_region of them, in rooms of the
// profile's portal_biomes (cities and roads are simply not among them). A
// site shows itself with a mutator (a line in the room description) and a
// noun, and while a player stands there it holds a pending run whose entry
// room its exit (portal_exit, e.g. `crystal`) leads into, so `enter crystal`,
// `go crystal` and `touch crystal` all work and followers follow. Once the
// first player goes in, the rest of a party has join_window_seconds to follow;
// then the site makes a fresh run for whoever comes next.
//
// A run remembers the room it was entered from (Run.OriginRoomId), and its
// way out leads back there even after the sites have moved on.

var (
	ErrNoProfile     = errors.New(`no such rift profile`)
	ErrNoRoom        = errors.New(`no such room`)
	ErrRoomEphemeral = errors.New(`a rift cannot open inside an instance or another rift`)
	ErrPortalTaken   = errors.New(`this room already has an exit by that name, or a portal`)
)

// Site is one place a profile's portal stands today.
type Site struct {
	RoomId    int
	ProfileId string
	Region    string
	run       *Run      // the pending or joinable run, if any
	retryAt   time.Time // after a failed build (no free chunk), wait until then
}

// SiteRecord and SiteState are what modules/rifts persists, so a reboot on
// the same day keeps the same sites.
type SiteRecord struct {
	RoomId    int    `yaml:"roomid"`
	ProfileId string `yaml:"profile"`
	Region    string `yaml:"region"`
}

type SiteState struct {
	Day   string       `yaml:"day"`
	Sites []SiteRecord `yaml:"sites"`
}

// SiteSettings are the operator's limits on where sites may appear.
type SiteSettings struct {
	ExcludeZones []string
}

var (
	sites    = map[int]*Site{}
	sitesDay string
)

// Today is the day key sites rotate on: the server's local calendar date.
func Today() string { return now().Format(`2006-01-02`) }

// Sites returns today's sites, by room id.
func Sites() []*Site {
	out := make([]*Site, 0, len(sites))
	for _, s := range sites {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoomId < out[j].RoomId })
	return out
}

// SiteAt returns the site in roomId, or nil.
func SiteAt(roomId int) *Site { return sites[roomId] }

// State is the persistable form of today's sites.
func State() SiteState {
	st := SiteState{Day: sitesDay}
	for _, s := range Sites() {
		st.Sites = append(st.Sites, SiteRecord{RoomId: s.RoomId, ProfileId: s.ProfileId, Region: s.Region})
	}
	return st
}

// Restore puts back sites saved earlier today. It returns false when the
// saved state is from another day (or none of its sites could be put back);
// the next DailyTick then chooses fresh sites. Either way the saved rooms of
// a stale day are cleared of their portal: the mutator line is saved with the
// room, and would otherwise describe a crystal that is no longer there.
func Restore(st SiteState) bool {
	if st.Day == `` || st.Day != Today() {
		for _, rec := range st.Sites {
			clearSiteRoom(rec.ProfileId, rec.RoomId)
		}
		return false
	}
	placed := 0
	for _, rec := range st.Sites {
		if err := addSite(rec.ProfileId, rec.RoomId, rec.Region, false); err != nil {
			mudlog.Warn(`rifts.Restore`, `room`, rec.RoomId, `error`, err)
			clearSiteRoom(rec.ProfileId, rec.RoomId)
			continue
		}
		placed++
	}
	if placed == 0 && len(st.Sites) > 0 {
		return false
	}
	sitesDay = st.Day
	return true
}

// clearSiteRoom takes a profile's portal mutator and noun out of a room that
// is no longer a site.
func clearSiteRoom(profileId string, roomId int) {
	p := GetProfile(profileId)
	if p == nil || sites[roomId] != nil {
		return
	}
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return
	}
	room.Mutators.Remove(p.PortalMutator)
	if room.Nouns[p.PortalExit] == p.PortalLook {
		delete(room.Nouns, p.PortalExit)
	}
}

// DailyTick moves the sites when the day has turned (or none were ever
// chosen). It returns true when it changed them, so the caller can save.
func DailyTick(s SiteSettings) bool {
	if sitesDay == Today() || len(profiles) == 0 {
		return false
	}
	Rotate(s)
	return true
}

// Rotate removes every site and chooses today's. Runs already entered are
// untouched: their way out still leads to where they came in.
func Rotate(s SiteSettings) {
	for _, site := range Sites() {
		removeSite(site, true)
	}
	for _, id := range ProfileIds() {
		p := profiles[id]
		for region, roomIds := range candidateRooms(p, s.ExcludeZones) {
			want := RollRange(p.PortalSites, rng)
			for i := len(roomIds) - 1; i > 0; i-- { // shuffle
				j := rng(i + 1)
				roomIds[i], roomIds[j] = roomIds[j], roomIds[i]
			}
			placed := 0
			for _, roomId := range roomIds {
				if placed >= want {
					break
				}
				if err := addSite(p.Id, roomId, region, true); err == nil {
					placed++
				}
			}
			mudlog.Info(`rifts.Rotate`, `profile`, p.Id, `region`, region, `sites`, placed)
		}
	}
	sitesDay = Today()
}

// AddSite places a site in roomId for the rest of today (admin).
func AddSite(profileId string, roomId int) error {
	region := ``
	if room := rooms.LoadRoom(roomId); room != nil {
		if zc := rooms.GetZoneConfig(room.Zone); zc != nil {
			region = zc.Region
		}
	}
	if err := addSite(profileId, roomId, region, true); err != nil {
		return err
	}
	if sitesDay == `` {
		sitesDay = Today()
	}
	return nil
}

// RemoveSite takes the site out of roomId (admin).
func RemoveSite(roomId int) bool {
	site := sites[roomId]
	if site == nil {
		return false
	}
	removeSite(site, true)
	return true
}

func addSite(profileId string, roomId int, region string, announce bool) error {
	p := GetProfile(profileId)
	if p == nil {
		return ErrNoProfile
	}
	if sites[roomId] != nil {
		return ErrPortalTaken
	}
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return ErrNoRoom
	}
	if room.IsEphemeral() {
		return ErrRoomEphemeral
	}
	if _, taken := room.Exits[p.PortalExit]; taken {
		return ErrPortalTaken
	}
	if look, taken := room.Nouns[p.PortalExit]; taken && look != p.PortalLook {
		return ErrPortalTaken // the room has a crystal of its own
	}
	site := &Site{RoomId: roomId, ProfileId: p.Id, Region: region}
	sites[roomId] = site
	site.show(room)
	if announce {
		room.SendTextVisual(messaging.CategoryRoomDescription, p.Msg(`portal_open`))
	}
	site.ensureRun()
	return nil
}

func removeSite(site *Site, announce bool) {
	delete(sites, site.RoomId)
	p := GetProfile(site.ProfileId)
	// Every run still joinable from here, not only the one waiting: a run
	// entered a moment ago keeps its join window, and without its site
	// nothing would route its portal exit any more.
	for _, run := range runs {
		if run.OriginRoomId == site.RoomId && run.PortalOpen {
			run.closePortal()
		}
	}
	room := rooms.LoadRoom(site.RoomId)
	if room == nil || p == nil {
		return
	}
	room.Mutators.Remove(p.PortalMutator)
	if room.Nouns[p.PortalExit] == p.PortalLook { // only the noun show() put there
		delete(room.Nouns, p.PortalExit)
	}
	if announce {
		room.SendTextVisual(messaging.CategoryRoomDescription, p.Msg(`portal_close`))
	}
}

// show makes the site visible: the mutator line in the description and the
// noun to look at. Nouns are not saved with a room, so this is repeated
// whenever the room may have been reloaded.
func (site *Site) show(room *rooms.Room) {
	p := GetProfile(site.ProfileId)
	if p == nil {
		return
	}
	if !room.Mutators.Has(p.PortalMutator) {
		room.Mutators.Add(p.PortalMutator)
		room.Mutators.Update(util.GetRoundCount())
	}
	if room.Nouns == nil {
		room.Nouns = map[string]string{}
	}
	room.Nouns[p.PortalExit] = p.PortalLook
}

// ensureRun gives the site a joinable run, with its exit, while a player is
// standing in the site's room. Nobody there: nothing is held.
func (site *Site) ensureRun() {
	room := rooms.LoadRoom(site.RoomId)
	if room == nil {
		return
	}
	site.show(room)
	if len(room.GetPlayers()) == 0 {
		return
	}
	if site.run != nil && runs[site.run.Id] != nil && site.run.PortalOpen {
		site.run.keepPortal()
		return
	}
	if now().Before(site.retryAt) {
		return
	}
	run, err := newRun(GetProfile(site.ProfileId), site.RoomId)
	if err != nil {
		// Every chunk in use, most likely: say so once, then try again in a
		// while rather than every round.
		mudlog.Error(`rifts.ensureRun`, `room`, site.RoomId, `error`, err)
		site.retryAt = now().Add(30 * time.Second)
		return
	}
	run.PortalOpen = true
	run.addPortalExit(room)
	site.run = run
}

// waitingRun is the site's run that nobody has claimed yet, built now if
// there is none (a chunk permitting): where the next player to enter who
// has no party run here is sent.
func (site *Site) waitingRun() *Run {
	if run := site.run; run != nil && runs[run.Id] != nil && run.PortalOpen && !run.Entered && len(run.company()) == 0 {
		return run
	}
	site.run = nil
	room := rooms.LoadRoom(site.RoomId)
	if room == nil || now().Before(site.retryAt) {
		return nil
	}
	run, err := newRun(GetProfile(site.ProfileId), site.RoomId)
	if err != nil {
		mudlog.Error(`rifts.waitingRun`, `room`, site.RoomId, `error`, err)
		site.retryAt = now().Add(30 * time.Second)
		return nil
	}
	run.PortalOpen = true
	run.addPortalExit(room)
	site.run = run
	return run
}

// OpenPortal places a site in roomId (if there is none) and returns its
// joinable run, building one even if nobody is standing there (admin, tests).
func OpenPortal(profileId string, roomId int) (*Run, error) {
	if sites[roomId] == nil {
		if err := AddSite(profileId, roomId); err != nil {
			return nil, err
		}
	}
	site := sites[roomId]
	if site.run == nil || runs[site.run.Id] == nil || !site.run.PortalOpen {
		run, err := newRun(GetProfile(site.ProfileId), roomId)
		if err != nil {
			return nil, err
		}
		run.PortalOpen = true
		if room := rooms.LoadRoom(roomId); room != nil {
			run.addPortalExit(room)
		}
		site.run = run
	}
	return site.run, nil
}

func (run *Run) addPortalExit(room *rooms.Room) {
	p := run.Profile
	room.AddTemporaryExit(p.PortalExit, exit.TemporaryRoomExit{
		RoomId: run.EntryRoomId,
		Title:  p.PortalTitle,
		// The run removes the exit itself; this is only a backstop.
		Expires: `24 real hours`,
	})
}

// keepPortal puts the portal exit back if the overworld room was unloaded and
// reloaded (temporary exits are not saved with the room).
func (run *Run) keepPortal() {
	room := rooms.LoadRoom(run.OriginRoomId)
	if room == nil {
		return
	}
	if t, ok := room.ExitsTemp[run.Profile.PortalExit]; ok && t.RoomId == run.EntryRoomId {
		return
	}
	run.addPortalExit(room)
}

// closePortal stops anyone else joining the run: the site's exit no longer
// leads into it. The site itself stays; it will make a new run for the next
// player who comes along.
func (run *Run) closePortal() {
	if !run.PortalOpen {
		return
	}
	run.PortalOpen = false
	room := rooms.LoadRoom(run.OriginRoomId)
	if room == nil {
		return
	}
	if t, ok := room.ExitsTemp[run.Profile.PortalExit]; ok && t.RoomId == run.EntryRoomId {
		room.RemoveTemporaryExit(t)
	}
}

// ClosePortal stops anyone else joining the run (admin).
func (run *Run) ClosePortal() { run.closePortal() }

// startJoinWindow is called when the first player goes in: the rest of the
// party has join_window_seconds to follow.
func (run *Run) startJoinWindow() {
	run.PortalUntil = now().Add(time.Duration(run.Profile.JoinWindowSecs) * time.Second)
}

// sweepSites keeps every site shown and holding a joinable run while someone
// stands at it.
func sweepSites() {
	for _, site := range Sites() {
		site.ensureRun()
	}
}

// PortalNoun reports whether thing names the portal in room (for `touch`).
func PortalNoun(room *rooms.Room, thing string) (exitName string, ok bool) {
	site := sites[room.RoomId]
	if site == nil {
		return ``, false
	}
	p := GetProfile(site.ProfileId)
	if p == nil {
		return ``, false
	}
	noun, _ := room.FindNoun(strings.ToLower(strings.TrimSpace(thing)))
	if noun != p.PortalExit {
		return ``, false
	}
	return p.PortalExit, true
}

// candidateRooms lists, by region, every room a site of p may stand in.
// It reads room templates rather than loading rooms, since it looks at all of
// them once a day.
func candidateRooms(p *Profile, excludeZones []string) map[string][]int {
	wantRegion := map[string]bool{}
	for _, r := range p.PortalRegions {
		wantRegion[strings.ToLower(r)] = true
	}
	wantBiome := map[string]bool{}
	for _, b := range p.PortalBiomes {
		wantBiome[strings.ToLower(b)] = true
	}
	excluded := map[string]bool{}
	for _, z := range excludeZones {
		excluded[strings.ToLower(z)] = true
	}
	startRoom := int(configs.GetSpecialRoomsConfig().StartRoom)

	out := map[string][]int{}
	for _, roomId := range rooms.GetAllRoomIds() {
		if rooms.IsEphemeralRoomId(roomId) || roomId == startRoom || sites[roomId] != nil || rooms.IsPrivateRoom(roomId) {
			continue
		}
		t := rooms.LoadRoomTemplate(roomId)
		if t == nil || excluded[strings.ToLower(t.Zone)] || t.IsBank || t.IsStorage || t.IsCharacterRoom || t.Pvp {
			continue
		}
		if _, taken := t.Exits[p.PortalExit]; taken {
			continue
		}
		if _, taken := t.Nouns[p.PortalExit]; taken {
			continue
		}
		zc := rooms.GetZoneConfig(t.Zone)
		if zc == nil || zc.Instanced || zc.NonCartesian || zc.Region == `` {
			continue
		}
		if skipZone(p, zc) {
			continue
		}
		if len(wantRegion) > 0 && !wantRegion[strings.ToLower(zc.Region)] {
			continue
		}
		biome := t.Biome
		if biome == `` {
			biome = zc.DefaultBiome
		}
		if !wantBiome[strings.ToLower(biome)] {
			continue
		}
		out[zc.Region] = append(out[zc.Region], roomId)
	}
	return out
}

// skipZone reports whether p's sites must stay out of a zone: a city (by its
// default biome) or a road (by a word in its name).
func skipZone(p *Profile, zc *rooms.ZoneConfig) bool {
	for _, b := range p.PortalSkipZoneBiomes {
		if strings.EqualFold(b, zc.DefaultBiome) {
			return true
		}
	}
	name := strings.ToLower(zc.Name)
	for _, w := range p.PortalSkipZoneWords {
		if w != `` && strings.Contains(name, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// String is a one-line description of a site (admin).
func (site *Site) String() string {
	state := `idle`
	if site.run != nil && runs[site.run.Id] != nil && site.run.PortalOpen {
		state = fmt.Sprintf(`run %d joinable`, site.run.Id)
	}
	return fmt.Sprintf(`room %d  %s  region %q  %s`, site.RoomId, site.ProfileId, site.Region, state)
}
