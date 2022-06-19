package mqttcron

import (
	"context"
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/rs/zerolog/log"
)

func DiscoverRemoteCronJobs(ctx context.Context, c Client, fs ...func() Plugin) ([]*CronJob, error) {
	d, err := CurrentDevice()
	if err != nil {
		return nil, err
	}

	pre := d.topicPrefix + "/"
	post := "/discovery"
	ms := make(chan mqtt.Message, 100)
	if err := discoverRetainedMessages(ctx, pre+"+"+post, c, 0, chan<- mqtt.Message(ms)); err != nil {
		return nil, err
	}

	var cjs []*CronJob
	for m := range ms {
		id := m.Topic()
		id = strings.TrimPrefix(id, pre)
		id = strings.TrimSuffix(id, post)
		var ps []Plugin
		for _, f := range fs {
			ps = append(ps, f())
		}
		cj, err := newCronJobNoCreate(id, c, ps)
		if err != nil {
			return nil, fmt.Errorf("could not create cron job %q: %w", id, err)
		}

		m.Ack()
		cjs = append(cjs, cj)
	}

	return cjs, nil
}

// discoverRetainedMessages subscribes to topic and reports any retained messages on ch.
//
// discoverRetainedMessages will continue discovering retained messages until either the provided context is done, or keepAlive has elapsed since we received the last retained message. If keepAlive is <= 0, it will be ignored.
func discoverRetainedMessages(ctx context.Context, topic string, c Client, keepAlive time.Duration, ch chan<- mqtt.Message) error {
	origCtx := ctx
	ctx, canc := context.WithCancel(ctx)
	// Do not defer canc(). This function returns before we'd want to call canc().

	ms := make(chan mqtt.Message, 100)
	if err := c.Subscribe(ctx, topic, mqtt.QoSExactlyOnce, chan<- mqtt.Message(ms)); err != nil {
		canc()
		close(ch)
		return fmt.Errorf("could not subscribe to MQTT: %w", err)
	}

	go func() {
		defer close(ch)
		defer canc()
		var timeout <-chan time.Time
		for {
			select {
			case <-timeout:
				canc() // Signal to Subscribe that we're done.
			case m, ok := <-ms:
				if !ok {
					return
				} else if !m.Retained() {
					continue
				}

				if keepAlive > 0 {
					timeout = time.After(keepAlive)
				}

				select {
				case <-origCtx.Done():
					return
				case ch <- m:
				}
			}
		}
	}()

	return nil
}

// DiscoverLocalCronJobsById looks at local crontabs for cronjobs identified by one of the entries in ids.
func DiscoverLocalCronJobsByID(cts []cron.Tab, u *user.User, ids []string) (map[string]*cron.Job, error) {
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	foundCt := make(map[string]cron.Tab)
	cjs := make(map[string]*cron.Job)

	for _, ct := range cts {
		t, err := ct.Load()
		if err != nil {
			return nil, fmt.Errorf("could not load %s: %w", ct, err)
		}

		for _, j := range t.Jobs() {
			if j.User == nil || j.User.Uid != u.Uid {
				continue
			}
			if !j.Command.IsCron2Mqtt() {
				continue
			}

			// Check to see if any of the cron job's arguments are one of the remote cron job IDs.
			args, ok := j.Command.Args()
			if !ok {
				continue
			}
			// The cron job's command will be at a minimum "cron2mqtt exec ID ...", so only start looking at the third element.
			// Technically we're looking at more arugments than necessary, but it seems unlikely we'd have a false positive.
			for _, arg := range args[2:] {
				if !idSet[arg] {
					continue
				}
				id := arg

				if cj, ok := cjs[id]; ok {
					log.Warn().Str("id", id).Msgf("Discovered ID multiple times:\n%s\n%s\n\n%s\n%s\n", foundCt[id], cj, ct, j)
				} else {
					foundCt[id] = ct
					cjs[id] = j
				}
				break
			}
		}
	}

	return cjs, nil
}
