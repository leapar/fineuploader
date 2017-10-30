package commands

import (
	"gopkg.in/urfave/cli.v1"
	HttpServer "../server"
	"gopkg.in/urfave/cli.v1/altsrc"
)

func Server()  cli.Command {
	var host,dir,mongo string
	var port int
	return cli.Command{
		Name:    "http",
		Aliases: []string{"s"},
		Usage:   "start a http server",
		Flags:[]cli.Flag{
			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "server.host",
				Value:       "0.0.0.0",
				Usage:       "the host",
				Destination: &host,
			}),

			altsrc.NewIntFlag(cli.IntFlag{
				Name:        "server.port",
				Value:       8080,
				Usage:       "bind port",
				Destination: &port,
			}),

			altsrc.NewStringFlag(
				cli.StringFlag{
					Name:        "server.directory",
					Value:       "uploads",
					Usage:       "the  Path of upload files",
					Destination: &dir,
				},
			),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "server.mongo",
				Value:       "127.0.0.1:27017",
				Usage:       "the mongodb url",
				Destination: &mongo,
			}),
		},

		Action: func(c *cli.Context) error {
			downloader := HttpServer.New(port ,host ,mongo ,dir )
			downloader.Start()
			return nil
		},
	}
}