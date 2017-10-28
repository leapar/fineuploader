package main

import (
	"os"
	"gopkg.in/urfave/cli.v1"
	"./commands"
	"gopkg.in/urfave/cli.v1/altsrc"
	"fmt"
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
		cli.StringFlag{
			Name: "config",
			Value: "config.yml",
			Usage: "Configuration file",
		},
	}

	registerCommand(app,flags,commands.Server())
	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))
	app.Run(os.Args)
}

func registerCommand(app *cli.App, flags []cli.Flag, cmd cli.Command) {

	cmd.Flags = append(flags, cmd.Flags...)
	cmd.Action = newCommandAction(cmd.Action.(func(c *cli.Context) error))
	cmd.Before = newConfigLoader("config", cmd.Flags)

	app.Commands = append(app.Commands, cmd)
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

		createInputSource := altsrc.NewYamlSourceFromFlagFunc(name)
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
		if sf, ok := f.(cli.StringFlag); ok && f.GetName() == name {
			flag = sf
			found = true
			break
		}
	}

	return flag, found
}