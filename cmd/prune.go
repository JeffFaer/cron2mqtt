package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/logutil"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/hass"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttcron"
)

func init() {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Looks for cron jobs on MQTT that don't exist locally, then purges them from MQTT.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadConfig()
			if err != nil {
				return err
			}
			cl, err := mqtt.NewClient(c)
			if err != nil {
				return fmt.Errorf("could not initialize MQTT: %w", err)
			}
			defer cl.Close(250)
			ctx := context.Background()
			timeoutCtx, canc := context.WithTimeout(ctx, timeout)
			defer canc()
			remote, err := discoverRemoteCronJobs(timeoutCtx, cl)
			if err != nil {
				return err
			}
			if len(remote) == 0 {
				fmt.Println("Discovered 0 cron jobs from MQTT.")
				return nil
			}
			var remoteCronJobIDs []string
			remoteCronJobByID := make(map[string]*mqttcron.CronJob)
			for _, cj := range remote {
				remoteCronJobIDs = append(remoteCronJobIDs, cj.ID)
				remoteCronJobByID[cj.ID] = cj
			}
			sort.Strings(remoteCronJobIDs)

			fmt.Printf("Discovered %d cron jobs from MQTT:\n", len(remoteCronJobIDs))
			for _, id := range remoteCronJobIDs {
				fmt.Printf("  %s\n", id)
			}

			cts, err := crontabs()
			if err != nil {
				return err
			}
			for _, ct := range cts {
				fmt.Println()
				fmt.Printf("Checking %s...\n", ct.name())
				t, err := ct.Load()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not load %s: %s\n", ct.name(), err)
					continue
				}

				for _, j := range t.Jobs() {
					if !j.Command.IsCron2Mqtt() {
						continue
					}

					// Check to see if any of the cron job's arguments are one of the remote cron job IDs.
					args, ok := j.Command.Args()
					if !ok {
						continue
					}
					// The cron job's command will be at a minimum "cron2mqtt exec ID ...", so only start looking at the third element.
					// Technically we're looking at more arugments than necessary, but it seems unlikely we'd have a false positive.
					for _, arg := range args[2:] {
						if _, ok := remoteCronJobByID[arg]; ok {
							fmt.Println()
							fmt.Printf("  Discovered cron job %s locally:\n", arg)
							fmt.Printf("  $ %s\n", j.Command)
							delete(remoteCronJobByID, arg)
							break
						}
					}
				}
			}

			for _, id := range remoteCronJobIDs {
				cj, ok := remoteCronJobByID[id]
				if !ok {
					continue
				}

				fmt.Println()
				fmt.Printf("Would you like to delete %s? [yN] ", cj.ID)
				var sel string
				fmt.Scanln(&sel)
				if strings.ToLower(sel) != "y" {
					continue
				}

				t := logutil.StartTimerLogger(log.With().Str("id", id).Logger(), zerolog.InfoLevel, "Pruning")
				if err := cj.Unpublish(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Could not delete %s: %s\n", cj.ID, err)
				}
				t.Stop()
			}
			return nil
		},
	}
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 500*time.Millisecond, "The amount of time to spend discovering remote cron jobs.")
	rootCmd.AddCommand(cmd)
}

func discoverRemoteCronJobs(ctx context.Context, cl *mqtt.Client) ([]*mqttcron.CronJob, error) {
	defer logutil.StartTimer(zerolog.InfoLevel, "Discovering cron jobs").Stop()

	cjs := make(chan *mqttcron.CronJob, 100)
	if err := mqttcron.DiscoverCronJobs(ctx, cl, chan<- *mqttcron.CronJob(cjs), hass.NewPlugin); err != nil {
		return nil, err
	}

	var res []*mqttcron.CronJob
	for cj := range cjs {
		res = append(res, cj)
	}

	return res, nil
}
