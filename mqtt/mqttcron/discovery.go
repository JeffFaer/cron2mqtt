package mqttcron

import (
	"context"
	"fmt"
	"strings"

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
	if err := c.Subscribe(ctx, pre+"+"+post, mqtt.QoSExactlyOnce, chan<- mqtt.Message(ms)); err != nil {
		close(ch)
		return fmt.Errorf("could not subscribe to MQTT: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case m := <-ms:
				cj, err := discoverCronJob(pre, post, m, c, fs)
				if err != nil {
					log.Warn().Err(err).Msg("Error while discovering cron jobs")
					continue
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

func discoverCronJob(pre, post string, m mqtt.Message, c Client, fs []func() Plugin) (*CronJob, error) {
	defer m.Ack()
	id := strings.TrimPrefix(m.Topic(), pre)
	id = strings.TrimSuffix(id, post)
	var ps []Plugin
	for _, f := range fs {
		ps = append(ps, f())
	}
	return newCronJobNoCreate(id, c, ps)
}
