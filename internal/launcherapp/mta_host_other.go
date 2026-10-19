//go:build !windows

package launcherapp

import (
	"context"
	"fmt"
	"os"
)

func StartNativeMTAHost(context.Context, string) (bool, error) { return false, nil }
func HandleMTAHost([]string) (bool, error)                     { return false, nil }
func ShowMTAHostError(err error)                               { fmt.Fprintln(os.Stderr, err) }
