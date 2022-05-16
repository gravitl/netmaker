package views

import (
	"fyne.io/fyne/v2"
)

// CurrentContent - the content currently being displayed
var CurrentContent *fyne.Container

// RemoveContent - removes a rendered content
func RemoveContent(name string) {
	CurrentContent.Remove(GetView(name))
}

// AddContent - adds content to be rendered
func AddContent(name string) {
	CurrentContent.Add(GetView(name))
}

// RefreshComponent - refreshes the component to re-render
func RefreshComponent(name string, c fyne.CanvasObject) {
	RemoveContent(name)
	SetView(name, c)
	AddContent(name)
}
