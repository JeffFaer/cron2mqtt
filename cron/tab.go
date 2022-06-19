package cron

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/JeffreyFalgout/cron2mqtt/logutil"
)

// Tab represents a crontab that exists somewhere.
type Tab interface {
	Load() (*TabConfig, error)
	Update(*TabConfig) error
}

type userTab struct {
	u *user.User
}

// TabForUser provides a reference to the given User's crontab on the system.
func TabForUser(u *user.User) Tab {
	return &userTab{u}
}

// TabsForUser provides references to all crontabs that might contain a job for the user.
func TabsForUser(u *user.User) []Tab {
	// TODO: Check for more crontabs than just the user's.
	return []Tab{TabForUser(u)}
}

func (t *userTab) Load() (*TabConfig, error) {
	defer logutil.StartTimerLogger(log.With().Str("user", t.u.Username).Logger(), zerolog.DebugLevel, "Loading crontab for user").Stop()
	var stdout bytes.Buffer
	cmd := exec.Command("crontab", "-u", t.u.Username, "-l")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not load crontab for %q: %w", t.u.Username, err)
	}

	return parseTabConfig(string(stdout.Bytes()), t.u)
}

func (t *userTab) Update(tc *TabConfig) error {
	defer logutil.StartTimerLogger(log.With().Str("user", t.u.Username).Logger(), zerolog.DebugLevel, "Updating crontab for user").Stop()

	cmd := exec.Command("crontab", "-u", t.u.Username, "-")
	cmd.Stdin = strings.NewReader(tc.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not update crontab for %q: %w", t.u.Username, err)
	}
	return nil
}

func (t *userTab) String() string {
	return fmt.Sprintf("crontab for %q", t.u.Username)
}
