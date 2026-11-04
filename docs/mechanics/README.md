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
| [monsters.md](monsters.md) | Village NPCs and monsters: where they stand, how they appear, and their strolls |
| [whispers.md](whispers.md) | A line for one player by name, wherever they are, and keeping every one |

## Conventions for the bytes

- All integers are little-endian; structs have no padding (the client packs them to one byte).
- `u8`/`u16`/`u32` are unsigned, `i32` signed. `char[n]` is a fixed field of `n` bytes, NUL-padded, holding at most `n−1` characters.
- Sizes include the header byte. A client packet marked *sequenced* is followed by one more byte on the wire.
