package hass

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"regexp"

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
	Publish(topic string, qos mqtt.QoS, retain bool, payload interface{}) error
}

// CronJob provides methods to publish data about cron jobs to homeassistant MQTT.
type CronJob struct {
	p         Publisher
	baseTopic string
}

// NewCronJob creates a new cron job and publishes its config to homeassistant MQTT.
func NewCronJob(p Publisher, id string, cmd string) (*CronJob, error) {
	if err := validateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}

	d, err := currentDevice()
	if err != nil {
		return nil, err
	}

	nodeID := fmt.Sprintf("cron_job_%s_%s", d.user, d.host)
	if err := validateTopicComponent(nodeID); err != nil {
		return nil, fmt.Errorf("calculated node ID is invalid: %w", err)
	}

	baseTopic := fmt.Sprintf("%s/binary_sensor/%s/%s", discoveryPrefix, nodeID, id)
	conf := map[string]interface{}{
		"~":                     baseTopic,
		"state_topic":           "~/" + stateTopicSuffix,
		"json_attributes_topic": "~/" + attributeTopicSuffix,

		"unique_id": id,
		"device": map[string]interface{}{
			"name":        d.host,
			"identifiers": d.macIDs,
		},
		"object_id":    id,
		"name":         fmt.Sprintf("[%s@%s] %s", d.user, d.host, cmd),
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
	if err := p.Publish(c.topic(configTopicSuffix), mqtt.QoSExactlyOnce, true, b); err != nil {
		return nil, fmt.Errorf("could not publish discovery config: %w", err)
	}

	return &c, nil
}

func validateTopicComponent(s string) error {
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
	return c.p.Publish(c.topic(configTopicSuffix), mqtt.QoSExactlyOnce, true, "")
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
	}
	b, err := json.Marshal(attr)
	if err != nil {
		return fmt.Errorf("could not marshal attributes: %w", err)
	}

	if err := c.p.Publish(c.topic(stateTopicSuffix), mqtt.QoSExactlyOnce, false, state); err != nil {
		return fmt.Errorf("could not update state to %q: %w", state, err)
	}

	if err := c.p.Publish(c.topic(attributeTopicSuffix), mqtt.QoSExactlyOnce, false, b); err != nil {
		return fmt.Errorf("could not update attributes to %q: %w", string(b), err)
	}

	return nil
}

type device struct {
	user, host string
	macIDs     []string
}

func currentDevice() (device, error) {
	u, err := user.Current()
	if err != nil {
		return device{}, fmt.Errorf("could not determine current user: %w", err)
	}
	h, err := os.Hostname()
	if err != nil {
		return device{}, fmt.Errorf("could not determine hostname: %w", err)
	}

	d := device{u.Username, h, nil}

	ifs, err := net.Interfaces()
	if err == nil {
		for _, i := range ifs {
			if i.Flags&net.FlagUp != 0 && len(i.HardwareAddr) > 0 {
				// Skip locally administered addresses
				if i.HardwareAddr[0]&2 == 2 {
					continue
				}
				d.macIDs = append(d.macIDs, i.HardwareAddr.String())
			}
		}
	}

	return d, nil
}
