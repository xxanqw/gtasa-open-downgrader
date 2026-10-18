package launcherapp

import (
	"context"
	"errors"
	"fmt"
)

// open.mp leaves all other mod loading intact, but pauses Widescreen Fix
// until the tracked GTA process exits. Recovery also runs before later launches.
func (runtime *Runtime) runWithGameCleanup(ctx context.Context, run func() error, cleanup func() error) error {
	before, err := runtime.gameProcessSnapshot()
	if err != nil {
		return fmt.Errorf("inspect running GTA processes: %w", err)
	}
	if len(before) != 0 {
		return fmt.Errorf("close GTA before preparing multiplayer compatibility")
	}
	if err := runtime.disableWidescreen(ctx); err != nil {
		return err
	}
	return runtime.runTrackedGame(ctx, before, run, func() error {
		if err := runtime.requireClosedForCompatibility(); err != nil {
			return err
		}
		var cleanupErr error
		if cleanup != nil {
			cleanupErr = cleanup()
		}
		return errors.Join(cleanupErr, runtime.restoreWidescreen())
	})
}

// A bootstrap finishing is not proof that GTA exited. Keep temporary client/
// overlay files intact until process tracking confirms a safe restore point.
func (runtime *Runtime) runTrackedGame(ctx context.Context, before gameProcessSet, run func() error, cleanup func() error) error {
	runDone := make(chan error, 1)
	launcherDone := make(chan struct{})
	go func() { runDone <- run(); close(launcherDone) }()
	started, waitErr := runtime.waitForSpawnedGame(ctx, before, launcherDone)
	if waitErr != nil {
		var runErr error
		select {
		case runErr = <-runDone:
		default:
		}
		return errors.Join(runErr, fmt.Errorf("wait for GTA to exit: %w", waitErr))
	}
	var runErr error
	if started {
		// Observing GTA is authoritative: bootstrappers can outlive it or return
		// a Wine status even though they launched the game successfully.
		select {
		case <-runDone:
		default:
		}
	} else {
		runErr = <-runDone
	}
	var cleanupErr error
	if cleanup != nil {
		cleanupErr = cleanup()
	}
	return errors.Join(runErr, cleanupErr)
}
