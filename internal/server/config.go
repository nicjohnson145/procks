package server

import (
	"strings"

	"github.com/spf13/viper"
)

const (
	// Port is the port the service will listen on
	Port = "port"
	// Url is the URL for the public ingress of the service, this will be used to construct the connection display
	// messages for attached clients
	Url = "url"

	// LogLevel controls the verbosity of logging
	LogLevel = "log.level"
	// LogFormat controls the format of log messages
	LogFormat = "log.format"
)

const (
	DefaultPort = "8080"
)

func InitConfig() {
	viper.SetDefault(Port, DefaultPort)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
