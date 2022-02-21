package logutil

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Timer makes it easy to measure how long a piece of code takes. You can use it in defer:
// defer StartTimer(zerolog.InfoLevel, "uploading").Stop()
type Timer struct {
	log   zerolog.Logger
	l     zerolog.Level
	msg   string
	start time.Time
}

func StartTimer(l zerolog.Level, msg string) *Timer {
	return start(log.Logger, l, msg)
}

func StartTimerLogger(log zerolog.Logger, l zerolog.Level, msg string) *Timer {
	return start(log, l, msg)
}

func start(log zerolog.Logger, l zerolog.Level, msg string) *Timer {
	now := zerolog.TimestampFunc()
	log.WithLevel(l).CallerSkipFrame(2).Msg(msg + " (1/2)")
	return &Timer{log, l, msg, now}
}

func (t *Timer) Stop() {
	t.log.WithLevel(t.l).CallerSkipFrame(1).TimeDiff("duration", zerolog.TimestampFunc(), t.start).Msg(t.msg + " (2/2)")
}

// FuncHook is an adapter that lets you use the same function for both zerolog.Event.Func and zerolog.Logger.Hook.
type FuncHook func(*zerolog.Event)

func (f FuncHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	f(e)
}

// FuncOnce makes it so that a FuncHook runs only the first time. This is mostly useful if you want to add some attributes to the StartTimerLogger call, but not the eventual Timer.Stop call.
func FuncOnce(f FuncHook) FuncHook {
	var once sync.Once
	return func(e *zerolog.Event) {
		once.Do(func() { f(e) })
	}
}
