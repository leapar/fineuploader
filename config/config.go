package config

import "../def"


type Config struct {
	Host string
	Port int
	Output string
	OutputMongo MongoConfig
	OutputNsq NsqConfig
	OutputWebHdfs WebHdfsConfig
	OutputHdfs HdfsConfig
	InputNsq NsqConfig
	StorageName string
	Storage def.Storager

	InputMongo MongoConfig
}

type MongoConfig struct {
	MongoServer string
	FileMd5 string
}

type NsqConfig struct {
	NsqServer string
	Enable bool
	Topic string
	Channel string
}

type WebHdfsConfig struct {
	Url string
}

type HdfsConfig struct {
	Url string
	User string
}

