package input
import (
	"../def"
	"../config"
)

var Iutputs = map[string]Creator{}

type Creator func(config2 config.Config) def.Iutputer

func Add(name string, creator Creator) {
	Iutputs[name] = creator
}
