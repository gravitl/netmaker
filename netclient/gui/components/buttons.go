package components

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ColoredButton - renders a colored button with text
func ColoredButton(text string, tapped func(), color color.Color) *fyne.Container {
	btn := widget.NewButton(text, tapped)
	bgColor := canvas.NewRectangle(color)
	return container.New(
		layout.NewMaxLayout(),
		bgColor,
		btn,
	)
}

// ColoredIconButton - renders a colored button with an icon
func ColoredIconButton(text string, icon fyne.Resource, tapped func(), color color.Color) *fyne.Container {
	btn := widget.NewButtonWithIcon(text, icon, tapped)
	bgColor := canvas.NewRectangle(color)
	return container.New(
		layout.NewMaxLayout(),
		btn,
		bgColor,
	)
}
