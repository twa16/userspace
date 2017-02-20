package userspaced

import (
	"github.com/twa16/go-cas/client"
	"github.com/spf13/viper"
)

var casServer gocas.CASServerConfig

func initCAS() {
	casServer.ServerHostname = viper.GetString("CASURL")
	casServer.IgnoreSSLErrors = false
}
