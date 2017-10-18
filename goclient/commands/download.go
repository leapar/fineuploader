package commands

import (
	"github.com/urfave/cli"
	"fmt"
)

func Download()  cli.Command {
	return cli.Command{
		Name:    "download",
		Aliases: []string{"d"},
		Usage:   "download a file",
		Action: func(c *cli.Context) error {
			fmt.Println("completed task: ", c.Args().First())
			return nil
		},
	}
}