package commands

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"encoding/json"
	Uploader "../uploader"
)

func Clear()  cli.Command {

	return cli.Command{
		Name:    "clear",
		Aliases: []string{"c"},
		Usage:   "clear all upload history",
		Action: func(c *cli.Context) error {
			boltDB, err := bolt.Open("uploader.db", 0600, nil)
			if err != nil {
				log.Fatal(err)
				return nil
			}

			defer boltDB.Close()

			boltDB.Update(func(tx *bolt.Tx) error {
				bk := tx.Bucket([]byte("upload"))
				if bk == nil {
					return  nil
				}
				if err := bk.ForEach(func(k, v []byte) error {
					//fmt.Printf("A %s is %s.\n", k, v)
					bolt := Uploader.BoltUploadStruct{}
					json.Unmarshal(v,&bolt)
					fmt.Printf("%s %s: %t.\t %s-%s  \n",bolt.CheckSum, bolt.FilePath, bolt.IsOver, bolt.StartTime, bolt.OverTime)
					bk.Delete([]byte(k))
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