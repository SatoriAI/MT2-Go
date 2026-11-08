// Package item is what each kind of item is, what a character owns, and where in its
// inventory an item fits. Only the kinds the game hands out are listed.
package item

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// MaxStack is the most of one kind a stack holds.
const MaxStack = 200

// Wear is where on a character an item is worn; "" for one that is not worn.
type Wear string

const (
	Weapon Wear = "weapon"
	Body   Wear = "body" // armour
	Head   Wear = "head" // a helmet
	Feet   Wear = "feet" // shoes
	Shield Wear = "shield"
)

// Proto is one kind of item, as the client's item_proto describes it.
type Proto struct {
	VNum  uint32 // the number the client knows the kind by, for its icon, name and tooltip
	Name  string
	Size  uint8 // how many cells tall it stands in the inventory, 1 to 3
	Stack uint8 // the most one item of it holds: 1 for a kind that does not stack

	// What a worn item is: where it goes, who may wear it, and what it adds to a blow or
	// sets against one.
	Wear      Wear
	Classes   []account.Class
	AttackMin uint16 // a weapon's attack, rolled for each blow
	AttackMax uint16
	Defence   uint16 // armour's, a helmet's, shoes' or a shield's
	MoveSpeed int8   // what shoes add to the speed the character moves at, and a shield takes off
	Sword     bool   // a one-handed sword, which some skills need in hand
	// Grade is how far the kind is upgraded, from +0 to MaxGrade, and Graded that it is
	// upgraded at all: each grade is the next vnum, as the client numbers them.
	Grade  uint8
	Graded bool

	// What a potion restores when it is drunk; a kind restoring neither is not drunk.
	HP, SP uint16

	// What a shop asks for one, and pays for one: the client's item_proto sell price
	// divided by 5, as its tooltip works it out, so what it shows is what is paid.
	Price, SellPrice uint32
}

var (
	// ErrNotWearable means the kind is not worn at all.
	ErrNotWearable = errors.New("item: not worn")
	// ErrWrongClass means the character's class cannot wear the kind.
	ErrWrongClass = errors.New("item: not for this class")
)

// CanWear reports why a character of class cannot wear the kind, or nil.
func (p Proto) CanWear(class account.Class) error {
	switch {
	case p.Wear == "":
		return ErrNotWearable
	case !slices.Contains(p.Classes, class):
		return ErrWrongClass
	}
	return nil
}

// Lookup returns the kind vnum, or false if the game has none of it.
func Lookup(vnum uint32) (Proto, bool) {
	p, ok := protos[vnum]
	return p, ok
}

// MaxGrade is the last grade gear is upgraded to.
const MaxGrade = 9

// VNums are the kinds the game has, in order.
func VNums() []uint32 { return slices.Sorted(maps.Keys(protos)) }

// protos are the client's item_proto values (--dump_proto--/item_proto.txt): size,
// stacking, ITEM_WEAR, the classes its ANTI_ flags leave, VALUE3 to VALUE4 for a weapon,
// VALUE1 for the rest, and its prices, but 100 for each first weapon, which it gives none.
// Only the first gear and its grades are listed, none needing a level. The names are the
// Polish client's own, from its locale item_proto (docs/mechanics/items.md), in UTF-8.
var protos = withGrades(map[uint32]Proto{
	10: {VNum: 10, Name: "Miecz+0", Size: 2, Stack: 1, Wear: Weapon,
		Classes: []account.Class{account.Warrior, account.Ninja, account.Sura}, AttackMin: 13, AttackMax: 15, Sword: true,
		Price: 100, SellPrice: 0},
	1000: {VNum: 1000, Name: "Sztylet+0", Size: 1, Stack: 1, Wear: Weapon,
		Classes: []account.Class{account.Ninja}, AttackMin: 8, AttackMax: 11, Price: 100, SellPrice: 0},
	2000: {VNum: 2000, Name: "Łuk+0", Size: 2, Stack: 1, Wear: Weapon,
		Classes: []account.Class{account.Ninja}, AttackMin: 7, AttackMax: 29, Price: 100, SellPrice: 0},
	3000: {VNum: 3000, Name: "Glewia+0", Size: 3, Stack: 1, Wear: Weapon,
		Classes: []account.Class{account.Warrior}, AttackMin: 14, AttackMax: 22, Price: 100, SellPrice: 0},
	7000: {VNum: 7000, Name: "Wachlarz+0", Size: 1, Stack: 1, Wear: Weapon,
		Classes: []account.Class{account.Shaman}, AttackMin: 11, AttackMax: 15, Price: 100, SellPrice: 0},
	11200: {VNum: 11200, Name: "Mnisia Zbr. Płyt.+0", Size: 2, Stack: 1, Wear: Body,
		Classes: []account.Class{account.Warrior}, Defence: 12, Price: 3000, SellPrice: 600},
	11400: {VNum: 11400, Name: "Błękitne Ubranie+0", Size: 2, Stack: 1, Wear: Body,
		Classes: []account.Class{account.Ninja}, Defence: 12, Price: 3000, SellPrice: 600},
	11600: {VNum: 11600, Name: "Żałobna Zbr. Płyt.+0", Size: 2, Stack: 1, Wear: Body,
		Classes: []account.Class{account.Sura}, Defence: 12, Price: 3000, SellPrice: 600},
	11800: {VNum: 11800, Name: "Błękitna Szata+0", Size: 2, Stack: 1, Wear: Body,
		Classes: []account.Class{account.Shaman}, Defence: 12, Price: 3000, SellPrice: 600},
	12200: {VNum: 12200, Name: "Tradycyjny Hełm+0", Size: 1, Stack: 1, Wear: Head,
		Classes: []account.Class{account.Warrior}, Defence: 5, Price: 500, SellPrice: 100},
	12340: {VNum: 12340, Name: "Skórzana Maska+0", Size: 1, Stack: 1, Wear: Head,
		Classes: []account.Class{account.Ninja}, Defence: 5, Price: 500, SellPrice: 100},
	12480: {VNum: 12480, Name: "Krwawy Hełm+0", Size: 1, Stack: 1, Wear: Head,
		Classes: []account.Class{account.Sura}, Defence: 5, Price: 500, SellPrice: 100},
	12620: {VNum: 12620, Name: "Czapka Mnicha+0", Size: 1, Stack: 1, Wear: Head,
		Classes: []account.Class{account.Shaman}, Defence: 5, Price: 500, SellPrice: 100},
	15000: {VNum: 15000, Name: "Skórzane Buty+0", Size: 1, Stack: 1, Wear: Feet,
		Classes: []account.Class{account.Warrior, account.Ninja, account.Sura, account.Shaman}, Defence: 1, MoveSpeed: 2,
		Price: 500, SellPrice: 100},
	13000: {VNum: 13000, Name: "Bojowa Tarcza+0", Size: 1, Stack: 1, Wear: Shield,
		Classes: []account.Class{account.Warrior, account.Ninja, account.Sura, account.Shaman}, Defence: 3, MoveSpeed: -2,
		Price: 400, SellPrice: 80},
	27001: {VNum: 27001, Name: "Czerwona Mikstura(M)", Size: 1, Stack: MaxStack, HP: 300, Price: 32, SellPrice: 6},
	27002: {VNum: 27002, Name: "Czerwona Mikstura(Ś)", Size: 1, Stack: MaxStack, HP: 800, Price: 96, SellPrice: 18},
	27004: {VNum: 27004, Name: "Niebieska Mikstura(M)", Size: 1, Stack: MaxStack, SP: 100, Price: 96, SellPrice: 24},
	30004: {VNum: 30004, Name: "Ząb Dzika", Size: 1, Stack: MaxStack, SellPrice: 100},
	30028: {VNum: 30028, Name: "Szpon Wilka", Size: 1, Stack: MaxStack, SellPrice: 100},
})

// gradeSteps are what each grade of the first gear adds over its +0: the client's
// item_proto VALUE5 per grade, which its tooltip adds to both ends of a weapon's attack
// and, twice, to the defence of anything else. Shoes add none; their grades are faster.
var gradeSteps = map[uint32]uint16{10: 7, 1000: 4, 2000: 8, 3000: 8, 7000: 7, 11200: 3, 11400: 3, 11600: 3, 11800: 3,
	12200: 2, 12340: 2, 12480: 2, 12620: 2, 15000: 0, 13000: 3}

// shoeSpeeds are what each grade of shoes adds to the speed: the client's APPLY_MOV_SPEED.
var shoeSpeeds = [MaxGrade + 1]int8{2, 3, 4, 5, 6, 7, 9, 11, 13, 15}

// gradeSales are what a shop pays for each grade, before the client's division by 5: its
// item_proto sell prices, the same for every first weapon, every first armour, and every
// helmet and pair of shoes.
var gradeSales = map[Wear][MaxGrade + 1]uint32{
	Weapon: {0, 100, 100, 150, 150, 200, 250, 350, 500, 750},
	Body:   {3000, 5000, 5000, 5000, 5000, 10000, 10000, 20000, 40000, 80000},
	Head:   {500, 800, 1100, 1700, 2500, 4000, 6000, 9000, 13000, 20000},
	Feet:   {500, 800, 1100, 1700, 2500, 4000, 6000, 9000, 13000, 20000},
	Shield: {400, 800, 1600, 2400, 5000, 7500, 10000, 13000, 15000, 20000},
}

// withGrades adds the grades +1 to +9 of each piece of the first gear to the kinds: named as
// the client names them, its +0's name with its grade, and no shop sells them.
func withGrades(kinds map[uint32]Proto) map[uint32]Proto {
	for vnum, step := range gradeSteps {
		base := kinds[vnum]
		base.Graded = true
		kinds[vnum] = base
		for g := uint8(1); g <= MaxGrade; g++ {
			p := base
			p.VNum, p.Grade, p.Price = vnum+uint32(g), g, 0
			p.Name = strings.TrimSuffix(base.Name, "+0") + "+" + strconv.Itoa(int(g))
			p.SellPrice = gradeSales[base.Wear][g] / 5
			bonus := uint16(g) * step
			switch p.Wear {
			case Weapon:
				p.AttackMin, p.AttackMax = base.AttackMin+bonus, base.AttackMax+bonus
			case Feet:
				p.MoveSpeed = shoeSpeeds[g]
			default:
				p.Defence = base.Defence + 2*bonus
			}
			kinds[p.VNum] = p
		}
	}
	return kinds
}

// Item is one item a character owns: a kind, how many of it, and either the inventory
// cell its top stands in or where it is worn.
type Item struct {
	ID    int64
	VNum  uint32
	Count uint8
	Cell  uint16 // meaningless while it is worn
	Worn  Wear
}

// The inventory is two pages of 45 cells, five to a row; an item stands in its column
// from its top cell down, on one page.
const (
	Cells     = 90
	PageCells = 45
	RowCells  = 5
)

// Inventory is what a character carries.
type Inventory []Item

// Fits reports whether an item of kind p fits with its top at cell: on one page, on no
// cell another item stands in. The item except, the one being moved, is not in the way.
func (inv Inventory) Fits(p Proto, cell uint16, except int64) bool {
	bottom := cell + uint16(p.Size-1)*RowCells
	if bottom >= Cells || bottom/PageCells != cell/PageCells {
		return false
	}
	for _, it := range inv {
		other, _ := Lookup(it.VNum)
		if it.Worn == "" && it.ID != except && it.Cell%RowCells == cell%RowCells &&
			it.Cell <= bottom && cell <= it.Cell+uint16(other.Size-1)*RowCells {
			return false
		}
	}
	return true
}

// Free returns the first cell an item of kind p fits at, or false if none is left.
func (inv Inventory) Free(p Proto) (uint16, bool) {
	for cell := range uint16(Cells) {
		if inv.Fits(p, cell, 0) {
			return cell, true
		}
	}
	return 0, false
}

// RoomFor is where count of kind p picked up goes: onto a stack of its kind with room for
// all of it, as that stack would then be, or a new item in the first cells it fits.
func (inv Inventory) RoomFor(p Proto, count uint8) (Item, bool) {
	for _, it := range inv {
		if it.VNum == p.VNum && it.Worn == "" && p.Stack > 1 && int(it.Count)+int(count) <= int(p.Stack) {
			it.Count += count
			return it, true
		}
	}
	cell, ok := inv.Free(p)
	return Item{VNum: p.VNum, Count: count, Cell: cell}, ok
}

// At returns the item whose top stands at cell.
func (inv Inventory) At(cell uint16) (Item, bool) {
	for _, it := range inv {
		if it.Worn == "" && it.Cell == cell {
			return it, true
		}
	}
	return Item{}, false
}

// ByID returns the item with the id.
func (inv Inventory) ByID(id int64) (Item, bool) {
	for _, it := range inv {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// Replace puts it in place of the item with its id.
func (inv Inventory) Replace(it Item) {
	for i := range inv {
		if inv[i].ID == it.ID {
			inv[i] = it
		}
	}
}

// WornIn returns the item worn in place, if any.
func (inv Inventory) WornIn(place Wear) (Item, bool) {
	for _, it := range inv {
		if it.Worn == place {
			return it, true
		}
	}
	return Item{}, false
}

// OrderKind is what an order does to what a character owns.
type OrderKind string

const (
	Give OrderKind = "give" // a new item of VNum, Count of it
	Take OrderKind = "take" // the item ItemID goes
)

// Order is a change to what a character owns, waiting for the character to be selected.
type Order struct {
	ID     int64
	Kind   OrderKind
	VNum   uint32
	Count  uint8
	ItemID int64
}

// GiveOrder is an order to give count of kind vnum, if the game has the kind and one item
// of it can hold that many.
func GiveOrder(vnum uint32, count int) (Order, error) {
	p, ok := Lookup(vnum)
	if !ok {
		return Order{}, fmt.Errorf("the game has no item %d", vnum)
	}
	if p.Stack == 1 && count != 1 {
		return Order{}, fmt.Errorf("%s does not stack: give 1 at a time", p.Name)
	}
	if count < 1 || count > int(p.Stack) {
		return Order{}, fmt.Errorf("a stack of %s holds 1 to %d, not %d", p.Name, p.Stack, count)
	}
	return Order{Kind: Give, VNum: vnum, Count: uint8(count)}, nil //nolint:gosec // at most the stack, 200
}
