package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
)

// Override theme roles so Fyne's own pickers and controls use the same icons as
// the application toolbar. Resources remain vectors and follow theme colors.
var fontAwesomeIcons = loadFontAwesomeIcons()

func loadFontAwesomeIcons() map[fyne.ThemeIconName]fyne.Resource {
	files := map[fyne.ThemeIconName]string{
		theme.IconNameCancel:             "solid-xmark.svg",
		theme.IconNameWindowClose:        "solid-xmark.svg",
		theme.IconNameConfirm:            "solid-check.svg",
		theme.IconNameDelete:             "solid-trash-can.svg",
		theme.IconNameSearch:             "solid-magnifying-glass.svg",
		theme.IconNameMenu:               "solid-bars.svg",
		theme.IconNameMenuExpand:         "solid-chevron-right.svg",
		theme.IconNameMoreHorizontal:     "solid-ellipsis.svg",
		theme.IconNameMoreVertical:       "solid-ellipsis-vertical.svg",
		theme.IconNameFolder:             "solid-folder.svg",
		theme.IconNameFolderOpen:         "solid-folder-open.svg",
		theme.IconNameFolderNew:          "solid-folder-plus.svg",
		theme.IconNameDownload:           "solid-download.svg",
		theme.IconNameUpload:             "solid-upload.svg",
		theme.IconNameSettings:           "solid-gear.svg",
		theme.IconNameInfo:               "solid-circle-info.svg",
		theme.IconNameQuestion:           "solid-circle-question.svg",
		theme.IconNameHelp:               "solid-circle-question.svg",
		theme.IconNameWarning:            "solid-triangle-exclamation.svg",
		theme.IconNameError:              "solid-circle-exclamation.svg",
		theme.IconNameContentCopy:        "regular-copy.svg",
		theme.IconNameContentPaste:       "solid-paste.svg",
		theme.IconNameContentCut:         "solid-scissors.svg",
		theme.IconNameContentAdd:         "solid-plus.svg",
		theme.IconNameContentRemove:      "solid-minus.svg",
		theme.IconNameContentClear:       "solid-xmark.svg",
		theme.IconNameContentUndo:        "solid-rotate-left.svg",
		theme.IconNameContentRedo:        "solid-rotate-right.svg",
		theme.IconNameDocumentSave:       "solid-floppy-disk.svg",
		theme.IconNameDocument:           "regular-file-lines.svg",
		theme.IconNameDocumentCreate:     "regular-file.svg",
		theme.IconNameFile:               "regular-file.svg",
		theme.IconNameFileText:           "regular-file-lines.svg",
		theme.IconNameFileApplication:    "solid-file-code.svg",
		theme.IconNameFileAudio:          "regular-file-audio.svg",
		theme.IconNameFileImage:          "regular-file-image.svg",
		theme.IconNameFileVideo:          "regular-file-video.svg",
		theme.IconNameNavigateBack:       "solid-arrow-left.svg",
		theme.IconNameNavigateNext:       "solid-arrow-right.svg",
		theme.IconNameMoveUp:             "solid-arrow-up.svg",
		theme.IconNameMoveDown:           "solid-arrow-down.svg",
		theme.IconNameArrowDropDown:      "solid-chevron-down.svg",
		theme.IconNameArrowDropUp:        "solid-chevron-up.svg",
		theme.IconNameHome:               "solid-house.svg",
		theme.IconNameComputer:           "solid-computer.svg",
		theme.IconNameDesktop:            "solid-desktop.svg",
		theme.IconNameStorage:            "solid-hard-drive.svg",
		theme.IconNameList:               "solid-list.svg",
		theme.IconNameGrid:               "solid-table-cells-large.svg",
		theme.IconNameViewRefresh:        "solid-arrows-rotate.svg",
		theme.IconNameHistory:            "solid-clock-rotate-left.svg",
		theme.IconNameMediaMusic:         "solid-music.svg",
		theme.IconNameMediaPhoto:         "regular-image.svg",
		theme.IconNameMediaVideo:         "solid-film.svg",
		theme.IconNameCheckButton:        "regular-square.svg",
		theme.IconNameCheckButtonChecked: "solid-square-check.svg",
		theme.IconNameCheckButtonFill:    "solid-square.svg",
		theme.IconNameCheckButtonPartial: "solid-square-minus.svg",
		theme.IconNameVisibility:         "solid-eye.svg",
		theme.IconNameVisibilityOff:      "solid-eye-slash.svg",
	}
	icons := make(map[fyne.ThemeIconName]fyne.Resource, len(files))
	for name, file := range files {
		data, err := assets.FontAwesome.ReadFile("fontawesome/" + file)
		if err != nil {
			panic(err)
		} // A missing embedded asset is a build defect.
		icons[name] = theme.NewThemedResource(fyne.NewStaticResource("fontawesome-"+file, data))
	}
	return icons
}

func (t compactTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if icon, ok := fontAwesomeIcons[name]; ok {
		return icon
	}
	return t.Theme.Icon(name)
}
