package gui

import (
	"embed"
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/agnivade/levenshtein"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/gui/components"
	"github.com/gravitl/netmaker/netclient/gui/components/views"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

//go:embed nm-logo-sm.png
var logoContent embed.FS

// Run - run's the netclient GUI
func Run(networks []string) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Log(0, "No monitor detected, please use CLI commands; use -help for more info.")
		}
	}()
	a := app.New()
	window := a.NewWindow("Netclient - " + ncutils.Version)

	img, err := logoContent.ReadFile("nm-logo-sm.png")
	if err != nil {
		logger.Log(0, "failed to read logo", err.Error())
		return err
	}

	window.SetIcon(&fyne.StaticResource{StaticName: "Netmaker logo", StaticContent: img})
	window.Resize(fyne.NewSize(600, 450))

	networkView := container.NewVScroll(views.GetNetworksView(networks))
	networkView.SetMinSize(fyne.NewSize(400, 300))
	views.SetView(views.Networks, networkView)

	netDetailsViews := container.NewVScroll(views.GetSingleNetworkView(""))
	netDetailsViews.SetMinSize(fyne.NewSize(400, 300))
	views.SetView(views.NetDetails, netDetailsViews)
	window.SetFixedSize(false)

	searchBar := widget.NewEntry()
	searchBar.PlaceHolder = "Search a Network ..."
	searchBar.TextStyle = fyne.TextStyle{
		Italic: true,
	}
	searchBar.OnChanged = func(text string) {
		if text == "" {
			networkView = container.NewVScroll(views.GetNetworksView(networks))
			networkView.SetMinSize(fyne.NewSize(400, 300))
			views.RefreshComponent(views.Networks, networkView)
			views.ShowView(views.Networks)
			return
		}

		opts := []string{}
		for _, n := range networks {
			r := levenshtein.ComputeDistance(text, n)
			if r <= 2 {
				opts = append(opts, n)
			}
		}

		// fmt.Println(opts)
		networkView = container.NewVScroll(views.GetNetworksView(opts))
		networkView.SetMinSize(fyne.NewSize(400, 300))
		views.RefreshComponent(views.Networks, networkView)
		views.ShowView(views.Networks)
		opts = nil
	}

	toolbar := container.NewCenter(widget.NewToolbar(
		components.NewToolbarLabelButton("Networks", theme.HomeIcon(), func() {
			searchBar.Show()
			views.ShowView(views.Networks)
			views.ClearNotification()
		}, components.Blue_color),
		components.NewToolbarLabelButton("Join new", theme.ContentAddIcon(), func() {
			searchBar.Hide()
			views.ShowView(views.Join)
		}, components.Gravitl_color),
		components.NewToolbarLabelButton("Uninstall", theme.ErrorIcon(), func() {
			searchBar.Hide()
			confirmView := views.GetConfirmation("Confirm Netclient uninstall?", func() {
				views.ShowView(views.Networks)
			}, func() {
				views.LoadingNotify()
				err := functions.Uninstall()
				if err != nil {
					views.ErrorNotify("Failed to uninstall: \n" + err.Error())
				} else {
					views.SuccessNotify("Uninstalled Netclient!")
				}
				networks, err := ncutils.GetSystemNetworks()
				if err != nil {
					networks = []string{}
				}
				views.RefreshComponent(views.Networks, views.GetNetworksView(networks))
				views.ShowView(views.Networks)
			})
			views.RefreshComponent(views.Confirm, confirmView)
			views.ShowView(views.Confirm)
		}, components.Red_color),
		components.NewToolbarLabelButton("Close", theme.ContentClearIcon(), func() {
			os.Exit(0)
		}, components.Purple_color),
	))

	joinView := views.GetJoinView()
	views.SetView(views.Join, joinView)

	confirmView := views.GetConfirmation("", func() {}, func() {})
	views.SetView(views.Confirm, confirmView)

	views.ShowView(views.Networks)

	initialNotification := views.GenerateNotification("", color.Transparent)
	views.SetView(views.Notify, initialNotification)

	views.CurrentContent = container.NewVBox()

	views.CurrentContent.Add(container.NewGridWithRows(
		2,
		toolbar,
		searchBar,
	))
	views.CurrentContent.Add(views.GetView(views.Networks))
	views.CurrentContent.Add(views.GetView(views.NetDetails))
	views.CurrentContent.Add(views.GetView(views.Notify))
	views.CurrentContent.Add(views.GetView(views.Join))

	window.SetContent(views.CurrentContent)
	window.ShowAndRun()

	return nil
}
