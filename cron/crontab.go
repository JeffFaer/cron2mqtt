package cron

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/shlex"
	cron "github.com/robfig/cron/v3"
)

func Load(u *user.User) (Tab, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("crontab", "-u", u.Username, "-l")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return Tab{}, fmt.Errorf("could not determine current user's crontab: %w", err)
	}

	return parse(string(stdout.Bytes()), false)
}

func Update(u *user.User, t Tab) error {
	cmd := exec.Command("crontab", "-u", u.Username, "-")
	cmd.Stdin = strings.NewReader(t.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not update current user's crontab: %w", err)
	}
	return nil
}

type Tab struct {
	entries []entry
	jobs    []*Job
}

func (t Tab) Jobs() []*Job {
	return t.jobs
}

func (t Tab) String() string {
	var s []string
	for _, e := range t.entries {
		s = append(s, e.String())
	}
	return strings.Join(s, "\n")
}

type entry interface {
	String() string
	isEntry()
}

type comment string

func (c comment) String() string {
	return string(c)
}
func (comment) isEntry() {}

type Job struct {
	schedule schedule
	sep1     string
	user     *string
	sep2     string
	Command  *Command
}

func (j *Job) String() string {
	var b strings.Builder
	b.WriteString(j.schedule.orig)
	b.WriteString(j.sep1)
	if j.user != nil {
		b.WriteString(*j.user)
		b.WriteString(j.sep2)
	}
	b.WriteString(j.Command.String())
	return b.String()
}
func (*Job) isEntry() {}

type schedule struct {
	schedule cron.Schedule
	orig     string
}

type Command struct {
	args []string
	orig string
}

// Prefix appends s as a prefix to this Command.
func (c *Command) Prefix(s string) error {
	args, err := shlex.Split(s)
	if err != nil {
		return fmt.Errorf("prefix %q is invalid: %w", s, err)
	}
	c.args = append(args, c.args...)
	c.orig = s + " " + c.orig
	return nil
}

// IsCron2Mqtt checks whether this command appears to execute cron2mqtt.
func (c *Command) IsCron2Mqtt() bool {
	return path.Base(c.args[0]) == "cron2mqtt"
}

func (c *Command) String() string {
	return c.orig
}

func parse(crontab string, includesUser bool) (Tab, error) {
	var t Tab
	ls := strings.Split(crontab, "\n")
	hasComments := false
	var comments strings.Builder
	for _, l := range ls {
		if isComment(l) || strings.TrimSpace(l) == "" {
			if hasComments {
				comments.WriteString("\n")
			}
			hasComments = true
			comments.WriteString(l)
			continue
		}

		if hasComments {
			t.entries = append(t.entries, comment(comments.String()))
			hasComments = false
			comments.Reset()
		}

		// schedule = 5, cmd = 1
		n := 5 + 1
		if includesUser {
			n++
		}
		seps, fs := fieldsN(l, n)

		if len(fs) != n {
			return Tab{}, fmt.Errorf("crontab has a badly formed line: %q", l)
		}

		var s strings.Builder
		i := 0
		for ; i < 5; i++ {
			s.WriteString(seps[i])
			s.WriteString(fs[i])
		}
		sched, err := cron.ParseStandard(s.String())
		if err != nil {
			return Tab{}, fmt.Errorf("crontab has a malformed schedule %q: %w", s.String(), err)
		}

		var j Job
		j.schedule = schedule{
			schedule: sched,
			orig:     s.String(),
		}
		j.sep1 = seps[i]
		if includesUser {
			j.user = &fs[i]
			i++
			j.sep2 = seps[i]
		}
		args, err := shlex.Split(fs[i])
		if err != nil {
			return Tab{}, fmt.Errorf("crontab has a malformded command %q: %w", fs[i], err)
		}
		j.Command = &Command{
			args: args,
			orig: fs[i],
		}

		t.entries = append(t.entries, &j)
		t.jobs = append(t.jobs, &j)
	}

	if hasComments {
		t.entries = append(t.entries, comment(comments.String()))
	}

	return t, nil
}

func isComment(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "#")
}

// fieldsN splits s into n fields separated by consecutive whitespace characters.
// s = seps[0] + fields[0] + seps[1] + fields[1] + ... + fields[n] + seps[n+1]
// fields is limited to n entries. That means the first n-1 entries are guaranteed
// to not contain any whitespace characters, but the last entry might contain whitespace.
// e.g.
// s = " a b c d ", n = 3
// seps = [" ", " ", " ", " "]
// fields = ["a", "b", "c d"]
func fieldsN(s string, n int) (seps []string, fields []string) {
	i := 0
	wasSpace := true
	for j, r := range s {
		isSpace := unicode.IsSpace(r)

		if wasSpace == isSpace {
			continue
		}

		if wasSpace {
			seps = append(seps, s[i:j])
		} else {
			fields = append(fields, s[i:j])
		}
		wasSpace = isSpace
		i = j
		if !isSpace && len(fields) == n-1 {
			break
		}
	}
	if wasSpace {
		seps = append(seps, s[i:])
	} else {
		s := s[i:]
		j := len(s)
		for j >= 0 {
			r, n := utf8.DecodeLastRuneInString(s[:j])
			if unicode.IsSpace(r) {
				j -= n
			} else {
				break
			}
		}
		fields = append(fields, s[:j])
		seps = append(seps, s[j:])
	}

	return
}
