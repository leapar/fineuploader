package commands

import (
	"github.com/urfave/cli"
	//"fmt"
	DownLoader"../download"
	"github.com/boltdb/bolt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func Download()  cli.Command {
	var url string
	var chunckSize uint64
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
			cli.Uint64Flag{
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
			chQuit := make(chan os.Signal, 1)

			signal.Notify(chQuit, syscall.SIGINT, syscall.SIGTERM)
			//fmt.Println("completed task: ", c.Args().First())
			downloader := DownLoader.New(chunckSize,conNum,boltDB,chQuit)
			downloader.Download(url,10,nil)
			return nil
		},
	}
}