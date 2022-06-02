package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/gui/components"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetJoinView - get's the join screen where a user inputs an access token
func GetJoinView() fyne.CanvasObject {

	input := widget.NewMultiLineEntry()
	input.SetPlaceHolder("access token here...")

	submitBtn := components.ColoredIconButton("Submit", theme.UploadIcon(), func() {
		// ErrorNotify("Could not process token")
		LoadingNotify()
		var cfg config.ClientConfig
		accesstoken, err := config.ParseAccessToken(input.Text)
		if err != nil {
			ErrorNotify("Failed to parse access token!")
			return
		}
		cfg.Network = accesstoken.ClientConfig.Network
		cfg.Node.Network = accesstoken.ClientConfig.Network
		cfg.Node.Name = ncutils.GetHostname()
		cfg.AccessKey = accesstoken.ClientConfig.Key
		cfg.Node.LocalRange = accesstoken.ClientConfig.LocalRange
		cfg.Server.API = accesstoken.APIConnString
		err = functions.JoinNetwork(&cfg, "")
		if err != nil {
			ErrorNotify("Failed to join " + cfg.Network + "!")
			return
		}
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			ErrorNotify("Failed to read local networks!")
			return
		}
		SuccessNotify("Joined " + cfg.Network + "!")
		input.Text = ""
		RefreshComponent(Networks, GetNetworksView(networks))
		ShowView(Networks)
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
