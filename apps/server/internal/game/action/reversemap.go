/*
===========================================================================

reversemap.go - the reverse return scroll's map destinations

Port-only, not native. v1.150's reverse return scroll offers two points,
the last recall point and the last death point (reversereturn.go). Later
clients add a third: "move to a location on the map". v1.188's
CGItemExpendable_UseReverseReturnScroll (4A00C0) reads that as choice 7
followed by a u32 point id, which it looks up in the optional-teleport
table (4A1F00 / 4A1ED0). v1.150 ships no such table, so the port builds
one at boot. When the operator imported the VSRO client's own table
(world/optionalteleport, scripts/import_vsro_optional_teleport.py), its
outdoor map rows are served under their own ids, the ids v1.188's client
sends, and a use outside a row's level band is refused. Without the
import the port infers one.

INFERENCE for the inferred table: every outdoor town recall gate (the
points appointed rebirth uses, OperatorTowns) and every outdoor unique
monster nest anchor. These are places the original already sends players
to, not arbitrary coordinates. The browser receives the table in its
public references, and a use sends back only a point id, which this owner
resolves. A position sent by the client is never accepted.

The option is off unless SRO_REVERSE_RETURN_MAP is on; off is native.
The map choice was first written by GrazKe in #336.

===========================================================================
*/
package action

import (
	"math"
	"os"
	"sort"
	"strings"

	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/world/instance"
	"opensro.online/server/internal/game/world/optionalteleport"
	"opensro.online/server/internal/game/world/simulation"
)

// EnvReverseReturnMap turns the map destinations on ("on", "1", "true").
const EnvReverseReturnMap = "SRO_REVERSE_RETURN_MAP"

const (
	// reverseReturnMapChoice is v1.188's saved-point choice (4A00C0 case 7).
	reverseReturnMapChoice uint8 = 7
	// reverseMapTailBytes is the choice byte and its u32 point id.
	reverseMapTailBytes = 5
	// maxReverseMapPoints bounds the public table.
	maxReverseMapPoints = 4096
	// reverseMapRegionSize is a region's local extent (x and z in [0, 1920)).
	reverseMapRegionSize = 1920
)

// reverseMapTownNames are the English names of the five town gates.
var reverseMapTownNames = map[string]string{
	"CH": "Jangan", "WC": "Donwhang", "KT": "Hotan", "EU": "Constantinople", "CA": "Samarkand",
}

/*
================
ReverseReturnMapFromEnv
================
*/
func ReverseReturnMapFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvReverseReturnMap))) {
	case "on", "1", "true":
		return true
	}
	return false
}

/*
================
ConfigureReverseReturnMap

Builds the destination table once at boot, after ConfigurePortals and the
monster authority. Disabled leaves it empty, so choice 7 stays refused.
An imported table (optionalteleport.Load) replaces the inferred one.
================
*/
func (rt *Runtime) ConfigureReverseReturnMap(enabled bool, imported []optionalteleport.Point) {
	rt.reverseMapPoints, rt.reverseMapRows = nil, nil
	if !enabled {
		return
	}
	if len(imported) > 0 {
		rt.useImportedReverseMap(imported)
		return
	}
	var points []enterworld.ReverseMapPoint
	if rt.portals != nil {
		for _, gate := range rt.portals.destinations {
			if !gate.recall || gate.ref == 0 || gate.fortressGate || gate.spawn.RegionID == 0 ||
				simulation.IsDungeonRegion(gate.spawn.RegionID) || portalWorld(gate) != instance.ID(domain.DefaultWorldInstance) {
				continue
			}
			name := strings.TrimPrefix(gate.code, "GATE_")
			if town, ok := reverseMapTownNames[name]; ok {
				name = town
			}
			points = append(points, enterworld.ReverseMapPoint{Name: name, RegionID: gate.spawn.RegionID,
				X: gate.spawn.X, Y: gate.spawn.Y, Z: gate.spawn.Z})
		}
	}
	if rt.Monsters != nil {
		for _, anchor := range rt.Monsters.UniqueReturnAnchors() {
			ref, ok := rt.Monsters.Reference(anchor.RefObjID)
			if !ok {
				continue
			}
			name := ref.Name
			if name == "" {
				name = ref.Codename
			}
			points = append(points, enterworld.ReverseMapPoint{Name: name, RegionID: anchor.RegionID,
				X: anchor.X, Y: anchor.Y, Z: anchor.Z})
		}
	}
	// A stable order keeps ids stable across restarts of the same data.
	sort.Slice(points, func(i, j int) bool {
		a, b := points[i], points[j]
		if a.RegionID != b.RegionID {
			return a.RegionID < b.RegionID
		}
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		return a.Name < b.Name
	})
	for _, point := range points {
		if !reverseMapPointValid(point) {
			continue
		}
		if n := len(rt.reverseMapPoints); n > 0 {
			last := rt.reverseMapPoints[n-1]
			if last.RegionID == point.RegionID && last.X == point.X && last.Z == point.Z {
				continue
			}
		}
		if len(rt.reverseMapPoints) == maxReverseMapPoints {
			break
		}
		point.ID = uint32(len(rt.reverseMapPoints) + 1)
		rt.reverseMapPoints = append(rt.reverseMapPoints, point)
	}
}

/*
================
useImportedReverseMap

The imported rows under their own ids. The public table holds the field
points and the dungeon entrances, each entrance listing its floors; the
destinations are the field points and the floors, with their level bands.
An entrance is never a destination, so an old client that sends its id is
refused and spends nothing.
================
*/
func (rt *Runtime) useImportedReverseMap(imported []optionalteleport.Point) {
	floors := map[uint32][]enterworld.ReverseMapFloor{}
	for _, point := range imported {
		if point.Kind == optionalteleport.KindFloor {
			floors[point.Group] = append(floors[point.Group], enterworld.ReverseMapFloor{ID: point.ID, Name: point.Name})
		}
	}
	rt.reverseMapRows = make(map[uint32]optionalteleport.Point, len(imported))
	for _, point := range imported {
		if point.Kind == optionalteleport.KindFloor {
			rt.reverseMapRows[point.ID] = point
			continue
		}
		public := enterworld.ReverseMapPoint{ID: point.ID, Name: point.Name, RegionID: point.RegionID,
			X: point.X, Y: point.Y, Z: point.Z}
		n := len(rt.reverseMapPoints)
		if !reverseMapPointValid(public) || n > 0 && rt.reverseMapPoints[n-1].ID >= point.ID {
			continue
		}
		if n == maxReverseMapPoints {
			break
		}
		if point.Kind == optionalteleport.KindEntrance {
			public.Floors = floors[point.Group]
		} else {
			rt.reverseMapRows[point.ID] = point
		}
		rt.reverseMapPoints = append(rt.reverseMapPoints, public)
	}
}

/*
================
ReverseReturnMapImported

True when the public table is the imported VSRO one, whose marker art the
client draws (the entry bootstrap's reverseMapImported).
================
*/
func (rt *Runtime) ReverseReturnMapImported() bool {
	return rt.reverseMapRows != nil
}

/*
================
reverseMapPointValid

An outdoor point inside its region with finite coordinates.
================
*/
func reverseMapPointValid(point enterworld.ReverseMapPoint) bool {
	if point.RegionID == 0 || simulation.IsDungeonRegion(point.RegionID) {
		return false
	}
	for _, v := range [3]float64{point.X, point.Y, point.Z} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return point.X >= 0 && point.X < reverseMapRegionSize && point.Z >= 0 && point.Z < reverseMapRegionSize
}

/*
================
ReverseReturnMapPoints

The public table: empty when the option is off.
================
*/
func (rt *Runtime) ReverseReturnMapPoints() []enterworld.ReverseMapPoint {
	return append([]enterworld.ReverseMapPoint(nil), rt.reverseMapPoints...)
}

/*
================
reverseMapDestination

The point an id names, or false for an unknown id, an entrance or a
disabled option. Every point, a dungeon floor too, is in the field world
(world 1). Inferred ids run 1..n in the public table.
================
*/
func (rt *Runtime) reverseMapDestination(id uint32) (travelPoint, bool) {
	destination := travelPoint{world: instance.ID(domain.DefaultWorldInstance)}
	if rt.reverseMapRows != nil {
		row, ok := rt.reverseMapRows[id]
		if !ok {
			return travelPoint{}, false
		}
		destination.spawn.RegionID = row.RegionID
		destination.spawn.X, destination.spawn.Y, destination.spawn.Z = row.X, row.Y, row.Z
		return destination, true
	}
	index := sort.Search(len(rt.reverseMapPoints), func(i int) bool { return rt.reverseMapPoints[i].ID >= id })
	if id == 0 || index == len(rt.reverseMapPoints) || rt.reverseMapPoints[index].ID != id {
		return travelPoint{}, false
	}
	point := rt.reverseMapPoints[index]
	destination.spawn.RegionID = point.RegionID
	destination.spawn.X, destination.spawn.Y, destination.spawn.Z = point.X, point.Y, point.Z
	return destination, true
}

/*
================
reverseMapAdmitsLevel

An imported point's level band (optionalteleport.Point.Admits); an
inferred point has none.
================
*/
func (rt *Runtime) reverseMapAdmitsLevel(id uint32, c *enterworld.Character) bool {
	point, banded := rt.reverseMapRows[id]
	if !banded {
		return true
	}
	level := int64(1)
	if c.Level != nil {
		level = *c.Level
	}
	return point.Admits(level)
}
