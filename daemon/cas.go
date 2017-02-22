package userspaced

import (
	"github.com/spf13/viper"
	"github.com/twa16/go-cas/client"
)

var casServer gocas.CASServerConfig

func initCAS() {
	casServer.ServerHostname = viper.GetString("CASURL")
	casServer.IgnoreSSLErrors = false
}
