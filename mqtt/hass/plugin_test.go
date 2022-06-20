package hass

import (
	"testing"
	"time"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
)

func TestCommandName(t *testing.T) {
	for _, tc := range []struct {
		name string

		id  string
		cmd string

		want string
	}{
		{
			name: "simple",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd echo true",

			want: "echo true",
		},
		{
			name: "simple with flags",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd -vvv echo true",

			want: "echo true",
		},
		{
			name: "ambiguous flag",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd echo true -vvv",

			want: "echo true",
		},
		{
			name: "dashdash",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd -- echo true",

			want: "echo true",
		},
		{
			name: "dashdash with flags",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd -vvv -- echo -n true",

			want: "echo -n true",
		},
		{
			name: "quoted command",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd 'echo true'",

			want: "echo true",
		},
		{
			name: "quoted command with flags",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd -vvv 'echo -n true'",

			want: "echo -n true",
		},
		{
			name: "unsplittable",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd 'echo true",

			want: "cron2mqtt exec abcd 'echo true",
		},
		{
			name: "missing command",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd",

			want: "cron2mqtt exec abcd",
		},
		{
			name: "dashdash missing command",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd --",

			want: "cron2mqtt exec abcd --",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cron.NewCommand(tc.cmd)
			if got := commandName(tc.id, cmd); got != tc.want {
				t.Errorf("commandName(%q, %q) = %q, want %q", tc.id, tc.cmd, got, tc.want)
			}
		})
	}
}

func TestExpireAfter(t *testing.T) {
	topOfTheHour, err := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Could not parse testing time: %s", err)
	}

	for _, tc := range []struct {
		name  string
		now   time.Time
		sched string
		want  time.Duration
	}{
		{
			name:  "simple",
			now:   topOfTheHour,
			sched: "0 * * * * ",
			want:  time.Hour + 60*time.Second,
		},
		{
			name:  "cron job more frequently than default delay",
			now:   topOfTheHour,
			sched: "* * * * * ",
			want:  time.Minute + 30*time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := cron.NewSchedule(tc.sched)
			if err != nil {
				t.Fatalf("cron.NewSchedule(%q) yielded an unexpected error: %s", tc.sched, err)
			}
			now = func() time.Time { return tc.now }
			if got := expireAfter(&s); got != tc.want {
				t.Errorf("expireAfter(%q) = %s, want %s", tc.sched, got, tc.want)
			}
		})
	}
}
