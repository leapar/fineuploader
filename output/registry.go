package output
import (
	"fineuploader/def"
	"fineuploader/config"
)

var Outputs = map[string]Creator{}

type Creator func(config2 config.Config) def.Outputer

func Add(name string, creator Creator) {
	Outputs[name] = creator
}
