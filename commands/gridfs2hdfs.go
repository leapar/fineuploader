package commands

import (
	"gopkg.in/urfave/cli.v1"
	Gf2Hdfs "../exporter"
	"gopkg.in/urfave/cli.v1/altsrc"
	"../config"
)

func Gridfs2Hdfs()  cli.Command {
	config := config.Config{}
	return cli.Command{
		Name:    "export",
		Aliases: []string{"e"},
		Usage:   "export file from mongodb to hadoop ",
		Flags:[]cli.Flag{
			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "input.mongodb.url",
				Value:       "127.0.0.1:27017",
				Usage:       "the mongodb url",
				Destination: &config.InputMongo.MongoServer,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "input.mongodb.md5",
				Value:       "",
				Usage:       "the file md5 value",
				Destination: &config.InputMongo.FileMd5,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "storage.hdfs.url",
				Value:       "localhost:19000",
				Usage:       "the hdfs url",
				Destination: &config.OutputHdfs.Url,
			}),

			altsrc.NewStringFlag(cli.StringFlag{
				Name:        "storage.hdfs.user",
				Value:       "",
				Usage:       "the hdfs user",
				Destination: &config.OutputHdfs.User,
			}),
		},

		Action: func(c *cli.Context) error {
			//fmt.Println(config)
			//fmt.Println(config.OutputMongo.MongoServer)
			downloader := Gf2Hdfs.New(config)
			downloader.Start()
			return nil
		},
	}
}