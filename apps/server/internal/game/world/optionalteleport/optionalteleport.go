/*
===========================================================================

optionalteleport.go - the later client's reverse return map table

Port-only, not native. v1.150 ships no optional-teleport table; its
reverse return scroll offers only the last recall and last death points.
v1.188's CGItemExpendable_UseReverseReturnScroll (4A00C0) reads choice 7
with a u32 id and looks the id up in this table (4A1F00 / 4A1ED0). The
operator imports the VSRO client's refoptionalteleport.txt
(scripts/import_vsro_optional_teleport.py) and the server bundle projects
it as textdata/vsro-optional-teleport.json; without the import there is no
file and the action runtime keeps its inferred table (reversemap.go).

This package owns which rows the port serves, all in service and in the
field world (WorldID 1, which also holds v1.150's dungeons: teleportdata's
GATE_DUNGEON_DH_OUT and GATE_JINSI_* rows are world 1):

  - a field point: drawn on the map (MapPoint 1), in no dungeon group
    (RegionIDGroup -1), in an outdoor region;
  - a dungeon entrance: drawn on the map in a group (RegionIDGroup > 0).
    VSRO names these rows "flag position only" (rows 29 and 31); they sit
    where worldmap_instanceinfo puts the dungeon on the world map
    (Worldmap_DONWHANG, Worldmap_JINSI). An entrance is not a destination;
  - a dungeon floor: not drawn (MapPoint 0) in the same group, in a dungeon
    region. The tomb's floors (regions 0x8002..0x8007, Worldmap_JINSI)
    are served only while SRO_QINSHI_TOMB opens the tomb, as its doors are.

Every region must be one the loaded textdata names (textzonename) and the
Alexandria overlay does not hide, so Alexandria's rows stay out unless its
regions are loaded and SRO_ALEXANDRIA is on. An entrance without a served
floor is dropped, and so is a floor without an entrance.

INFERENCE: LevelMin / LevelMax are the table's level band (0 means no
limit); the port refuses a use outside it. The shipped rows use 0/0,
50/999, 70/999 and 95/999. v1.150 has no dungeon map pages (its
worldmap_mapinfo has no Dungeonmap section), so the client lists an
entrance's floors when the entrance is clicked; the entrance takes the
English name of the field region it stands in.

===========================================================================
*/
package optionalteleport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"

	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/world/alexandria"
	"opensro.online/server/internal/game/world/simulation"
)

// File is the projected import, inside the textdata directory.
const File = "vsro-optional-teleport.json"

const (
	importFormat = "opensro-vsro-optional-teleport-v1"
	// maxImportBytes bounds the file (the VSRO table is 38 rows, ~8 KB).
	maxImportBytes = 1 << 20
	// maxPoints bounds the served table (the public reference bound).
	maxPoints = 4096
	// maxNameBytes bounds a point name (the public reference bound).
	maxNameBytes = 128
	// regionSize is a region's local extent (x and z in [0, 1920)).
	regionSize = 1920

	serviceOn       = 1
	fieldWorldID    = 1
	mapPointShown   = 1
	mapPointHidden  = 0
	noRegionGroup   = -1
	zoneNameColumns = 9
	zoneNameRegion  = 1
	zoneNameEnglish = 8

	// The Qin-Shi tomb's floor regions, B6 (0x8002) to B1 (0x8007):
	// worldmap_instanceinfo Worldmap_JINSI, regions 2..7 of row 128.
	tombFirstRegion uint16 = 0x8002
	tombLastRegion  uint16 = 0x8007
)

/*
================
Kind

What a served row is (see the file header).
================
*/
type Kind uint8

const (
	KindField Kind = iota
	KindEntrance
	KindFloor
)

/*
================
Point

One served row, by the table's own id. Group ties an entrance to its
floors; it is zero for a field point.
================
*/
type Point struct {
	ID       uint32
	Kind     Kind
	Group    uint32
	Name     string
	RegionID uint16
	X, Y, Z  float64
	LevelMin int64
	LevelMax int64
}

/*
================
Options

The world switches a row depends on.
================
*/
type Options struct {
	// QinShiTomb is SRO_QINSHI_TOMB (world/monster QinShiTombFromEnv).
	QinShiTomb bool
}

/*
================
Point.Admits

The level band: zero on either side is no limit there.
================
*/
func (p Point) Admits(level int64) bool {
	if p.LevelMin > 0 && level < p.LevelMin {
		return false
	}
	return p.LevelMax <= 0 || level <= p.LevelMax
}

type importRow struct {
	Service     int64   `json:"service"`
	ID          int64   `json:"id"`
	Zone        string  `json:"zone"`
	Name        string  `json:"name"`
	RegionID    int64   `json:"regionId"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           float64 `json:"z"`
	WorldID     int64   `json:"worldId"`
	RegionGroup int64   `json:"regionGroup"`
	MapPoint    int64   `json:"mapPoint"`
	LevelMin    int64   `json:"levelMin"`
	LevelMax    int64   `json:"levelMax"`
}

type importFile struct {
	Format string          `json:"format"`
	Source json.RawMessage `json:"source"`
	Rows   []importRow     `json:"rows"`
}

/*
================
Load

The served points of the projected import, sorted by id. A missing file
is a game root without the import (nil, nil); a present file must parse
exactly.
================
*/
func Load(textdataDir string, options Options) ([]Point, error) {
	path := filepath.Join(textdataDir, File)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("optional teleport: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxImportBytes {
		return nil, fmt.Errorf("optional teleport %s: not a regular file of at most %d bytes", path, maxImportBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("optional teleport: %w", err)
	}
	var raw importFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("optional teleport %s: %w", path, err)
	}
	if raw.Format != importFormat || len(raw.Rows) == 0 {
		return nil, fmt.Errorf("optional teleport %s: format %q with %d rows", path, raw.Format, len(raw.Rows))
	}
	hidden, err := alexandria.Hidden(textdataDir)
	if err != nil {
		return nil, err
	}
	return selectPoints(raw.Rows, selection{zones: zoneNames(textdataDir), hidden: hidden, options: options})
}

/*
================
zoneNames

Region id -> English zone name, from the loaded textzonename.
================
*/
func zoneNames(textdataDir string) map[uint16]string {
	names := map[uint16]string{}
	for _, row := range enterworld.ReadTextdataFile(filepath.Join(textdataDir, "textzonename.txt")) {
		if len(row) < zoneNameColumns || row[0] != "1" {
			continue
		}
		region, err := strconv.ParseUint(row[zoneNameRegion], 10, 16)
		if err != nil {
			continue
		}
		if _, seen := names[uint16(region)]; !seen {
			names[uint16(region)] = row[zoneNameEnglish]
		}
	}
	return names
}

/*
================
selection

What selectPoints checks a row against.
================
*/
type selection struct {
	zones   map[uint16]string
	hidden  *alexandria.Overlay
	options Options
}

/*
================
selectPoints

The rows the port serves (see the file header), sorted by id. A repeated
id refuses the table.
================
*/
func selectPoints(rows []importRow, s selection) ([]Point, error) {
	var points []Point
	seen := map[int64]bool{}
	entrances, floors := map[uint32]bool{}, map[uint32]bool{}
	for _, row := range rows {
		if row.ID < 1 || row.ID > math.MaxUint32 || seen[row.ID] {
			return nil, fmt.Errorf("optional teleport: row id %d is not a new positive id", row.ID)
		}
		seen[row.ID] = true
		point, ok := s.point(row)
		if !ok {
			continue
		}
		switch point.Kind {
		case KindEntrance:
			entrances[point.Group] = true
		case KindFloor:
			floors[point.Group] = true
		}
		points = append(points, point)
	}
	// An entrance needs a floor to list, a floor an entrance to be chosen at.
	points = slices.DeleteFunc(points, func(p Point) bool {
		return p.Kind != KindField && (!entrances[p.Group] || !floors[p.Group])
	})
	if len(points) > maxPoints {
		return nil, fmt.Errorf("optional teleport: %d points, at most %d", len(points), maxPoints)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	return points, nil
}

/*
================
selection.point

The served form of one row, or false when the port does not serve it.
================
*/
func (s selection) point(row importRow) (Point, bool) {
	if row.Service != serviceOn || row.WorldID != fieldWorldID || !finite(row) {
		return Point{}, false
	}
	region, ok := tableRegion(row.RegionID)
	if !ok {
		return Point{}, false
	}
	zone, known := s.zones[region]
	if !known || s.hidden.HidesRegion(region) {
		return Point{}, false
	}
	point := Point{ID: uint32(row.ID), Name: row.Name, RegionID: region,
		X: row.X, Y: row.Y, Z: row.Z, LevelMin: row.LevelMin, LevelMax: row.LevelMax}
	dungeon := simulation.IsDungeonRegion(region)
	switch {
	case row.MapPoint == mapPointShown && row.RegionGroup == noRegionGroup && !dungeon && inRegion(row):
		point.Kind = KindField
	case row.MapPoint == mapPointShown && row.RegionGroup > 0 && row.RegionGroup <= math.MaxUint32 && !dungeon && inRegion(row):
		// The entrance row has no zone of its own ("xxx"): it takes its field's.
		point.Kind, point.Group, point.Name = KindEntrance, uint32(row.RegionGroup), zone
	case row.MapPoint == mapPointHidden && row.RegionGroup > 0 && row.RegionGroup <= math.MaxUint32 && dungeon:
		if region >= tombFirstRegion && region <= tombLastRegion && !s.options.QinShiTomb {
			return Point{}, false
		}
		point.Kind, point.Group = KindFloor, uint32(row.RegionGroup)
	default:
		return Point{}, false
	}
	if point.Name == "" {
		point.Name = zone
	}
	if point.Name == "" || len(point.Name) > maxNameBytes {
		return Point{}, false
	}
	return point, true
}

/*
================
tableRegion

The table writes a dungeon region as a signed 16-bit value (-32767 is
0x8001) and a field region as its unsigned id.
================
*/
func tableRegion(value int64) (uint16, bool) {
	switch {
	case value >= math.MinInt16 && value < 0:
		return uint16(int16(value)), true
	case value > 0 && value <= math.MaxUint16:
		return uint16(value), true
	}
	return 0, false
}

/*
================
finite
================
*/
func finite(row importRow) bool {
	for _, v := range [3]float64{row.X, row.Y, row.Z} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

/*
================
inRegion

A field point inside its region's local extent.
================
*/
func inRegion(row importRow) bool {
	return row.X >= 0 && row.X < regionSize && row.Z >= 0 && row.Z < regionSize
}
