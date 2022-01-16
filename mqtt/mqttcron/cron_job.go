package mqttcron

import (
	"crypto/hmac"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"regexp"

	"github.com/btcsuite/btcutil/base58"
	"github.com/denisbrodbeck/machineid"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
)

const (
	ExitCodeAttributeName = "exit_code"
)

func ValidateTopicComponent(s string) error {
	if allowedTopicCharactersRegexp.FindString(s) != s {
		return fmt.Errorf("%q cannot be used in a topic string. Topic strings can only contain %s", s, allowedTopicCharacters)
	}

	return nil
}

const (
	allowedTopicCharacters = "[a-zA-Z0-9_-]"
)

var (
	allowedTopicCharactersRegexp = regexp.MustCompile(allowedTopicCharacters + "+")
)

type Publisher interface {
	Publish(topic string, qos mqtt.QoS, retain mqtt.RetainMode, payload interface{}) error
}

type CronJob struct {
	ID    string
	Topic string
}

func NewCronJob(id string) (*CronJob, error) {
	if err := ValidateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}
	d, err := CurrentDevice()
	if err != nil {
		return nil, err
	}

	return &CronJob{
		ID:    id,
		Topic: fmt.Sprintf("cron2mqtt/%s/%s/%s", d.ID, d.User.Uid, id),
	}, nil
}

// PublishResults updates MQTT about the given execution result.
func (c *CronJob) PublishResults(p Publisher, res exec.Result) error {
	results := map[string]interface{}{
		"args":                res.Args,
		"start_time":          res.Start,
		"end_time":            res.End,
		"duration_ms":         res.End.Sub(res.Start).Milliseconds(),
		"stdout":              string(res.Stdout),
		"stderr":              string(res.Stderr),
		ExitCodeAttributeName: res.ExitCode,
		// TODO: Include an entry for the estimated next execution time, if we can find the schedule for this cron job.
	}
	b, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("could not marshal results: %w", err)
	}

	if err := p.Publish(c.Topic, mqtt.QoSExactlyOnce, mqtt.DoNotRetain, b); err != nil {
		return fmt.Errorf("could not update results to %q: %w", string(b), err)
	}
	return nil
}

type Device struct {
	ID       string
	User     *user.User
	Hostname string
}

func CurrentDevice() (Device, error) {
	id, err := machineid.ID()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine machineid: %w", err)
	}
	u, err := user.Current()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine current user: %w", err)
	}
	h, err := os.Hostname()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine hostname: %w", err)
	}

	return Device{protect(id), u, h}, nil
}

func protect(id string) string {
	mac := hmac.New(md5.New, []byte(id))
	mac.Write([]byte("cron2mqtt"))
	b := mac.Sum(nil)
	return base58.Encode(b)
}
