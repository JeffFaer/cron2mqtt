package hass

import (
	"testing"

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
			name: "dashdash",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd -- echo true",

			want: "echo true",
		},
		{
			name: "quoted command",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd 'echo true'",

			want: "echo true",
		},
		{
			name: "unsplittable",

			id:  "abcd",
			cmd: "cron2mqtt exec abcd 'echo true",

			want: "cron2mqtt exec abcd 'echo true",
		},
		{
			name: "ambiguous flag",

			id: "abcd",
			// The -vvv flag technically applies to cron2mqtt.
			cmd: "cron2mqtt exec abcd echo true -vvv",

			want: "echo true -vvv",
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
