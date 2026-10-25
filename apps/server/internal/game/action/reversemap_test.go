/*
===========================================================================

reversemap_test.go - the reverse return scroll's map destinations

Port-only, not native. Off, choice 7 is refused as v1.150 refuses it and
the public table is empty. On, a use names a published point id and the
server resolves it; an unknown or malformed id spends nothing.

===========================================================================
*/

package action

import (
	"bytes"
	"testing"
	"time"

	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/wire"
	"opensro.online/server/internal/game/world/instance"
	"opensro.online/server/internal/game/world/monster"
	"opensro.online/server/internal/game/world/optionalteleport"
	"opensro.online/server/internal/game/world/simulation"
	"opensro.online/server/internal/testsupport/gamedatatest"
	"opensro.online/server/internal/testsupport/licensed"
)

const (
	// reverseMapTestRegion is an outdoor region (bit 15 clear).
	reverseMapTestRegion uint16 = 25000
	// reverseMapTestDungeon is a dungeon region (bit 15 set).
	reverseMapTestDungeon uint16 = 0x8000 | 25000
)

/*
================
reverseMapFixture

The reverse return fixture with three recall gates: a town in the field,
one in a dungeon region and a fortress gate. Only the first is a map
destination.
================
*/
func reverseMapFixture(t *testing.T, enabled bool) (*Runtime, *enterworld.Character, *fakeClock) {
	t.Helper()
	rt, c, clock := reverseReturnFixture(t)
	field := instance.ID(domain.DefaultWorldInstance).Definition()
	rt.portals.destinations = map[uint32]portalDestination{
		1: {},
		2: {id: 2, ref: 20, code: "GATE_CH", recall: true, world: field,
			spawn: simulation.Spawn{RegionID: reverseMapTestRegion, X: 900, Y: 12, Z: 640}},
		3: {id: 3, ref: 30, code: "GATE_CAVE", recall: true, world: field,
			spawn: simulation.Spawn{RegionID: reverseMapTestDungeon, X: 100, Z: 100}},
		4: {id: 4, ref: 40, code: "GATE_FORT", recall: true, fortressGate: true, world: field,
			spawn: simulation.Spawn{RegionID: reverseMapTestRegion, X: 10, Z: 10}},
	}
	rt.ConfigureReverseReturnMap(enabled, nil)
	return rt, c, clock
}

/*
================
reverseMapTail

Choice 7 and a little-endian u32 point id.
================
*/
func reverseMapTail(id uint32) []byte {
	return []byte{reverseReturnMapChoice, byte(id), byte(id >> 8), byte(id >> 16), byte(id >> 24)}
}

/*
================
TestReverseMapOffKeepsTheNativeTwoPoints
================
*/
func TestReverseMapOffKeepsTheNativeTwoPoints(t *testing.T) {
	rt, c, _ := reverseMapFixture(t, false)
	if points := rt.ReverseReturnMapPoints(); len(points) != 0 {
		t.Fatalf("the native default publishes map points: %+v", points)
	}
	useReverseScroll(rt, c, reverseMapTail(1)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("choice 7 was accepted with the option off")
	}
}

/*
================
TestReverseMapOnTravelsToAPublishedPoint
================
*/
func TestReverseMapOnTravelsToAPublishedPoint(t *testing.T) {
	rt, c, clock := reverseMapFixture(t, true)
	points := rt.ReverseReturnMapPoints()
	if len(points) != 1 || points[0].ID != 1 || points[0].Name != "Jangan" || points[0].RegionID != reverseMapTestRegion {
		t.Fatalf("published points %+v, want the field town only", points)
	}
	out := useReverseScroll(rt, c, reverseMapTail(points[0].ID)...)
	assertOpcodes(t, out.Frames, 0x3122, wire.OpItemUseResponse, opCommerceItemReferences, wire.OpItemUseVisual)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
		t.Fatalf("the map choice did not spend one scroll and start the cast: %+v", out.Frames)
	}
	var sent []wire.Frame
	rt.PushCharacterFrames = func(_, _ string, f []wire.Frame) { sent = append(sent, f...) }
	clock.Advance(time.Second)
	rt.TickHook()(clock.NowMs())
	if len(sent) == 0 || sent[0].Opcode != enterworld.OpcodeResetClient {
		t.Fatalf("missing native reentry: %+v", sent)
	}
	got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{})
	if got.RegionID != points[0].RegionID || got.X != points[0].X || got.Z != points[0].Z {
		t.Fatalf("arrived at %+v, not the chosen point %+v", got, points[0])
	}
}

/*
================
TestReverseMapRefusesUnknownAndMalformedIDs
================
*/
func TestReverseMapRefusesUnknownAndMalformedIDs(t *testing.T) {
	rt, c, _ := reverseMapFixture(t, true)
	for _, tail := range [][]byte{reverseMapTail(0), reverseMapTail(2), reverseMapTail(1)[:4], {reverseReturnMapChoice}} {
		useReverseScroll(rt, c, tail...)
	}
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("an unknown or malformed point id spent the scroll or started a cast")
	}
}

/*
================
TestReverseReturnMapFromEnv
================
*/
func TestReverseReturnMapFromEnv(t *testing.T) {
	for value, want := range map[string]bool{"": false, "off": false, "0": false, "on": true, "1": true, "TRUE": true} {
		t.Setenv(EnvReverseReturnMap, value)
		if got := ReverseReturnMapFromEnv(); got != want {
			t.Fatalf("%s=%q: %v, want %v", EnvReverseReturnMap, value, got, want)
		}
	}
}

/*
================
TestReverseMapTableFromShippedData

On the shipped data the table holds the five towns and the outdoor unique
nests, every point named and inside its field region.
================
*/
func TestReverseMapTableFromShippedData(t *testing.T) {
	licensed.RequireGameData(t)
	dir := gamedatatest.TextdataDir(t)
	rt, _ := newTestRuntime(rebirthTestCharacter(20, 100), testItems())
	if err := rt.ConfigurePortals(dir); err != nil {
		t.Fatal(err)
	}
	rt.Monsters = simulation.NewMonsterState(monster.LoadTemplate(dir))
	rt.ConfigureReverseReturnMap(true, nil)
	points := rt.ReverseReturnMapPoints()
	towns := map[string]bool{}
	for index, point := range points {
		if point.ID != uint32(index+1) || point.Name == "" || !reverseMapPointValid(point) {
			t.Fatalf("invalid point %+v", point)
		}
		for _, town := range reverseMapTownNames {
			if point.Name == town {
				towns[town] = true
			}
		}
	}
	if len(towns) != len(reverseMapTownNames) || len(points) <= len(towns) {
		t.Fatalf("%d points, towns %v: want the five towns and the unique nests", len(points), towns)
	}
	t.Logf("%d reverse return map points", len(points))
}

/*
================
importedReverseMap

Two imported points with the VSRO table's sparse ids: an open field town
and a level-95 band.
================
*/
func importedReverseMap() []optionalteleport.Point {
	return []optionalteleport.Point{
		{ID: 26, Name: "Jangan", RegionID: reverseMapTestRegion, X: 995, Y: -32, Z: 1132},
		{ID: 39, Name: "Alexandria (South)", RegionID: reverseMapTestRegion, X: 663, Y: 863, Z: 526, LevelMin: 95, LevelMax: 999},
	}
}

/*
================
TestReverseMapImportedTableKeepsItsOwnIDs

On, an imported table replaces the inferred one and publishes its own
ids; a use names one of them and travels there.
================
*/
func TestReverseMapImportedTableKeepsItsOwnIDs(t *testing.T) {
	rt, c, clock := reverseReturnFixture(t)
	rt.ConfigureReverseReturnMap(true, importedReverseMap())
	points := rt.ReverseReturnMapPoints()
	if len(points) != 2 || points[0].ID != 26 || points[1].ID != 39 || points[0].Name != "Jangan" {
		t.Fatalf("published points %+v, want the imported ids 26 and 39", points)
	}
	for _, id := range []uint32{1, 25, 27} {
		useReverseScroll(rt, c, reverseMapTail(id)...)
	}
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("an id the imported table does not hold spent the scroll")
	}
	useReverseScroll(rt, c, reverseMapTail(26)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
		t.Fatal("the imported point did not start the cast")
	}
	rt.PushCharacterFrames = func(string, string, []wire.Frame) {}
	clock.Advance(time.Second)
	rt.TickHook()(clock.NowMs())
	if got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{}); got.X != 995 || got.Z != 1132 {
		t.Fatalf("arrived at %+v, not the imported point", got)
	}
}

/*
================
TestReverseMapImportedLevelBandRefusesWithoutSpending
================
*/
func TestReverseMapImportedLevelBandRefusesWithoutSpending(t *testing.T) {
	rt, c, _ := reverseReturnFixture(t)
	rt.ConfigureReverseReturnMap(true, importedReverseMap())
	out := useReverseScroll(rt, c, reverseMapTail(39)...)
	if len(out.Frames) != 1 || !bytes.Equal(out.Frames[0].Payload, wire.EncodeItemUseError(wire.ErrCodeItemUseLevelRequired)) {
		t.Fatalf("a level-1 use of a level-95 point answered %+v", out.Frames)
	}
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("the refused band spent the scroll or started a cast")
	}
	level := int64(95)
	c.Level = &level
	useReverseScroll(rt, c, reverseMapTail(39)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
		t.Fatal("a level-95 use of the band was refused")
	}
}

/*
================
TestReverseMapOffIgnoresAnImport

Off is native even when the game data carries the import.
================
*/
func TestReverseMapOffIgnoresAnImport(t *testing.T) {
	rt, c, _ := reverseReturnFixture(t)
	rt.ConfigureReverseReturnMap(false, importedReverseMap())
	if points := rt.ReverseReturnMapPoints(); len(points) != 0 {
		t.Fatalf("off publishes %+v", points)
	}
	useReverseScroll(rt, c, reverseMapTail(26)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("choice 7 was accepted with the option off")
	}
}

/*
================
TestReverseMapImportedEntranceListsItsFloors

An entrance is published with its floors and is never a destination; a
floor id takes the player into its dungeon region at the table's
coordinates.
================
*/
func TestReverseMapImportedEntranceListsItsFloors(t *testing.T) {
	const caveRegion uint16 = 0x8001
	rt, c, clock := reverseReturnFixture(t)
	rt.ConfigureReverseReturnMap(true, []optionalteleport.Point{
		{ID: 26, Kind: optionalteleport.KindField, Name: "Jangan", RegionID: reverseMapTestRegion, X: 995, Z: 1132},
		{ID: 29, Kind: optionalteleport.KindEntrance, Group: 1, Name: "Okmungwan Field", RegionID: reverseMapTestRegion, X: 900, Z: 900},
		{ID: 30, Kind: optionalteleport.KindFloor, Group: 1, Name: "Donwhang Stone Cave", RegionID: caveRegion, X: 1115, Z: -935},
	})
	points := rt.ReverseReturnMapPoints()
	if len(points) != 2 || points[1].ID != 29 || len(points[1].Floors) != 1 || points[1].Floors[0].ID != 30 ||
		!rt.ReverseReturnMapImported() {
		t.Fatalf("published %+v: want Jangan and the entrance listing floor 30", points)
	}
	useReverseScroll(rt, c, reverseMapTail(29)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("an entrance id spent the scroll")
	}
	useReverseScroll(rt, c, reverseMapTail(30)...)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
		t.Fatal("the floor did not start the cast")
	}
	rt.PushCharacterFrames = func(string, string, []wire.Frame) {}
	clock.Advance(time.Second)
	rt.TickHook()(clock.NowMs())
	if got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{}); got.RegionID != caveRegion || got.X != 1115 || got.Z != -935 {
		t.Fatalf("arrived at %+v, not the cave floor", got)
	}
}
