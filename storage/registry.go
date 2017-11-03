package storage
import (
	"../def"
	"../config"
)

var Storages = map[string]Creator{}

type Creator func(config2 config.Config) def.Storager

func Add(name string, creator Creator) {
	Storages[name] = creator
}
