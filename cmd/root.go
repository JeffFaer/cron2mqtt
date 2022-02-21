package cmd

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "cron2mqtt",
	}
	verbosity int

	logLevels = map[int]zerolog.Level{
		0: zerolog.WarnLevel,
		1: zerolog.InfoLevel,
		2: zerolog.DebugLevel,
		3: zerolog.TraceLevel,
	}
)

func init() {
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "Log verbosely.")
	rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
		l, ok := logLevels[verbosity]
		if !ok {
			i := -1
			for j, ll := range logLevels {
				if i < j && j < verbosity {
					i = j
					l = ll
				}
			}
		}
		zerolog.SetGlobalLevel(l)

		out := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = os.Stderr
			w.TimeFormat = time.RFC3339
		})
		log.Logger = zerolog.New(out).With().Timestamp().Logger().Level(l)
		zerolog.DefaultContextLogger = &log.Logger
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
