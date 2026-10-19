/*
===========================================================================

dungeon-rooms.ts - which dungeon rooms a character stands in, for culling

The dungeon scenery is culled by room: a group draws only when its block is
in the camera block's visibleBlocks (world.ts). Characters carry no block, so
a mob in a room the camera cannot see still drew and could be clicked, floating
in the void where the culled room's walls should be.

The client has no navmesh on this thread, so a room is the box around its
scenery. Boxes of neighbouring rooms overlap, so the test is conservative: a
character is hidden only when it is inside a hidden room's box and inside no
visible room's box. A character in no room's box that is also well clear of
every visible room (the void behind the culled rooms) is hidden too.

===========================================================================
*/
import type { WorldGroup } from "@/engine/contracts/scene";
import { geometryPickBounds, type PickBounds } from "./picking";

// Distance (10 units to the metre) past a visible room's box that still counts as that room.
const ROOM_VOID_MARGIN = 100;
export type RoomBoxes = ReadonlyMap<number, PickBounds>;

/*
================
dungeonRoomBoxes

The world-space box of each dungeon block: the union of every instance of
every unskinned group of that block.
================
*/
export function dungeonRoomBoxes( groups: readonly WorldGroup[] ): RoomBoxes {
	const boxes = new Map<number, number[]>(), local = new WeakMap<Float32Array, PickBounds>();
	for ( const group of groups ) {
		const block = group.dungeonBlock, source = group.geometry;
		if ( block === undefined || source.bones || !source.instances || group.material.sky ) continue;
		let bounds = local.get( source.positions );
		if ( !bounds ) local.set( source.positions, bounds = geometryPickBounds( source.positions ) );
		let box = boxes.get( block );
		if ( !box ) boxes.set( block, box = [ Infinity, Infinity, Infinity, -Infinity, -Infinity, -Infinity ] );
		const m = source.instances;
		for ( let at = 0; at + 16 <= m.length; at += 16 ) {
			for ( let axis = 0; axis < 3; axis++ ) {
				let center = m[at + 12 + axis]!, extent = 0;
				for ( let k = 0; k < 3; k++ ) {
					center += m[at + k * 4 + axis]! * (bounds[k]! + bounds[k + 3]!) * .5;
					extent += Math.abs( m[at + k * 4 + axis]! ) * (bounds[k + 3]! - bounds[k]!) * .5;
				}
				box[axis] = Math.min( box[axis]!, center - extent );
				box[axis + 3] = Math.max( box[axis + 3]!, center + extent );
			}
		}
	}
	return boxes as unknown as RoomBoxes;
}

/*
================
roomHidesPoint

True when the point stands only in rooms the camera's block cannot see.
================
*/
export function roomHidesPoint(
	boxes: RoomBoxes,
	visibleBlocks: readonly number[],
	x: number,
	y: number,
	z: number
): boolean {
	let hidden = false, near = false;
	for ( const [block, b] of boxes ) {
		const visible = visibleBlocks.includes( block );
		if ( visible && !near ) {
			near = x >= b[0] - ROOM_VOID_MARGIN && x <= b[3] + ROOM_VOID_MARGIN &&
				y >= b[1] - ROOM_VOID_MARGIN && y <= b[4] + ROOM_VOID_MARGIN &&
				z >= b[2] - ROOM_VOID_MARGIN && z <= b[5] + ROOM_VOID_MARGIN;
		}
		if ( x < b[0] || y < b[1] || z < b[2] || x > b[3] || y > b[4] || z > b[5] ) continue;
		if ( visible ) return false;
		hidden = true;
	}
	// In no room at all and well clear of every visible room: the void behind
	// the culled rooms, where a mob would otherwise float.
	return hidden || boxes.size > 0 && !near;
}
