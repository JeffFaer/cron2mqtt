package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron-mqtt/exec"
	"github.com/JeffreyFalgout/cron-mqtt/mqtt"
	"github.com/JeffreyFalgout/cron-mqtt/mqtt/hass"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:  "exec",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, configErr := loadConfig()
			if configErr != nil {
				fmt.Fprintln(os.Stderr, configErr)
			}

			id := args[0]
			res := exec.Run(args[1], args[2:]...)

			if configErr == nil {
				if err := publish(id, c, res); err != nil {
					fmt.Fprintf(os.Stderr, "Ran into an issue publishing to MQTT: %s\n", err)
				}
			}

			if res.ExitCode != 0 {
				if res.Err != nil {
					fmt.Fprintln(os.Stderr, res.Err)
				}
				os.Exit(res.ExitCode)
			}
			return nil
		},
	})
}

func publish(id string, conf mqtt.Config, res exec.Result) error {
	c, err := mqtt.NewClient(conf)
	if err != nil {
		return fmt.Errorf("could not initialize MQTT: %w", err)
	}
	defer c.Close(250)

	cj, err := hass.NewCronJob(c, id, res.Cmd)
	if err != nil {
		return fmt.Errorf("could not create hass.CronJob: %w", err)
	}

	if err := cj.PublishResults(res); err != nil {
		return fmt.Errorf("problem publishing results to hass.CronJob: %w", err)
	}

	return nil
}
