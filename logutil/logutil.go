package logutil

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Timer struct {
	log   zerolog.Logger
	l     zerolog.Level
	msg   string
	start time.Time
}

func StartTimer(l zerolog.Level, msg string) *Timer {
	return StartTimerLogger(log.Logger, l, msg)
}

func StartTimerLogger(log zerolog.Logger, l zerolog.Level, msg string) *Timer {
	now := zerolog.TimestampFunc()
	log.WithLevel(l).CallerSkipFrame(1).Msg(msg + " (1/2)")
	return &Timer{log, l, msg, now}
}

func (t *Timer) Stop() {
	t.log.WithLevel(t.l).CallerSkipFrame(1).TimeDiff("duration", zerolog.TimestampFunc(), t.start).Msg(t.msg + " (2/2)")
}
