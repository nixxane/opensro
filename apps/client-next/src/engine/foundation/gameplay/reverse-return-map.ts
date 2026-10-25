/*
===========================================================================

reverse-return-map.ts - the reverse return scroll's map destinations

Port-only, not native. v1.150's scroll offers two points (count-job.ts
REVERSE_RETURN_LAST_*). Later clients add "move to a location on the map":
v1.188's 4A00C0 reads choice 7 and a u32 saved-point id. The server
publishes its table of points in the world references only when its
SRO_REVERSE_RETURN_MAP option is on; the box offers the map whenever a
table is published. A use sends only the point id; the server resolves it.
The map choice was first written by GrazKe in #336.

===========================================================================
*/

import { isDungeonRegion, REGION_SIZE } from "@/engine/foundation/world/world-space";

// REVERSE_RETURN_MAP is v1.188's saved-point choice (4A00C0 case 7).
export const REVERSE_RETURN_MAP = 7;
// MAX_REVERSE_MAP_POINTS bounds the published table (server maxReverseMapPoints).
const MAX_REVERSE_MAP_POINTS = 4096;
// MAX_REVERSE_MAP_ID is the largest id the u32 use tail carries.
const MAX_REVERSE_MAP_ID = 0xffffffff;
// MAX_REVERSE_MAP_NAME bounds a point name (server maxReverseMapNameBytes).
const MAX_REVERSE_MAP_NAME = 128;
// MAX_REVERSE_MAP_FLOORS bounds an entrance's floor list (the VSRO table's
// largest dungeon, the Qin-Shi tomb, has six floors).
const MAX_REVERSE_MAP_FLOORS = 16;

/*
================
ReverseMapFloor

A dungeon floor an entrance lists. The server numbers floors after the
points (n+1 on), so no floor id is a point id.
================
*/
export interface ReverseMapFloor {
	readonly id: number;
	readonly name: string;
}

/*
================
ReverseMapPoint

One published point. The map paints it from these coordinates. A dungeon
entrance (an imported VSRO table only) lists its floors and is not a
destination itself: a use names one of its floors.
================
*/
export interface ReverseMapPoint {
	readonly id: number;
	readonly name: string;
	readonly regionId: number;
	readonly x: number;
	readonly y: number;
	readonly z: number;
	readonly floors?: readonly ReverseMapFloor[];
}

/*
================
decodeReverseMapFloors

A point's floors: absent is a field point. points is the table's length.
================
*/
function decodeReverseMapFloors( value: unknown, points: number ): readonly ReverseMapFloor[] | undefined {
	if ( value === undefined ) return undefined;
	if ( !Array.isArray( value ) || value.length < 1 || value.length > MAX_REVERSE_MAP_FLOORS ) {
		throw Error( "Invalid reverse return map floors" );
	}
	return value.map( raw => {
		const floor = raw as Partial<ReverseMapFloor> | null;
		if (
			!floor || typeof floor !== "object" || !Number.isInteger( floor.id ) || floor.id! <= points ||
			floor.id! > MAX_REVERSE_MAP_ID || typeof floor.name !== "string" || floor.name.length > MAX_REVERSE_MAP_NAME
		) throw Error( "Invalid reverse return map floor" );
		return { id: floor.id!, name: floor.name };
	} );
}

/*
================
reverseMapDestination

Whether a use may name this id: a field point or an entrance's floor,
never the entrance itself.
================
*/
export function reverseMapDestination( points: readonly ReverseMapPoint[], id: number ): boolean {
	return points.some( point => point.floors ? point.floors.some( floor => floor.id === id ) : point.id === id );
}

/*
================
decodeReverseMapPoints

The references' reverseMapPoints: absent is the native empty table. IDs
run 1..n in order, as the server numbers them.
================
*/
export function decodeReverseMapPoints( value: unknown ): readonly ReverseMapPoint[] {
	if ( value === undefined ) return [];
	if ( !Array.isArray( value ) || value.length > MAX_REVERSE_MAP_POINTS ) {
		throw Error( "Invalid reverse return map table" );
	}
	return value.map( ( raw, index ) => {
		const point = raw as Partial<ReverseMapPoint> | null;
		if (
			!point || typeof point !== "object" || point.id !== index + 1 || typeof point.name !== "string" ||
			point.name.length > MAX_REVERSE_MAP_NAME || !Number.isInteger( point.regionId ) ||
			point.regionId! < 1 || isDungeonRegion( point.regionId! ) || point.regionId! > 0xffff ||
			![ point.x, point.y, point.z ].every( Number.isFinite ) || point.x! < 0 ||
			point.x! >= REGION_SIZE || point.z! < 0 || point.z! >= REGION_SIZE
		) throw Error( "Invalid reverse return map point" );
		const floors = decodeReverseMapFloors( point.floors, value.length );
		return {
			id: point.id!,
			name: point.name,
			regionId: point.regionId!,
			x: point.x!,
			y: point.y!,
			z: point.z!,
			...(floors ? { floors } : {})
		};
	} );
}

/*
================
reverseMapTail

The item-use tail for a map point: choice 7 and the id, little-endian.
================
*/
export function reverseMapTail( id: number ): Uint8Array {
	if ( !Number.isInteger( id ) || id < 1 || id > 0xffffffff ) {
		throw Error( "Choose a point on the map for the reverse return scroll" );
	}
	const tail = new Uint8Array( 5 );
	tail[0] = REVERSE_RETURN_MAP;
	new DataView( tail.buffer ).setUint32( 1, id, true );
	return tail;
}
