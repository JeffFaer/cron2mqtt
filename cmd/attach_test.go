package cmd

import (
	"testing"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/google/go-cmp/cmp"
)

func TestUpdateCommand(t *testing.T) {
	exe = "cron2mqtt"
	for _, tc := range []struct {
		name string
		cmd  string

		want string
	}{
		{
			name: "simple",
			cmd:  "echo true",

			want: "cron2mqtt exec id echo true",
		},
		{
			name: "special shell symbols",
			cmd:  `export foo=bar; echo "${foo}"`,

			want: `cron2mqtt exec id 'export foo=bar; echo "${foo}"'`,
		},
		{
			name: "weird whitespace",
			cmd:  "echo    foo  bar",

			want: "cron2mqtt exec id echo    foo  bar",
		},
		{
			name: "weird whitespace and special symbols",
			cmd:  "echo    foo  bar\t${baz}",

			want: "cron2mqtt exec id 'echo    foo  bar\t${baz}'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cron.NewCommand(tc.cmd)

			updateCommand("id", cmd)

			if diff := cmp.Diff(tc.want, cmd.String()); diff != "" {
				t.Errorf("updateCommand(%q) mismatch (-want +got):\n%s", tc.cmd, diff)
			}
		})
	}

}
