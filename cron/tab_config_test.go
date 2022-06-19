package cron

import (
	"os/user"
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

func TestTabConfigRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		u    *user.User
		s    string

		wantUser string
	}{
		{
			name: "empty string",
			u:    currentUserOrDie(),
			s:    "",
		},
		{
			name: "blank line",
			u:    currentUserOrDie(),
			s:    "\n",
		},
		{
			name: "blank lines",
			u:    currentUserOrDie(),
			s:    "\n\n\n\n",
		},
		{
			name: "comment",
			u:    currentUserOrDie(),
			s:    "# comment",
		},
		{
			name: "comment with newlines",
			u:    currentUserOrDie(),
			s:    "\n# comment\n",
		},
		{
			name: "comment with leading whitespace",
			u:    currentUserOrDie(),
			s:    "\n  # comment\n",
		},
		{
			name: "crontab",
			u:    currentUserOrDie(),
			s: `
* * * * * echo foo
  * * * * * echo bar
`,
		},
		{
			name: "crontab and comments",
			u:    currentUserOrDie(),
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
			u:    currentUserOrDie(),
			s: `
* * * * * echo "foo bar"
`,
		},
		{
			name: "crontab with users",
			u:    nil,
			s: `
* * * * * root echo foo
`,
			wantUser: "root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tab, err := parseTabConfig(tc.s, tc.u); err != nil {
				t.Errorf("parse(tc.s) generated an error: %s", err)
			} else if diff := cmp.Diff(strings.Split(tc.s, "\n"), strings.Split(tab.String(), "\n")); diff != "" {
				t.Errorf("parse(tc.s) does not roundtrip (-want +got):\n%s", diff)
			} else {
				for i, j := range tab.Jobs() {
					u := tc.u
					if u == nil {
						var err error
						u, err = user.Lookup(tc.wantUser)
						if err != nil {
							t.Errorf("Could not lookup wantUser %q: %s", tc.wantUser, err)
						}
					}

					if u != nil && j.User.Uid != u.Uid {
						t.Errorf("tab job #%d has user %q, expected %q", i, j.User.Username, u.Username)
					}
				}
			}
		})
	}
}

func currentUserOrDie() *user.User {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	return u
}

func TestParseTabConfig(t *testing.T) {
	cmd1 := "* * * * * foo echo 1 2 3"
	cmd2 := "* * * * * bar echo 4 5 6"
	tab, err := parseTabConfig(cmd1+"\n"+cmd2, nil)
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

func TestTransform(t *testing.T) {
	cmd1 := "* * * * * foo echo 1 2 3"
	cmd2 := "* * * * * bar echo 4 5 6"
	tab, err := parseTabConfig(cmd1+"\n"+cmd2, nil)
	if err != nil {
		t.Fatalf("Unexpected error genreating cron.Tab: %s", err)
	}

	tab.Jobs()[1].Command.Transform(func(cmd string) string { return "cron2mqtt exec abcd " + cmd })

	var jobs []string
	for _, j := range tab.Jobs() {
		jobs = append(jobs, j.String())
	}

	if diff := cmp.Diff([]string{cmd1, "* * * * * bar cron2mqtt exec abcd echo 4 5 6"}, jobs); diff != "" {
		t.Errorf("jobs diff (-want +got):\n%s", diff)
	}
}
