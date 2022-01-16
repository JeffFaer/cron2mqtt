package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/hass"
)

var (
	exe string

	dryRun bool
)

func init() {
	var err error
	exe, err = os.Executable()
	if err != nil {
		panic(fmt.Errorf("couldn't determine path of current executable: %w", err))
	}

	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attaches monitoring to existing cron jobs.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			pcts, err := possibleCrontabs()
			if err != nil {
				return err
			}

			var updates []func()
			for _, pct := range pcts {
				fmt.Printf("Checking %s\n", pct.name())
				t, err := pct.load()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not load %s: %s\n", pct.name(), err)
					continue
				}

				if attachTo(t) {
					pct := pct
					updates = append(updates, func() {
						fmt.Println()
						fmt.Printf("Updating %s...\n", pct.name())
						if dryRun {
							fmt.Print(t)
						} else {
							if err := pct.update(t); err != nil {
								fmt.Fprintf(os.Stderr, "Could not update %s: %s\n", pct.name(), err)
							}
						}
					})
				}
			}

			for _, u := range updates {
				u()
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry_run", false, "Print the updated crontabs instead of actually updating them.")
	rootCmd.AddCommand(cmd)
}

func attachTo(t *cron.Tab) (updated bool) {
	for i, j := range t.Jobs() {
		if j.Command.IsCron2Mqtt() {
			fmt.Printf("  Skipping job #%d: It already appears to be monitored.\n", i+1)
			continue
		}

		fmt.Println()
		fmt.Println()
		fmt.Printf("  $ %s\n", j.Command.String())
		fmt.Println()
		fmt.Printf("  Do you want to attach monitoring to this cron job? [yN] ")
		var sel string
		fmt.Scanln(&sel)
		if strings.ToLower(sel) != "y" {
			continue
		}

		id := promptID()

		j.Command.Prefix(fmt.Sprintf("%s exec %s", exe, id))
		// TODO: Also update MQTT with the configuration of this cron job, even though it hasn't run yet.
		updated = true
	}

	return
}

func promptID() string {
	id := randomID(8)
	for {
		fmt.Printf("  Enter a job ID [default: %s]: ", id)
		var sel string
		fmt.Scanln(&sel)
		if strings.TrimSpace(sel) != "" {
			// TODO: Do we need to generalize this validation logic? We might want to support other MQTT destinations than hass.
			if err := hass.ValidateTopicComponent(sel); err != nil {
				fmt.Fprintf(os.Stderr, "  ID %q is invalid: %s\n\n", sel, err)
				continue
			}
			id = sel
		}

		return id
	}
}

func randomID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base58.Encode(b)
}
