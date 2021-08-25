package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/gravitl/netmaker/netclient/command"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "Netclient CLI"
	app.Usage = "Netmaker's netclient agent and CLI. Used to perform interactions with Netmaker server and set local WireGuard config."
	app.Version = "v0.7.3"

	cliFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "network",
			Aliases: []string{"n"},
			EnvVars: []string{"NETCLIENT_NETWORK"},
			Value:   "all",
			Usage:   "Network to perform specified action against.",
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			EnvVars: []string{"NETCLIENT_PASSWORD"},
			Value:   "",
			Usage:   "Password for authenticating with netmaker.",
		},
		&cli.StringFlag{
			Name:    "endpoint",
			Aliases: []string{"e"},
			EnvVars: []string{"NETCLIENT_ENDPOINT"},
			Value:   "",
			Usage:   "Reachable (usually public) address for WireGuard (not the private WG address).",
		},
		&cli.StringFlag{
			Name:    "macaddress",
			Aliases: []string{"m"},
			EnvVars: []string{"NETCLIENT_MACADDRESS"},
			Value:   "",
			Usage:   "Mac Address for this machine. Used as a unique identifier within Netmaker network.",
		},
		&cli.StringFlag{
			Name:    "publickey",
			Aliases: []string{"pubkey"},
			EnvVars: []string{"NETCLIENT_PUBLICKEY"},
			Value:   "",
			Usage:   "Public Key for WireGuard Interface.",
		},
		&cli.StringFlag{
			Name:    "privatekey",
			Aliases: []string{"privkey"},
			EnvVars: []string{"NETCLIENT_PRIVATEKEY"},
			Value:   "",
			Usage:   "Private Key for WireGuard Interface.",
		},
		&cli.StringFlag{
			Name:    "port",
			EnvVars: []string{"NETCLIENT_PORT"},
			Value:   "",
			Usage:   "Port for WireGuard Interface.",
		},
		&cli.IntFlag{
			Name:    "keepalive",
			EnvVars: []string{"NETCLIENT_KEEPALIVE"},
			Value:   0,
			Usage:   "Default PersistentKeepAlive for Peers in WireGuard Interface.",
		},
		&cli.StringFlag{
			Name:    "operatingsystem",
			Aliases: []string{"os"},
			EnvVars: []string{"NETCLIENT_OS"},
			Value:   "",
			Usage:   "Identifiable name for machine within Netmaker network.",
		},
		&cli.StringFlag{
			Name:    "name",
			EnvVars: []string{"NETCLIENT_NAME"},
			Value:   "",
			Usage:   "Identifiable name for machine within Netmaker network.",
		},
		&cli.StringFlag{
			Name:    "localaddress",
			EnvVars: []string{"NETCLIENT_LOCALADDRESS"},
			Value:   "",
			Usage:   "Local address for machine. Can be used in place of Endpoint for machines on the same LAN.",
		},
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			EnvVars: []string{"NETCLIENT_ADDRESS"},
			Value:   "",
			Usage:   "WireGuard address for machine within Netmaker network.",
		},
		&cli.StringFlag{
			Name:    "addressIPv6",
			Aliases: []string{"a6"},
			EnvVars: []string{"NETCLIENT_ADDRESSIPV6"},
			Value:   "",
			Usage:   "WireGuard address for machine within Netmaker network.",
		},
		&cli.StringFlag{
			Name:    "interface",
			Aliases: []string{"i"},
			EnvVars: []string{"NETCLIENT_INTERFACE"},
			Value:   "",
			Usage:   "WireGuard local network interface name.",
		},
		&cli.StringFlag{
			Name:    "apiserver",
			EnvVars: []string{"NETCLIENT_API_SERVER"},
			Value:   "",
			Usage:   "Address + GRPC Port (e.g. 1.2.3.4:50051) of Netmaker server.",
		},
		&cli.StringFlag{
			Name:    "grpcserver",
			EnvVars: []string{"NETCLIENT_GRPC_SERVER"},
			Value:   "",
			Usage:   "Address + API Port (e.g. 1.2.3.4:8081) of Netmaker server.",
		},
		&cli.StringFlag{
			Name:    "key",
			Aliases: []string{"k"},
			EnvVars: []string{"NETCLIENT_ACCESSKEY"},
			Value:   "",
			Usage:   "Access Key for signing up machine with Netmaker server during initial 'add'.",
		},
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			EnvVars: []string{"NETCLIENT_ACCESSTOKEN"},
			Value:   "",
			Usage:   "Access Token for signing up machine with Netmaker server during initial 'add'.",
		},
		&cli.StringFlag{
			Name:    "localrange",
			EnvVars: []string{"NETCLIENT_LOCALRANGE"},
			Value:   "",
			Usage:   "Local Range if network is local, for instance 192.168.1.0/24.",
		},
		&cli.StringFlag{
			Name:    "dnson",
			EnvVars: []string{"NETCLIENT_DNS"},
			Value:   "yes",
			Usage:   "Sets private dns if 'yes'. Ignores if 'no'. Will retrieve from network if unset.",
		},
		&cli.StringFlag{
			Name:    "islocal",
			EnvVars: []string{"NETCLIENT_IS_LOCAL"},
			Value:   "",
			Usage:   "Sets endpoint to local address if 'yes'. Ignores if 'no'. Will retrieve from network if unset.",
		},
		&cli.StringFlag{
			Name:    "isdualstack",
			EnvVars: []string{"NETCLIENT_IS_DUALSTACK"},
			Value:   "",
			Usage:   "Sets ipv6 address if 'yes'. Ignores if 'no'. Will retrieve from network if unset.",
		},
		&cli.StringFlag{
			Name:    "udpholepunch",
			EnvVars: []string{"NETCLIENT_UDP_HOLEPUNCH"},
			Value:   "",
			Usage:   "Turns on udp holepunching if 'yes'. Ignores if 'no'. Will retrieve from network if unset.",
		},
		&cli.StringFlag{
			Name:    "ipforwarding",
			EnvVars: []string{"NETCLIENT_IPFORWARDING"},
			Value:   "on",
			Usage:   "Sets ip forwarding on if 'on'. Ignores if 'off'. On by default.",
		},
		&cli.StringFlag{
			Name:    "postup",
			EnvVars: []string{"NETCLIENT_POSTUP"},
			Value:   "",
			Usage:   "Sets PostUp command for WireGuard.",
		},
		&cli.StringFlag{
			Name:    "postdown",
			EnvVars: []string{"NETCLIENT_POSTDOWN"},
			Value:   "",
			Usage:   "Sets PostDown command for WireGuard.",
		},
		&cli.StringFlag{
			Name:    "daemon",
			EnvVars: []string{"NETCLIENT_DAEMON"},
			Value:   "on",
			Usage:   "Installs daemon if 'on'. Ignores if 'off'. On by default.",
		},
		&cli.StringFlag{
			Name:    "roaming",
			EnvVars: []string{"NETCLIENT_ROAMING"},
			Value:   "on",
			Usage:   "Checks for IP changes if 'on'. Ignores if 'off'. On by default.",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "join",
			Usage: "Join a Netmaker network.",
			Flags: cliFlags,
			Action: func(c *cli.Context) error {
				cfg, pvtKey, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				if cfg.Network == "all" {
					err = errors.New("No network provided.")
					return err
				}
				if cfg.Server.GRPCAddress == "" {
					err = errors.New("No server address provided.")
					return err
				}
				err = command.Join(cfg, pvtKey)
				return err
			},
		},
		{
			Name:  "leave",
			Usage: "Leave a Netmaker network.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Leave(cfg)
				return err
			},
		},
		{
			Name:  "checkin",
			Usage: "Checks for local changes and then checks into the specified Netmaker network to ask about remote changes.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.CheckIn(cfg)
				return err
			},
		},
		{
			Name:  "push",
			Usage: "Push configuration changes to server.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Push(cfg)
				return err
			},
		},
		{
			Name:  "pull",
			Usage: "Pull latest configuration and peers from server.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Pull(cfg)
				return err
			},
		},
		{
			Name:  "list",
			Usage: "Get list of networks.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.List(cfg)
				return err
			},
		},
		{
			Name:  "uninstall",
			Usage: "Uninstall the netclient system service.",
			Flags: cliFlags,
			// the action, or code that will be executed when
			// we execute our `ns` command
			Action: func(c *cli.Context) error {
				err := command.Uninstall()
				return err
			},
		},
	}

	// start our application
	out, err := local.RunCmd("id -u")

	if err != nil {
		log.Fatal(out, err)
	}
	id, err := strconv.Atoi(string(out[:len(out)-1]))

	if err != nil {
		log.Fatal(err)
	}

	if id != 0 {
		log.Fatal("This program must be run with elevated privileges (sudo). This program installs a SystemD service and configures WireGuard and networking rules. Please re-run with sudo/root.")
	}

	_, err = exec.LookPath("wg")
	if err != nil {
		log.Println(err)
		log.Fatal("WireGuard not installed. Please install WireGuard (wireguard-tools) and try again.")
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
