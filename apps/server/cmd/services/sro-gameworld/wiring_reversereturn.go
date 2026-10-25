/*
===========================================================================

wiring_reversereturn.go - the reverse return scroll's map destinations

wiring_gameplay.go is at the source-size limit, so the port-only
SRO_REVERSE_RETURN_MAP wiring lives here. Off is native: the scroll offers
the last recall and last death points only. On, the action runtime serves
the imported VSRO optional-teleport table when the game data carries it
(world/optionalteleport), else its inferred table, and world entry tells
the client which one it is (reverseMapImported).

===========================================================================
*/
package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"opensro.online/server/internal/game/action"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/world/monster"
	"opensro.online/server/internal/game/world/optionalteleport"
)

/*
================
configureReverseReturnMap

Runs after ConfigurePortals and the monster authority. A malformed import
refuses the boot rather than falling back to the inferred table silently.
================
*/
func configureReverseReturnMap(items *action.Runtime, deps *enterworld.Deps, textdataDir string) error {
	if !action.ReverseReturnMapFromEnv() {
		return nil
	}
	imported, err := optionalteleport.Load(textdataDir, optionalteleport.Options{QinShiTomb: monster.QinShiTombFromEnv()})
	if err != nil {
		return fmt.Errorf("reverse return map: %w", err)
	}
	items.ConfigureReverseReturnMap(true, imported)
	deps.ReverseMapImported = items.ReverseReturnMapImported()
	source := "inferred"
	if deps.ReverseMapImported {
		source = "imported " + optionalteleport.File
	}
	log.Infof("reverse return: map destinations ON (%s), %d %s points of %d rows", action.EnvReverseReturnMap,
		len(items.ReverseReturnMapPoints()), source, len(imported))
	return nil
}
