package views

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/gui/components"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetNetworksView - displays the view of all networks
func GetNetworksView(networks []string) fyne.CanvasObject {
	// renders := []fyne.CanvasObject{}
	if len(networks) == 0 {
		return container.NewCenter(widget.NewLabel("No networks present"))
	}
	grid := container.New(layout.NewGridLayout(5),
		container.NewCenter(widget.NewLabel("Network Name")),
		container.NewCenter(widget.NewLabel("Node Info")),
		container.NewCenter(widget.NewLabel("Pull Latest")),
		container.NewCenter(widget.NewLabel("Leave network")),
		container.NewCenter(widget.NewLabel("Connection status")),
	)
	for i := range networks {
		network := &networks[i]
		grid.Add(
			container.NewCenter(widget.NewLabel(*network)),
		)
		grid.Add(
			components.ColoredIconButton("info", theme.InfoIcon(), func() {
				RefreshComponent(NetDetails, GetSingleNetworkView(*network))
				ShowView(NetDetails)
			}, components.Gold_color),
		)
		grid.Add(
			components.ColoredIconButton("pull", theme.DownloadIcon(), func() {
				// TODO call pull with network name
				pull(*network)
			}, components.Blue_color),
		)
		grid.Add(
			components.ColoredIconButton("leave", theme.DeleteIcon(), func() {
				leave(*network)
			}, components.Danger_color),
		)
		cfg, err := config.ReadConfig(*network)
		if err != nil {
			fmt.Println(err)
		}
		if cfg.Node.Connected == "yes" {
			grid.Add(
				components.ColoredIconButton("disconnect", theme.CheckButtonCheckedIcon(), func() {
					disconnect(*network)
				}, components.Gravitl_color),
			)
		} else {
			grid.Add(
				components.ColoredIconButton("connect", theme.CheckButtonIcon(), func() {
					connect(*network)
				}, components.Danger_color),
			)
		}

		// renders = append(renders, container.NewCenter(netToolbar))
	}

	return container.NewCenter(grid)
}

// GetSingleNetworkView - returns details and option to pull a network
func GetSingleNetworkView(network string) fyne.CanvasObject {
	if len(network) == 0 {
		return container.NewCenter(widget.NewLabel("No valid network selected"))
	}

	// == read node values ==
	LoadingNotify()
	nets, err := functions.List(network)
	if err != nil || len(nets) < 1 {
		ClearNotification()
		return container.NewCenter(widget.NewLabel("No data retrieved."))
	}
	var nodecfg config.ClientConfig
	nodecfg.Network = network
	nodecfg.ReadConfig()
	nodeID := nodecfg.Node.ID
	lastCheckInTime := time.Unix(nodecfg.Node.LastCheckIn, 0)
	lastCheckIn := lastCheckInTime.Format("2006-01-02 15:04:05")
	privateAddr := nodecfg.Node.Address
	privateAddr6 := nodecfg.Node.Address6
	endpoint := nodecfg.Node.Endpoint
	health := " (HEALTHY)"
	if time.Now().After(lastCheckInTime.Add(time.Minute * 30)) {
		health = " (ERROR)"
	} else if time.Now().After(lastCheckInTime.Add(time.Minute * 5)) {
		health = " (WARNING)"
	}
	lastCheckIn += health
	version := nodecfg.Node.Version

	pullBtn := components.ColoredButton("pull "+network, func() { pull(network) }, components.Blue_color)
	pullBtn.Resize(fyne.NewSize(pullBtn.Size().Width, 50))

	view := container.NewGridWithColumns(1, widget.NewRichTextFromMarkdown(fmt.Sprintf(`### %s
- ID: %s
- Last Check In: %s
- Endpoint: %s
- Address (IPv4): %s
- Address6 (IPv6): %s
- Version: %s
### Peers
	`, network, nodeID, lastCheckIn, endpoint, privateAddr, privateAddr6, version)),
	)
	netDetailsView := container.NewCenter(
		view,
	)

	peerView := container.NewVBox()

	for _, p := range nets[0].Peers {
		peerString := ""
		endpointEntry := widget.NewEntry()
		endpointEntry.Text = fmt.Sprintf("Endpoint: %s", p.PublicEndpoint)
		endpointEntry.Disable()
		newEntry := widget.NewEntry()
		for i, addr := range p.Addresses {
			if i > 0 && i < len(p.Addresses) {
				peerString += ", "
			}
			peerString += addr.IP
		}
		newEntry.Text = peerString
		newEntry.Disable()
		peerView.Add(widget.NewLabel(fmt.Sprintf("Peer: %s", p.PublicKey)))
		peerView.Add(container.NewVBox(container.NewVBox(endpointEntry), container.NewVBox(newEntry)))
	}
	peerScroller := container.NewVScroll(peerView)
	view.Add(peerScroller)
	view.Add(container.NewVBox(pullBtn))
	netDetailsView.Refresh()
	ClearNotification()
	return netDetailsView
}

// == private ==
func pull(network string) {
	LoadingNotify()
	_, err := functions.Pull(network, true)
	if err != nil {
		ErrorNotify("Failed to pull " + network + " : " + err.Error())
	} else {
		SuccessNotify("Pulled " + network + "!")
	}
}

func leave(network string) {

	confirmView := GetConfirmation("Confirm leaving "+network+"?", func() {
		ShowView(Networks)
	}, func() {
		LoadingNotify()
		err := functions.LeaveNetwork(network)
		if err != nil {
			ErrorNotify("Failed to leave " + network + " : " + err.Error())
		} else {
			SuccessNotify("Left " + network)
		}
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			networks = []string{}
			ErrorNotify("Failed to read local networks!")
		}
		RefreshComponent(Networks, GetNetworksView(networks))
		ShowView(Networks)
	})
	RefreshComponent(Confirm, confirmView)
	ShowView(Confirm)
}
func connect(network string) {

	confirmView := GetConfirmation("Confirm connecting "+network+"?", func() {
		ShowView(Networks)
	}, func() {
		LoadingNotify()
		err := functions.Connect(network)
		if err != nil {

			ErrorNotify("Failed to connect " + network + " : " + err.Error())

		} else {
			SuccessNotify("connected to " + network)
		}
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			networks = []string{}
			ErrorNotify("Failed to read local networks!")
		}
		RefreshComponent(Networks, GetNetworksView(networks))
		ShowView(Networks)
	})
	RefreshComponent(Confirm, confirmView)
	ShowView(Confirm)
}
func disconnect(network string) {

	confirmView := GetConfirmation("Confirm disconnecting  "+network+"?", func() {
		ShowView(Networks)
	}, func() {
		LoadingNotify()
		fmt.Println(network)
		err := functions.Disconnect(network)
		if err != nil {

			ErrorNotify("Failed to disconnect " + network + " : " + err.Error())

		} else {
			SuccessNotify("disconnected from " + network)
		}
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			networks = []string{}
			ErrorNotify("Failed to read local networks!")
		}
		RefreshComponent(Networks, GetNetworksView(networks))
		ShowView(Networks)
	})
	RefreshComponent(Confirm, confirmView)
	ShowView(Confirm)
}
