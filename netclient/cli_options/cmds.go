package cli_options

import (
	"github.com/gravitl/netmaker/logger"
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
				parseVerbosity(c)
				cfg, pvtKey, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Join(&cfg, pvtKey)
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
				parseVerbosity(c)
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Leave(&cfg)
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
				parseVerbosity(c)
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				err = command.Pull(&cfg)
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
				parseVerbosity(c)
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
				parseVerbosity(c)
				err := command.Uninstall()
				return err
			},
		},
		{
			Name:  "daemon",
			Usage: "run netclient as daemon",
			Flags: cliFlags,
			Action: func(c *cli.Context) error {
				// set max verbosity for daemon regardless
				logger.Verbosity = 4
				err := command.Daemon()
				return err
			},
		},
		{
			Name:  "install",
			Usage: "install binary and daemon",
			Flags: cliFlags,
			Action: func(c *cli.Context) error {
				parseVerbosity(c)
				return command.Install()
			},
		},
		{
			Name:  "connect",
			Usage: "connect netclient to a given network if disconnected",
			Flags: cliFlags,
			Action: func(c *cli.Context) error {
				parseVerbosity(c)
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				return command.Connect(cfg)
			},
		},
		{
			Name:  "disconnect",
			Usage: "disconnect netclient from a given network if connected",
			Flags: cliFlags,
			Action: func(c *cli.Context) error {
				parseVerbosity(c)
				cfg, _, err := config.GetCLIConfig(c)
				if err != nil {
					return err
				}
				return command.Disconnect(cfg)
			},
		},
	}
}

// == Private funcs ==

func parseVerbosity(c *cli.Context) {
	if c.Bool("v") {
		logger.Verbosity = 1
	} else if c.Bool("vv") {
		logger.Verbosity = 2
	} else if c.Bool("vvv") {
		logger.Verbosity = 3
	} else if c.Bool("vvvv") {
		logger.Verbosity = 4
	}
}
