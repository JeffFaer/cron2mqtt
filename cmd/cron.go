package cmd

import (
	"fmt"
	"os/user"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
)

type crontab interface {
	cron.Tab
	name() string
}

type userCrontab struct {
	cron.Tab
	u *user.User
}

func (c userCrontab) name() string {
	return fmt.Sprintf("crontab for %q", c.u.Username)
}

func crontabs() ([]crontab, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("could not determine current user: %w", err)
	}

	// TODO: Check for more crontabs than just the current user's.
	return []crontab{userCrontab{cron.TabForUser(u), u}}, nil
}
