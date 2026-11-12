package ui

import (
	"context"
	"errors"
	"strings"
)

type setupActivity int

const (
	setupIdle setupActivity = iota
	setupDiscovering
	setupIdentifying
	setupInstallingPatches
	setupImportingPatches
	setupScanning
)

var errMissingGameExecutable = errors.New("no gta_sa.exe or gta-sa.exe found in this folder")

type setupStep int

const (
	setupChooseFolder setupStep = iota
	setupCheckFolder
	setupNeedPatches
	setupReadyToScan
	setupFolderProblem
	setupReadOnly
	setupWorking
	setupResults
)

// A path string is only a candidate. scanPath is set only after a successful
// executable identification. Actual file verification also requires rows.
func (w *Window) nextSetupStep() setupStep {
	if w.busy && w.setupActivity != setupIdle {
		return setupWorking
	}
	if strings.TrimSpace(w.path.Text) == "" {
		return setupChooseFolder
	}
	if w.gameSetupError != nil {
		return setupFolderProblem
	}
	if w.scanPath == "" || w.scanPath != w.path.Text {
		return setupCheckFolder
	}
	if len(w.rows) > 0 {
		return setupResults
	}
	if w.readonly {
		return setupReadOnly
	}
	if w.patchDir == "" {
		return setupNeedPatches
	}
	return setupReadyToScan
}

func (w *Window) runSetup(activity setupActivity, title string, work func(context.Context) error, done func(error)) {
	if w.busy || w.closing {
		return
	}
	w.setupActivity = activity
	finish := func(err error) {
		w.setupActivity = setupIdle
		if done != nil {
			done(err)
		}
		w.refreshEnabled()
	}
	if activity == setupInstallingPatches || activity == setupImportingPatches {
		w.run(title, work, finish)
	} else {
		w.runHandled(title, work, finish)
	}
}
