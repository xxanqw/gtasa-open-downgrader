//go:build !windows

package launcherapp

import "context"

func snapshotGameProcesses() (gameProcessSet, error) {
	return gameProcessSet{}, nil
}

func waitForSpawnedGame(context.Context, gameProcessSet, <-chan struct{}) (bool, error) {
	return false, nil
}
