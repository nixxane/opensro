/*
===========================================================================

npcmagicpop_test.go - the Magic Pop machine is placed in the static NPC world

===========================================================================
*/
package simulation

import (
	"testing"

	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/testsupport/gamedatatest"
	"opensro.online/server/internal/testsupport/licensed"
)

/*
================
TestAppendMagicPopMachinesPlacesOnePerTown

The machine row comes from reference data alone, so a synthetic reference row
proves the placements, identities, facing and Magic Pop service without the
licensed data set.
================
*/
func TestAppendMagicPopMachinesPlacesOnePerTown(t *testing.T) {
	refs := map[uint32]npcWorldRef{9251: {
		refObjID: 9251, tidWord: 0x0146, codename: magicPopMachineCodename, scale: 1,
	}}
	roster := appendMagicPopMachines(nil, refs, worldNpcLookups{})
	if len(roster) != len(magicPopPlacements) {
		t.Fatalf("appended %d machines, want %d", len(roster), len(magicPopPlacements))
	}
	if err := ValidateNpcRoster(roster); err != nil {
		t.Fatal(err)
	}
	for index, npc := range roster {
		if npc.Spawn.RegionID != magicPopPlacements[index].RegionID {
			t.Fatalf("machine %d region = %d", index, npc.Spawn.RegionID)
		}
		if npc.TalkFlags&NpcTalkFlagGachaMachine == 0 {
			t.Fatalf("machine %d lacks the Magic Pop talk row", index)
		}
		if _, found := npcPlacementHeading(npc.Codename, npc.Spawn.RegionID, npc.Spawn.X, npc.Spawn.Y, npc.Spawn.Z); !found {
			t.Fatalf("machine %d has no recovered facing", index)
		}
	}
}

/*
================
TestAppendMagicPopMachinesSkipsMissingReference
================
*/
func TestAppendMagicPopMachinesSkipsMissingReference(t *testing.T) {
	if got := appendMagicPopMachines(nil, map[uint32]npcWorldRef{}, worldNpcLookups{}); len(got) != 0 {
		t.Fatalf("appended %d machines without a reference row", len(got))
	}
}

/*
================
TestShippedWorldHasMagicPopMachines

The published roster carries the machines beside the npcpos rows: unique
identities in the machine band, each with the Magic Pop talk row, and the
npcpos identities below it are untouched.
================
*/
func TestShippedWorldHasMagicPopMachines(t *testing.T) {
	licensed.RequireGameData(t)
	roster := LoadNpcWorldRoster(gamedatatest.TextdataDir(t))
	regions := map[uint16]bool{}
	for _, npc := range roster {
		if npc.Codename != magicPopMachineCodename {
			if npc.ObjectID >= magicPopGIDBase {
				t.Fatalf("%s id %d intrudes on the machine band", npc.Codename, npc.ObjectID)
			}
			continue
		}
		if npc.ObjectID < magicPopGIDBase || npc.ObjectID >= domain.GroundItemGIDBase {
			t.Fatalf("machine id %d outside its band", npc.ObjectID)
		}
		if npc.TalkFlags&NpcTalkFlagGachaMachine == 0 || npc.ModelPath == "" {
			t.Fatalf("machine %d: talk flags %#x model %q", npc.ObjectID, npc.TalkFlags, npc.ModelPath)
		}
		regions[npc.Spawn.RegionID] = true
	}
	for _, spawn := range magicPopPlacements {
		if !regions[spawn.RegionID] {
			t.Fatalf("no machine in region %d", spawn.RegionID)
		}
	}
}
