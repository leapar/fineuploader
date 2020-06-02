package main

import (
	"fineuploader/commands"
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"os"
	"sort"
)

func main() {
	app := cli.NewApp()
	app.Name = "upload server"
	app.Description = "Description"
	app.Usage = "support multipart"
	app.Version = "0.1"

	/*app.Commands = []cli.Command{
		commands.Server(),
	}*/
	flags := []cli.Flag{
		&cli.StringFlag{
			Name: "config",
			Value: "config.toml",
			Usage: "Configuration file",
		},
	}

	registerCommand(app,flags,commands.Server())
	registerCommand(app,flags,commands.Gridfs2Hdfs())
	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))
	app.Run(os.Args)
}

func registerCommand(app *cli.App, flags []cli.Flag, cmd cli.Command) {

	cmd.Flags = append(flags, cmd.Flags...)
	cmd.Action = newCommandAction(cmd.Action)
	cmd.Before = newConfigLoader("config", cmd.Flags)

	app.Commands = append(app.Commands, &cmd)
}

func newConfigLoader(name string, flags []cli.Flag) cli.BeforeFunc {
	return func(c *cli.Context) error {

		configFlag, found := findFlag(name, flags)

		if !found {
			return fmt.Errorf("Unknown flag %v", name)
		}

		value := c.String(name)

		if !Exists(value) {
			if value == configFlag.Value {
				return nil
			}
			return fmt.Errorf("Configuration file %s not found", value)
		}

		//createInputSource := altsrc.NewYamlSourceFromFlagFunc(name)

		createInputSource := altsrc.NewTomlSourceFromFlagFunc(name)
		inputSource, err := createInputSource(c)

		if err != nil {
			return fmt.Errorf("Unable to create input source with context: inner error: \n'%v'", err.Error())
		}

		return altsrc.ApplyInputSourceValues(c, inputSource, flags)
	}
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func newCommandAction(action func(c *cli.Context) error) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		err := action(c)

		if err == nil {
			return nil
		}

		exitCoder, ok := err.(cli.ExitCoder)

		if ok {
			return exitCoder
		}

		return cli.NewExitError(err.Error(), 1)
	}
}


func findFlag(name string, flags []cli.Flag) (cli.StringFlag, bool) {
	var flag cli.StringFlag

	found := false

	for _, f := range flags {
		sf, ok := f.(*cli.StringFlag)
		if ok {
			for i := 0; i < len(f.Names()); i++ {
				if f.Names()[i] == name {
					flag = *sf
					found = true
					break
				}
			}
		}

		if found {
			break
		}
	}

	return flag, found
}