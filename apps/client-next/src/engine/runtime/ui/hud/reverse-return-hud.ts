/*
===========================================================================

reverse-return-hud.ts - the bag scroll's pending destination choice

Owns the slot and character bound to message box 0x1E. Reconcile before
input and painting so a delayed answer cannot use a replacement item or
survive a world transition.

Port-only, not native: when the server publishes a map table
(SRO_REVERSE_RETURN_MAP), the box has a third row. It hands the same
pending scroll to the world map, where a published point is the answer
(reverse-return-map.ts). A dungeon entrance on the map opens its floor
list; a floor is the answer.

===========================================================================
*/
import type { GameplayCommand, InventoryItem } from "@/engine/contracts/gameplay";
import type { UiView } from "@/engine/contracts/ui";
import { isReverseReturnScroll } from "@/engine/foundation/gameplay/cos-item-use";
import { REVERSE_RETURN_LAST_DEATH, REVERSE_RETURN_LAST_RECALL } from "@/engine/foundation/gameplay/count-job";
import { REVERSE_RETURN_MAP, reverseMapDestination } from "@/engine/foundation/gameplay/reverse-return-map";

/*
================
createReverseReturnHud
================
*/
export function createReverseReturnHud() {
	let pending: { slot: number; refObjId: number; gid: number; } | null = null;
	// mapping: the box handed the pending scroll to the world map.
	let mapping = false;
	// entrance: the map's dungeon entrance whose floor list is open.
	let entrance: number | null = null;
	/*
	================
	reconcile
	================
	*/
	function reconcile( view: UiView | null ): boolean {
		if ( !pending ) return false;
		const game = view?.gameplay;
		const row = game?.inventory.find( row => row.slot === pending?.slot );
		if (
			view?.session?.phase === "world" && !view.travel && game?.localGid === pending.gid &&
			!game.inventoryPending && !game.reverseReturnChoice && row?.refObjId === pending.refObjId &&
			isReverseReturnScroll( row.typeFlags ) && row.quantity > 0
		) return false;
		pending = null;
		mapping = false;
		entrance = null;
		return true;
	}
	return {
		reconcile,
		/*
		================
		open
		================
		*/
		open( item: InventoryItem, gid: number ) {
			pending = { slot: item.slot, refObjId: item.refObjId, gid };
			mapping = false;
			entrance = null;
		},
		/*
		================
		active

		The box is up: a scroll waits and the map has not taken it.
		================
		*/
		active() {
			return pending !== null && !mapping;
		},
		/*
		================
		mapping

		The world map holds the pending scroll (port-only).
		================
		*/
		mapping() {
			return pending !== null && mapping;
		},
		/*
		================
		openMap

		The box's map row: the scroll waits for a point on the world map.
		================
		*/
		openMap( view: UiView | null ): boolean {
			reconcile( view );
			if ( !pending || !view?.gameplay?.reverseMapPoints?.length ) return false;
			mapping = true;
			return true;
		},
		/*
		================
		openEntrance

		A dungeon entrance on the map opens its floor list (port-only).
		================
		*/
		openEntrance( id: number, view: UiView | null ): boolean {
			reconcile( view );
			const point = view?.gameplay?.reverseMapPoints?.find( row => row.id === id );
			if ( !pending || !mapping || !point?.floors ) return false;
			entrance = id;
			return true;
		},
		/*
		================
		entrance

		The open floor list's entrance, or null.
		================
		*/
		entrance( view: UiView | null ) {
			return mapping && entrance !== null ?
				view?.gameplay?.reverseMapPoints?.find( row => row.id === entrance && row.floors ) ?? null :
				null;
		},
		/*
		================
		closeEntrance

		The floor list's Cancel: back to the map, the scroll still waiting.
		================
		*/
		closeEntrance() {
			entrance = null;
		},
		/*
		================
		choosePoint

		A published field point or an entrance's floor answers the pending
		scroll (choice 7).
		================
		*/
		choosePoint( id: number, view: UiView | null ): GameplayCommand | null {
			reconcile( view );
			if ( !pending || !mapping || !reverseMapDestination( view?.gameplay?.reverseMapPoints ?? [], id ) ) {
				return null;
			}
			const slot = pending.slot;
			pending = null;
			mapping = false;
			entrance = null;
			return { kind: "item-use", slot, reverseChoice: REVERSE_RETURN_MAP, reverseMapPoint: id };
		},
		/*
		================
		close
		================
		*/
		close() {
			pending = null;
			mapping = false;
			entrance = null;
		},
		/*
		================
		choose
		================
		*/
		choose( choice: number, view: UiView | null ): GameplayCommand | null {
			reconcile( view );
			if ( !pending || choice !== REVERSE_RETURN_LAST_RECALL && choice !== REVERSE_RETURN_LAST_DEATH ) {
				return null;
			}
			const slot = pending.slot;
			pending = null;
			mapping = false;
			entrance = null;
			return { kind: "item-use", slot, reverseChoice: choice };
		}
	};
}
