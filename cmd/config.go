package cmd

import (
	"fmt"
	"github.com/spf13/viper"

	"github.com/JeffreyFalgout/cron-mqtt/mqtt"
)

func init() {
	viper.SetConfigName("cron-mqtt")
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME/.config")
}

func loadConfig() (mqtt.Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return mqtt.Config{}, err
		}
		return mqtt.Config{}, fmt.Errorf("error reading config %s: %w", viper.ConfigFileUsed(), err)
	}

	var c mqtt.Config
	if err := viper.Unmarshal(&c); err != nil {
		return mqtt.Config{}, err
	}
	return c, nil
}
