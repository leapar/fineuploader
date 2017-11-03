package config

import "../def"


type Config struct {
	Host string
	Port int
	Output string
	OutputMongo MongoConfig
	OutputNsq NsqConfig
	InputNsq NsqConfig
	StorageName string
	Storage def.Storager
}

type MongoConfig struct {
	MongoServer string
}

type NsqConfig struct {
	NsqServer string
	Enable bool
	Topic string
	Channel string
}