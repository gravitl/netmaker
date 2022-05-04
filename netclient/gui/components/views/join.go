package views

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gravitl/netmaker/netclient/gui/components"
)

// GetJoinView - get's the join screen where a user inputs an access token
func GetJoinView() fyne.CanvasObject {

	input := widget.NewMultiLineEntry()
	input.SetPlaceHolder("access token here...")

	submitBtn := components.ColoredIconButton("Submit", theme.UploadIcon(), func() {
		fmt.Printf("got text %s \n", input.Text)
		// ErrorNotify("Could not process token")
		LoadingNotify()
		time.Sleep(time.Second)
		SuccessNotify("Joined!")
		// TODO
		// - call join
		// - display loading
		// - on error display error notification
		// - on success notify success, refresh networks & networks view, display networks view
	}, components.Blue_color)

	return container.NewGridWithColumns(1,
		container.NewCenter(widget.NewLabel("Join new network with Access Token")),
		input,
		container.NewCenter(submitBtn),
	)
}
