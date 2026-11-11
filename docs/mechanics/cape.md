# The bravery cape

## What the player sees

Peleryna Męstwa (70038) is sold by Handlarka Różności in each first village, for 1000 yang each or 20,000 for twenty, in stacks of up to 200. A double-click uses one up, and the monsters around the player turn and run at them, to be fought where they stand. Nothing else is shown: no line in the chat and no effect, as the veteran remembers the original.

- **What comes:** every living monster within 2500 of the player that fights nobody, passive ones too, the nearest 50 at most, whatever their level. One already fighting the player comes on, its fight renewed.
- **What stays:** monsters fighting another player, metin stones, and anything further off. Their packmates come only as the pack always does, once one of them is hit ([monsters.md](monsters.md)).
- **When:** at most once every two seconds. A use sooner, or by the dead, does nothing and keeps the cape. A cape is used up even when no monster is near.
- **Then** each pulled monster fights as any monster does: it gives up 10 s without a bite or a blow, or 5000 from where it was pulled or last bit. Those 10 s start once it could have reached the player, after any stun and at the speed it moves at then, slowed or not, so none turns back on the way.
- **Again:** a monster that just gave up, walking home, fights no one, so the next cape pulls it again. That is the original's way: a player keeps a field on them by wearing capes.

## What the client does

A double-click sends ITEM_USE ([items.md](items.md)), and the client shows the stack one fewer when the server sends the item again. It draws the pulled monsters running as it draws any chase, from their moves.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_USE `0B`, client | 4, then the sequence byte | the cape's position |
| ITEM_SET `14` | 42 | the stack one fewer, or vnum 0 for the last ([items.md](items.md)) |
| MOVE `03` | 24 | each pulled monster's steps, as in a fight ([combat.md](combat.md)) |

## What the server does

`internal/game/cape.go`:

- **The use** is the session's: it checks the two seconds and that the player is alive, uses one cape up as a potion is, and leaves the player in the world record's news for the loop.
- **The pull** is the world loop's, which owns every monster's fight. At its next tick it takes the player's position and the monsters in their view within 2500, nearest first, under one lock, then sets each that may come on the player, up to 50. One that is not fighting starts a fight, as a hit would start it, but no pack is rallied. One fighting the player has its fight renewed. Each fight counts its 10 s from now plus what is left of its stun and its walk to the player at its present speed, both read under the same lock.
- **A player who died or left** between the use and the tick pulls nothing; the cape is gone.

The rules were settled on 2026-11-11 in a debate between a Metin2 veteran and a game designer, with the veteran deciding ties:
- **The price:** the veteran's 1000 won over the designer's 300.
- **Consumption:** a cape is used up with nothing near. The designer would have kept it.
- **Levels:** high levels pull low monsters too. The designer would have kept the 10-level rule.
- **Agreed by both:** the radius and cap, which are open-mt2's too, leaving monsters fighting others alone, no rally from the pull, the two seconds, and counting the fight from arrival.

## How we know

- **The item:** the client's `--dump_proto--/item_proto.txt` (ITEM_USE, stackable, shop price 0) and the Polish `itemdesc.txt`, which says the cape's bright colours lure every monster from around. 70057 is the same cape, tradeable, for an item shop; the store sells 70038, which the client flags against trading, dropping and selling.
- **The rules:** open-mt2's cape use (radius 2500, cap 50, idle monsters only, two seconds) and the veteran's memory of the original.
- **Seen from a bot** against the dev server (2026-11-11): Ringo walked west of Jinno's start, to (959445, 278390), until the nearest monster she saw was within 1200, and wore a cape. The stack went from 9 to 8, and all four monsters within 2500 moved towards her in the next four seconds.
- **Seen in MT2-Godot** (its 6158bd6, which knows the cape's kind): on Jinno's first map, a cape used up its stack of one. Within 2.5 s the four Alfa Wilk and a Niebieski Wilk near the character ran at her and began biting.

## What proves it

- `internal/game/cape_test.go`:
  - idle monsters within 2500 pulled, however far below the player's level, one further off not, and a cape used;
  - a pulled monster's fight counted from its arrival, and a stunned one's from after its stun;
  - one already on the player renewed without fighting twice;
  - a player who left or died before the tick pulling nothing;
  - a stone left;
  - a monster fighting another player left on them, its fight unchanged;
  - at most the nearest 50;
  - a cape used up with nothing near;
  - a second within two seconds kept, one after worn;
  - the dead wearing none.
- `internal/game/shops_test.go`: the general store's window with the cape at cells 3 and 8.
