package netmaker

import (
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var netmakerPluginName = "netmaker"
var log = clog.NewWithPlugin(netmakerPluginName)

func init() {
	plugin.Register(netmakerPluginName, setup)
}

func setup(c *caddy.Controller) error {
	nm, err := netmakerParse(c)
	if err != nil {
		return plugin.Error(netmakerPluginName, err)
	}

	config := dnsserver.GetConfig(c)
	nm.Zone = config.Zone

	c.OnStartup(func() error {
		go nm.runFetchEntries(nm.Entries)
		return nil
	})

	config.AddPlugin(func(next plugin.Handler) plugin.Handler {
		nm.Next = next
		return nm
	})
	return nil
}

func netmakerParse(c *caddy.Controller) (*Netmaker, error) {
	entries := make(DNSEntries)

	nm := Netmaker{
		RefreshDuration: time.Second * 5,
	}
	nm.Entries = &entries

	var APIURL string
	var APIKey string

	for c.Next() {
		for c.NextBlock() {
			switch c.Val() {
			case "api_url":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.ArgErr()
				}
				APIURL = args[0]
			case "api_key":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.ArgErr()
				}
				APIKey = args[0]
			case "refresh_duration":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.ArgErr()
				}
				d, err := time.ParseDuration(strings.Join(args, " "))
				if err != nil {
					return nil, c.ArgErr()
				}
				nm.RefreshDuration = d
			case "fallthrough":
				nm.Fall.SetZonesFromArgs(c.RemainingArgs())

			default:
				return nil, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}

	cl := NewClient(APIURL, APIKey)
	nm.DNSClient = cl

	return &nm, nil
}
