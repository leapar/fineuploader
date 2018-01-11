package commands

import (
	"github.com/urfave/cli"
	"os"
	"syscall"
	"os/signal"
	//"log"
	"github.com/boltdb/bolt"
	Uploader "../uploader"
	"log"
)

var chunckSize uint64//= flag.Uint64("c", 2*1024*1024, "chunk size,defaults to 2*1024*1024")
var filePath string// = flag.String("f", "", "Upload file Path")
var conNum int//= flag.Int("n", 5, "Batch Connections,defaults to 5")
var host string//= flag.String("h", "172.29.231.80:8082", "Upload host Path")

func Upload()  cli.Command{
	return cli.Command{
			Name:    "upload",
			Aliases: []string{"u"},
			Usage:   "upload a file",

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "host,u",
					Value:       "172.29.231.80:8082",
					Usage:       "the host Path",
					Destination: &host,
				},
				cli.StringFlag{
					Name:        "file,f",
					Value:       "",
					Usage:       "the file Path",
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
				var boltDB *bolt.DB
				boltDB, err := bolt.Open("uploader.db", 0600, nil)
				if err != nil {
					log.Fatal(err)
					return nil
				}

				defer boltDB.Close()

				if filePath == ""  {
					cli.ShowCommandHelp(c,"upload")
					return nil
				}
				chQuit := make(chan os.Signal, 1)

				signal.Notify(chQuit, syscall.SIGINT, syscall.SIGTERM)
				//log.Println(<-chQuit)

				uploader := Uploader.New(conNum, chunckSize, boltDB, host, chQuit)
				uploader.UploadAll(filePath,nil)

				return nil
			},

		}
}