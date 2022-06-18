package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/logutil"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/hass"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttcron"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "exec",
		Short: "Executes a command, and publishes its results to MQTT.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, canc := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer canc()
			id := args[0]
			args = args[1:]
			res := run(ctx, args)

			if len(res.Stderr) == 0 && res.Err != nil {
				fmt.Fprintln(os.Stderr, res.Err)
				res.Stderr = []byte(res.Err.Error())
			}

			if c, err := loadConfig(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else if err := publish(id, c, args, res); err != nil {
				fmt.Fprintf(os.Stderr, "Could not publish to MQTT: %s\n", err)
			}

			if res.ExitCode != 0 {
				os.Exit(res.ExitCode)
			}
			return nil
		},
	})
}

func run(ctx context.Context, args []string) exec.Result {
	defer logutil.StartTimer(zerolog.InfoLevel, "Executing command").Stop()
	sh := os.Getenv("SHELL")
	return exec.Run(ctx, sh, "-c", strings.Join(args, " "))
}

func publish(id string, conf mqtt.Config, args []string, res exec.Result) error {
	defer logutil.StartTimer(zerolog.InfoLevel, "Publishing to MQTT").Stop()
	c, err := mqtt.NewClient(conf)
	if err != nil {
		return fmt.Errorf("could not initialize MQTT: %w", err)
	}
	defer c.Close(250)

	// TODO: Make the plugins configurable.
	cj, err := mqttcron.NewCronJob(id, c, &hass.Plugin{})
	if err != nil {
		return fmt.Errorf("could not create mqttcron.CronJob: %w", err)
	}

	if err := cj.PublishResult(res); err != nil {
		return fmt.Errorf("could not publish result to mqttcron.CronJob: %w", err)
	}

	return nil
}
