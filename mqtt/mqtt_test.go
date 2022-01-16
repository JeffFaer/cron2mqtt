package mqtt

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/JeffreyFalgout/cron2mqtt/mqtt/mqttfake"
)

func TestSubscribe(t *testing.T) {
	topic := "topic"
	c := Client{mqttfake.NewClient()}

	for _, tc := range []struct {
		name string

		numMessages int
		numWaitFor  int

		check func(*testing.T, map[int]bool)
	}{
		{
			name: "straightforward",

			numMessages: 10,
			numWaitFor:  10,

			check: func(t *testing.T, got map[int]bool) {
				for i := 0; i < 10; i++ {
					if !got[i] {
						t.Errorf("%d was not received", i)
					}
				}
			},
		},
		{
			name: "cancelled part way through",

			numMessages: 10,
			numWaitFor:  5,

			check: func(t *testing.T, got map[int]bool) {
				// We use an unbuffered messages channel, so we should expact at most one more message than we wanted.
				if n := len(got); n != 5 && n != 6 {
					t.Errorf("Got %d messages, want 5", n)
				}
				for i := range got {
					if 0 <= i && i < 10 {
						continue
					}
					t.Errorf("Got unexpected message %d", i)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, canc := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer canc()
			ch := make(chan Message)
			if err := c.Subscribe(ctx, topic, QoSExactlyOnce, ch); err != nil {
				t.Fatalf("Subscribe error: %s", err)
			}

			for i := 0; i < tc.numMessages; i++ {
				i := i
				go func() {
					c.Publish(topic, QoSExactlyOnce, Retain, fmt.Sprintf("%d", i))
				}()
			}

			ms := messages(ctx, ch, tc.numWaitFor, canc)
			got := make(map[int]bool)
			for _, m := range ms {
				i, err := strconv.Atoi(string(m.Payload()))
				if err != nil {
					t.Errorf("Got unexpected payload: %v", m.Payload())
					continue
				}

				got[i] = true
			}

			tc.check(t, got)
		})
	}
}

func messages(ctx context.Context, ms <-chan Message, n int, canc func()) []Message {
	var got []Message
	for {
		select {
		case m, ok := <-ms:
			if !ok {
				return got
			}
			got = append(got, m)
			if len(got) == n {
				canc()
			}
		case <-ctx.Done():
			return got
		}
	}
}
