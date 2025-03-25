package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// ServerUrl is the base url of the procks server to connect to, ex: https://procks.example.com
	ServerUrl = "server.url"

	// LogLevel controls the verbosity of logging
	LogLevel = "log.level"
	// LogFormat controls the format of log messages
	LogFormat = "log.format"
)

func InitConfig() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("error getting user config directory: %w", err)
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AddConfigPath(filepath.Join(configDir, "procks"))
	viper.SetConfigType("yaml")
	viper.SetConfigName("client")

	if err := viper.ReadInConfig(); err != nil {
		// the config being missing is ok
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("error reading config file: %s", err)
	}

	return nil
}
