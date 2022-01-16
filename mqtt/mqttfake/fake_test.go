package mqttfake

import (
	"sync"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/go-cmp/cmp"
)

func TestSubscribe(t *testing.T) {
	topic := "foo/bar"
	c := NewClient()

	var mut sync.Mutex
	var got []string
	if tok := c.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
		mut.Lock()
		defer mut.Unlock()
		got = append(got, string(m.Payload()))
	}); tok.Wait() && tok.Error() != nil {
		t.Fatalf("Could not subscribe: %s", tok.Error())
	}

	if tok := c.Publish("foo/bar", 0, false, "hello"); tok.Wait() && tok.Error() != nil {
		t.Errorf("Could not publish %q: %s", "hello", tok.Error())
	}
	if tok := c.Publish("foo/bar", 0, false, "world"); tok.Wait() && tok.Error() != nil {
		t.Errorf("Could not publish %q: %s", "world", tok.Error())
	}

	if diff := cmp.Diff([]string{"hello", "world"}, got); diff != "" {
		t.Errorf("Subscribe did not receive expected messages (-want +got):\n%s", diff)
	}
}
