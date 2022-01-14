package cron

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFieldsN(t *testing.T) {
	for _, tc := range []struct {
		name string

		in string
		n  int

		wantSeps   []string
		wantFields []string
	}{
		{
			name: "single field",

			in: "abcd",
			n:  1,

			wantSeps:   []string{"", ""},
			wantFields: []string{"abcd"},
		},
		{
			name: "leading whitespace",

			in: "  abcd",
			n:  1,

			wantSeps:   []string{"  ", ""},
			wantFields: []string{"abcd"},
		},
		{
			name: "trailing whitespace",

			in: "abcd  ",
			n:  1,

			wantSeps:   []string{"", "  "},
			wantFields: []string{"abcd"},
		},
		{
			name: "multiple fields,n=3",

			in: " a  b   cd    ",
			n:  3,

			wantSeps:   []string{" ", "  ", "   ", "    "},
			wantFields: []string{"a", "b", "cd"},
		},
		{
			name: "multiple fields,n=2",

			in: " a  b   cd    ",
			n:  2,

			wantSeps:   []string{" ", "  ", "    "},
			wantFields: []string{"a", "b   cd"},
		},
		{
			name: "multiple fields,n=1",

			in: " a  b   cd    ",
			n:  1,

			wantSeps:   []string{" ", "    "},
			wantFields: []string{"a  b   cd"},
		},
		{
			name: "multiple fields,n=4",

			in: " a  b   cd    ",
			n:  4,

			wantSeps:   []string{" ", "  ", "   ", "    "},
			wantFields: []string{"a", "b", "cd"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotSeps, gotFields := fieldsN(tc.in, tc.n)

			var out strings.Builder
			for i, f := range gotFields {
				out.WriteString(gotSeps[i])
				out.WriteString(f)
			}
			out.WriteString(gotSeps[len(gotSeps)-1])

			if out.String() != tc.in {
				t.Errorf("recombining fieldsN output = %q, want %q", out.String(), tc.in)
			}
			if diff := cmp.Diff(tc.wantSeps, gotSeps); diff != "" {
				t.Errorf("seps diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantFields, gotFields); diff != "" {
				t.Errorf("fields diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCrontabRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name          string
		includesUsers bool
		s             string
	}{
		{
			name: "empty string",
			s:    "",
		},
		{
			name: "blank line",
			s:    "\n",
		},
		{
			name: "blank lines",
			s:    "\n\n\n\n",
		},
		{
			name: "comment",
			s:    "# comment",
		},
		{
			name: "comment with newlines",
			s:    "\n# comment\n",
		},
		{
			name: "comment with leading whitespace",
			s:    "\n  # comment\n",
		},
		{
			name: "crontab",
			s: `
* * * * * echo foo
  * * * * * echo bar
`,
		},
		{
			name: "crontab and comments",
			s: `
# comment
* * * * * echo foo
# comment 2
 *  *   *     *      *       echo        bar
# comment 3
`,
		},
		{
			name: "complicated command",
			s: `
* * * * * echo "foo bar"
`,
		},
		{
			name:          "crontab with users",
			includesUsers: true,
			s: `
* * * * * root echo foo
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tab, err := parse(tc.s, tc.includesUsers); err != nil {
				t.Errorf("parse(tc.s) generated an error: %s", err)
			} else if diff := cmp.Diff(strings.Split(tc.s, "\n"), strings.Split(tab.String(), "\n")); diff != "" {
				t.Errorf("parse(tc.s) does not roundtrip (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse(t *testing.T) {
	cmd1 := "* * * * * foo echo 1 2 3"
	cmd2 := "* * * * * bar echo 4 5 6"
	tab, err := parse(cmd1+"\n"+cmd2, true)
	if err != nil {
		t.Fatalf("Unexpected error genreating cron.Tab: %s", err)
	}

	var jobs []string
	for _, j := range tab.Jobs() {
		jobs = append(jobs, j.String())
	}

	if diff := cmp.Diff([]string{cmd1, cmd2}, jobs); diff != "" {
		t.Errorf("jobs diff (-want +got):\n%s", diff)
	}
}

func TestPrefix(t *testing.T) {
	cmd1 := "* * * * * foo echo 1 2 3"
	cmd2 := "* * * * * bar echo 4 5 6"
	tab, err := parse(cmd1+"\n"+cmd2, true)
	if err != nil {
		t.Fatalf("Unexpected error genreating cron.Tab: %s", err)
	}

	if err := tab.Jobs()[1].PrefixCommand("cron2mqtt exec abcd"); err != nil {
		t.Errorf("PrefixCommand(cron2mqtt exec abcd) has an error: %s", err)
	}
	var jobs []string
	for _, j := range tab.Jobs() {
		jobs = append(jobs, j.String())
	}

	if diff := cmp.Diff([]string{cmd1, "* * * * * bar cron2mqtt exec abcd echo 4 5 6"}, jobs); diff != "" {
		t.Errorf("jobs diff (-want +got):\n%s", diff)
	}
}
