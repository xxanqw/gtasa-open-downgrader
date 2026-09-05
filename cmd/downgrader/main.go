package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/xxanqw/gtasa-open-downgrader/internal/ui"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) == 5 && os.Args[1] == "--apply-update" {
		pid, e := strconv.Atoi(os.Args[2])
		if e == nil {
			e = updater.ApplyHelper(context.Background(), pid, os.Args[3], os.Args[4])
		}
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	gamePath := flag.String("game", "", "game directory to scan")
	patchPath := flag.String("patches", "", "existing patch directory")
	version := flag.Bool("version", false, "print application version")
	flag.Parse()
	if *version {
		fmt.Println(updater.Version)
		return
	}
	ui.Run(*gamePath, *patchPath)
}
