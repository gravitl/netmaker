package views

import (
	"fyne.io/fyne/v2"
)

var (
	// Views - the map of all the view components
	views = make(map[string]fyne.CanvasObject)
)

const (
	Networks   = "networks"
	NetDetails = "netdetails"
	Notify     = "notification"
	Join       = "join"
	Confirm    = "confirm"
)

// GetView - returns the requested view and sets the CurrentView state
func GetView(viewName string) fyne.CanvasObject {
	return views[viewName]
}

// SetView - sets a view in the views map
func SetView(viewName string, component fyne.CanvasObject) {
	views[viewName] = component
}

// HideView - hides a specific view
func HideView(viewName string) {
	views[viewName].Hide()
}

// ShowView - show's a specific view
func ShowView(viewName string) {
	for k := range views {
		if k == Notify {
			continue
		}
		HideView(k)
	}
	views[viewName].Show()
}
