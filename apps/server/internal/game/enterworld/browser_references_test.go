/*
===========================================================================

browser_references_test.go - the public references' reverse return map table

Port-only, not native (action/reversemap.go). The table's ids ascend: the
inferred table numbers 1..n, an imported VSRO table keeps its own sparse
ids. A repeated, descending or zero id is refused.

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
TestReverseMapReferencesTakeAscendingIDs
================
*/
func TestReverseMapReferencesTakeAscendingIDs(t *testing.T) {
	point := func(id uint32) ReverseMapPoint {
		return ReverseMapPoint{ID: id, Name: "Jangan", RegionID: 25000, X: 995, Z: 1132}
	}
	refs, err := NewBrowserReferences(BrowserReferenceSources{Skills: oneSkillCatalogue{},
		ReverseMapPoints: []ReverseMapPoint{point(26), point(39)}})
	if err != nil {
		t.Fatalf("sparse ascending ids refused: %v", err)
	}
	recorder := httptest.NewRecorder()
	refs.ServeHTTP(recorder, httptest.NewRequest("GET", refs.Path, nil))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"id":39`) {
		t.Fatalf("the published table lost id 39: status %d", recorder.Code)
	}
	for _, ids := range [][]uint32{{0}, {3, 3}, {5, 4}} {
		points := make([]ReverseMapPoint, 0, len(ids))
		for _, id := range ids {
			points = append(points, point(id))
		}
		if _, err := NewBrowserReferences(BrowserReferenceSources{Skills: oneSkillCatalogue{}, ReverseMapPoints: points}); err == nil {
			t.Fatalf("ids %v accepted", ids)
		}
	}
}
