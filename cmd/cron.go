package cmd

import (
	"fmt"
	"os/user"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
)

type possibleCrontab interface {
	name() string
	load() (cron.Tab, error)
	update(cron.Tab) error
}

type userCrontab struct {
	u *user.User
}

func (c userCrontab) name() string {
	return fmt.Sprintf("crontab for %q", c.u.Username)
}
func (c userCrontab) load() (cron.Tab, error) {
	return cron.Load(c.u)
}
func (c userCrontab) update(t cron.Tab) error {
	return cron.Update(c.u, t)
}

func possibleCrontabs() ([]possibleCrontab, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("could not determine current user: %w", err)
	}

	// TODO: Check for more crontabs than just the current user's.
	return []possibleCrontab{userCrontab{u}}, nil
}
