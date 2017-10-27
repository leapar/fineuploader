package commands

import (
	"github.com/urfave/cli"
	HttpServer "../server"
)

func Server()  cli.Command {
	var host,dir,mongo string
	var port int
	return cli.Command{
		Name:    "http",
		Aliases: []string{"s"},
		Usage:   "start a http server",
		Flags:[]cli.Flag{
			cli.StringFlag{
				Name:        "host",
				Value:       "0.0.0.0",
				Usage:       "the host",
				Destination: &host,
			},

			cli.IntFlag{
				Name:        "port,p",
				Value:       8080,
				Usage:       "bind port",
				Destination: &port,
			},

			cli.StringFlag{
				Name:        "directory,d",
				Value:       "uploads",
				Usage:       "the  Path of upload files",
				Destination: &dir,
			},

			cli.StringFlag{
				Name:        "mongo,c",
				Value:       "127.0.0.1:27017",
				Usage:       "the mongodb url",
				Destination: &mongo,
			},
		},
		Action: func(c *cli.Context) error {
			downloader := HttpServer.New(port ,host ,mongo ,dir )
			downloader.Start()
			return nil
		},
	}
}