// Command patch-assets prepares and validates the assets used by offline releases.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"os"
	"os/signal"
)

func main() {
	target := flag.String("target", "internal/assets/Patches", "patch directory")
	url := flag.String("url", download.PatchURL, "direct ZIP URL")
	archive := flag.String("archive", "", "local ZIP archive")
	verify := flag.Bool("verify", false, "only validate existing assets")
	release := flag.Bool("release", false, "require the current Global/German release")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var e error
	if *verify {
		if *release {
			_, e = patch.ValidateReleaseAssets(*target)
		} else {
			_, e = patch.ValidateAssets(*target)
		}
	} else if *archive != "" {
		if *release {
			e = patch.ImportReleaseAssets(ctx, *archive, *target)
		} else {
			e = patch.ImportAssets(ctx, *archive, *target)
		}
	} else {
		e = patch.DownloadAssets(ctx, download.New(), *url, *target, nil)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("Patch assets validated:", *target)
}
