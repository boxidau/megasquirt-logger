package msdecoder

import "github.com/go-ini/ini"

func New(configFile string) {
	ini.Load(configFile)
}
