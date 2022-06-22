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

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/logutil"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
)

const (
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
	id string
	// The configuration for this cron job on the host, if it's known.
	Schedule *cron.Schedule
	Command  *cron.Command

	client      Client
	topicPrefix string
	plugins     []Plugin
	topics      map[Plugin]map[string]mqtt.RetainMode
}

type CronJobOption func(*CronJob)

func CronJobPlugins(ps ...Plugin) CronJobOption {
	return func(cj *CronJob) {
		cj.plugins = append(cj.plugins, ps...)
	}
}

func CronJobConfig(j *cron.Job) CronJobOption {
	return func(cj *CronJob) {
		cj.Schedule = &j.Schedule
		cj.Command = j.Command
	}
}

func CronJobCommand(args []string) CronJobOption {
	return func(cj *CronJob) {
		cj.Command = cron.NewCommand(strings.Join(args, " "))
	}
}

// NewCronJob creates a new CronJob, and runs the provided plugins through Init and OnCreate.
func NewCronJob(id string, c Client, opts ...CronJobOption) (*CronJob, error) {
	cj, err := newCronJobNoCreate(id, c, opts)
	if err != nil {
		return nil, err
	}

	if err := cj.onCreate(); err != nil {
		return nil, err
	}

	return cj, nil
}

func (c *CronJob) ID() string {
	return c.id
}

func newCronJobNoCreate(id string, c Client, opts []CronJobOption) (*CronJob, error) {
	if err := ValidateTopicComponent(id); err != nil {
		return nil, fmt.Errorf("provided cron job ID is invalid: %w", err)
	}
	d, err := CurrentDevice()
	if err != nil {
		return nil, err
	}

	cj := &CronJob{
		id:          id,
		client:      c,
		topicPrefix: fmt.Sprintf("%s/%s", d.topicPrefix, id),
		plugins:     []Plugin{&CorePlugin{}},
	}
	for _, opt := range opts {
		opt(cj)
	}

	if err := cj.initPlugins(); err != nil {
		return nil, err
	}
	return cj, nil
}

func (c *CronJob) initPlugins() error {
	defer logutil.StartTimer(zerolog.TraceLevel, "Plugin#Init").Stop()
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
	defer logutil.StartTimer(zerolog.TraceLevel, "Plugin#OnCreate").Stop()

	t := logutil.StartTimerLogger(log.Logger.With().Str("plugin", "discoverLocalCronJobIfNecessary").Logger(), zerolog.TraceLevel, "Plugin#OnCreate")
	c.discoverLocalCronJobIfNecessary()
	t.Stop()

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

func (c *CronJob) discoverLocalCronJobIfNecessary() {
	if c.Schedule != nil && c.Command != nil {
		return
	}

	d, err := CurrentDevice()
	if err != nil {
		log.Warn().Str("id", c.id).Err(err).Msg("Could not find cron job configuration.")
		return
	}

	js, err := DiscoverLocalCronJobsByID(cron.TabsForUser(d.User), d.User, []string{c.id})
	j, ok := js[c.id]
	if !ok {
		log.Warn().Str("id", c.id).Str("user", d.User.Username).Err(err).Msg("Could not find cron job configuration in any crontab for user.")
		return
	}

	if c.Command != nil && !sameCron2mqttCommand(j.Command, c.Command) {
		log.Warn().Str("id", c.id).Str("found", j.Command.String()).Str("current", c.Command.String()).Msgf("Found cron job configuration that does not match currently executing command.")
	}
	if c.Schedule == nil {
		c.Schedule = &j.Schedule
	}
	if c.Command == nil {
		c.Command = j.Command
	}
}

func sameCron2mqttCommand(c1 *cron.Command, c2 *cron.Command) bool {
	if !c1.IsCron2Mqtt() || !c2.IsCron2Mqtt() {
		return false
	}
	if c1.String() == c2.String() {
		return true
	}
	a1, ok1 := c1.Args()
	a2, ok2 := c2.Args()
	if !ok1 || !ok2 {
		return false
	}
	if len(a1) != len(a2) {
		return false
	}
	for i := 1; i < len(a1); i++ {
		if a1[i] != a2[i] {
			return false
		}
	}
	return true
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
