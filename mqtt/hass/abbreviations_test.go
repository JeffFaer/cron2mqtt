package hass

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAbbrevate(t *testing.T) {
	m := map[string]interface{}{
		// "foo" doesn't have an abbrevation.
		"foo": "bar",

		// "action_topic" and "command_topic" both have abbrevations.
		"action_topic": "abc",
		"command_topic": map[string]string{
			// ...it shouldn't abbreviate recursively.
			"action_topic": "never",
		},

		// ...except for "device" config.
		"device": map[string]string{
			"manufacturer": "you",
		},
		"manufacturer": "me",

		// It shouldn't abbreviate if the abbreviated key already exists.
		"subtype":       1,
		abbr["subtype"]: 2,
	}

	abbreviateConfig(m)

	want := map[string]interface{}{
		"foo": "bar",

		abbr["action_topic"]: "abc",
		abbr["command_topic"]: map[string]string{
			"action_topic": "never",
		},

		abbr["device"]: map[string]string{
			deviceAbbr["manufacturer"]: "you",
		},
		"manufacturer": "me",

		"subtype":       1,
		abbr["subtype"]: 2,
	}
	if diff := cmp.Diff(want, m); diff != "" {
		t.Errorf("abbreivate mismatch (-want +got):\n%s", diff)
	}
}
