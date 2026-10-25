/*
===========================================================================

optionalteleport_test.go - which imported rows the port serves

Synthetic textdata directories: no import is the native table (nil), the
outdoor map rows of named regions are served by their own ids, dungeon
groups and unnamed regions are dropped, Alexandria rows follow its
overlay and SRO_ALEXANDRIA, and a malformed import is refused.

===========================================================================
*/
package optionalteleport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"opensro.online/server/internal/game/world/alexandria"
)

const (
	testJangan     = 25000
	testConstant   = 26959
	testAlexandria = 23088
)

/*
================
outdoorRow
================
*/
func outdoorRow(id, region int64, name string) importRow {
	return importRow{Service: 1, ID: id, Zone: "SN_ZONE", Name: name, RegionID: region,
		X: 995, Y: -32, Z: 1132, WorldID: 1, RegionGroup: -1, MapPoint: 1}
}

/*
================
writeTextdata

A textdata directory holding the import (when rows is not nil), a
textzonename naming the given regions, and optionally an Alexandria
overlay.
================
*/
func writeTextdata(t *testing.T, rows []importRow, zones map[int64]string, overlay bool) string {
	t.Helper()
	dir := t.TempDir()
	if rows != nil {
		data, err := json.Marshal(importFile{Format: importFormat, Source: json.RawMessage(`{}`), Rows: rows})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, File), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var lines []string
	for region, name := range zones {
		cells := []string{"1", strconv.FormatInt(region, 10), "k", "", "", "", "", "", name}
		lines = append(lines, strings.Join(cells, "\t"))
	}
	if err := os.WriteFile(filepath.Join(dir, "textzonename.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if overlay {
		body := `{"format":"sro-server-alexandria-overlay","version":1,"regions":[23088],` +
			`"teleports":["GATE_SD1"],"buildings":["STORE_SD_GATE1"]}`
		if err := os.WriteFile(filepath.Join(dir, alexandria.OverlayFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

/*
================
ids
================
*/
func ids(points []Point) []uint32 {
	out := make([]uint32, 0, len(points))
	for _, point := range points {
		out = append(out, point.ID)
	}
	return out
}

/*
================
TestNoImportIsTheNativeTable
================
*/
func TestNoImportIsTheNativeTable(t *testing.T) {
	points, err := Load(writeTextdata(t, nil, map[int64]string{testJangan: "Jangan"}, false), Options{})
	if err != nil || points != nil {
		t.Fatalf("no import: %+v, %v; want nil", points, err)
	}
}

/*
================
TestServesOutdoorMapRowsByTheirOwnIDs
================
*/
func TestServesOutdoorMapRowsByTheirOwnIDs(t *testing.T) {
	flag := outdoorRow(29, testJangan, "Cave flag")
	flag.RegionGroup = 1
	floor := outdoorRow(30, -32767, "Cave 1F")
	floor.MapPoint, floor.RegionGroup, floor.LevelMin, floor.LevelMax = 0, 1, 50, 999
	retired := outdoorRow(5, testJangan, "Retired")
	retired.Service = 0
	unnamedRegion := outdoorRow(7, 26205, "Pond Ruins")
	unnamed := outdoorRow(9, testConstant, "")
	rows := []importRow{outdoorRow(26, testJangan, "Jangan"), unnamed, flag, floor, retired, unnamedRegion}
	dir := writeTextdata(t, rows, map[int64]string{testJangan: "Jangan", testConstant: "Constantinople"}, false)
	points, err := Load(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(points); len(got) != 2 || got[0] != 9 || got[1] != 26 {
		t.Fatalf("served ids %v, want [9 26]", got)
	}
	// A row without its own English name takes the region's.
	if points[0].Name != "Constantinople" || points[1].RegionID != testJangan || points[1].X != 995 {
		t.Fatalf("points %+v", points)
	}
}

/*
================
TestDungeonEntrancesListTheirFloors

The VSRO shape: an entrance flag drawn on the map (MapPoint 1, group 1) and
its floor (MapPoint 0, group 1, a signed dungeon region). The tomb's floors
follow SRO_QINSHI_TOMB; an entrance without a served floor is dropped.
================
*/
func TestDungeonEntrancesListTheirFloors(t *testing.T) {
	const (
		caveRegion = 0x8001
		tombRegion = 0x8006
	)
	caveFlag := outdoorRow(29, testJangan, "")
	caveFlag.RegionGroup = 1
	cave := outdoorRow(30, -32767, "Donwhang Stone Cave")
	cave.MapPoint, cave.RegionGroup, cave.X, cave.Z, cave.LevelMin, cave.LevelMax = 0, 1, 1115, -935, 0, 0
	tombFlag := outdoorRow(31, testConstant, "")
	tombFlag.RegionGroup = 2
	tomb := outdoorRow(32, -32762, "Tomb lv.2")
	tomb.MapPoint, tomb.RegionGroup, tomb.X, tomb.Z, tomb.LevelMin, tomb.LevelMax = 0, 2, -11719, 5742, 70, 999
	orphan := outdoorRow(40, -32767, "Orphan floor")
	orphan.MapPoint, orphan.RegionGroup = 0, 9
	zones := map[int64]string{testJangan: "Jangan", testConstant: "Constantinople", caveRegion: "Cave", tombRegion: "Tomb B2"}
	dir := writeTextdata(t, []importRow{caveFlag, cave, tombFlag, tomb, orphan}, zones, false)
	for _, tc := range []struct {
		tomb bool
		want []uint32
	}{{false, []uint32{29, 30}}, {true, []uint32{29, 30, 31, 32}}} {
		points, err := Load(dir, Options{QinShiTomb: tc.tomb})
		if err != nil || !slices.Equal(ids(points), tc.want) {
			t.Fatalf("tomb %v: %v, %v; want %v", tc.tomb, ids(points), err, tc.want)
		}
		if points[0].Kind != KindEntrance || points[0].Group != 1 || points[0].Name != "Jangan" {
			t.Fatalf("entrance %+v: want group 1 named after its field", points[0])
		}
		if points[1].Kind != KindFloor || points[1].RegionID != caveRegion || points[1].X != 1115 || points[1].Z != -935 {
			t.Fatalf("floor %+v: want the cave region at its dungeon coordinates", points[1])
		}
	}
}

/*
================
TestAlexandriaRowsFollowItsOverlay

A native root names no Alexandria region, so the row is dropped; a variant
root names it and serves it only while SRO_ALEXANDRIA is on.
================
*/
func TestAlexandriaRowsFollowItsOverlay(t *testing.T) {
	south := outdoorRow(39, testAlexandria, "Alexandria (South)")
	south.LevelMin, south.LevelMax = 95, 999
	rows := []importRow{outdoorRow(26, testJangan, "Jangan"), south}
	native := writeTextdata(t, rows, map[int64]string{testJangan: "Jangan"}, false)
	variant := writeTextdata(t, rows, map[int64]string{testJangan: "Jangan", testAlexandria: "Alexandria"}, true)
	for _, tc := range []struct {
		dir, flag string
		want      int
	}{{native, "on", 1}, {variant, "off", 1}, {variant, "on", 2}} {
		t.Setenv(alexandria.EnvAlexandria, tc.flag)
		points, err := Load(tc.dir, Options{})
		if err != nil || len(points) != tc.want {
			t.Fatalf("SRO_ALEXANDRIA=%s: %v, %v; want %d points", tc.flag, ids(points), err, tc.want)
		}
	}
}

/*
================
TestAdmitsTheLevelBand
================
*/
func TestAdmitsTheLevelBand(t *testing.T) {
	open, band := Point{}, Point{LevelMin: 50, LevelMax: 60}
	for level, want := range map[int64]bool{1: false, 49: false, 50: true, 60: true, 61: false} {
		if !open.Admits(level) || band.Admits(level) != want {
			t.Fatalf("level %d: open %v band %v, want true %v", level, open.Admits(level), band.Admits(level), want)
		}
	}
}

/*
================
TestRefusesAMalformedImport
================
*/
func TestRefusesAMalformedImport(t *testing.T) {
	zones := map[int64]string{testJangan: "Jangan"}
	repeated := writeTextdata(t, []importRow{outdoorRow(1, testJangan, "A"), outdoorRow(1, testJangan, "B")}, zones, false)
	if _, err := Load(repeated, Options{}); err == nil {
		t.Fatal("a repeated id was accepted")
	}
	empty := writeTextdata(t, []importRow{}, zones, false)
	if _, err := Load(empty, Options{}); err == nil {
		t.Fatal("an import with no rows was accepted")
	}
	unknown := writeTextdata(t, nil, zones, false)
	if err := os.WriteFile(filepath.Join(unknown, File), []byte(`{"format":"`+importFormat+`","rows":[{"extra":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(unknown, Options{}); err == nil {
		t.Fatal("an unknown field was accepted")
	}
}
