package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
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
			u, err := user.Current()
			if err != nil {
				return fmt.Errorf("could not determine current user: %w", err)
			}

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

			fmt.Println()
			fmt.Println("Checking local crontabs...")
			cts := cron.TabsForUser(u)
			for _, ct := range cts {
				fmt.Printf("  %s\n", ct)
				local, err := mqttcron.DiscoverLocalCronJobsByID([]cron.Tab{ct}, u, remoteCronJobIDs)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not check cron jobs: %s\n", err)
					continue
				}

				for _, id := range remoteCronJobIDs {
					if j, ok := local[id]; ok {
						fmt.Println()
						fmt.Printf("  Discovered cron job %s locally\n", id)
						fmt.Printf("  $ %s\n", j.Command)

						if _, ok := remoteCronJobByID[id]; !ok {
							fmt.Fprintf(os.Stderr, "Discovered cron job %s more than once!\n", id)
						}
						delete(remoteCronJobByID, id)
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
	defer logutil.StartTimer(zerolog.InfoLevel, "Discovering remote cron jobs").Stop()
	return mqttcron.DiscoverRemoteCronJobs(ctx, cl, hass.NewPlugin)
}
