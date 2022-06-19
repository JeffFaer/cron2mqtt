package hass

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttcron"
)

const (
	failureState = "failure"
	successState = "success"

	discoveryPrefix = "homeassistant"
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

// Plugin provides home assistant specific funcionality to mqttcron.CronJob.
type Plugin struct {
	mqttcron.NopPlugin
	configTopic string
}

func NewPlugin() mqttcron.Plugin {
	return &Plugin{}
}

func (p *Plugin) Init(cj *mqttcron.CronJob, reg mqttcron.TopicRegister) error {
	d, err := mqttcron.CurrentDevice()
	if err != nil {
		return err
	}
	nodeID, err := nodeID(d)
	if err != nil {
		return err
	}

	p.configTopic = fmt.Sprintf("%s/binary_sensor/%s/%s/config", discoveryPrefix, nodeID, cj.ID)
	reg.RegisterTopic(p.configTopic, mqtt.Retain)
	return nil
}

func (p *Plugin) OnCreate(cj *mqttcron.CronJob, pub mqttcron.Publisher) error {
	d, err := mqttcron.CurrentDevice()
	if err != nil {
		return err
	}
	var cp *mqttcron.CorePlugin
	if !cj.Plugin(&cp) {
		return fmt.Errorf("could not retrieve mqttcron.CorePlugin")
	}
	conf := config{
		BaseTopic:       cp.ResultsTopic,
		StateTopic:      "~",
		ValueTemplate:   fmt.Sprintf("{%% if value_json.%s == 0 %%}%s{%% else %%}%s{%% endif %%}", mqttcron.ExitCodeAttributeName, successState, failureState),
		AttributesTopic: "~",

		Device: deviceConfig{
			Name:        d.Hostname,
			Identifiers: []string{d.ID},
		},
		UniqueID: cj.ID,
		ObjectID: "cron_job_" + cj.ID,
		Name:     fmt.Sprintf("[%s@%s] %s", d.User.Username, d.Hostname, commandName(cj.ID, cj.Command)),

		DeviceClass: "problem",
		Icon:        "mdi:robot",

		// These are inverted on purpose thanks to "device_class": "problem"
		PayloadOn:  failureState,
		PayloadOff: successState,
	}
	b, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("could not marshal discovery config: %w", err)
	}
	if err := pub.Publish(p.configTopic, mqtt.QoSExactlyOnce, mqtt.Retain, b); err != nil {
		return fmt.Errorf("could not publish discovery config: %w", err)
	}
	return nil
}

func nodeID(d mqttcron.Device) (string, error) {
	id := fmt.Sprintf("cron2mqtt_%s_%s", d.ID, d.User.Uid)
	if err := mqttcron.ValidateTopicComponent(id); err != nil {
		return "", fmt.Errorf("calculated node ID is invalid: %w", err)
	}
	return id, nil
}

func commandName(id string, c *cron.Command) string {
	if c == nil {
		return id
	}
	args, ok := c.Args()
	if !ok {
		return c.String()
	}

	var shArgs []string
	for i, arg := range args {
		if arg == "--" {
			shArgs = args[i+1:]
			break
		}
		if arg == id && i < len(args)-1 {
			shArgs = []string{}
			continue
		}
		if shArgs == nil || strings.HasPrefix(arg, "-") {
			continue
		}
		shArgs = append(shArgs, arg)
	}
	if len(shArgs) > 0 {
		if sp, err := shellquote.Split(strings.Join(shArgs, " ")); err == nil {
			return strings.Join(sp, " ")
		}
	}
	return strings.Join(args, " ")
}

func index(haystack []string, needle string) int {
	for i, s := range haystack {
		if s == needle {
			return i
		}
	}
	return -1
}
