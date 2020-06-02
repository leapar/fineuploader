package commands

import (
	"github.com/urfave/cli/v2"
	"github.com/boltdb/bolt"
	"log"
	Deleter "../delete"
	"net/http"
)

var uuid string

func Delete()  cli.Command {

	return cli.Command{
		Name:    "delete",
		Aliases: []string{"r"},
		Usage:   "delete the remote file",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:        "host,u",
				Value:       "172.29.231.80:8082",
				Usage:       "the host Path",
				Destination: &host,
			},
			cli.StringFlag{
				Name:        "uuid",
				Value:       "",
				Usage:       "the file checksum",
				Destination: &uuid,
			},
		},

		Action: func(c *cli.Context) error {
			boltDB, err := bolt.Open("uploader.db", 0600, nil)
			if err != nil {
				log.Fatal(err)
				return nil
			}

			defer boltDB.Close()

			deleter := Deleter.New(host)
			if deleter.Delete(uuid) == http.StatusOK{
				log.Printf("delete %s ok \n",uuid)
				boltDB.Update(func(tx *bolt.Tx) error {
					bk := tx.Bucket([]byte("upload"))
					if bk == nil {
						return  nil
					}
					bk.Delete([]byte(uuid))
					return nil
				})
			} else {
				log.Printf("delete %s error \n",uuid)

			}

			return nil
		},
	}
}