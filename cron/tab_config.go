package cron

import (
	"fmt"
	"os/user"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kballard/go-shellquote"
	cron "github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

var (
	numScheduleFields = 5 // The number of line fields that constitute a cron schedule.
	numUserFields     = 1
	numCommandFields  = 1
)

// TabConfig is the actual content of a Tab.
// We take pains so that updating the Jobs in the TabConfig won't overwrite any comments or change any whitespace formatting that might exist.
type TabConfig struct {
	entries []entry
	jobs    []*Job
}

func parseTabConfig(crontab string, u *user.User) (*TabConfig, error) {
	var tc TabConfig
	ls := strings.Split(crontab, "\n")
	hasComments := false // Whether we've attempted to write anything to comments. Use this instead of len(comments) to avoid collapsing multiple empty lines together.
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
			tc.entries = append(tc.entries, comment(comments.String()))
			hasComments = false
			comments.Reset()
		}

		n := numScheduleFields + numCommandFields
		if u == nil {
			n += numUserFields
		}
		seps, fs := fieldsN(l, n)

		if len(fs) != n {
			return nil, fmt.Errorf("crontab has a malformed line: %q", l)
		}

		var s strings.Builder
		i := 0
		for ; i < numScheduleFields; i++ {
			s.WriteString(seps[i])
			s.WriteString(fs[i])
		}
		sched, err := NewSchedule(s.String())
		if err != nil {
			return nil, fmt.Errorf("crontab has a malformed schedule %q: %w", s.String(), err)
		}

		var j Job
		j.Schedule = sched
		j.sep1 = seps[i]
		if u == nil {
			j.user = &fs[i]
			i++
			j.sep2 = seps[i]

			if u, err := user.Lookup(*j.user); err != nil {
				log.Warn().Err(err).Str("user", *j.user).Msg("Error looking up crontab user")
			} else {
				j.User = u
			}
		} else {
			j.User = u
		}
		j.Command = NewCommand(fs[i])

		tc.entries = append(tc.entries, &j)
		tc.jobs = append(tc.jobs, &j)
	}

	if hasComments {
		tc.entries = append(tc.entries, comment(comments.String()))
	}

	return &tc, nil
}

func isComment(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "#")
}

func (tc *TabConfig) Jobs() []*Job {
	return tc.jobs
}

func (tc *TabConfig) String() string {
	var s []string
	for _, e := range tc.entries {
		s = append(s, e.String())
	}
	return strings.Join(s, "\n")
}

type entry interface {
	isEntry()
	String() string
}

type comment string

func (comment) isEntry() {}
func (c comment) String() string {
	return string(c)
}

type Job struct {
	Schedule Schedule
	sep1     string
	user     *string // may or may not be present. Not all crontabs specify a user per job.
	sep2     string  // may or may not exist depending on whether user is present.
	Command  *Command

	User *user.User // may or may not be present depending on whether we were able to lookup user.
}

func (*Job) isEntry() {}
func (j *Job) String() string {
	var b strings.Builder
	b.WriteString(j.Schedule.orig)
	b.WriteString(j.sep1)
	if j.user != nil {
		b.WriteString(*j.user)
		b.WriteString(j.sep2)
	}
	b.WriteString(j.Command.String())
	return b.String()
}

type Schedule struct {
	orig     string
	schedule cron.Schedule
}

func NewSchedule(s string) (Schedule, error) {
	sched, err := cron.ParseStandard(s)
	if err != nil {
		return Schedule{}, nil
	}
	return Schedule{
		orig:     s,
		schedule: sched,
	}, nil
}

// Next returns the estimated next exectuion time of this Schedule that happens strictly after t.
func (s Schedule) Next(t time.Time) time.Time {
	return s.schedule.Next(t)
}

func (s Schedule) String() string {
	f := strings.Fields(s.orig)
	return strings.Join(f, " ")
}

type Command struct {
	orig string
	args []string // Present if we're able to split orig into shell-parsed indivual arguments.
}

func NewCommand(orig string) *Command {
	cmd := &Command{}
	cmd.Transform(func(string) string { return orig })
	return cmd
}

func (c *Command) Args() ([]string, bool) {
	return c.args, c.args != nil
}

// IsCron2Mqtt checks whether this command appears to execute cron2mqtt.
func (c *Command) IsCron2Mqtt() bool {
	return c.args != nil && path.Base(c.args[0]) == "cron2mqtt"
}

func (c *Command) String() string {
	return c.orig
}

func (c *Command) Transform(f func(cmd string) string) {
	orig := f(c.orig)
	args, err := shellquote.Split(orig)
	if err != nil {
		log.Warn().Err(err).Msgf("command %q is malformed", orig)
		args = nil
	}

	c.orig = orig
	c.args = args
}

// fieldsN splits s into n fields separated by consecutive whitespace characters.
// s = seps[0] + fields[0] + seps[1] + fields[1] + ... + fields[n-1] + seps[n]
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
