/*
===========================================================================

particle-catchup.ts - how far behind an effect graph may replay

A looping or emitting particle graph never ends, so a history started on an
old actor clock would replay every 20 Hz tick since that clock began. This
module owns the bound: the steady-state window, and whether a history has
fallen outside it. particle-graph.ts applies it.

===========================================================================
*/

import type { ParticleEmitter } from "@/engine/foundation/animation/particle-graph";

// A2E980: whole 50 ms effect ticks.
const TICKS_PER_SECOND = 20;

/*
================
catchUpWindow

How many ticks a graph needs to reach steady state from nothing: the longest
chain of parent lifetimes. An element alive at tick T was born within its own
lifetime of T, and its parent within the parent's lifetime of that, so a replay
begun this many ticks before T meets every element that is alive at T.
================
*/
export function catchUpWindow( graph: readonly ParticleEmitter[] ): number {
	let window = 0;
	// Walk each emitter up its parents: graphs are small and this allocates nothing.
	for ( let n = 0; n < graph.length; n++ ) {
		let chain = 0;
		for ( let at = n; at >= 0 && at < graph.length; at = graph[at]!.parent ) chain += graph[at]!.frames;
		window = Math.max( window, chain );
	}
	return window + 1;
}

/*
================
particleGraphStale

True when advancing a history from tick `frame` to `time` would replay more
than the graph's steady-state window. Only a graph that loops or emits can:
any other ends at its last birth plus lifetime. The caller restarts such a
history instead of replaying minutes of ticks in one frame (a hidden tab, or
an actor not drawn for a long while).
================
*/
export function particleGraphStale( frame: number, graph: readonly ParticleEmitter[], time: number ): boolean {
	return graph.some( def => def.loop || def.emission ) &&
		Math.floor( time * TICKS_PER_SECOND + 1e-6 ) - frame > catchUpWindow( graph );
}
