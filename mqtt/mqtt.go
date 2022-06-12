package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/JeffreyFalgout/cron2mqtt/logutil"
)

// Config configures how a Client connects to an MQTT broker.
type Config struct {
	Broker     string
	Username   string
	Password   string
	ServerName string `mapstructure:"server_name,omitempty"`
}

// QoS represents the different MQTT QoS levels.
type QoS byte

const (
	QoSAtMostOnce QoS = iota
	QoSAtLeastOnce
	QoSExactlyOnce
)

type RetainMode bool

const (
	Retain      RetainMode = true
	DoNotRetain RetainMode = false
)

func (m RetainMode) String() string {
	switch m {
	case Retain:
		return "Retain"
	case DoNotRetain:
		return "DoNotRetain"
	}
	panic(fmt.Errorf("unknown RetainMode: %t", m))
}

type Message = mqtt.Message

// Client is an MQTT client.
type Client struct {
	c mqtt.Client
}

// NewClient constructs a new MQTT client and connects it to the broker.
func NewClient(c Config) (*Client, error) {
	defer logutil.StartTimerLogger(log.With().Str("broker", c.Broker).Logger(), zerolog.DebugLevel, "Connecting to MQTT broker").Stop()

	opts := mqtt.NewClientOptions().
		SetClientID("cron-mqtt").
		SetOrderMatters(false).
		AddBroker(c.Broker).
		SetUsername(c.Username).
		SetPassword(c.Password)
	if c.ServerName != "" {
		opts.SetTLSConfig(&tls.Config{
			ServerName: c.ServerName,
		})
	}

	cl := Client{mqtt.NewClient(opts)}
	if t := cl.c.Connect(); t.Wait() && t.Error() != nil {
		return nil, fmt.Errorf("could not connect to broker: %w", t.Error())
	}

	return &cl, nil
}

func NewClientForTesting(c mqtt.Client) *Client {
	return &Client{c}
}

// Publish publishes the given payload on the given topic on the connected broker.
func (c *Client) Publish(topic string, qos QoS, retain RetainMode, payload interface{}) error {
	payloadHook := logutil.FuncOnce(func(e *zerolog.Event) {
		var b []byte
		switch p := payload.(type) {
		case string:
			b = []byte(p)
		case []byte:
			b = []byte(p)
		default:
			b = []byte(fmt.Sprintf("%s", p))
		}

		if json.Valid(b) {
			e.RawJSON("payload", b)
		} else {
			e.Str("payload", string(b))
		}
	})
	payloadHook = func(h logutil.FuncHook) logutil.FuncHook {
		return func(e *zerolog.Event) {
			if log.Logger.GetLevel() <= zerolog.TraceLevel {
				h(e)
			}
		}
	}(payloadHook)
	defer logutil.StartTimerLogger(log.With().Str("topic", topic).Bool("retained", bool(retain)).Logger().Hook(payloadHook), zerolog.DebugLevel, "Publishing message to MQTT topic").Stop()
	t := c.c.Publish(topic, byte(qos), bool(retain), payload)
	t.Wait()
	return t.Error()
}

func (c *Client) Subscribe(ctx context.Context, topic string, qos QoS, messages chan<- Message) error {
	start := zerolog.TimestampFunc()
	log := log.Ctx(ctx).Hook(logutil.FuncHook(func(e *zerolog.Event) { e.TimeDiff("offset", zerolog.TimestampFunc(), start) })).With().Str("topic", topic).Logger()
	log.Debug().Msg("Subscribing to MQTT topic")

	var closeOnce sync.Once
	unsub := func() error {
		log.Debug().Msg("Unsubscribing from MQTT topic")
		if !c.c.IsConnected() {
			log.Debug().Msg("Client is not connected")
			closeOnce.Do(func() { close(messages) })
			return nil
		}

		if t := c.c.Unsubscribe(topic); t.Wait() && t.Error() != nil {
			return t.Error()
		}
		closeOnce.Do(func() { close(messages) })
		log.Debug().Msg("Unsubscribed from MQTT topic")
		return nil
	}

	var i int32
	if t := c.c.Subscribe(topic, byte(qos), func(c mqtt.Client, m mqtt.Message) {
		i := atomic.AddInt32(&i, 1)
		log := log.Debug().
			Int32("n", i).
			Uint16("id", m.MessageID()).
			Bool("retained", m.Retained()).
			Func(func(e *zerolog.Event) {
				if m.Topic() != topic {
					e.Str("message_topic", m.Topic())
				}
			}).
			Func(func(e *zerolog.Event) {
				if log.GetLevel() <= zerolog.TraceLevel {
					if p := m.Payload(); json.Valid(p) {
						e.RawJSON("payload", p)
					} else {
						e.Str("payload", fmt.Sprintf("%s", string(p)))
					}
				}
			})
		select {
		case <-ctx.Done():
			log.Msg("Dropping message")
		case messages <- m:
			log.Msg("Received message")
		}
	}); t.Wait() && t.Error() != nil {
		close(messages)
		return t.Error()
	}

	go func() {
		<-ctx.Done()
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for ; true; <-t.C {
			if err := unsub(); err != nil {
				log.Warn().Err(err).Msg("Unable to unsubscribe from MQTT topic")
				continue
			}

			break
		}
	}()

	return nil
}

// Close disconnects this client from the broker.
func (c *Client) Close(quiesce uint) {
	opts := c.c.OptionsReader()
	defer logutil.StartTimerLogger(log.With().Stringer("broker", opts.Servers()[0]).Logger(), zerolog.DebugLevel, "Disconnecting from MQTT broker").Stop()
	c.c.Disconnect(quiesce)
}
