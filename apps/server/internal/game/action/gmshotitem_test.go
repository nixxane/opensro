/*
===========================================================================

gmshotitem_test.go - the GM one-shot ring's equipment and damage gates

===========================================================================
*/
package action

import (
	"testing"

	"opensro.online/server/internal/game/combat"
	"opensro.online/server/internal/game/enterworld"
)

/*
================
gmRingRow
================
*/
func gmRingRow(slot int64, refObjID uint32, codename string) enterworld.InventoryRow {
	return enterworld.InventoryRow{
		Slot: slot, RefObjID: refObjID, Codename: codename, TypeFlags: 0x1c, StackCount: 1,
	}
}

/*
================
TestGMOneShotRingRequiresEquipmentSlot
================
*/
func TestGMOneShotRingRequiresEquipmentSlot(t *testing.T) {
	for _, tc := range []struct {
		name     string
		slot     int64
		refObjID uint32
		codename string
		want     bool
	}{
		{"ch ring worn", 7, gmOneShotRingCHID, gmOneShotRingCH, true},
		{"eu ring worn", 8, gmOneShotRingEUID, gmOneShotRingEU, true},
		{"ch ring in bag", 13, gmOneShotRingCHID, gmOneShotRingCH, false},
		{"other ring worn", 7, 999999, "ITEM_CH_RING_02_A_RARE", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &enterworld.Character{}
			c.MissionInventory = append(c.MissionInventory, gmRingRow(tc.slot, tc.refObjID, tc.codename))
			if got := wearsGMOneShotRing(c); got != tc.want {
				t.Fatalf("wears ring = %v, want %v", got, tc.want)
			}
		})
	}
}

/*
================
TestGMOneShotPlanKillsOnlyForAWornGM
================
*/
func TestGMOneShotPlanKillsOnlyForAWornGM(t *testing.T) {
	rt, clock, character, target := newCombatTestRuntime(t, 500)
	character.GMPrivilege = true
	character.MissionInventory = append(character.MissionInventory, gmRingRow(7, gmOneShotRingCHID, gmOneShotRingCH))
	rt.GMOneShotItem = true

	plan := func() uint32 {
		plans, ok := rt.planMonsterImpacts(testDivision, character, enterworld.SkillRow{}, target,
			[]combat.Result{{Damage: 5}}, clock.NowMs())
		if !ok || len(plans) != 1 {
			t.Fatalf("impact planning: ok=%v plans=%+v", ok, plans)
		}
		return plans[0].Damage
	}

	if got := plan(); got != target.CurrentHP {
		t.Fatalf("one-shot damage = %d, want the target's %d HP", got, target.CurrentHP)
	}
	rt.GMOneShotItem = false
	if got := plan(); got != 5 {
		t.Fatalf("flag off damage = %d, want 5", got)
	}
	rt.GMOneShotItem = true
	character.GMPrivilege = false
	if got := plan(); got != 5 {
		t.Fatalf("non-GM damage = %d, want 5", got)
	}
	character.GMPrivilege = true
	character.MissionInventory[len(character.MissionInventory)-1].Slot = 13
	if got := plan(); got != 5 {
		t.Fatalf("bag-slot damage = %d, want 5", got)
	}
	character.MissionInventory[len(character.MissionInventory)-1].Slot = 7

	target.Ref.Structure = true
	if got := plan(); got != 5 {
		t.Fatalf("structure damage = %d, want 5", got)
	}
}
