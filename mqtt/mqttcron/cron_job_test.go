package mqttcron

import (
	"errors"
	"regexp"
	"testing"

	"github.com/JeffreyFalgout/cron2mqtt/cron"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttfake"
)

func TestPluginInit(t *testing.T) {
	for _, tc := range []struct {
		name string

		plugins []Plugin
		wantErr *regexp.Regexp
	}{
		{
			name: "conflicting suffix",

			plugins: []Plugin{&plugin{suffix: "foo"}, &plugin{suffix: "foo"}},
			wantErr: regexp.MustCompile(regexp.QuoteMeta("tried to register suffix \"foo\" which was already registered")),
		},
		{
			name: "conflicting topic",

			plugins: []Plugin{&plugin{topic: "foo"}, &plugin{topic: "foo"}},
			wantErr: regexp.MustCompile(regexp.QuoteMeta("tried to register topic \"foo\" which was already registered")),
		},
		{
			name: "conflicting suffix and topic",

			plugins: []Plugin{&plugin{suffix: "foo"}, &plugin{manualSuffix: "foo"}},
			wantErr: regexp.MustCompile("tried to register topic \"[^\"]+/foo\" which was already registered"),
		},
		{
			name: "init error",

			plugins: []Plugin{&plugin{initErr: errors.New("foo")}},
			wantErr: regexp.MustCompile("foo"),
		},
		{
			name: "ok",

			plugins: []Plugin{
				&plugin{topic: "foo"}, &plugin{topic: "bar"},
				&plugin{suffix: "foo"}, &plugin{suffix: "bar"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mqttfake.NewClient()
			_, err := NewCronJob("id", mqtt.NewClientForTesting(c), CronJobPlugins(tc.plugins...))
			if err != nil {
				if tc.wantErr == nil {
					t.Errorf("NewCronJob unexpectedly failed with %v", err)
				} else if !tc.wantErr.Match([]byte(err.Error())) {
					t.Errorf("NewCronJob failed with %v, wanted to match %q", err, tc.wantErr)
				}
			} else if err == nil && tc.wantErr != nil {
				t.Errorf("NewCronJob did not fail, but expected error message to match %q", tc.wantErr)
			}
		})
	}
}

func TestPluginPublish(t *testing.T) {
	for _, tc := range []struct {
		name string

		plugins []Plugin
		wantErr *regexp.Regexp
	}{
		{
			name: "wrong suffix",

			plugins: []Plugin{&plugin{suffix: "foo", createPublishTopic: "bar", createPublishPayload: "baz"}},
			wantErr: regexp.MustCompile(regexp.QuoteMeta("did not register topic bar")),
		},
		{
			name: "wrong topic",

			plugins: []Plugin{&plugin{topic: "foo", createPublishTopic: "bar", createPublishPayload: "baz"}},
			wantErr: regexp.MustCompile(regexp.QuoteMeta("did not register topic bar")),
		},
		{
			name:    "wrong retain",
			plugins: []Plugin{&plugin{topic: "foo", topicRetain: mqtt.DoNotRetain, createPublishPayload: "baz", createPublishRetain: mqtt.Retain}},
			wantErr: regexp.MustCompile(regexp.QuoteMeta("did not register topic foo for mqtt.Retain")),
		},
		{
			name: "ok",
			plugins: []Plugin{
				&plugin{topic: "do_not_retain", topicRetain: mqtt.DoNotRetain, createPublishPayload: "baz", createPublishRetain: mqtt.DoNotRetain},
				&plugin{topic: "do_not_retain_on_retain", topicRetain: mqtt.Retain, createPublishPayload: "baz", createPublishRetain: mqtt.DoNotRetain},
				&plugin{topic: "retain", topicRetain: mqtt.Retain, createPublishPayload: "baz", createPublishRetain: mqtt.Retain},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mqttfake.NewClient()
			_, err := NewCronJob("id", mqtt.NewClientForTesting(c), CronJobPlugins(tc.plugins...))
			if err != nil {
				if tc.wantErr == nil {
					t.Errorf("NewCronJob unexpectedly failed with %v", err)
				} else if !tc.wantErr.Match([]byte(err.Error())) {
					t.Errorf("NewCronJob failed with %v, wanted to match %q", err, tc.wantErr)
				}
			} else if err == nil && tc.wantErr != nil {
				t.Errorf("NewCronJob did not fail, but expected error message to match %q", tc.wantErr)
			}
		})
	}
}

func TestSameCron2mqttCommand(t *testing.T) {
	for _, tc := range []struct {
		name string

		c1 string
		c2 string

		want bool
	}{
		{
			name: "simple",
			c1:   "cron2mqtt exec abcd echo true",
			c2:   "cron2mqtt exec abcd echo true",
			want: true,
		},
		{
			name: "different cron2mqtt path",
			c1:   "cron2mqtt exec abcd echo true",
			c2:   "/usr/bin/cron2mqtt exec abcd echo true",
			want: true,
		},
		{
			name: "different flags",
			c1:   "cron2mqtt exec abcd echo true",
			c2:   "cron2mqtt exec -vvv abcd echo true",
			want: false,
		},
		{
			name: "different commands",
			c1:   "cron2mqtt exec abcd echo true",
			c2:   "cron2mqtt exec abcd echo false",
			want: false,
		},
		{
			name: "significantly different commands",
			c1:   "cron2mqtt exec abcd echo true",
			c2:   "cron2mqtt exec abcd echo",
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c1 := cron.NewCommand(tc.c1)
			c2 := cron.NewCommand(tc.c2)
			if got := sameCron2mqttCommand(c1, c2); got != tc.want {
				t.Errorf("sameCron2mqttCommand(%q, %q) = %t, want %t", tc.c1, tc.c2, got, tc.want)
			}
		})
	}
}

func TestDiscoverLocalCronjobIfPossible(t *testing.T) {
	// TODO
}

func TestUnpublish(t *testing.T) {
	// TODO
}

type plugin struct {
	NopPlugin

	suffix       string
	manualSuffix string // Like suffix, but manually figure out the prefix, and register it as a topic.
	topic        string
	topicRetain  mqtt.RetainMode
	initErr      error

	suffixTopic string

	createPublishTopic   string
	createPublishPayload string
	createPublishRetain  mqtt.RetainMode
}

func (p *plugin) Init(cj *CronJob, reg TopicRegister) error {
	if p.suffix != "" {
		p.suffixTopic = reg.RegisterSuffix(p.suffix)
	}
	if p.manualSuffix != "" {
		reg.RegisterTopic(cj.topicPrefix+"/"+p.manualSuffix, p.topicRetain)
	}
	if p.topic != "" {
		reg.RegisterTopic(p.topic, p.topicRetain)
	}
	return p.initErr
}

func (p *plugin) OnCreate(cj *CronJob, pub Publisher) error {
	if p.createPublishPayload != "" {
		t := firstNonEmpty(p.createPublishTopic, p.suffixTopic, p.topic)
		return pub.Publish(t, mqtt.QoSExactlyOnce, p.createPublishRetain, p.createPublishPayload)
	}

	return nil
}

func firstNonEmpty(s ...string) string {
	for _, s := range s {
		if s != "" {
			return s
		}
	}

	return ""
}
