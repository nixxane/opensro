/*
===========================================================================

browser_references_test.go - the public references' reverse return map table

Port-only, not native (action/reversemap.go). The points' ids run 1..n,
the contract every shipped client decodes; an imported table's dungeon
entrances list floors numbered after the points.

===========================================================================
*/
package enterworld

import (
	"net/http/httptest"
	"strings"
	"testing"
)

/*
================
TestReverseMapReferencesKeepDenseIDsAndFloorsAfterThem
================
*/
func TestReverseMapReferencesKeepDenseIDsAndFloorsAfterThem(t *testing.T) {
	point := func(id uint32, floors ...ReverseMapFloor) ReverseMapPoint {
		return ReverseMapPoint{ID: id, Name: "Jangan", RegionID: 25000, X: 995, Z: 1132, Floors: floors}
	}
	refs, err := NewBrowserReferences(BrowserReferenceSources{Skills: oneSkillCatalogue{},
		ReverseMapPoints: []ReverseMapPoint{point(1), point(2, ReverseMapFloor{ID: 3, Name: "Donwhang Stone Cave"})}})
	if err != nil {
		t.Fatalf("a dense table with a floor after it refused: %v", err)
	}
	recorder := httptest.NewRecorder()
	refs.ServeHTTP(recorder, httptest.NewRequest("GET", refs.Path, nil))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"floors":[{"id":3`) {
		t.Fatalf("the published table lost the floor: status %d", recorder.Code)
	}
	for _, points := range [][]ReverseMapPoint{
		{point(2)},
		{point(1), point(3)},
		{point(1, ReverseMapFloor{ID: 1, Name: "x"})},
		{point(1, ReverseMapFloor{ID: 0, Name: "x"})},
	} {
		if _, err := NewBrowserReferences(BrowserReferenceSources{Skills: oneSkillCatalogue{}, ReverseMapPoints: points}); err == nil {
			t.Fatalf("points %+v accepted", points)
		}
	}
}
