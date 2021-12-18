package cli_options

import (
	"errors"

	"github.com/gravitl/netmaker/netclient/command"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/urfave/cli/v2"
)

// GetCommands - return commands that CLI uses
func GetCommands(cliFlags []cli.Flag) []*cli.Command {
	return []*cli.Command{
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
}
