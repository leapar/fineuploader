package main

import (
	"os"
	"github.com/urfave/cli"
	"./commands"
)

func main() {
	app := cli.NewApp()
	app.Name = "upload server"
	app.Description = "Description"
	app.Usage = "support multipart"
	app.Version = "0.1"

	app.Commands = []cli.Command{
		commands.Server(),
	}

	app.Run(os.Args)
}

