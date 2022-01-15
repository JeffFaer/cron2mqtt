package hass

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"regexp"
	"strings"

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

type Subscriber interface {
	Publisher
	Subscribe(ctx context.Context, topic string, qos mqtt.QoS, ms chan<- mqtt.Message) error
}

// CronJob provides methods to publish data about cron jobs to homeassistant MQTT.
type CronJob struct {
	p               Publisher
	ID              string
	configTopic     string
	stateTopic      string
	attributesTopic string
}

type config struct {
	BaseTopic       string `json:"~"`
	StateTopic      string `json:"state_topic"`
	AttributesTopic string `json:"json_attributes_topic"`

	Device   deviceConfig `json:"device"`
	UniqueID string       `json:"unique_id"`
	ObjectID string       `json:"object_id"`
	Name     string       `json:"name"`

	DeviceClass string `json:"device_class"`
	Icon        string `json:"icon"`

	PayloadOn  string `json:"payload_on"`
	PayloadOff string `json:"payload_off"`
}

func (c config) stateTopic() string {
	return c.resolve(c.StateTopic)
}

func (c config) attributesTopic() string {
	return c.resolve(c.AttributesTopic)
}

func (c config) resolve(s string) string {
	if strings.HasPrefix(s, "~/") {
		return c.BaseTopic + "/" + s[2:]
	}
	return s
}

func (c config) MarshalJSON() ([]byte, error) {
	type marshal config
	b, err := json.Marshal(marshal(c))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	abbreviateConfig(m)
	return json.Marshal(m)
}

func (c *config) UnmarshalJSON(b []byte) error {
	type marshal *config
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	expandConfig(m)
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, marshal(c))
}

type deviceConfig struct {
	Name        string   `json:"name"`
	Identifiers []string `json:"identifiers"`
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
	nodeID, err := d.nodeID()
	if err != nil {
		return nil, err
	}

	// TODO: Should we be checking to see if the config already exists?
	//       Does publishing the config again reset any of the entity's history?
	//       What happens if we change the entity in the UI (e.g. change its icon), then publish the config again?
	baseTopic := fmt.Sprintf("%s/binary_sensor/%s/%s", discoveryPrefix, nodeID, id)
	conf := config{
		BaseTopic:       baseTopic,
		StateTopic:      "~/" + stateTopicSuffix,
		AttributesTopic: "~/" + attributeTopicSuffix,

		Device: deviceConfig{
			Name:        d.hostname,
			Identifiers: []string{d.id},
		},
		UniqueID: id,
		ObjectID: "cron_job_" + id,
		Name:     fmt.Sprintf("[%s@%s] %s", d.user.Username, d.hostname, cmd),

		DeviceClass: "problem",
		Icon:        "mdi:robot",

		// These are inverted on purpose thanks to "device_class": "problem"
		PayloadOn:  failureState,
		PayloadOff: successState,
	}
	b, err := json.Marshal(conf)
	if err != nil {
		return nil, fmt.Errorf("could not marshal discovery config: %w", err)
	}

	c := CronJob{
		p:               p,
		ID:              id,
		configTopic:     conf.resolve("~/" + configTopicSuffix),
		stateTopic:      conf.stateTopic(),
		attributesTopic: conf.attributesTopic(),
	}
	if err := p.Publish(c.configTopic, mqtt.QoSExactlyOnce, mqtt.Retain, b); err != nil {
		return nil, fmt.Errorf("could not publish discovery config: %w", err)
	}

	return &c, nil
}

func DiscoverCronJobs(ctx context.Context, s Subscriber, cjs chan<- *CronJob) error {
	d, err := currentDevice()
	if err != nil {
		close(cjs)
		return err
	}
	nodeID, err := d.nodeID()
	if err != nil {
		close(cjs)
		return err
	}

	t := fmt.Sprintf("%s/binary_sensor/%s/+/config", discoveryPrefix, nodeID)
	ms := make(chan mqtt.Message, 100)
	if err := s.Subscribe(ctx, t, mqtt.QoSExactlyOnce, chan<- mqtt.Message(ms)); err != nil {
		close(cjs)
		return fmt.Errorf("could not subscribe to MQTT: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(cjs)
				return
			case m := <-ms:
				cj, err := cronJobFromConfigMessage(s, m)
				if err != nil {
					log.Println(err)
					continue
				}

				select {
				case <-ctx.Done():
					close(cjs)
					return
				case cjs <- cj:
				}
			}
		}
	}()

	return nil
}

func cronJobFromConfigMessage(s Subscriber, m mqtt.Message) (*CronJob, error) {
	defer m.Ack()
	var c config
	if err := json.Unmarshal(m.Payload(), &c); err != nil {
		return nil, fmt.Errorf("%s has invalid config: %w", m.Topic(), err)
	}
	return &CronJob{
		p:               s,
		ID:              c.UniqueID,
		configTopic:     m.Topic(),
		stateTopic:      c.stateTopic(),
		attributesTopic: c.attributesTopic(),
	}, nil
}

func ValidateTopicComponent(s string) error {
	if allowedTopicCharactersRegexp.FindString(s) != s {
		return fmt.Errorf("%q cannot be used in a topic string. Topic strings can only contain %s", s, allowedTopicCharacters)
	}

	return nil
}

// UnpublishConfig deletes this CronJob from homeassistant MQTT.
func (c *CronJob) UnpublishConfig() error {
	return c.p.Publish(c.configTopic, mqtt.QoSExactlyOnce, mqtt.Retain, "")
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

	if err := c.p.Publish(c.stateTopic, mqtt.QoSExactlyOnce, mqtt.DoNotRetain, state); err != nil {
		return fmt.Errorf("could not update state to %q: %w", state, err)
	}

	if err := c.p.Publish(c.attributesTopic, mqtt.QoSExactlyOnce, mqtt.DoNotRetain, b); err != nil {
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

func (d device) nodeID() (string, error) {
	id := fmt.Sprintf("cron2mqtt_%s_%s", d.id, d.user.Uid)
	if err := ValidateTopicComponent(id); err != nil {
		return "", fmt.Errorf("calculated node ID is invalid: %w", err)
	}
	return id, nil
}
