# Mechanics

One page per game mechanic, written for a person who wants to understand the game rather than read the code. Each page answers the same questions in the same order:

1. **What the player sees** — the behaviour, in game terms.
2. **What the client does** — when it sends what, and what it expects back.
3. **The bytes** — every packet involved, field by field.
4. **What the server does** — the rule, and where it lives in the code.
5. **How we know** — the client script, the observed traffic or the open-mt2 fact it rests on.
6. **What proves it** — the tests.

Pages state facts in our own words. They never paste code from another server or from the client.

| Page | Covers |
|---|---|
| [connection-and-login.md](connection-and-login.md) | Connecting, the handshake, framing, logging in, the kingdom choice, the character list |
| [character-creation.md](character-creation.md) | The create screen: the packet, the rules, starting stats and positions, the replies |
| [entering-the-world.md](entering-the-world.md) | Start, the loading screen, the main character, points and the player appearing in the world |
| [chat.md](chat.md) | Packets with their own length, chat, and leaving the world through the game menu |
| [other-players.md](other-players.md) | Seeing the others in range: arriving, moving, talking, leaving, and the view range |
| [combat.md](combat.md) | Picking a monster, hitting it, its bar, its death and its return |
| [bow.md](bow.md) | A ninja shooting a monster with a bow: aiming, the shot, arrows used up, and others seeing it fly |
| [dying.md](dying.md) | A player's death to a monster, the restart dialog, and the experience it costs |
| [experience.md](experience.md) | Experience from kills, shared by damage; levels, stat points and their buttons |
| [equipment.md](equipment.md) | Putting on and taking off a weapon, body armour, a helmet, shoes and a shield, others seeing them, and what they add in a fight and to speed |
| [bonuses.md](bonuses.md) | Item bonuses: which gear carries them and how they roll, how they count in a fight, /bonusy, and the tooltip slots |
| [drops.md](drops.md) | What monsters leave and players drop, items and yang, who it is kept for, what lies on the ground, and picking it up |
| [taskbar.md](taskbar.md) | The bar of keys 1–4 and F1–F4: potions put on it, swapped, kept between logins, and following their items |
| [skills.md](skills.md) | Skill groups chosen at a class teacher, skill points, raising skills, and using Weaponry's skills |
| [shops.md](shops.md) | The village shopkeepers: opening a shop, buying and selling, and the prices |
| [trading.md](trading.md) | Trading between players: the window on both sides, offering items and yang, accepting, and the swap kept whole |
| [upgrading.md](upgrading.md) | The blacksmith: upgrading the first gear to +9, its cost, chance and materials, and the grades |
| [potions.md](potions.md) | Drinking red and blue potions: healing, mana, the stack and the effect |
| [items.md](items.md) | Item kinds, what a character owns, the inventory window, moving items, and giving or taking them on the command line |
| [events.md](events.md) | Scheduled events multiplying experience and yang, their command line and their notices |
| [monsters.md](monsters.md) | Village NPCs and monsters: where they stand, how they appear, and their strolls |
| [stones.md](stones.md) | Metin stones: where they stand, the waves they call, their smoke, what they leave, and their return |
| [whispers.md](whispers.md) | A line for one player by name, wherever they are, and keeping every one |

## Conventions for the bytes

- All integers are little-endian; structs have no padding (the client packs them to one byte).
- `u8`/`u16`/`u32` are unsigned, `i32` signed. `char[n]` is a fixed field of `n` bytes, NUL-padded, holding at most `n−1` characters.
- Sizes include the header byte. A client packet marked *sequenced* is followed by one more byte on the wire.
