package npcidle

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Limits on what a request carries, so one is a few hundred tokens.
const (
	maxDescription     = 700 // runes of the NPC's description
	maxRoomDescription = 900 // runes of the room's description
	maxListed          = 10  // wares, floor items, others present
	maxExamples        = 4   // the NPC's own set lines, for its voice
)

// NPC is the NPC a moment is for: authored text only.
type NPC struct {
	Name        string
	Description string
	Merchant    bool
	Wares       []string // what a merchant has on hand, by item name
	Examples    []string // a few of its own set idle lines ("emote ...", "say ...")
}

// Place is the room the NPC is in: authored text only.
type Place struct {
	Title       string
	Description string
	Biome       string
	Zone        string
}

// Request is everything a generator is told. Players in the room are only
// counted (Travellers), never named, and nothing a player wrote is in it:
// floor items are named by their model-safe names (items.Item.ModelName).
type Request struct {
	NPC        NPC
	Place      Place
	Others     []string // other NPCs present, by name
	FloorItems []string // what lies about, by name
	Travellers int      // players present
	TimeOfDay  string   // "day" or "night"
}

// Snapshot builds a Request for mob in room. Runs under the mud lock.
func Snapshot(mob *mobs.Mob, room *rooms.Room) Request {
	req := Request{
		NPC: NPC{
			Name:        plain(mob.Character.Name, 80),
			Description: plain(mob.Character.GetDescription(), maxDescription),
			Merchant:    mob.HasShop(),
		},
		Place: Place{
			Title:       plain(room.Title, 120),
			Description: plain(room.GetDescription(), maxRoomDescription),
			Biome:       plain(room.Biome, 40),
			Zone:        plain(room.Zone, 60),
		},
		Travellers: len(room.GetPlayers()),
		TimeOfDay:  `day`,
	}
	if gametime.GetDate().Night {
		req.TimeOfDay = `night`
	}
	if req.NPC.Merchant {
		req.NPC.Wares = wares(mob)
	}
	for _, cmd := range mob.IdleCommands {
		if len(req.NPC.Examples) >= maxExamples {
			break
		}
		if IsFlavor(cmd) {
			req.NPC.Examples = append(req.NPC.Examples, plain(cmd, 240))
		}
	}

	others := newNameList()
	for _, id := range room.GetMobs() {
		if id == mob.InstanceId {
			continue
		}
		if o := mobs.GetInstance(id); o != nil {
			others.add(o.Character.Name)
		}
	}
	req.Others = others.names

	floor := newNameList()
	for i := range room.Items {
		floor.add(room.Items[i].ModelName())
	}
	req.FloorItems = floor.names
	return req
}

// wares is what a merchant has on hand: its shop's live stock, else the
// stock its template lists.
func wares(mob *mobs.Mob) []string {
	list := newNameList()
	if inv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId); inv != nil {
		for _, e := range inv.Stock {
			if e.Current > 0 {
				if spec := items.GetItemSpec(e.ItemId); spec != nil {
					list.add(spec.Name)
				}
			}
		}
	}
	if len(list.names) == 0 {
		for _, s := range mob.Character.Shop {
			if s.ItemId > 0 && (s.Quantity > 0 || s.QuantityMax == 0) {
				if spec := items.GetItemSpec(s.ItemId); spec != nil {
					list.add(spec.Name)
				}
			}
		}
	}
	return list.names
}

// nameList collects up to maxListed distinct, non-empty plain names.
type nameList struct {
	seen  map[string]bool
	names []string
}

func newNameList() *nameList { return &nameList{seen: map[string]bool{}} }

func (l *nameList) add(name string) {
	name = plain(name, 60)
	key := strings.ToLower(name)
	if name == `` || l.seen[key] || len(l.names) >= maxListed {
		return
	}
	l.seen[key] = true
	l.names = append(l.names, name)
}

// plain strips markup and folds typography (baubles.PlainText, the same
// rule every model prompt's room text goes through), then cuts it to at
// most n runes.
func plain(s string, n int) string {
	s = baubles.PlainText(s)
	if r := []rune(s); len(r) > n {
		s = strings.TrimSpace(string(r[:n])) + `...`
	}
	return s
}

// sendToKeyholder shows a moment the server could not moderate to the
// player whose key wrote it, and to no one else, if they are still there.
// It goes through the room's own sight-gated sender with every other player
// excluded, so the light rules hold exactly as for any NPC emote, judged
// now: nothing in the dark, "a figure" at shapes. A say is shown as a seen
// mutter rather than heard speech: actions.Say owns the say line and would
// reach the whole room.
func sendToKeyholder(userId int, room *rooms.Room, mobName string, res Result) {
	u := users.GetByUserId(userId)
	if u == nil || room == nil || u.Character.RoomId != room.RoomId {
		return
	}
	others := []int{}
	for _, uid := range room.GetPlayers() {
		if uid != userId {
			others = append(others, uid)
		}
	}
	body := res.Text
	if res.Kind == KindSay {
		body = `mutters, "` + res.Text + `"`
	}
	room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
		actions.FormatEmoteText(mobName, body, `mobname`), []string{mobName}, others...)
}

// seesClearly reports whether a player in room sees it clearly now. Only
// such a player's key writes a moment: the moment is about what can be seen
// there, and a player in the dark gets the set line under its own rules.
func seesClearly(userId int, light messaging.RoomVisibility) bool {
	u := users.GetByUserId(userId)
	return u != nil && light != nil && messaging.CanSeeClearly(u.Character, light)
}
