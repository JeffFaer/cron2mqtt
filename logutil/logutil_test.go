package logutil

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestCallerStackFrames(t *testing.T) {
	var out bytes.Buffer
	log.Logger = zerolog.New(&out).With().Caller().Logger()
	oldFunc := zerolog.CallerMarshalFunc
	var file string
	zerolog.CallerMarshalFunc = func(f string, line int) string {
		file = f
		return oldFunc(f, line)
	}

	for _, tc := range []struct {
		name  string
		start func()
	}{
		{
			name:  "StartTimer",
			start: func() { StartTimer(zerolog.InfoLevel, "foo") },
		},
		{
			name:  "StartTimerLogger",
			start: func() { StartTimerLogger(log.Logger, zerolog.InfoLevel, "foo") },
		},
		{
			name:  "Stop",
			start: func() { StartTimer(zerolog.InfoLevel, "foo").Stop() },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.start()

			_, self, _, ok := runtime.Caller(0)
			if !ok {
				t.Fatalf("Could not determine file of this test.")
			}
			if self != file {
				t.Errorf("StartTimer's Caller is %s, want %s", file, self)
			}
		})
	}
}
