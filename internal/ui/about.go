package ui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func (w *Window) showAbout() {
	information := "GTA SA Open Downgrader · " + updater.Version + "\nMIT licensed\nBundled xdelta3: Apache License 2.0"
	choices := []string{"Source and releases", "Read bundled xdelta3 license", "Copy app information"}
	w.prompt(promptRequest{kind: promptChoice, title: "About GTA SA Open Downgrader", text: information, choices: choices, initial: choices[0], accept: "Open", dismiss: "Close", done: func(choice string, ok bool) {
		if !ok {
			return
		}
		switch choice {
		case choices[0]:
			w.error(w.app.OpenURL(mustURL("https://github.com/xxanqw/gtasa-open-downgrader")))
		case choices[1]:
			w.openLicense()
		case choices[2]:
			w.app.Clipboard().SetContent(information)
			w.status.SetText("App information copied.")
		}
	}})
}

func (w *Window) openLicense() {
	if err := os.MkdirAll(w.cache, 0700); err != nil {
		w.error(err)
		return
	}
	path := filepath.Join(w.cache, "xdelta3-LICENSE.txt")
	if err := os.WriteFile(path, []byte(assets.XdeltaLicense), 0644); err != nil {
		w.error(err)
		return
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	w.error(w.app.OpenURL(&url.URL{Scheme: "file", Path: path}))
}
