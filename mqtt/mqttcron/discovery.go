package mqttcron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
)

func DiscoverCronJobs(ctx context.Context, c Client, fs ...func() Plugin) ([]*CronJob, error) {
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
