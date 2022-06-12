package mqttcron

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"fmt"
	"os"
	"os/user"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/denisbrodbeck/machineid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/logutil"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
)

const (
	ExitCodeAttributeName  = "exit_code"
	allowedTopicCharacters = "[a-zA-Z0-9_-]"
)

var (
	allowedTopicCharactersRegexp = regexp.MustCompile(allowedTopicCharacters + "+")
)

func ValidateTopicComponent(s string) error {
	if allowedTopicCharactersRegexp.FindString(s) != s {
		return fmt.Errorf("%q cannot be used in a topic string. Topic strings can only contain %s", s, allowedTopicCharacters)
	}

	return nil
}

type Publisher interface {
	Publish(topic string, qos mqtt.QoS, retain mqtt.RetainMode, payload interface{}) error
}
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, qos mqtt.QoS, messages chan<- mqtt.Message) error
}
type Client interface {
	Publisher
	Subscriber
}

type CronJob struct {
	ID string

	client      Client
	topicPrefix string
	plugins     []Plugin
	topics      map[Plugin]map[string]mqtt.RetainMode
}

// NewCronJob creates a new CronJob, and runs the provided plugins through Init and OnCreate.
func NewCronJob(id string, c Client, ps ...Plugin) (*CronJob, error) {
	cj, err := newCronJobNoCreate(id, c, ps)
	if err != nil {
		return nil, err
	}

	if err := cj.onCreate(); err != nil {
		return nil, err
	}

	return cj, nil
}

func newCronJobNoCreate(id string, c Client, ps []Plugin) (*CronJob, error) {
	if err := ValidateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}
	d, err := CurrentDevice()
	if err != nil {
		return nil, err
	}

	ps = append([]Plugin{&CorePlugin{}}, ps...)
	cj := &CronJob{
		ID:          id,
		client:      c,
		topicPrefix: fmt.Sprintf("%s/%s", d.topicPrefix, id),
		plugins:     ps,
	}

	if err := cj.initPlugins(); err != nil {
		return nil, err
	}
	return cj, nil
}

func (c *CronJob) initPlugins() error {
	reg := &topicRegister{
		prefix:         c.topicPrefix,
		suffixes:       make(map[string]Plugin),
		topics:         make(map[string]Plugin),
		topicsByPlugin: make(map[Plugin]map[string]mqtt.RetainMode),
	}
	var err error
	for _, p := range c.plugins {
		reg.p = p
		reg.err = nil
		if e := p.Init(c, reg); e != nil {
			err = multierr.Append(err, e)
		}
		err = multierr.Append(err, reg.err)

		var ts []string
		for t := range reg.topicsByPlugin[p] {
			ts = append(ts, t)
		}
		sort.Strings(ts)
		log.Trace().Strs("topics", ts).Str("plugin", fmt.Sprintf("%T", p)).Msg("Registered topics")
	}

	c.topics = reg.topicsByPlugin
	return err
}

type topicRegister struct {
	prefix         string
	suffixes       map[string]Plugin
	topics         map[string]Plugin
	topicsByPlugin map[Plugin]map[string]mqtt.RetainMode

	p   Plugin
	err error
}

func (reg *topicRegister) RegisterSuffix(suffix string) string {
	if p, ok := reg.suffixes[suffix]; ok {
		reg.err = multierr.Append(reg.err, fmt.Errorf("plugin %T tried to register suffix %q which was already registered by %T", reg.p, suffix, p))
		return ""
	}
	t := reg.prefix + "/" + suffix
	if !reg.registerTopic(t, mqtt.Retain) {
		return ""
	}
	reg.suffixes[suffix] = reg.p
	return t
}

func (reg *topicRegister) RegisterTopic(topic string, retain mqtt.RetainMode) {
	reg.registerTopic(topic, retain)
}

func (reg *topicRegister) registerTopic(topic string, retain mqtt.RetainMode) bool {
	if p, ok := reg.topics[topic]; ok {
		reg.err = multierr.Append(reg.err, fmt.Errorf("plugin %T tried to register topic %q which was already registered by %T", reg.p, topic, p))
		return false
	}
	reg.topics[topic] = reg.p

	m, ok := reg.topicsByPlugin[reg.p]
	if !ok {
		m = make(map[string]mqtt.RetainMode)
		reg.topicsByPlugin[reg.p] = m
	}
	m[topic] = retain
	return true
}

// Plugin gives callers the ability to inspect Plugins.
//
// This will panic if it's called during Plugin.Init.
func (c *CronJob) Plugin(ptr interface{}) bool {
	if c.topics == nil {
		panic(fmt.Errorf("cannot call Plugin before plugins are initialized"))
	}

	v := reflect.ValueOf(ptr).Elem()
	var ps []Plugin
	for _, p := range c.plugins {
		if reflect.TypeOf(p) == v.Type() {
			ps = append(ps, p)
		}
	}
	if len(ps) == 0 {
		return false
	}
	if len(ps) > 1 {
		panic(fmt.Errorf("multiple plugins matched %T", ptr))
	}
	v.Set(reflect.ValueOf(ps[0]))
	return true
}

func (c *CronJob) onCreate() error {
	var fs []func() error
	for _, p := range c.plugins {
		p := p
		fs = append(fs, func() error {
			defer logutil.StartTimerLogger(log.Logger.With().Str("plugin", fmt.Sprintf("%T", p)).Logger(), zerolog.TraceLevel, "Plugin#OnCreate").Stop()
			return p.OnCreate(c, limitedPublisher{c.client, p, c.topics[p]})
		})
	}
	return MultiPublish(fs...)
}

// PublishResult publishes one or more messages to MQTT about the given execution result.
func (c *CronJob) PublishResult(res exec.Result) error {
	var fs []func() error
	for _, p := range c.plugins {
		p := p
		fs = append(fs, func() error {
			defer logutil.StartTimerLogger(log.Logger.With().Str("plugin", fmt.Sprintf("%T", p)).Logger(), zerolog.TraceLevel, "Plugin#PublishResult").Stop()
			return p.PublishResult(c, limitedPublisher{c.client, p, c.topics[p]}, res)
		})
	}
	return MultiPublish(fs...)
}

type limitedPublisher struct {
	pub    Publisher
	p      Plugin
	topics map[string]mqtt.RetainMode
}

func (p limitedPublisher) Publish(topic string, qos mqtt.QoS, retain mqtt.RetainMode, payload interface{}) error {
	if ret, ok := p.topics[topic]; !ok {
		return fmt.Errorf("plugin %T did not register topic %s", p.p, topic)
	} else if retain == mqtt.Retain && ret != mqtt.Retain {
		return fmt.Errorf("plugin %T did not register topic %s for mqtt.%s", p.p, topic, retain.String())
	}
	return p.pub.Publish(topic, qos, retain, payload)
}

// Unpublish clears all MQTT topics used by this CronJob that have retained data.
func (c *CronJob) Unpublish(ctx context.Context) error {
	var fs []func() error
	fs = append(fs, func() error { return c.unpublishPrefix(ctx, c.topicPrefix) })
	for _, ts := range c.topics {
		for t, retain := range ts {
			if retain != mqtt.Retain {
				continue
			}
			if strings.HasPrefix(t, c.topicPrefix+"/") {
				continue
			}
			t := t
			fs = append(fs, func() error { return c.unpublishTopic(t) })
		}
	}
	return MultiPublish(fs...)
}

func (c *CronJob) unpublishPrefix(ctx context.Context, prefix string) error {
	defer logutil.StartTimerLogger(log.Ctx(ctx).With().Str("prefix", prefix).Logger(), zerolog.DebugLevel, "Unpublishing by prefix").Stop()

	ms := make(chan mqtt.Message, 100)
	timeout := 200 * time.Millisecond // TODO: Make this configutable
	if err := discoverRetainedMessages(ctx, prefix+"/#", c.client, timeout, chan<- mqtt.Message(ms)); err != nil {
		return err
	}

	var errs error
	for m := range ms {
		if err := c.unpublishTopic(m.Topic()); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			m.Ack()
		}
	}
	return errs
}

func (c *CronJob) unpublishTopic(topic string) error {
	return c.client.Publish(topic, mqtt.QoSExactlyOnce, mqtt.Retain, "")
}

type Device struct {
	ID       string
	User     *user.User
	Hostname string

	topicPrefix string
}

func CurrentDevice() (Device, error) {
	id, err := machineid.ID()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine machineid: %w", err)
	}
	id = protect(id)

	u, err := user.Current()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine current user: %w", err)
	}
	h, err := os.Hostname()
	if err != nil {
		return Device{}, fmt.Errorf("could not determine hostname: %w", err)
	}

	pre := fmt.Sprintf("cron2mqtt/%s/%s", id, u.Uid)
	return Device{id, u, h, pre}, nil
}

func protect(id string) string {
	mac := hmac.New(md5.New, []byte(id))
	mac.Write([]byte("cron2mqtt"))
	b := mac.Sum(nil)
	return base58.Encode(b)
}
