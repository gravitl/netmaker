package components

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ColoredText - renders a colored label
func ColoredText(text string, color color.Color) *fyne.Container {
	btn := widget.NewLabel(text)
	btn.Wrapping = fyne.TextWrapWord
	bgColor := canvas.NewRectangle(color)
	return container.New(
		layout.NewMaxLayout(),
		bgColor,
		btn,
	)
}
