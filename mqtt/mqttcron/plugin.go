package mqttcron

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.uber.org/multierr"

	"github.com/JeffreyFalgout/cron2mqtt/exec"
	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
	"github.com/JeffreyFalgout/cron2mqtt/new"
)

// Plugin provides hooks to customize the CronJob's behavior.
type Plugin interface {
	Init(cj *CronJob, reg TopicRegister) error
	OnCreate(cj *CronJob, pub Publisher) error
	PublishResult(cj *CronJob, pub Publisher, res exec.Result) error
}

type NopPlugin struct{}

func (NopPlugin) Init(*CronJob, TopicRegister) error                   { return nil }
func (NopPlugin) OnCreate(*CronJob, Publisher) error                   { return nil }
func (NopPlugin) PublishResult(*CronJob, Publisher, exec.Result) error { return nil }

// TopicRegister lets Plugins declare that they would like to publish to a particular topic.
//  - Only one plugin may publish to a particular topic.
//  - You may only publish to a topic with mqtt.Retain if it is registered with mqtt.Retain. You are allowed to publish to a topic with mqtt.DoNotRetain even if it's registered with mqtt.Retain.
type TopicRegister interface {
	// RegisterSuffix registers a topic that is prefixed with the standard topic prefix for the cron job. The complete topic string is returned from this method. You may publish to this topic with mqtt.Retain if you wish.
	RegisterSuffix(suffix string) string
	RegisterTopic(topic string, retain mqtt.RetainMode)
}

type CorePlugin struct {
	DiscoveryTopic   string
	MetadataTopic    string
	ResultsTopic     string
	LastSuccessTopic string
}

var (
	ExitCodeAttributeName = loadAttributeName(results{}, "ExitCode")
	DurationAttributeName = loadAttributeName(results{}, "Duration")
)

func loadAttributeName(s any, f string) string {
	field, ok := reflect.TypeOf(s).FieldByName(f)
	if !ok {
		panic(fmt.Errorf("%T has no field %s", s, f))
	}
	return strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
}

type metadata struct {
	Schedule          string     `json:"schedule,omitempty"`
	NextExecutionTime *time.Time `json:"next_execution_time,omitempty"`
}

type results struct {
	Args      []string     `json:"args"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Duration  milliseconds `json:"duration_ms"`
	Stdout    string       `json:"stdout"`
	Stderr    string       `json:"stderr"`
	ExitCode  int          `json:"exit_code"`
}

type milliseconds time.Duration

func (m milliseconds) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(m).Milliseconds())
}
func (m *milliseconds) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*m = milliseconds(time.Duration(i) * time.Millisecond)
	return nil
}

func (p *CorePlugin) Init(cj *CronJob, reg TopicRegister) error {
	p.DiscoveryTopic = reg.RegisterSuffix("discovery")
	p.MetadataTopic = reg.RegisterSuffix("metadata")
	p.ResultsTopic = reg.RegisterSuffix("results")
	p.LastSuccessTopic = reg.RegisterSuffix("last_success")
	return nil
}

func (p *CorePlugin) OnCreate(cj *CronJob, pub Publisher) error {
	m := metadata{}
	if cj.Schedule != nil {
		m.Schedule = cj.Schedule.String()
		m.NextExecutionTime = new.Ptr(cj.Schedule.Next(time.Now()))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return MultiPublish(
		func() error { return pub.Publish(p.DiscoveryTopic, mqtt.QoSExactlyOnce, mqtt.Retain, "1") },
		func() error { return pub.Publish(p.MetadataTopic, mqtt.QoSExactlyOnce, mqtt.Retain, b) })
}

func (p *CorePlugin) PublishResult(cj *CronJob, pub Publisher, res exec.Result) error {
	results := results{
		Args:      res.Args,
		StartTime: res.Start,
		EndTime:   res.End,
		Duration:  milliseconds(res.End.Sub(res.Start)),
		Stdout:    string(res.Stdout),
		Stderr:    string(res.Stderr),
		ExitCode:  res.ExitCode,
	}
	b, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("could not marshal results: %w", err)
	}

	return MultiPublish(
		func() error { return pub.Publish(p.ResultsTopic, mqtt.QoSExactlyOnce, mqtt.DoNotRetain, b) },
		func() error {
			if res.ExitCode != 0 {
				return nil
			}
			return pub.Publish(p.LastSuccessTopic, mqtt.QoSExactlyOnce, mqtt.Retain, b)
		})
}

// MultiPublish runs all of the functions in parallel and returns a multierr for all those that failed.
// Expected to be used with parallel calls to Publisher.Publish.
func MultiPublish(fs ...func() error) error {
	errs := make(chan error, len(fs))
	var wg sync.WaitGroup
	wg.Add(len(fs))
	for _, f := range fs {
		f := f
		go func() {
			defer wg.Done()
			errs <- f()
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	var err error
	for e := range errs {
		err = multierr.Append(err, e)
	}
	return err
}
