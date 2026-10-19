/*
===========================================================================

dungeon-light.test.mjs - the light a dungeon room block gives its scene

SRO_Client.exe 0x8B17E0 (the dungeon render pass) builds one D3D light of
type 3 (directional), Diffuse and Specular zero, Ambient the block colour
(CRTDunBlock vf[6], 0xAARRGGBB) split into bytes and divided by 255.0.

===========================================================================
*/
import "../helpers/native-source-loader.mjs";
import { test } from "node:test";
import assert from "node:assert/strict";
const { dungeonBlockAmbient } = await import( "../../src/engine/foundation/rendering/world-environment.ts" );
const { createWorldDecoder } = await import( "../../src/engine/runtime/assets/worker/world/world.ts" );

test("the block colour splits into 0..1 RGB, alpha ignored", () => {
	assert.deepEqual( dungeonBlockAmbient( 0xff000000 ), [ 0, 0, 0 ] );
	assert.deepEqual( dungeonBlockAmbient( 0x00ffffff ), [ 1, 1, 1 ] );
	const [r, g, b] = dungeonBlockAmbient( 0xff442230 );
	assert.equal( r, Math.fround( 0x44 / 255 ) );
	assert.equal( g, Math.fround( 0x22 / 255 ) );
	assert.equal( b, Math.fround( 0x30 / 255 ) );
});

test("a negative colour word is read as its unsigned bytes", () => {
	assert.deepEqual( dungeonBlockAmbient( 0xff442230 | 0 ), dungeonBlockAmbient( 0xff442230 ) );
});

test("a dungeon bundle publishes one ambient per block, from the block colour", () => {
	const block = ( index, color ) => ({
		index,
		visibleBlocks: [ index ],
		fog: { color, nearPlane: -200, farPlane: 1000, intensity: 0, heightFog: null }
	});
	const bundle = {
		source: { sectorX: 1, sectorY: 128 },
		terrain: { blocks: [] },
		terrainTextures: { tileCatalog: { referencedTiles: [] } },
		dungeonBlocks: [ block( 0, 0xff442230 ), block( 1, 0xff2c2306 ) ],
		objects: {
			resources: { bsr: [], meshes: [], materialSets: [], textures: [], missing: [], missingCount: 0 },
			placements: []
		}
	};
	const scene = createWorldDecoder().decode(
		new TextEncoder().encode( JSON.stringify( bundle ) ),
		false,
		{ origin: 0x8001, part: "all" }
	);
	assert.deepEqual( scene.dungeonAmbient, [ dungeonBlockAmbient( 0xff442230 ), dungeonBlockAmbient( 0xff2c2306 ) ] );
});

test("an outdoor scene has no dungeon ambient", () => {
	const scene = createWorldDecoder().decode(
		new TextEncoder().encode( JSON.stringify( {
			source: { sectorX: 0x6a, sectorY: 0x48 },
			terrain: { blocks: [] },
			terrainTextures: { tileCatalog: { referencedTiles: [] } },
			objects: {
				resources: { bsr: [], meshes: [], materialSets: [], textures: [], missing: [], missingCount: 0 },
				placements: []
			}
		} ) ),
		false,
		{ origin: 0x486a, part: "all" }
	);
	assert.equal( scene.dungeonAmbient, undefined );
});
