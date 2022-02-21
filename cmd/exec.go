package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/logutil"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/hass"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "exec",
		Short: "Executes a command, and publishes its results to MQTT.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			args = args[1:]
			res := run(args)

			if res.Err != nil {
				fmt.Fprintln(os.Stderr, res.Err)
				if len(res.Stderr) == 0 {
					res.Stderr = []byte(res.Err.Error())
				}
			}

			if c, err := loadConfig(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				if err := publish(id, c, args, res); err != nil {
					fmt.Fprintf(os.Stderr, "Could not publish to MQTT: %s\n", err)
				}
			}

			if res.ExitCode != 0 {
				os.Exit(res.ExitCode)
			}
			return nil
		},
	})
}

func run(args []string) exec.Result {
	defer logutil.StartTimer(zerolog.InfoLevel, "Executing command").Stop()
	sh := os.Getenv("SHELL")
	return exec.Run(sh, "-c", strings.Join(args, " "))
}

func publish(id string, conf mqtt.Config, args []string, res exec.Result) error {
	defer logutil.StartTimer(zerolog.InfoLevel, "publishing to MQTT").Stop()
	c, err := mqtt.NewClient(conf)
	if err != nil {
		return fmt.Errorf("could not initialize MQTT: %w", err)
	}
	defer c.Close(250)

	cj, err := hass.NewCronJob(c, id, args[0])
	if err != nil {
		return fmt.Errorf("could not create hass.CronJob: %w", err)
	}

	if err := cj.PublishResults(c, res); err != nil {
		return fmt.Errorf("problem publishing results to hass.CronJob: %w", err)
	}

	return nil
}
