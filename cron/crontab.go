package cron

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"strings"
	"unicode"
	"unicode/utf8"

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
	cmd      string
}

func (j *Job) Command() string {
	return j.cmd
}

func (j *Job) PrefixCommand(s string) {
	j.cmd = s + " " + j.cmd
}

func (j *Job) String() string {
	var b strings.Builder
	b.WriteString(j.schedule.orig)
	b.WriteString(j.sep1)
	if j.user != nil {
		b.WriteString(*j.user)
		b.WriteString(j.sep2)
	}
	b.WriteString(j.cmd)
	return b.String()
}
func (*Job) isEntry() {}

type schedule struct {
	schedule cron.Schedule
	orig     string
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

		var sched strings.Builder
		i := 0
		for ; i < 5; i++ {
			sched.WriteString(seps[i])
			sched.WriteString(fs[i])
		}

		schedu, err := cron.ParseStandard(sched.String())
		if err != nil {
			return Tab{}, fmt.Errorf("crontab has a malformed schedule %q: %w", sched.String(), err)
		}

		var j Job
		j.schedule = schedule{
			schedule: schedu,
			orig:     sched.String(),
		}
		j.sep1 = seps[i]
		if includesUser {
			j.user = &fs[i]
			i++
			j.sep2 = seps[i]
		}
		j.cmd = fs[i]

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

// fieldsN splits s into n fields separates by consecutive whitespace characters.
// s = seps[0] + fields[0] + seps[1] + fields[1] + ... + fields[n] + seps[n+1]
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
