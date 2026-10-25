/*
===========================================================================

reverse-return-floors.test.mjs - the imported table's dungeon entrances

Port-only, not native. An imported VSRO table publishes dungeon entrances
that list their floors, numbered after the points (the server keeps the
points 1..n). The decoder checks the floors, a use may name a floor or a
field point but never an entrance, and the reverse return HUD opens an
entrance's floor list while the map holds the scroll.

===========================================================================
*/
import "../helpers/native-source-loader.mjs";
import assert from "node:assert/strict";
import { test } from "node:test";

const { decodeReverseMapPoints, reverseMapDestination, REVERSE_RETURN_MAP } = await import(
	"../../src/engine/foundation/gameplay/reverse-return-map.ts"
);
const { createReverseReturnHud } = await import( "../../src/engine/runtime/ui/hud/reverse-return-hud.ts" );

// The reverse return scroll's type word (3/3/3/3), as the bag reports it.
const REVERSE_SCROLL = 3 << 2 | 3 << 5 | 3 << 7 | 3 << 11;
const LOCAL_GID = 77;
const SLOT = 21;

const JANGAN = { id: 1, name: "Jangan", regionId: 25000, x: 995, y: -32, z: 1132 };
const CAVE = {
	id: 2,
	name: "Okmungwan Field",
	regionId: 27027,
	x: 1814,
	y: 452,
	z: 1834,
	floors: [ { id: 3, name: "Donwhang Stone Cave" } ]
};

/*
================
worldView

A world view holding one reverse return scroll and the given table: only
the fields the HUD reads.
================
*/
function worldView( points ) {
	return /** @type {import("../../src/engine/contracts/ui.ts").UiView} */ (/** @type {unknown} */ ({
		session: { phase: "world" },
		travel: null,
		gameplay: {
			localGid: LOCAL_GID,
			inventoryPending: false,
			reverseReturnChoice: false,
			inventory: [ { slot: SLOT, refObjId: 3795, typeFlags: REVERSE_SCROLL, quantity: 2 } ],
			reverseMapPoints: points
		}
	}));
}

test("an entrance's floors decode and only floors or field points are destinations", () => {
	const points = decodeReverseMapPoints( [ JANGAN, CAVE ] );
	assert.deepEqual( points[1].floors, [ { id: 3, name: "Donwhang Stone Cave" } ] );
	assert.equal( points[0].floors, undefined );
	assert.equal( reverseMapDestination( points, 1 ), true );
	assert.equal( reverseMapDestination( points, 3 ), true );
	assert.equal( reverseMapDestination( points, 2 ), false );
	for ( const floors of [ [], [ { id: 2, name: "x" } ], [ { id: 3, name: 5 } ], "floors" ] ) {
		assert.throws( () => decodeReverseMapPoints( [ { ...CAVE, floors } ] ) );
	}
});

test("the map's entrance opens its floor list and a floor answers the scroll", () => {
	const view = worldView( decodeReverseMapPoints( [ JANGAN, CAVE ] ) ), hud = createReverseReturnHud();
	hud.open(
		/** @type {import("../../src/engine/contracts/gameplay.ts").InventoryItem} */ (
			/** @type {unknown} */ ({ slot: SLOT, refObjId: 3795 })
		),
		LOCAL_GID
	);
	// Before the box hands the scroll to the map, no entrance opens.
	assert.equal( hud.openEntrance( 2, view ), false );
	assert.equal( hud.openMap( view ), true );
	assert.equal( hud.openEntrance( 1, view ), false, "a field point lists no floors" );
	assert.equal( hud.openEntrance( 2, view ), true );
	assert.equal( hud.entrance( view )?.id, 2 );
	assert.equal( hud.choosePoint( 2, view ), null, "the entrance itself is not a destination" );
	hud.closeEntrance();
	assert.equal( hud.entrance( view ), null );
	assert.equal( hud.mapping(), true, "Cancel leaves the scroll waiting on the map" );
	hud.openEntrance( 2, view );
	assert.deepEqual( hud.choosePoint( 3, view ), {
		kind: "item-use",
		slot: SLOT,
		reverseChoice: REVERSE_RETURN_MAP,
		reverseMapPoint: 3
	} );
	assert.equal( hud.entrance( view ), null );
	assert.equal( hud.mapping(), false );
});
