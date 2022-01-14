package mqtt

import (
	"crypto/tls"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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

// Client is an MQTT client.
type Client struct {
	c mqtt.Client
}

// NewClient constructs a new MQTT client and connects it to the broker.
func NewClient(c Config) (*Client, error) {
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

// Publish publishes the given payload on the given topic on the connected broker.
func (c *Client) Publish(topic string, qos QoS, retain RetainMode, payload interface{}) error {
	t := c.c.Publish(topic, byte(qos), bool(retain), payload)
	t.Wait()
	return t.Error()
}

// Close disconnects this client from the broker.
func (c *Client) Close(quiesce uint) {
	c.c.Disconnect(quiesce)
}
