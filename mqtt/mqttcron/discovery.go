package mqttcron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/rs/zerolog/log"
)

func DiscoverCronJobs(ctx context.Context, c Client, ch chan<- *CronJob, fs ...func() Plugin) error {
	d, err := CurrentDevice()
	if err != nil {
		close(ch)
		return err
	}

	pre := d.topicPrefix + "/"
	post := "/discovery"
	ms := make(chan mqtt.Message, 100)
	if err := discoverRetainedMessages(ctx, pre+"+"+post, c, 0, chan<- mqtt.Message(ms)); err != nil {
		close(ch)
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case m, ok := <-ms:
				if !ok {
					close(ch)
					return
				}
				id := m.Topic()
				id = strings.TrimPrefix(id, pre)
				id = strings.TrimSuffix(id, post)
				var ps []Plugin
				for _, f := range fs {
					ps = append(ps, f())
				}
				cj, err := newCronJobNoCreate(id, c, ps)
				if err != nil {
					log.Warn().Err(err).Msg("Error while discovering cron jobs")
					continue
				} else {
					m.Ack()
				}

				select {
				case <-ctx.Done():
					close(ch)
					return
				case ch <- cj:
				}
			}
		}
	}()

	return nil
}

// discoverRetainedMessages subscribes to topic and reports any retained messages on ch.
//
// discoverRetainedMessages will continue discovering retained messages until either the provided context is done, or timeout has elapsed since we received the last retained message. If timeout is <= 0, it will be ignored.
func discoverRetainedMessages(ctx context.Context, topic string, c Client, timeout time.Duration, ch chan<- mqtt.Message) error {
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
		defer canc()
		var timeoutCh <-chan time.Time
		for {
			select {
			case <-origCtx.Done():
				close(ch)
				return
			case <-timeoutCh:
				canc() // Signal to Subscribe that we're done.
			case m, ok := <-ms:
				if !ok {
					close(ch)
					return
				} else if !m.Retained() {
					continue
				}

				if timeout > 0 {
					timeoutCh = time.After(timeout)
				}

				select {
				case <-origCtx.Done():
					close(ch)
					return
				case ch <- m:
				}
			}
		}
	}()

	return nil
}
