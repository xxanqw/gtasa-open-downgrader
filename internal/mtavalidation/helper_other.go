//go:build !linux

package mtavalidation

import (
	"context"
	"fmt"
)

func Supported() bool { return false }
func HelperMain(args []string) (bool, error) {
	if len(args) > 0 && args[0] == helperFlag {
		return true, fmt.Errorf("MTA serial helper is only available on Linux x86-64")
	}
	return false, nil
}
func Run(context.Context, *Packages, Progress) (Result, error) {
	return Result{}, fmt.Errorf("MTA serial helper is only available on Linux x86-64")
}
