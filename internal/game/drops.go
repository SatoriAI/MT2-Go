package game

import (
	"context"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// A drop is its owner's alone for ownedFor, then anyone's, and lies on the ground for
// lieFor before it is gone; pickupReach is how near a player must be to take it, a blow's
// reach.
const (
	ownedFor    = 20 * time.Second
	lieFor      = 60 * time.Second
	pickupReach = 300
	dropSpread  = 100 // how far from the body a drop may land, either way
)

// owner is the character a drop is kept for, by id, which outlives a login, and by name,
// for the label over it. The zero owner is anyone.
type owner struct {
	id   uint32
	name string
}

// groundItem is an item lying in the world. Ground items are kept apart from the world
// record's characters, which they would slow, and never move, so who sees one follows
// from where the players stand. Guarded by the world's lock.
type groundItem struct {
	vid     uint32
	world   string
	x, y    int32
	vnum    uint32
	count   uint8
	gold    uint32 // yang, in place of an item
	owner   owner
	dropped time.Time
}

// timed is a ground item's vid and when its next change is due, for the world loop.
type timed struct {
	vid uint32
	at  time.Time
}

// dropOwner is who a dead monster's drops are kept for: the player who took the most of
// its hit points among those still in the world and alive, the killer on a tie, so a last
// blow does not take a drop from whoever fought it. Called under the lock.
func (w *presence) dropOwner(f *fight, killer *present) owner {
	best, most := killer, f.damageBy[killer]
	for p, damage := range f.damageBy {
		if damage > most && w.all[p.add.VID] == p && !p.dead() {
			best, most = p, damage
		}
	}
	return owner{id: best.char.ID, name: best.info.Name}
}

// dropFrom rolls what a dead monster leaves, each of its drops on its own, then its yang,
// and lays what falls near its body for its owner; the loop then frees each one and clears
// it away in turn.
func (s *Service) dropFrom(m *monster, d death, now time.Time) {
	var fallen []*groundItem
	for _, drop := range m.proto.Drops {
		if s.rand.IntN(10000) < int(drop.Chance) {
			fallen = append(fallen, &groundItem{vid: s.lastVID.Add(1), x: m.x + s.spread(dropSpread),
				y: m.y + s.spread(dropSpread), vnum: drop.VNum, count: drop.Count, owner: d.owner, dropped: now})
		}
	}
	if y := m.proto.Yang; s.rand.IntN(10000) < int(y.Chance) {
		fallen = append(fallen, &groundItem{vid: s.lastVID.Add(1), x: m.x + s.spread(dropSpread),
			y: m.y + s.spread(dropSpread), gold: y.Min + uint32(s.rand.IntN(int(y.Max-y.Min+1))), //nolint:gosec // a small range
			owner: d.owner, dropped: now})
	}
	if len(fallen) == 0 {
		return
	}
	s.presence.drop(m.p, fallen)
	for _, it := range fallen {
		s.freeing = append(s.freeing, timed{it.vid, now.Add(ownedFor)})
		s.vanishing = append(s.vanishing, timed{it.vid, now.Add(lieFor)})
	}
}

// keepGround frees the drops whose owner's time is up and clears away those that have lain
// long enough. Every drop has the same times, so each queue is in order and only its head
// is looked at; with nothing on the ground the loop does nothing.
func (s *Service) keepGround(now time.Time) {
	for len(s.freeing) > 0 && !now.Before(s.freeing[0].at) {
		s.presence.free(s.freeing[0].vid)
		s.freeing = s.freeing[1:]
	}
	for len(s.vanishing) > 0 && !now.Before(s.vanishing[0].at) {
		s.presence.vanish(s.vanishing[0].vid)
		s.vanishing = s.vanishing[1:]
	}
}

// drop lays the items on the ground where the body is, yang multiplied by the event that
// runs, and shows each, with its owner, to the players who see it.
func (w *presence) drop(body *present, items []*groundItem) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, it := range items {
		it.world = body.world
		if it.gold > 0 && w.event != nil {
			it.gold = w.event.Yang(it.gold)
		}
		w.ground[it.vid] = it
		w.toNear(it, shown(it)...)
	}
}

// free lets anyone pick the item up, if it still lies there.
func (w *presence) free(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if it := w.ground[vid]; it != nil {
		it.owner = owner{}
		w.toNear(it, protocol.EncodeItemOwner(vid, ""))
	}
}

// vanish clears the item away, if it still lies there.
func (w *presence) vanish(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if it := w.ground[vid]; it != nil {
		delete(w.ground, vid)
		w.toNear(it, protocol.EncodeGroundDel(vid))
	}
}

// shown is how a ground item comes into a player's view: laid down, then its owner's name,
// which the client takes only for an item it has.
func shown(it *groundItem) [][]byte {
	add := protocol.EncodeGroundAdd(it.vid, it.x, it.y, groundVNum(it))
	if it.owner.id == 0 {
		return [][]byte{add}
	}
	return [][]byte{add, protocol.EncodeItemOwner(it.vid, it.owner.name)}
}

// toNear sends packets to every player who sees the item. Called under the lock.
func (w *presence) toNear(it *groundItem, packets ...[]byte) {
	for _, p := range w.byName {
		if seesAt(p.world, p.add.X, p.add.Y, it) {
			p.send(packets...)
		}
	}
}

// seesAt is whether a player on world at (x, y) sees the item.
func seesAt(world string, x, y int32, it *groundItem) bool {
	dx, dy := int64(x)-int64(it.x), int64(y)-int64(it.y)
	return world == it.world && dx*dx+dy*dy <= viewRange*viewRange
}

// groundSeen is how the items on the ground come into or go out of view of a player who
// moved from (fromX, fromY) to where they stand. Called under the lock.
func (w *presence) groundSeen(p *present, fromX, fromY int32) [][]byte {
	var packets [][]byte
	for _, it := range w.ground {
		was, is := seesAt(p.world, fromX, fromY, it), seesAt(p.world, p.add.X, p.add.Y, it)
		switch {
		case is && !was:
			packets = append(packets, shown(it)...)
		case was && !is:
			packets = append(packets, protocol.EncodeGroundDel(it.vid))
		}
	}
	return packets
}

// groundInView is every item on the ground the player sees, as entering shows them, or,
// with gone, as leaving takes them away. Called under the lock.
func (w *presence) groundInView(p *present, gone bool) [][]byte {
	var packets [][]byte
	for _, it := range w.ground {
		switch {
		case !seesAt(p.world, p.add.X, p.add.Y, it):
		case gone:
			packets = append(packets, protocol.EncodeGroundDel(it.vid))
		default:
			packets = append(packets, shown(it)...)
		}
	}
	return packets
}

// pickResult is what came of a player's reach for an item on the ground.
type pickResult int

const (
	picked      pickResult = iota
	pickRefused            // not there, too far, theirs to see but not to take, or they are dead
	pickNoRoom             // nowhere in their bag for it
)

// pickUp takes the item off the ground for the player, if they may have it and room says
// they have room for it, and shows everyone near it that it is gone. room runs under the
// lock, so it must only look at what the player's own connection keeps. The first of two
// players to reach for an item has it; the second finds nothing.
func (w *presence) pickUp(playerVID, vid uint32, room func(*groundItem) bool) (groundItem, pickResult) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, it := w.all[playerVID], w.ground[vid]
	switch {
	case it == nil || p.dead() || p.world != it.world || !near(p.add.X, p.add.Y, it.x, it.y, pickupReach+1) ||
		(it.owner.id != 0 && it.owner.id != p.char.ID):
		return groundItem{}, pickRefused
	case !room(it):
		return groundItem{}, pickNoRoom
	}
	delete(w.ground, vid)
	w.toNear(it, protocol.EncodeGroundDel(vid))
	return *it, picked
}

// putBack lays an item that could not be kept down again where it was, if it would still
// lie there, with the owner it had, free if its owner's time is up.
func (w *presence) putBack(it groundItem, now time.Time) {
	if !now.Before(it.dropped.Add(lieFor)) {
		return
	}
	if !now.Before(it.dropped.Add(ownedFor)) {
		it.owner = owner{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ground[it.vid] = &it
	w.toNear(&it, shown(&it)...)
}

// onItemPickup takes an item off the ground into the player's bag: onto a stack of its kind
// with room for it, or the first cells it fits. The world record decides whether they may
// have it, asking about room under the world's lock, which is safe as only this connection
// touches its inventory. Without room, they are told and it stays where it lies. If it
// cannot be kept, it goes back on the ground; a write that timed out after the database
// took it would leave it both kept and lying there, which is left as it is for now.
func (s *session) onItemPickup(body []byte) error {
	vid, err := protocol.DecodeItemPickup(body)
	if err != nil {
		return err
	}
	var into item.Item
	var yang bool
	it, result := s.svc.presence.pickUp(s.playing.vid, vid, func(it *groundItem) bool {
		if yang = it.gold > 0; yang {
			return uint64(s.gold)+uint64(it.gold) <= maxGold
		}
		p, _ := item.Lookup(it.vnum)
		var ok bool
		into, ok = s.inventory.RoomFor(p, it.count)
		return ok
	})
	switch result {
	case pickRefused:
		s.conn.Log().Debug("pick-up refused", "vid", vid)
		return nil
	case pickNoRoom:
		if yang {
			s.conn.Log().Debug("yang past what a character may hold left lying", "vid", vid)
			return nil
		}
		return s.conn.Send(protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient("Nie masz miejsca w ekwipunku.")))
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if it.gold > 0 {
		return s.pickGold(ctx, it)
	}
	if into.ID != 0 {
		err = s.svc.items.SetCount(ctx, into.ID, into.Count)
	} else {
		into.ID, err = s.svc.items.Add(ctx, s.playing.ID, into)
	}
	if err != nil {
		s.conn.Log().Error("item picked up not kept", "vnum", it.vnum, "err", err)
		s.svc.presence.putBack(it, s.svc.now()) // after the write gave up, which may be seconds on
		return nil
	}
	if _, held := s.inventory.ByID(into.ID); held {
		s.inventory.Replace(into)
	} else {
		s.inventory = append(s.inventory, into)
	}
	return s.conn.Send(itemSet(into))
}

// maxGold is the most yang a character may hold, as the characters table checks too: a
// round cap under what the client's signed 32-bit points hold.
const maxGold = 2_000_000_000

// pickGold keeps the yang picked up and tells the player their new balance; the client
// says how much they got by itself. Yang that cannot be kept goes back on the ground.
func (s *session) pickGold(ctx context.Context, it groundItem) error {
	gold, err := s.svc.items.AddGold(ctx, s.playing.ID, it.gold)
	if err != nil {
		s.conn.Log().Error("yang picked up not kept", "amount", it.gold, "err", err)
		s.svc.presence.putBack(it, s.svc.now())
		return nil
	}
	s.gold = gold
	return s.conn.Send(protocol.EncodePointChange(s.playing.vid, protocol.PointGold, int32(it.gold), int32(gold))) //nolint:gosec // under 2^31
}
