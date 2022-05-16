package components

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewToolbarLabelButton - makes a toolbar button cell with label
func NewToolbarLabelButton(label string, icon fyne.Resource, onclick func(), colour color.Color) widget.ToolbarItem {
	l := ColoredIconButton(label, icon, onclick, colour)
	l.MinSize()
	return &toolbarLabelButton{l}
}

// NewToolbarLabel - makes a toolbar text cell
func NewToolbarLabel(label string) widget.ToolbarItem {
	l := widget.NewLabel(label)
	l.MinSize()
	return &toolbarLabel{l}
}

type toolbarLabelButton struct {
	*fyne.Container
}

type toolbarLabel struct {
	*widget.Label
}

func (t *toolbarLabelButton) ToolbarObject() fyne.CanvasObject {
	return container.NewCenter(t.Container)
}

func (t *toolbarLabel) ToolbarObject() fyne.CanvasObject {
	return container.NewCenter(t.Label)
}
