/*
===========================================================================

npcmagicpop.go - the Magic Pop machine placements missing from npcpos.txt

The v1.150 npcpos.txt table places no NPC_CH_GACHA_MACHINE (RefObj 9251), so
the shipped world had Magic Pop tickets and a working roll path but no machine
to hand them to. The later server places the machine through its nest table
(Tab_RefNest); this file carries those placements for the towns v1.150 has and
appends them to the NPC world roster beside the npcpos rows.

===========================================================================
*/
package simulation

const (
	// magicPopMachineCodename is RefObj 9251, the machine the Magic Pop
	// reference data binds (see npcGachaMachines).
	magicPopMachineCodename = "NPC_CH_GACHA_MACHINE"

	// magicPopGIDBase starts the machine band inside the static NPC range:
	// above any npcpos row (the roster is a few hundred NPCs) and below the
	// teleport gates at 250000 + RefObjID. Identities stay stable when npcpos
	// rows are added.
	magicPopGIDBase uint32 = 240000
)

// worldNpcLookups are the reference joins every static NPC row shares,
// loaded once per roster build.
type worldNpcLookups struct {
	fortresses    map[string]uint32
	storeGroups   map[string][]NpcTalkStoreGroup
	fixtureByCode map[string]NpcDef
}

/*
================
magicPopPlacements

Tab_RefNest placements of NPC_CH_GACHA_MACHINE in the v1.188 server's nest
table, restricted to the towns that exist in v1.150: Jangan (25000), Donwhang
(26265), Hotan (23687) and Samarkand (27244). The fifth later placement, region
23088, is a town the v1.150 world has no NPC in, so it is left out. Every
position has a recovered facing in npc_headings_generated.go, and the
published-roster heading test keeps them joined.

INFERENCE: the later nest table is the evidence that the original placed one
machine per town; the v1.150 positions are taken from it unchanged.
================
*/
var magicPopPlacements = [...]Spawn{
	{RegionID: 25000, X: 1612.97, Y: 0, Z: 1193.0},
	{RegionID: 26265, X: 543.91, Y: -106.74, Z: 1533.36},
	{RegionID: 23687, X: 1652.38, Y: 243.82, Z: 776.57},
	{RegionID: 27244, X: 913.69, Y: 180, Z: 1828.76},
}

/*
================
newWorldNpc

One static NPC row from its reference data and placement: identity, facing,
shop groups and the service and talk words derived from them.
================
*/
func newWorldNpc(ref npcWorldRef, objectID uint32, spawn Spawn, lookups worldNpcLookups) NpcDef {
	spawn.Angle, _ = npcPlacementHeading(ref.codename, spawn.RegionID, spawn.X, spawn.Y, spawn.Z)
	row := NpcDef{
		ObjectID:          objectID,
		RefObjID:          ref.refObjID,
		FortressID:        lookups.fortresses[ref.fortress],
		TidWord:           ref.tidWord,
		Codename:          ref.codename,
		NameStrID:         ref.nameID,
		Name:              ref.name,
		ModelPath:         ref.model,
		Level:             ref.level,
		MaxHP:             ref.maxHP,
		BaseSpeechSymbol:  ref.base,
		QuestSpeechSymbol: ref.quest,
		AuthoredSpawn:     true,
		Spawn:             spawn,
		WalkSpeed:         ref.walk,
		RunSpeed:          ref.run,
		ScaleDenom:        ref.scale,
	}
	row.NpcTalkStoreGroups = cloneNpcStoreGroups(lookups.storeGroups[row.Codename])
	// Rebirth destinations are server gameplay policy and are not present in
	// the four shipped refshop tables. Preserve the pinned guide policy only;
	// shop rows are always rebuilt from authored data above.
	if pinned, found := lookups.fixtureByCode[row.Codename]; found {
		row.RebirthPoint = pinned.RebirthPoint
	}
	row.Services = ResolveNpcServices(row)
	row.TalkFlags = ResolveNpcTalkFlags(row)
	return row
}

/*
================
appendMagicPopMachines

Appends one machine per placement. A reference table without the machine row
(a partial data set) appends nothing rather than inventing a model-less NPC.
================
*/
func appendMagicPopMachines(roster []NpcDef, refs map[uint32]npcWorldRef, lookups worldNpcLookups) []NpcDef {
	var machine npcWorldRef
	found := false
	for _, ref := range refs {
		if ref.codename == magicPopMachineCodename {
			machine, found = ref, true
			break
		}
	}
	if !found {
		return roster
	}
	for index, spawn := range magicPopPlacements {
		roster = append(roster, newWorldNpc(machine, magicPopGIDBase+uint32(index), spawn, lookups))
	}
	return roster
}
