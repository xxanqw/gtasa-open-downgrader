package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func launcherCardIcon() fyne.CanvasObject {
	return multiplayerImage(fyne.NewStaticResource("steam-launcher-icon.png", assets.SteamLauncherIcon))
}

func multiplayerImage(resource fyne.Resource) *canvas.Image {
	image := canvas.NewImageFromResource(resource)
	image.FillMode = canvas.ImageFillContain
	image.ScaleMode = canvas.ImageScaleSmooth
	image.SetMinSize(fyne.NewSquareSize(56))
	return image
}

// Use the supplied client artwork unchanged, without a tile or glyph overlay.
func multiplayerCardIcon(id multiplayer.ClientID) fyne.CanvasObject {
	switch id {
	case multiplayer.ClientSAMP:
		return multiplayerImage(fyne.NewStaticResource("samp.png", assets.SAMPClientIcon))
	case multiplayer.ClientOpenMP:
		return multiplayerImage(fyne.NewStaticResource("openmp.png", assets.OpenMPClientIcon))
	case multiplayer.ClientMTA:
		return multiplayerImage(fyne.NewStaticResource("mta.png", assets.MTAClientIcon))
	default:
		data, err := assets.FontAwesome.ReadFile("fontawesome/solid-gamepad.svg")
		if err != nil {
			panic(err)
		}
		return newClassicIcon(fyne.NewStaticResource("multiplayer-solid-gamepad.svg", data))
	}
}

func multiplayerPurpose(id multiplayer.ClientID) string {
	switch id {
	case multiplayer.ClientSAMP:
		return "Classic San Andreas multiplayer client"
	case multiplayer.ClientOpenMP:
		return "Open Multiplayer launcher"
	case multiplayer.ClientMTA:
		return "Multi Theft Auto multiplayer client"
	}
	return "Multiplayer client"
}

type multiplayerIdentityLayout struct{}

func (multiplayerIdentityLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(200, 48) }
func (multiplayerIdentityLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	top := max(float32(0), (size.Height-48)/2)
	for i, object := range objects {
		object.Move(fyne.NewPos(0, top+float32(i)*24))
		object.Resize(fyne.NewSize(size.Width, 24))
	}
}

func newMultiplayerCard(title, purpose, state string, importance widget.Importance, icon fyne.CanvasObject, actions fyne.CanvasObject, detail string) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	description := compactLabel(purpose)
	description.Importance = widget.LowImportance
	label := widget.NewLabel(state)
	label.Importance = importance
	symbol := widget.NewIcon(modStateSymbol(state, importance))
	status := container.New(modStatusLayout{alignment: fyne.TextAlignTrailing}, symbol, label)
	statusView := container.NewThemeOverride(status, modStateTheme{fyne.CurrentApp().Settings().Theme()})
	right := container.NewVBox(statusView, container.NewHBox(layout.NewSpacer(), actions))
	identity := container.New(multiplayerIdentityLayout{}, name, description)
	body := container.NewVBox(container.NewBorder(nil, nil, container.NewCenter(icon), right, identity))
	if detail != "" {
		warning := widget.NewLabel(detail)
		warning.Wrapping = fyne.TextWrapWord
		warning.Importance = widget.WarningImportance
		body.Add(container.NewThemeOverride(warning, multiplayerNoticeTheme{fyne.CurrentApp().Settings().Theme()}))
	}
	panel := newFramedPanel(container.New(layout.NewCustomPaddedLayout(8, 8, 12, 12), body))
	return container.New(layout.NewCustomPaddedLayout(3, 3, 0, 0), panel)
}
