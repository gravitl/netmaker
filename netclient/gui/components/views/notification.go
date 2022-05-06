package views

import (
	"image/color"

	"fyne.io/fyne/v2"
	"github.com/gravitl/netmaker/netclient/gui/components"
)

// GenerateNotification - generates a notification
func GenerateNotification(text string, c color.Color) fyne.CanvasObject {
	return components.ColoredText(text, c)
}

// ChangeNotification - changes the current notification in the view
func ChangeNotification(text string, c color.Color) {
	RefreshComponent(Notify, GenerateNotification(text, c))
}

// ClearNotification - hides the notifications
func ClearNotification() {
	RefreshComponent(Notify, GenerateNotification("", color.Transparent))
}

// LoadingNotify - changes notification to loading...
func LoadingNotify() {
	RefreshComponent(Notify, GenerateNotification("loading...", components.Blue_color))
}

// ErrorNotify - changes notification to a specified error
func ErrorNotify(msg string) {
	RefreshComponent(Notify, GenerateNotification(msg, components.Danger_color))
}

// SuccessNotify - changes notification to a specified success message
func SuccessNotify(msg string) {
	RefreshComponent(Notify, GenerateNotification(msg, components.Gravitl_color))
}
