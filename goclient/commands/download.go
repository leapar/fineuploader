package commands

import (
	"github.com/urfave/cli"
	//"fmt"
	DownLoader"../download"
	"github.com/boltdb/bolt"
	"log"
)

func Download()  cli.Command {
	var url string
	var chunckSize int64
	//"https://raw.githubusercontent.com/alvatar/multipart-downloader/master/test/quijote.txt"
	return cli.Command{
		Name:    "download",
		Aliases: []string{"d"},
		Usage:   "download a file",
		Flags:[]cli.Flag{
			cli.StringFlag{
				Name:        "url,u",
				Value:       "",
				Usage:       "the host Path",
				Destination: &url,
			},
			cli.StringFlag{
				Name:        "file,f",
				Value:       "",
				Usage:       "the file Path to save",
				Destination: &filePath,
			},
			cli.IntFlag{
				Name:        "con,n",
				Value:       5,
				Usage:       "con Num",
				Destination: &conNum,
			},
			cli.Int64Flag{
				Name:        "chunck,c",
				Value:       2*1024*1024,
				Usage:       "chunck size",
				Destination: &chunckSize,
			},
		},
		Action: func(c *cli.Context) error {
			boltDB, err := bolt.Open("uploader.db", 0600, nil)
			if err != nil {
				log.Fatal(err)
				return nil
			}

			defer boltDB.Close()
			//fmt.Println("completed task: ", c.Args().First())
			downloader := DownLoader.New(chunckSize,conNum,boltDB)
			downloader.Download(url,10)
			return nil
		},
	}
}