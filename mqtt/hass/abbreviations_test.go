package hass

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMinimize(t *testing.T) {
	m := map[string]interface{}{
		// "foo" doesn't have an abbrevation.
		"foo": "bar",

		// "action_topic" and "command_topic" both have abbrevations.
		"action_topic": "abc",
		"command_topic": map[string]string{
			// ...it shouldn't minimize recursively.
			"action_topic": "never",
		},

		// ...except for "device" config.
		"device": map[string]string{
			"manufacturer": "you",
		},
		"manufacturer": "me",

		// It shouldn't minimize if the minimized key already exists.
		"subtype":       1,
		abbr["subtype"]: 2,
	}

	minimize(m)

	want := map[string]interface{}{
		// "foo" doesn't have an abbrevation.
		"foo": "bar",

		// "action_topic" and "command_topic" both have abbrevations.
		abbr["action_topic"]: "abc",
		abbr["command_topic"]: map[string]string{
			// ...it shouldn't minimize recursively.
			"action_topic": "never",
		},

		// ...except for "device" config.
		abbr["device"]: map[string]string{
			deviceAbbr["manufacturer"]: "you",
		},
		"manufacturer": "me",

		// It shouldn't minimize if the minimized key already exists.
		"subtype":       1,
		abbr["subtype"]: 2,
	}
	if diff := cmp.Diff(want, m); diff != "" {
		t.Errorf("minimize mismatch (-want +got):\n%s", diff)
	}
}
