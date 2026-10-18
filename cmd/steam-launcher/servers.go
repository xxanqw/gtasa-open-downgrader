package main

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/openmpapi"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

type serverLister interface {
	List(context.Context) ([]openmpapi.Server, error)
}
type serverBrowser struct {
	ui                               *launcherWindow
	client                           serverLister
	window                           fyne.Window
	all, visible                     []openmpapi.Server
	favorites                        map[string]bool
	selected                         string
	search                           *widget.Entry
	filter, ordering                 *widget.Select
	table                            *widget.Table
	details, status                  *widget.Label
	host, port, nickname, password   *widget.Entry
	version                          *widget.Select
	versionIDs                       map[string]string
	favorite, copy, connect, refresh *appwidget.Button
	cancel                           context.CancelFunc
	closed, loading                  bool
	generation                       uint64
	lastFetch                        time.Time
	fetchedAt                        string
}

func newServerBrowser(ui *launcherWindow) *serverBrowser {
	b := &serverBrowser{ui: ui, client: openmpapi.New(), favorites: make(map[string]bool), versionIDs: make(map[string]string)}
	prefs := ui.app.Preferences()
	for _, address := range prefs.StringList("openmp-favorites") {
		if len(b.favorites) < 1000 {
			b.favorites[address] = true
		}
	}
	if cached, err := openmpapi.FromCache(prefs.String("openmp-server-cache")); err == nil {
		b.all = cached
		b.fetchedAt = prefs.String("openmp-server-cache-time")
	}
	b.status = widget.NewLabel("Cached API index | direct connection works offline")
	b.status.Truncation = fyne.TextTruncateEllipsis
	b.details = detailLabel("Select a server to inspect its API-reported information and rules.\n\nNo live ping or player roster is supplied by this index.")
	b.search = widget.NewEntry()
	b.search.SetPlaceHolder("Search name, address, mode or language")
	b.filter = widget.NewSelect([]string{"All servers", "With players", "Not full", "open.mp only", "Favorites"}, func(string) { b.applyFilter() })
	b.filter.SetSelected("All servers")
	b.ordering = widget.NewSelect([]string{"Most players", "Name A–Z"}, func(string) { b.applyFilter() })
	b.ordering.SetSelected("Most players")
	b.search.OnChanged = func(string) { b.applyFilter() }
	b.refresh = appwidget.NewButton("Refresh", func() { b.fetch() })
	b.copy = appwidget.NewButton("Copy address", func() {
		if b.selected != "" {
			ui.app.Clipboard().SetContent(b.selected)
		}
	})
	b.copy.Disable()
	b.favorite = appwidget.NewButton("Add favorite", func() {
		if b.selected == "" {
			return
		}
		if b.favorites[b.selected] {
			delete(b.favorites, b.selected)
		} else if len(b.favorites) < 1000 {
			b.favorites[b.selected] = true
		}
		addresses := make([]string, 0, len(b.favorites))
		for address := range b.favorites {
			addresses = append(addresses, address)
		}
		sort.Strings(addresses)
		prefs.SetStringList("openmp-favorites", addresses)
		b.updateDetails()
		b.applyFilter()
	})
	b.favorite.Disable()
	b.table = widget.NewTable(func() (int, int) { return len(b.visible), 4 }, func() fyne.CanvasObject {
		l := widget.NewLabel(" ")
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}, func(id widget.TableCellID, object fyne.CanvasObject) {
		if id.Row < 0 || id.Row >= len(b.visible) {
			return
		}
		s := b.visible[id.Row]
		value := ""
		switch id.Col {
		case 0:
			value = s.Core.Name
			if s.Core.Password {
				value = "[Locked] " + value
			}
			if b.favorites[s.Core.Address] {
				value = "★ " + value
			}
		case 1:
			value = fmt.Sprintf("%d / %d", s.Core.Players, s.Core.Capacity)
		case 2:
			value = s.Core.Mode
		case 3:
			value = s.Core.Language
		}
		object.(*widget.Label).SetText(value)
	})
	b.table.ShowHeaderRow = true
	b.table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	b.table.UpdateHeader = func(id widget.TableCellID, object fyne.CanvasObject) {
		if id.Col >= 0 && id.Col < 4 {
			object.(*widget.Label).SetText([]string{"Server", "Players", "Mode", "Language"}[id.Col])
		}
	}
	b.table.SetColumnWidth(0, 280)
	b.table.SetColumnWidth(1, 85)
	b.table.SetColumnWidth(2, 125)
	b.table.SetColumnWidth(3, 95)
	b.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Row >= len(b.visible) {
			return
		}
		s := b.visible[id.Row]
		changed := b.selected != s.Core.Address
		b.selected = s.Core.Address
		b.updateDetails()
		if changed {
			host, port, _ := net.SplitHostPort(b.selected)
			b.host.SetText(host)
			b.port.SetText(port)
		}
	}
	b.host = widget.NewEntry()
	b.host.SetPlaceHolder("IP address or hostname")
	b.host.SetText(prefs.String("openmp-host"))
	b.port = widget.NewEntry()
	b.port.SetText(prefs.StringWithFallback("openmp-port", "7777"))
	b.nickname = widget.NewEntry()
	b.nickname.SetPlaceHolder("Nickname")
	b.nickname.SetText(prefs.String("openmp-nickname"))
	b.password = widget.NewPasswordEntry()
	b.password.SetPlaceHolder("Optional; never saved")
	names := make([]string, 0, len(multiplayer.OpenMPSAMPVersions))
	selectedName := ""
	preferred := prefs.StringWithFallback("openmp-client-version", multiplayer.DefaultOpenMPSAMPVersion)
	for _, version := range multiplayer.OpenMPSAMPVersions {
		names = append(names, version.Name)
		b.versionIDs[version.Name] = version.ID
		if version.ID == preferred {
			selectedName = version.Name
		}
	}
	if selectedName == "" {
		selectedName = names[0]
	}
	b.version = widget.NewSelect(names, func(string) { b.updateConnect() })
	b.version.SetSelected(selectedName)
	b.connect = appwidget.NewButton("Connect", func() {
		if b.closed || ui.busy {
			return
		}
		connection := b.connection()
		if _, err := connection.Arguments(ui.gameDir); err != nil {
			appwidget.ShowError(b.window, err)
			return
		}
		prefs.SetString("openmp-host", strings.TrimSpace(connection.Host))
		prefs.SetString("openmp-port", strings.TrimSpace(connection.Port))
		prefs.SetString("openmp-nickname", strings.TrimSpace(connection.Nickname))
		prefs.SetString("openmp-client-version", connection.ClientVersion)
		b.close()
		ui.launch(func(ctx context.Context) error { return ui.actions.openMP(ctx, connection) })
	})
	b.connect.Importance = widget.HighImportance
	for _, entry := range []*widget.Entry{b.host, b.port, b.nickname, b.password} {
		entry.OnChanged = func(string) { b.updateConnect() }
	}
	b.host.Validator = func(value string) error {
		_, err := (multiplayer.OpenMPConnection{Host: value, Port: "7777", Nickname: "Player"}).Arguments(ui.gameDir)
		return err
	}
	b.port.Validator = func(value string) error {
		_, err := (multiplayer.OpenMPConnection{Host: "example.com", Port: value, Nickname: "Player"}).Arguments(ui.gameDir)
		return err
	}
	b.nickname.Validator = func(value string) error {
		_, err := (multiplayer.OpenMPConnection{Host: "example.com", Port: "7777", Nickname: value}).Arguments(ui.gameDir)
		return err
	}
	b.applyFilter()
	b.updateConnect()
	return b
}
func (b *serverBrowser) connection() multiplayer.OpenMPConnection {
	return multiplayer.OpenMPConnection{Host: b.host.Text, Port: b.port.Text, Nickname: b.nickname.Text, Password: b.password.Text, ClientVersion: b.versionIDs[b.version.Selected]}
}
func (b *serverBrowser) updateConnect() {
	if b.connect == nil {
		return
	}
	if _, err := b.connection().Arguments(b.ui.gameDir); err == nil && !b.closed {
		b.connect.Enable()
	} else {
		b.connect.Disable()
	}
}
func (b *serverBrowser) show() {
	toolbar := container.NewVBox(container.NewBorder(nil, nil, nil, b.refresh, launcherInput(b.search)), container.NewHBox(launcherInput(b.filter), launcherInput(b.ordering)))
	details := container.NewBorder(nil, container.NewHBox(b.copy, b.favorite), nil, nil, container.NewVScroll(b.details))
	split := container.NewHSplit(container.New(&serverTableLayout{table: b.table}, b.table), details)
	split.Offset = 0.66
	form := widget.NewForm(
		widget.NewFormItem("Server", launcherInput(b.host)), widget.NewFormItem("Port", launcherInput(b.port)),
		widget.NewFormItem("Nickname", launcherInput(b.nickname)), widget.NewFormItem("Password", launcherInput(b.password)), widget.NewFormItem("SA-MP client", launcherInput(b.version)),
	)
	footer := container.NewVBox(widget.NewSeparator(), widget.NewLabelWithStyle("Direct connection", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), form, container.NewHBox(layout.NewSpacer(), appwidget.NewButton("Close", func() { b.close() }), b.connect), b.status)
	body := container.NewBorder(toolbar, footer, nil, nil, split)
	b.window = b.ui.app.NewWindow("open.mp servers")
	b.window.SetIcon(clientArtwork(multiplayer.ClientOpenMP).Resource)
	b.window.SetContent(container.NewPadded(body))
	b.window.SetOnClosed(func() { appwidget.DismissModals(b.window); b.stop() })
	b.window.Resize(fyne.NewSize(980, 720))
	b.window.CenterOnScreen()
	b.window.Show()
	b.fetch()
}
func (b *serverBrowser) close() {
	if b.closed {
		return
	}
	b.stop()
	if b.window != nil {
		b.window.Close()
	}
}
func (b *serverBrowser) stop() {
	if b.closed {
		return
	}
	b.closed = true
	b.generation++
	if b.cancel != nil {
		b.cancel()
	}
	b.password.SetText("")
}
func (b *serverBrowser) fetch() {
	if b.closed || b.loading {
		return
	}
	if !b.lastFetch.IsZero() && time.Since(b.lastFetch) < 5*time.Second {
		b.status.SetText("Please wait a few seconds before refreshing the API index.")
		return
	}
	b.lastFetch = time.Now()
	b.loading = true
	b.refresh.Disable()
	b.status.SetText("Loading open.mp API index | direct connection remains available")
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.generation++
	generation := b.generation
	go func() {
		servers, err := b.client.List(ctx)
		b.ui.dispatch(func() {
			if b.closed || generation != b.generation {
				return
			}
			b.loading = false
			b.refresh.Enable()
			cancel()
			if err != nil {
				b.status.SetText("API unavailable | cached list / direct connection available")
				b.details.SetText("Could not refresh the open.mp server index:\n" + err.Error() + "\n\nExisting rows are cached; use Refresh to retry or connect directly.\nNo files in the game were changed.")
				return
			}
			b.all = servers
			b.fetchedAt = time.Now().Format("2006-01-02 15:04:05")
			b.ui.app.Preferences().SetString("openmp-server-cache", openmpapi.Encode(servers))
			b.ui.app.Preferences().SetString("openmp-server-cache-time", b.fetchedAt)
			b.applyFilter()
			b.updateDetails()
		})
	}()
}
func (b *serverBrowser) applyFilter() {
	// Select callbacks can fire while controls are still being constructed.
	if b.table == nil || b.filter == nil || b.ordering == nil || b.search == nil {
		return
	}
	b.visible = filterServers(b.all, b.search.Text, b.filter.Selected, b.ordering.Selected, b.favorites)
	b.table.UnselectAll()
	b.table.Refresh()
	selectedRow := -1
	for row, server := range b.visible {
		if server.Core.Address == b.selected {
			selectedRow = row
			break
		}
	}
	if selectedRow >= 0 {
		b.table.Select(widget.TableCellID{Row: selectedRow, Col: 0})
	} else {
		b.selected = ""
		b.copy.Disable()
		b.favorite.Disable()
		b.details.SetText("Select a server to inspect its API-reported information and rules.")
	}
	b.status.SetText(fmt.Sprintf("%d / %d servers | cached API counts | %s", len(b.visible), len(b.all), b.fetchedAt))
}
func filterServers(servers []openmpapi.Server, query, filter, ordering string, favorites map[string]bool) []openmpapi.Server {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]openmpapi.Server, 0, len(servers))
	for _, s := range servers {
		if query != "" && !strings.Contains(strings.ToLower(s.Core.Name+" "+s.Core.Address+" "+s.Core.Mode+" "+s.Core.Language), query) {
			continue
		}
		if filter == "With players" && s.Core.Players == 0 || filter == "Not full" && s.Core.Players >= s.Core.Capacity || filter == "open.mp only" && !s.Core.OpenMP || filter == "Favorites" && !favorites[s.Core.Address] {
			continue
		}
		result = append(result, s)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, z := result[i].Core, result[j].Core
		if ordering == "Most players" && a.Players != z.Players {
			return a.Players > z.Players
		}
		an, zn := strings.ToLower(a.Name), strings.ToLower(z.Name)
		if an != zn {
			return an < zn
		}
		return a.Address < z.Address
	})
	return result
}
func (b *serverBrowser) updateDetails() {
	for _, s := range b.all {
		if s.Core.Address != b.selected {
			continue
		}
		software := "SA-MP"
		if s.Core.OpenMP {
			software = "open.mp"
		}
		password := "No"
		if s.Core.Password {
			password = "Required"
		}
		text := fmt.Sprintf("%s\n%s\n\nPlayers: %d / %d (API cache)\nMode: %s\nLanguage: %s\nSoftware: %s\nVersion: %s\nPassword: %s", s.Core.Name, s.Core.Address, s.Core.Players, s.Core.Capacity, s.Core.Mode, s.Core.Language, software, s.Core.Version, password)
		for _, rule := range []struct{ key, label string }{{"allowed_clients", "Allowed clients"}, {"mapname", "Map"}, {"worldtime", "World time"}, {"weather", "Weather"}, {"lagcomp", "Lag compensation"}, {"artwork", "Custom artwork"}, {"weburl", "Website"}} {
			if value := s.Rules[rule.key]; value != "" {
				text += "\n" + rule.label + ": " + value
			}
		}
		if s.Updated != "" {
			text += "\n\nAPI server timestamp: " + s.Updated
		}
		text += "\n\nNo live ping or player roster. Choose the SA-MP client appropriate for this server; the version is not automatically changed."
		b.details.SetText(text)
		b.copy.Enable()
		b.favorite.Enable()
		if b.favorites[b.selected] {
			b.favorite.SetText("Remove favorite")
		} else {
			b.favorite.SetText("Add favorite")
		}
		return
	}
}
