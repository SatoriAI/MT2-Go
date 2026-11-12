package item

import "github.com/SatoriAI/MT2-Go/internal/account"

// books are the skill books of the skills the game has: the client's per-skill books,
// 50400 and the client's number for the skill, whose item_proto VALUE0 names it, with the
// Polish client's names. Each is one to a cell, as the client does not stack them, and
// worth 1000 as item_proto prices it.
var books = []struct {
	vnum    uint32
	teaches account.Skill
	name    string
}{
	{50401, "three_way_cut", "Instr. Trzystr. Cięcie"},
	{50402, "sword_spin", "Instr. Wir Miecza"},
	{50403, "berserk", "Instr. Berserk"},
	{50404, "sword_aura", "Instr. Aura Miecza"},
	{50416, "spirit_strike", "Instr. Duchowe Uderzenie"},
	{50417, "bash", "Instr. Walnięcie"},
	{50418, "stump", "Instr. Tąpnięcie"},
	{50419, "strong_body", "Instr. Silne Ciało"},
	{50420, "sword_strike", "Instr. Uderzenie Miecza"},
	{50431, "ambush", "Instr. Zasadzka"},
	{50432, "fast_attack", "Instr. Szybki Atak"},
	{50433, "rolling_dagger", "Instr. Wirujący Sztylet"},
	{50446, "repetitive_shot", "Instr. Powtarz. Strzał"},
	{50447, "arrow_shower", "Instr. Deszcz Strzał"},
	{50448, "fire_arrow", "Instr. Ognista Strzała"},
	{50449, "feather_walk", "Instr. Bezszelest. Chód"},
	{50450, "poison_arrow", "Instr. Trująca Strzała"},
	{50461, "finger_strike", "Instr. Uderzenie Palcem"},
	{50462, "dragon_swirl", "Instr. Smoczy Wir"},
	{50463, "enchanted_blade", "Instr. Czarowane Ostrze"},
	{50465, "enchanted_armour", "Instr. Czarowana Zbroja"},
	{50476, "dark_strike", "Instr. Mrocz. Uderzenie"},
	{50477, "flame_strike", "Instr. Ognist. Uderzenie"},
	{50479, "dark_protection", "Instr. Mroczna Ochrona"},
	{50480, "ghost_strike", "Instr. Duchowy Cios"},
	{50481, "dark_orb", "Instr. Mroczna Sfera"},
	{50491, "flying_talisman", "Instr. Latający Talizman"},
	{50492, "shooting_dragon", "Instr. Strzelający Smok"},
	{50493, "dragon_roar", "Instr. Smoczy Skowyt"},
	{50494, "blessing", "Instr. Błogosławieństwo"},
	{50495, "reflection", "Instr. Odbicie"},
	{50496, "dragons_help", "Instr. Pomoc Smoka"},
	{50506, "lightning_throw", "Instr. Błyskawiczny Rzut"},
	{50507, "summon_lightning", "Instr. Przywoł. Błysk."},
	{50508, "lightning_claw", "Instr. Burzowy Szpon"},
	{50509, "heal", "Instr. Leczenie"},
	{50510, "swiftness", "Instr. Zwinność"},
	{50511, "attack_up", "Instr. Podwyż. Ataku"},
}

// SoulStone is Kamień Duchowy, which takes a Master at M10 on to Grand Master and raises
// one to G10 (docs/mechanics/skills.md).
const SoulStone = 50513

// The two helpers of a read (docs/mechanics/skills.md): Zwój Egzorcyzmu lets the next read
// its wait holds up go ahead, and Rada Pustelnika raises the next book read's chance.
const (
	ExorcismScroll = 71001
	HermitsAdvice  = 71094
)

// withBooks adds the skill books and the soul stone to the kinds.
func withBooks(kinds map[uint32]Proto) map[uint32]Proto {
	for _, b := range books {
		kinds[b.vnum] = Proto{VNum: b.vnum, Name: b.name, Size: 1, Stack: 1, Teaches: b.teaches, Price: 1000, SellPrice: 200}
	}
	kinds[SoulStone] = Proto{VNum: SoulStone, Name: "Kamień Duchowy", Size: 1, Stack: 1, Price: 1000, SellPrice: 200}
	kinds[ExorcismScroll] = Proto{VNum: ExorcismScroll, Name: "Zwój Egzorcyzmu", Size: 1, Stack: 1, Aid: account.NoReadWait, Bound: true}
	kinds[HermitsAdvice] = Proto{VNum: HermitsAdvice, Name: "Rada Pustelnika", Size: 1, Stack: 1, Aid: account.ReadBonus, Bound: true}
	return kinds
}

// BookOf returns the book that trains the skill.
func BookOf(s account.Skill) (Proto, bool) {
	for _, b := range books {
		if b.teaches == s {
			return protos[b.vnum], true
		}
	}
	return Proto{}, false
}

// Books are the vnums of every skill book, in order.
func Books() []uint32 {
	vnums := make([]uint32, 0, len(books))
	for _, b := range books {
		vnums = append(vnums, b.vnum)
	}
	return vnums
}
