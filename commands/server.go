package commands

import (
	"gopkg.in/urfave/cli.v1"
	HttpServer "../server"
	"gopkg.in/urfave/cli.v1/altsrc"
	"fmt"
	"../config"
)

func Server()  cli.Command {
	config := config.Config{}
	return cli.Command{
		Name:    "http",
		Aliases: []string{"s"},
		Usage:   "start a http server",
		Flags:[]cli.Flag{
			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "server.host",
				Value:       "0.0.0.0",
				Usage:       "the host",
				Destination: &config.Host,
			}),

			altsrc.NewIntFlag(cli.IntFlag{
				Name:        "server.port",
				Value:       8080,
				Usage:       "bind port",
				Destination: &config.Port,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "server.output",
				Value:       "mongodb",
				Usage:       "the output ",
				Destination: &config.Output,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "server.storage",
				Value:       "direct",
				Usage:       "the storage ",
				Destination: &config.StorageName,
			}),


			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "storage.mongodb.mongo",
				Value:       "127.0.0.1:27017",
				Usage:       "the mongodb url",
				Destination: &config.OutputMongo.MongoServer,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "output.nsq.nsqd",
				Value:       "127.0.0.1:4150",
				Usage:       "the nsq url",
				Destination: &config.OutputNsq.NsqServer,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "output.nsq.topic",
				Value:       "",
				Usage:       "the nsq topic",
				Destination: &config.OutputNsq.Topic,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "input.nsq.nsqd",
				Value:       "127.0.0.1:4150",
				Usage:       "the nsq url",
				Destination: &config.InputNsq.NsqServer,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "input.nsq.topic",
				Value:       "",
				Usage:       "the nsq topic",
				Destination: &config.InputNsq.Topic,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "input.nsq.channel",
				Value:       "",
				Usage:       "",
				Destination: &config.InputNsq.Channel,
			}),

			altsrc.NewBoolFlag(cli.BoolFlag{
				Name:        "input.nsq.enable",
				Usage:       "the nsq url",
				Destination: &config.InputNsq.Enable,
			}),
		},

		Action: func(c *cli.Context) error {
			fmt.Println(config)
			fmt.Println(config.OutputMongo.MongoServer)
			downloader := HttpServer.New(config)
			downloader.Start()
			return nil
		},
	}
}