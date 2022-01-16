package hass

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttcron"
)

const (
	failureState = "failure"
	successState = "success"

	discoveryPrefix      = "homeassistant"
	configTopicSuffix    = "config"
	stateTopicSuffix     = "state"
	attributeTopicSuffix = "attributes"
)

type config struct {
	BaseTopic       string `json:"~"`
	StateTopic      string `json:"state_topic"`
	ValueTemplate   string `json:"value_template"`
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

type Subscriber interface {
	Subscribe(ctx context.Context, topic string, qos mqtt.QoS, ms chan<- mqtt.Message) error
}

// CronJob provides methods to publish data about cron jobs to homeassistant MQTT.
type CronJob struct {
	cj          *mqttcron.CronJob
	configTopic string
}

// NewCronJob creates a new cron job and publishes its config to homeassistant MQTT.
func NewCronJob(p mqttcron.Publisher, id string, cmd string) (*CronJob, error) {
	cj, err := mqttcron.NewCronJob(id)
	if err != nil {
		return nil, err
	}
	if err := mqttcron.ValidateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}
	d, err := mqttcron.CurrentDevice()
	if err != nil {
		return nil, err
	}
	nodeID, err := nodeID(d)
	if err != nil {
		return nil, err
	}

	conf := config{
		BaseTopic:       cj.Topic,
		StateTopic:      "~",
		ValueTemplate:   fmt.Sprintf("{%% if value_json.%s == 0 %%}%s{%% else %%}%s{%% endif %%}", mqttcron.ExitCodeAttributeName, successState, failureState),
		AttributesTopic: "~",

		Device: deviceConfig{
			Name:        d.Hostname,
			Identifiers: []string{d.ID},
		},
		UniqueID: id,
		ObjectID: "cron_job_" + id,
		Name:     fmt.Sprintf("[%s@%s] %s", d.User.Username, d.Hostname, cmd),

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
		cj:          cj,
		configTopic: fmt.Sprintf("%s/binary_sensor/%s/%s/config", discoveryPrefix, nodeID, id),
	}
	if err := p.Publish(c.configTopic, mqtt.QoSExactlyOnce, mqtt.Retain, b); err != nil {
		return nil, fmt.Errorf("could not publish discovery config: %w", err)
	}

	return &c, nil
}

func DiscoverCronJobs(ctx context.Context, s Subscriber, cjs chan<- *CronJob) error {
	d, err := mqttcron.CurrentDevice()
	if err != nil {
		close(cjs)
		return err
	}
	nodeID, err := nodeID(d)
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
	cj, err := mqttcron.NewCronJob(c.UniqueID)
	if err != nil {
		return nil, err
	}
	cj.Topic = c.BaseTopic
	return &CronJob{
		cj:          cj,
		configTopic: m.Topic(),
	}, nil
}

func (c *CronJob) ID() string {
	return c.cj.ID
}

// UnpublishConfig deletes this CronJob from homeassistant MQTT.
func (c *CronJob) UnpublishConfig(p mqttcron.Publisher) error {
	return p.Publish(c.configTopic, mqtt.QoSExactlyOnce, mqtt.Retain, "")
}

// PublishResults publishes messages updating homeassistant MQTT about the invocation results.
func (c *CronJob) PublishResults(p mqttcron.Publisher, res exec.Result) error {
	return c.cj.PublishResults(p, res)
}

func nodeID(d mqttcron.Device) (string, error) {
	id := fmt.Sprintf("cron2mqtt_%s_%s", d.ID, d.User.Uid)
	if err := mqttcron.ValidateTopicComponent(id); err != nil {
		return "", fmt.Errorf("calculated node ID is invalid: %w", err)
	}
	return id, nil
}
