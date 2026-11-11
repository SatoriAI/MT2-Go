package game

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
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
	bonuses item.Bonuses
	gold    uint32 // yang, in place of an item
	owner   owner
	dropped time.Time
}

// timed is a ground item's vid and when its next change is due, for the world loop.
type timed struct {
	vid uint32
	at  time.Time
}

func (t timed) due() time.Time { return t.at }

// topDamager is who a dead monster's drops are kept for: the player who took the most of
// its hit points among those still in the world and alive, the killer on a tie, so a last
// blow does not take a drop from whoever fought it.
func (l locked) topDamager(f *fight, killer *present) *present {
	best, most := killer, f.damageBy[killer]
	for p, damage := range f.damageBy {
		if damage > most && l.all[p.add.VID] == p && !p.dead() {
			best, most = p, damage
		}
	}
	return best
}

// dropFrom rolls what a dead monster leaves, each of its drops on its own, then its yang,
// and lays what falls near its body for its owner; the loop then frees each one and clears
// it away in turn.
func (svc *Service) dropFrom(m *monster, d death, now time.Time) {
	var fallen []*groundItem
	for _, drop := range m.proto.Drops {
		if svc.rand.IntN(10000) < dropChance(drop.Chance, d.lucky) {
			fallen = append(fallen, &groundItem{vid: svc.lastVID.Add(1), x: m.x + svc.spread(dropSpread),
				y: m.y + svc.spread(dropSpread), vnum: drop.VNum, count: drop.Count, owner: d.owner, dropped: now})
		}
	}
	for _, pick := range m.proto.Picks {
		if svc.rand.IntN(10000) < dropChance(pick.Chance, d.lucky && !pick.Steady) {
			fallen = append(fallen, &groundItem{vid: svc.lastVID.Add(1), x: m.x + svc.spread(dropSpread),
				y: m.y + svc.spread(dropSpread), vnum: pick.From[svc.rand.IntN(len(pick.From))], count: pick.Count, owner: d.owner, dropped: now})
		}
	}
	for _, it := range fallen {
		svc.rollBonuses(m, it)
	}
	if y := m.proto.Yang; svc.rand.IntN(10000) < int(y.Chance) {
		fallen = append(fallen, &groundItem{vid: svc.lastVID.Add(1), x: m.x + svc.spread(dropSpread),
			y: m.y + svc.spread(dropSpread), gold: y.Min + uint32(svc.rand.IntN(int(y.Max-y.Min+1))), //nolint:gosec // a small range
			owner: d.owner, dropped: now})
	}
	if len(fallen) == 0 {
		return
	}
	svc.record.drop(m.p, fallen)
	for _, it := range fallen {
		if it.bonuses.Remarkable() {
			p, _ := item.Lookup(it.vnum)
			svc.record.tellMap(m.p, fmt.Sprintf("%s zdobywa %s z niezwykłymi bonusami!", d.owner.name, strings.TrimSuffix(p.Name, "+0")))
		}
		svc.freeing = append(svc.freeing, timed{it.vid, now.Add(ownedFor)})
		svc.vanishing = inOrder(svc.vanishing, timed{it.vid, now.Add(lieFor)})
	}
}

// dropChance is an item's chance to drop, in 10,000: doubled, up to certain, for an owner
// wearing the thief's gloves. Yang is never doubled, as the debaters settled.
func dropChance(c uint16, doubled bool) int {
	if doubled {
		return min(10000, 2*int(c))
	}
	return int(c)
}

// wearsLuck reports whether the player wears what doubles drops, and its time has not run out.
func wearsLuck(p *present) bool {
	if p.timed == nil || p.timed.spent {
		return false
	}
	worn, _ := item.Lookup(p.timed.it.VNum)
	return worn.DoublesDrops
}

// rollBonuses gives a piece of gear that falls its bonuses (docs/mechanics/bonuses.md):
// from a stone, one to three, and from anything a level-30 weapon sometimes its pair.
// Each roll is logged, so the odds can be weighed against what players found.
func (svc *Service) rollBonuses(m *monster, it *groundItem) {
	p, _ := item.Lookup(it.vnum)
	if it.bonuses = item.Roll(p, m.proto.Kind == mob.Stone, svc.rand); len(it.bonuses) > 0 {
		svc.log.Info("bonuses rolled", "item", it.vnum, "monster", m.proto.VNum, "bonuses", it.bonuses)
	}
}

// inOrder puts t into the queue in order of when it is due. What a player's connection
// starts, a drop or a buff, is timed from then, which can fall after the tick it reaches
// the loop in, so it may be due after what the loop then queues itself; most come last, so
// the search is from the end.
func inOrder[T interface{ due() time.Time }](queue []T, t T) []T {
	i := len(queue)
	for i > 0 && queue[i-1].due().After(t.due()) {
		i--
	}
	return slices.Insert(queue, i, t)
}

// keepGround frees the drops whose owner's time is up and clears away those that have lain
// long enough. Every drop has the same times, so each queue is in order and only its head
// is looked at; with nothing on the ground the loop does nothing.
func (svc *Service) keepGround(now time.Time) {
	for len(svc.freeing) > 0 && !now.Before(svc.freeing[0].at) {
		svc.record.free(svc.freeing[0].vid)
		svc.freeing = svc.freeing[1:]
	}
	for len(svc.vanishing) > 0 && !now.Before(svc.vanishing[0].at) {
		svc.record.vanish(svc.vanishing[0].vid)
		svc.vanishing = svc.vanishing[1:]
	}
}

// drop lays the items on the ground where the body is, yang multiplied by the event that
// runs, and shows each, with its owner, to the players who see it.
func (w *record) drop(body *present, items []*groundItem) {
	l, unlock := w.lock()
	defer unlock()
	for _, it := range items {
		it.world = body.world
		if it.gold > 0 && l.event != nil {
			it.gold = l.event.Yang(it.gold)
		}
		l.lay(it)
		l.toNear(it, shown(it)...)
	}
}

// lay puts the item on the ground, by its vid and on its map.
func (l locked) lay(it *groundItem) {
	l.ground[it.vid] = it
	if l.groundOn[it.world] == nil {
		l.groundOn[it.world] = map[uint32]*groundItem{}
	}
	l.groundOn[it.world][it.vid] = it
}

// lift takes the item off the ground.
func (l locked) lift(it *groundItem) {
	delete(l.ground, it.vid)
	delete(l.groundOn[it.world], it.vid)
}

// layDown puts an item the player dropped on the ground, dx and dy from where they stand,
// shows it to everyone who sees it, and leaves it for the world loop to clear away in its
// time. It has no owner, so anyone may pick it up at once.
func (w *record) layDown(playerVID uint32, it *groundItem, dx, dy int32) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[playerVID]
	it.world, it.x, it.y = p.world, p.add.X+dx, p.add.Y+dy
	l.lay(it)
	l.toNear(it, shown(it)...)
	l.news.laid = append(l.news.laid, timed{it.vid, it.dropped.Add(lieFor)})
}

// free lets anyone pick the item up, if it still lies there.
func (w *record) free(vid uint32) {
	l, unlock := w.lock()
	defer unlock()
	if it := l.ground[vid]; it != nil {
		it.owner = owner{}
		l.toNear(it, protocol.EncodeItemOwner(vid, ""))
	}
}

// vanish clears the item away, if it still lies there.
func (w *record) vanish(vid uint32) {
	l, unlock := w.lock()
	defer unlock()
	if it := l.ground[vid]; it != nil {
		l.lift(it)
		l.toNear(it, protocol.EncodeGroundDel(vid))
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

// toNear sends packets to every player who sees the item.
func (l locked) toNear(it *groundItem, packets ...[]byte) {
	for _, p := range l.byName {
		if seesAt(p.world, p.add.X, p.add.Y, it) {
			p.send(packets...)
		}
	}
}

// seesAt is whether a player on world at (x, y) sees the item.
func seesAt(world string, x, y int32, it *groundItem) bool {
	return world == it.world && apart(x, y, it.x, it.y) <= viewRange*viewRange
}

// groundSeen is how the items on the ground come into or go out of view of a player who
// moved from (fromX, fromY) to where they stand.
func (l locked) groundSeen(p *present, fromX, fromY int32) [][]byte {
	var packets [][]byte
	for _, it := range l.groundOn[p.world] {
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
// with gone, as leaving takes them away.
func (l locked) groundInView(p *present, gone bool) [][]byte {
	var packets [][]byte
	for _, it := range l.groundOn[p.world] {
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
func (w *record) pickUp(playerVID, vid uint32, room func(*groundItem) bool) (groundItem, pickResult) {
	l, unlock := w.lock()
	defer unlock()
	p, it := l.all[playerVID], l.ground[vid]
	switch {
	case it == nil || p.dead() || p.world != it.world || !near(p.add.X, p.add.Y, it.x, it.y, pickupReach+1) ||
		(it.owner.id != 0 && it.owner.id != p.char.ID):
		return groundItem{}, pickRefused
	case !room(it):
		return groundItem{}, pickNoRoom
	}
	l.lift(it)
	l.toNear(it, protocol.EncodeGroundDel(vid))
	return *it, picked
}

// putBack lays an item that could not be kept down again where it was, if it would still
// lie there, with the owner it had, free if its owner's time is up.
func (w *record) putBack(it groundItem, now time.Time) {
	if !now.Before(it.dropped.Add(lieFor)) {
		return
	}
	if !now.Before(it.dropped.Add(ownedFor)) {
		it.owner = owner{}
	}
	l, unlock := w.lock()
	defer unlock()
	l.lay(&it)
	l.toNear(&it, shown(&it)...)
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
	it, result := s.svc.record.pickUp(s.playing.vid, vid, func(it *groundItem) bool {
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
		return s.conn.Send(infoLine("Nie masz miejsca w ekwipunku."))
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if it.gold > 0 {
		return s.pickGold(ctx, it)
	}
	if into.ID == 0 {
		into.Bonuses = it.bonuses
	}
	if into, err = s.receive(ctx, into); err != nil {
		s.conn.Log().Error("item picked up not kept", "vnum", it.vnum, "err", err)
		s.svc.record.putBack(it, s.svc.now()) // after the write gave up, which may be seconds on
		return nil
	}
	return s.conn.Send(itemSet(into))
}

// receive keeps into where RoomFor put it, a stack grown or a new item, and holds it.
func (s *session) receive(ctx context.Context, into item.Item) (item.Item, error) {
	var err error
	if into.ID != 0 {
		err = s.svc.items.SetCount(ctx, s.playing.ID, into.ID, into.Count)
	} else {
		into.ID, err = s.svc.items.Add(ctx, s.playing.ID, into)
	}
	if err != nil {
		return into, err
	}
	if _, held := s.inventory.ByID(into.ID); held {
		s.inventory.Replace(into)
	} else {
		s.inventory = append(s.inventory, into)
	}
	return into, nil
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
		s.svc.record.putBack(it, s.svc.now())
		return nil
	}
	s.gold = gold
	return s.conn.Send(s.yangChanged(int64(it.gold)))
}
