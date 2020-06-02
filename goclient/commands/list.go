package commands

import (
	"github.com/urfave/cli/v2"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"encoding/json"
	Uploader "../uploader"
)

func List()  cli.Command {
	return cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "list all upload history",
		Action: func(c *cli.Context) error {
			boltDB, err := bolt.Open("uploader.db", 0600, nil)
			if err != nil {
				log.Fatal(err)
				return nil
			}

			defer boltDB.Close()
			boltDB.View(func(tx *bolt.Tx) error {
				bk := tx.Bucket([]byte("upload"))
				if bk == nil {
					return  nil
				}
				if err := bk.ForEach(func(k, v []byte) error {
					//fmt.Printf("A %s is %s.\n", k, v)
					bolt := Uploader.BoltUploadStruct{}
					json.Unmarshal(v,&bolt)
					fmt.Printf("%s %s: %t.\t %s-%s  \n",bolt.CheckSum, bolt.FilePath, bolt.IsOver, bolt.StartTime, bolt.OverTime)
					return nil
				}); err != nil {
					return err
				}
				return nil
			})
			return nil
		},
	}
}