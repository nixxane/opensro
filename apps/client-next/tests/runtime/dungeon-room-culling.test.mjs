/*
===========================================================================

dungeon-room-culling.test.mjs - the dungeon room an entity stands in

Port-only, not native (Experimental "Dungeon room culling"). The simulation
worker publishes each dungeon entity's room from its navigation owner
(entities.ts withBlock), and the presenter hides an actor whose room the
camera's room does not draw. Neither runs outdoors, and the option is off
until turned on.

===========================================================================
*/
import "../helpers/native-source-loader.mjs";
import { test } from "node:test";
import assert from "node:assert/strict";
import { defined } from "../helpers/defined.mjs";
const { createEntities } = await import(
	"../../src/engine/runtime/simulation/worker/session/world/entities/entities.ts"
);
const { publishCharacters } = await import( "../../src/engine/runtime/characters/presentation-finalize.ts" );
const { experimentalOptions, experimentalVideo } = await import(
	"../../src/engine/foundation/ui/experimental-options.ts"
);

const DUNGEON = 0x8001, OUTDOOR = 25000, WEST_ROOM = 3, EAST_ROOM = 4, DOOR_X = 500;
const roomAt = x => x < DOOR_X ? WEST_ROOM : EAST_ROOM;
function flush( e ) {
	const b = e.take();
	if ( b ) e.ack( b.sequence );
	return b;
}
function world( calls ) {
	const e = createEntities(
		pose => pose,
		undefined,
		undefined,
		( from, to ) => to,
		( pose, owner ) => {
			calls.push( { regionId: pose.regionId, owner } );
			return roomAt( pose.x );
		}
	);
	e.bootstrap( {
		protocolVersion: 2,
		nativeResult: 1,
		refObjSnapshot: [ { refObjId: 2029, kind: "monster", tidWord: 0 } ],
		localPlayerEntry: { modelRef: 1933, startProfile: { regionId: OUTDOOR, x: 1, y: 2, z: 3, angle: 0 } }
	} );
	flush( e );
	return e;
}
function spawn( region, x, { moving = false, distance = 100 } = {} ) {
	const p = Buffer.alloc( 49 + (moving ? 5 : 0) + 1 );
	p.writeUInt32LE( 2029 );
	p.writeUInt32LE( 7, 4 );
	p.writeUInt16LE( region, 8 );
	p.writeFloatLE( x, 10 );
	p.writeFloatLE( 11, 14 );
	p.writeFloatLE( 40, 18 );
	p[24] = moving ? 1 : 0;
	p[25] = 3;
	let o = 29;
	if ( moving ) {
		p.writeUInt16LE( region, 26 );
		p.writeInt16LE( Math.round( x ) + distance, 28 );
		p.writeInt16LE( 11, 30 );
		p.writeInt16LE( 40, 32 );
		o = 34;
	}
	p[o] = 1;
	p.writeFloatLE( 10, o + 3 );
	p.writeFloatLE( 20, o + 7 );
	p.writeFloatLE( 1, o + 11 );
	p[o + 16] = 1;
	return { opcode: 0x30d7, payload: p };
}

test("a dungeon spawn publishes the room its navigation resolves", () => {
	const calls = [], e = world( calls );
	e.receive( spawn( DUNGEON, 200 ), 0 );
	flush( e );
	assert.equal( defined( e.read( 7 ) ).dungeonBlock, WEST_ROOM );
	assert.ok( calls.length > 0 && calls.every( c => c.regionId === DUNGEON ) );
	e.dispose();
});

test("an outdoor spawn never asks for a room and publishes none", () => {
	const calls = [], e = world( calls );
	e.receive( spawn( OUTDOOR, 200 ), 0 );
	flush( e );
	assert.equal( calls.length, 0 );
	assert.equal( defined( e.read( 7 ) ).dungeonBlock, undefined );
	e.dispose();
});

test("a walking dungeon entity changes room when it crosses the door", () => {
	const e = world( [] );
	e.receive( spawn( DUNGEON, 460, { moving: true, distance: 100 } ), 0 );
	flush( e );
	assert.equal( defined( e.read( 7 ) ).dungeonBlock, WEST_ROOM );
	for ( let at = 1000; at <= 30000; at += 1000 ) {
		e.step( at );
		flush( e );
	}
	const done = defined( e.read( 7 ) );
	assert.ok( done.x >= DOOR_X, `walked to ${done.x}` );
	assert.equal( done.dungeonBlock, EAST_ROOM );
	e.dispose();
});

test("an unchanged room adds nothing to the journal", () => {
	const e = world( [] );
	e.receive( spawn( DUNGEON, 200 ), 0 );
	flush( e );
	e.groundSpawns();
	assert.equal( flush( e ), null );
	e.dispose();
});

test("the presenter hides only actors in a room the camera does not draw", () => {
	const published = [];
	const renderer = {
		setCharacterAssembly() {},
		setCharacterActors( actors ) {
			published.push( ...actors );
		},
		dungeonRoomHides: block => block === EAST_ROOM
	};
	// The presenter's frame is wider than this test feeds it; only the fields it reads are given.
	const frame = ( overrides = {} ) => /** @type {any} */ ({
		local: undefined,
		gameplay: null,
		next: new Map( [ 1, 2, 3 ].map( gid => [ gid, { gid, model: "m" + gid } ] ) ),
		entitiesByGid: new Map( [ [ 1, { gid: 1, dungeonBlock: WEST_ROOM } ], [ 2, {
			gid: 2,
			dungeonBlock: EAST_ROOM
		} ], [ 3, { gid: 3 } ] ] ),
		seconds: 0,
		blindHeld: false,
		...overrides
	});
	const bindings = ( r ) => /** @type {any} */ ({
		renderer: r,
		resources: { retainWanted() {} },
		mallPreview: { skin: () => null, step: () => [] }
	});
	const catalog = /** @type {any} */ ({ catalog: new Map(), dress: null, manifest: 0 });
	publishCharacters( frame(), { displayed: new Map() }, catalog, bindings( renderer ) );
	const byGid = new Map( published.map( a => [ a.gid, a ] ) );
	assert.equal( byGid.get( 1 ).pickable, undefined );
	assert.equal( byGid.get( 2 ).opacity, 0 );
	assert.equal( byGid.get( 2 ).pickable, false );
	assert.equal( byGid.get( 3 ).opacity, undefined, "an entity with no room is never culled" );

	published.length = 0;
	publishCharacters(
		frame(),
		{ displayed: new Map() },
		catalog,
		bindings( { ...renderer, dungeonRoomHides: undefined } )
	);
	assert.ok( published.every( a => a.pickable === undefined && a.opacity === undefined ), "no hook, no culling" );
});

test("dungeon room culling is off until turned on", () => {
	for ( const value of [ undefined, null, {}, { dungeonRoomCulling: 1 }, { dungeonRoomCulling: "true" } ] ) {
		assert.equal( experimentalOptions( value ).dungeonRoomCulling, false );
	}
	const on = experimentalOptions( { dungeonRoomCulling: true } );
	assert.equal( on.dungeonRoomCulling, true );
	assert.equal( experimentalVideo( on ).dungeonRoomCulling, true );
});
