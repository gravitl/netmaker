package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gravitl/netmaker/netclient/gui/components"
)

// GetConfirmation - displays a confirmation message
func GetConfirmation(msg string, onCancel, onConfirm func()) fyne.CanvasObject {
	return container.NewGridWithColumns(1,
		container.NewCenter(widget.NewLabel(msg)),
		container.NewCenter(
			container.NewHBox(
				components.ColoredIconButton("Confirm", theme.ConfirmIcon(), onConfirm, components.Gravitl_color),
				components.ColoredIconButton("Cancel", theme.CancelIcon(), onCancel, components.Danger_color),
			)),
	)
}
