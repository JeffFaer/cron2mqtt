package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttcron"
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
			cts, err := crontabs()
			if err != nil {
				return err
			}

			var updates []func()
			for _, ct := range cts {
				fmt.Printf("Checking %s\n", ct.name())
				tc, err := ct.Load()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not load %s: %s\n", ct.name(), err)
					continue
				}

				if attachTo(tc) {
					ct := ct
					updates = append(updates, func() {
						fmt.Println()
						fmt.Printf("Updating %s...\n", ct.name())
						if dryRun {
							fmt.Print(ct)
						} else {
							if err := ct.Update(tc); err != nil {
								fmt.Fprintf(os.Stderr, "Could not update %s: %s\n", ct.name(), err)
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

func attachTo(c *cron.TabConfig) (updated bool) {
	for i, j := range c.Jobs() {
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

		updateCommand(id, j.Command)
		// TODO: Also update MQTT with the configuration of this cron job, even though it hasn't run yet.
		updated = true
	}

	return
}

func updateCommand(id string, cmd *cron.Command) {
	pre := fmt.Sprintf("%s exec %s", exe, id)

	// Do we need to quote the entire pre-existing command?
	args, ok := cmd.Args()
	quote := !ok
	for _, arg := range args {
		if shellquote.Join(arg) != arg {
			quote = true
			break
		}
	}

	cmd.Transform(func(cmd string) string {
		if quote {
			cmd = shellquote.Join(cmd)
		}
		return pre + " " + cmd
	})
}

func promptID() string {
	id := randomID(8)
	for {
		fmt.Printf("  Enter a job ID [default: %s]: ", id)
		var sel string
		fmt.Scanln(&sel)
		if strings.TrimSpace(sel) != "" {
			// TODO: Do we need to generalize this validation logic? We might want to support other MQTT destinations than hass.
			if err := mqttcron.ValidateTopicComponent(sel); err != nil {
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
