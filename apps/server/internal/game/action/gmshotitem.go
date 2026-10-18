/*
===========================================================================

gmshotitem.go - the GM one-shot ring (port-only, not native)

SRO_GM_ONESHOT_ITEM off, the default, is native: the ring below is an
ordinary degree-1 ring and every hit uses the ordinary damage formula. On,
a GM wearing either fast-path ring kills any monster or unique in one hit
through the ordinary skill-impact plan, so EXP, loot, the kill lifecycle
and the damage number stay the normal path. Players, fortress structures and
bags are excluded: the ring must sit in an equipment socket (0..12) and the
attacker must be a live GM.

The ring is the shipped ITEM_CH_RING_01_A_RARE (5742) or ITEM_EU_RING_01_A
(14501), so it needs no new item row, icon or model. A GM spawns it with the
existing /MAKEITEM catalogue.

===========================================================================
*/
package action

import (
	"os"
	"strings"

	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/inventory"
	"opensro.online/server/internal/game/world/monster"
)

// EnvGMOneShotItem turns the GM one-shot ring on ("on", "1", "true").
const EnvGMOneShotItem = "SRO_GM_ONESHOT_ITEM"

// The fast-path rings, one per race. Both are degree-1 rings a GM can spawn
// and wear without new client data.
const (
	gmOneShotRingCH   = "ITEM_CH_RING_01_A_RARE"
	gmOneShotRingEU   = "ITEM_EU_RING_01_A"
	gmOneShotRingCHID = uint32(5742)
	gmOneShotRingEUID = uint32(14501)
)

/*
================
GMOneShotItemFromEnv
================
*/
func GMOneShotItemFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvGMOneShotItem))) {
	case "on", "1", "true":
		return true
	}
	return false
}

/*
================
wearsGMOneShotRing

The ring must be in an equipment socket (wire slots 0..12), not the bag.
================
*/
func wearsGMOneShotRing(c *enterworld.Character) bool {
	if c == nil {
		return false
	}
	for _, row := range c.MissionInventory {
		if row.Slot < 0 || row.Slot >= int64(inventory.EquipmentSlotEnd) {
			continue
		}
		if row.Codename == gmOneShotRingCH || row.Codename == gmOneShotRingEU ||
			row.RefObjID == gmOneShotRingCHID || row.RefObjID == gmOneShotRingEUID {
			return true
		}
	}
	return false
}

/*
================
gmOneShot

A live GM wearing the ring, against a monster or unique. Structures keep
their ordinary HP: they are not monsters for this tool.
================
*/
func (rt *Runtime) gmOneShot(c *enterworld.Character, target monster.Instance) bool {
	return rt != nil && rt.GMOneShotItem && c != nil && c.GMPrivilege && !c.DeletePending &&
		enterworld.CharacterAlive(c) && !target.Ref.Structure && wearsGMOneShotRing(c)
}
