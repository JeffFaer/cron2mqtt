package hass

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
	failureState = "failure"
	successState = "success"

	discoveryPrefix      = "homeassistant"
	configTopicSuffix    = "config"
	stateTopicSuffix     = "state"
	attributeTopicSuffix = "attributes"

	allowedTopicCharacters = "[a-zA-Z0-9_-]"
)

var (
	allowedTopicCharactersRegexp = regexp.MustCompile(allowedTopicCharacters + "+")
)

// Publisher publishes data to MQTT topics.
type Publisher interface {
	Publish(topic string, qos mqtt.QoS, retain mqtt.RetainMode, payload interface{}) error
}

// CronJob provides methods to publish data about cron jobs to homeassistant MQTT.
type CronJob struct {
	p         Publisher
	baseTopic string
}

// NewCronJob creates a new cron job and publishes its config to homeassistant MQTT.
func NewCronJob(p Publisher, id string, cmd string) (*CronJob, error) {
	if err := ValidateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}

	d, err := currentDevice()
	if err != nil {
		return nil, err
	}

	nodeID := fmt.Sprintf("cron_job_%s_%s", d.user.Uid, d.id)
	if err := ValidateTopicComponent(nodeID); err != nil {
		return nil, fmt.Errorf("calculated node ID is invalid: %w", err)
	}

	// TODO: Should we be checking to see if the config already exists?
	//       Does publishing the config again reset any of the entity's history?
	//       What happens if we change the entity in the UI (e.g. change its icon), then publish the config again?
	baseTopic := fmt.Sprintf("%s/binary_sensor/%s/%s", discoveryPrefix, nodeID, id)
	conf := map[string]interface{}{
		"~":                     baseTopic,
		"state_topic":           "~/" + stateTopicSuffix,
		"json_attributes_topic": "~/" + attributeTopicSuffix,

		"unique_id": id,
		"device": map[string]interface{}{
			"name":        d.hostname,
			"identifiers": []string{d.id},
		},
		"object_id":    "cron_job_" + id,
		"name":         fmt.Sprintf("[%s@%s] %s", d.user.Username, d.hostname, cmd),
		"device_class": "problem",
		"icon":         "mdi:robot",

		// These are inverted on purpose thanks to "device_class": "problem"
		"payload_on":  failureState,
		"payload_off": successState,
	}
	minimize(conf)
	b, err := json.Marshal(conf)
	if err != nil {
		return nil, fmt.Errorf("could not marshal discovery config: %w", err)
	}

	c := CronJob{p, baseTopic}
	if err := p.Publish(c.topic(configTopicSuffix), mqtt.QoSExactlyOnce, mqtt.Retain, b); err != nil {
		return nil, fmt.Errorf("could not publish discovery config: %w", err)
	}

	return &c, nil
}

func ValidateTopicComponent(s string) error {
	if allowedTopicCharactersRegexp.FindString(s) != s {
		return fmt.Errorf("%q cannot be used in a topic string. Topic strings can only contain %s", s, allowedTopicCharacters)
	}

	return nil
}

func (c *CronJob) topic(suffix string) string {
	return c.baseTopic + "/" + suffix
}

// UnpublishConfig deletes this CronJob from homeassistant MQTT.
func (c *CronJob) UnpublishConfig() error {
	return c.p.Publish(c.topic(configTopicSuffix), mqtt.QoSExactlyOnce, mqtt.Retain, "")
}

// PublishResults publishes messages updating homeassistant MQTT about the invocation results.
func (c *CronJob) PublishResults(res exec.Result) error {
	state := successState
	if res.ExitCode != 0 {
		state = failureState
	}
	attr := map[string]interface{}{
		"cmd":         res.Cmd,
		"args":        res.Args,
		"start_time":  res.Start,
		"end_time":    res.End,
		"duration_ms": res.End.Sub(res.Start).Milliseconds(),
		"stdout":      string(res.Stdout),
		"stderr":      string(res.Stderr),
		"exit_code":   res.ExitCode,
		// TODO: Include an attribute for the estimated next execution time, if we can find the schedule for this cron job.
	}
	b, err := json.Marshal(attr)
	if err != nil {
		return fmt.Errorf("could not marshal attributes: %w", err)
	}

	if err := c.p.Publish(c.topic(stateTopicSuffix), mqtt.QoSExactlyOnce, mqtt.DoNotRetain, state); err != nil {
		return fmt.Errorf("could not update state to %q: %w", state, err)
	}

	if err := c.p.Publish(c.topic(attributeTopicSuffix), mqtt.QoSExactlyOnce, mqtt.DoNotRetain, b); err != nil {
		return fmt.Errorf("could not update attributes to %q: %w", string(b), err)
	}

	return nil
}

type device struct {
	id       string
	user     *user.User
	hostname string
}

func currentDevice() (device, error) {
	id, err := machineid.ID()
	if err != nil {
		return device{}, fmt.Errorf("could not determine machineid: %w", err)
	}
	u, err := user.Current()
	if err != nil {
		return device{}, fmt.Errorf("could not determine current user: %w", err)
	}
	h, err := os.Hostname()
	if err != nil {
		return device{}, fmt.Errorf("could not determine hostname: %w", err)
	}

	return device{protect(id), u, h}, nil
}

func protect(id string) string {
	mac := hmac.New(md5.New, []byte(id))
	mac.Write([]byte("cron2mqtt"))
	b := mac.Sum(nil)
	return base58.Encode(b)
}
