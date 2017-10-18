package main

import (
	"github.com/urfave/cli"
	"os"
	"./commands"
)

func main() {
	app := cli.NewApp()
	app.Name = "upload and download app"
	app.Description = "Description"
	app.Usage = "support multipart"
	app.Version = "0.1"

	app.Commands = []cli.Command{
		commands.Upload(),
		commands.Download(),
		commands.List(),
		commands.Clear(),
		commands.Delete(),
	}

	app.Run(os.Args)
}
