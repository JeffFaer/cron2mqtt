package mqttfake

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type message struct {
	topic    string
	qos      byte
	retained bool
	payload  []byte
}

func (message) Duplicate() bool {
	return false
}
func (m message) Qos() byte {
	return m.qos
}
func (m message) Retained() bool {
	return m.retained
}
func (m message) Topic() string {
	return m.topic
}
func (message) MessageID() uint16 {
	return 0
}
func (m message) Payload() []byte {
	return m.payload
}
func (message) Ack() {}

type token struct {
	done <-chan struct{}

	mut sync.Mutex
	err error
}

var ok *token = nil

func newToken(err <-chan error) *token {
	done := make(chan struct{})
	t := token{
		done: done,
	}
	go func() {
		defer close(done)
		err := <-err

		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
	}()
	return &token{
		done: done,
	}
}

func (t *token) Wait() bool {
	if t == nil {
		return true
	}

	<-t.Done()
	return true
}
func (t *token) WaitTimeout(d time.Duration) bool {
	if t == nil {
		return true
	}

	select {
	case <-t.Done():
		return true
	case <-time.After(d):
		return false
	}
}
func (t *token) Done() <-chan struct{} {
	if t == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return t.done
}
func (t *token) Error() error {
	if t == nil {
		return nil
	}

	t.mut.Lock()
	defer t.mut.Unlock()
	return t.err
}

type Client struct {
	mut      sync.Mutex
	messages map[string][]message
	handlers map[string]mqtt.MessageHandler
}

func NewClient() *Client {
	return &Client{
		messages: make(map[string][]message),
		handlers: make(map[string]mqtt.MessageHandler),
	}
}

func (*Client) IsConnected() bool {
	return true
}
func (*Client) IsConnectionOpen() bool {
	return true
}
func (*Client) Connect() mqtt.Token {
	panic("not yet implemented")
}
func (*Client) Disconnect(quiesce uint) {
	panic("not yet implemented")
}
func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	var b []byte
	switch p := payload.(type) {
	case []byte:
		b = p
	case string:
		b = []byte(p)
	default:
		panic(fmt.Errorf("unhandled payload type %T", p))
	}
	m := message{topic, qos, retained, b}

	c.mut.Lock()
	c.messages[topic] = append(c.messages[topic], m)
	h, ok := c.handlers[topic]
	if !ok {
		c.mut.Unlock()
		return nil
	}

	err := make(chan error)
	go func() {
		defer close(err)
		h(c, m)
		c.mut.Unlock()
	}()
	return newToken(err)
}
func (c *Client) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return c.SubscribeMultiple(map[string]byte{topic: qos}, callback)
}
func (c *Client) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	c.mut.Lock()
	defer c.mut.Unlock()
	for t := range filters {
		c.handlers[t] = callback
	}
	return ok
}
func (c *Client) Unsubscribe(topics ...string) mqtt.Token {
	c.mut.Lock()
	defer c.mut.Unlock()
	for _, t := range topics {
		delete(c.handlers, t)
	}
	return ok
}
func (*Client) AddRoute(topic string, callback mqtt.MessageHandler) {
	panic("not yet implemented")
}
func (*Client) OptionsReader() mqtt.ClientOptionsReader {
	panic("not yet implemented")
}
