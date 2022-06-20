package hass

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/JeffreyFalgout/cron2mqtt/new"
)

func TestBinarySensor(t *testing.T) {
	c := binarySensor{
		common: common{
			BaseTopic:       "baseTopic",
			StateTopic:      "stateTopic",
			ValueTemplate:   "valueTemplate",
			AttributesTopic: "attributesTopic",

			Device: deviceConfig{
				Name:        "deviceConfigName",
				Identifiers: []string{"deviceConfigIdentifier"},
			},
			UniqueID: "uniqueID",
			ObjectID: "objectID",
			Name:     "name",

			Icon: "icon",

			ExpireAfter: new.Ptr(seconds(70*time.Second + 5*time.Millisecond)),
		},

		DeviceClass: binarySensorDeviceClasses.problem,

		PayloadOn:  "payloadOn",
		PayloadOff: "payloadOff",
	}

	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Could not marshal config: %s", err)
	}

	var c2 binarySensor
	if err := json.Unmarshal(b, &c2); err != nil {
		t.Fatalf("Could not unmarshal config: %s", err)
	}

	if diff := cmp.Diff(c, c2, cmpopts.IgnoreFields(binarySensor{}, "common", "ExpireAfter")); diff != "" {
		t.Errorf("Config did not roundtrip (-want +got):\n%s", diff)
	}

	if c.ExpireAfter == c2.ExpireAfter {
		t.Errorf("Expected ExpireAfter to be different, but it was the same. got %s, want %s", (*time.Duration)(c2.ExpireAfter), (*time.Duration)(c.ExpireAfter))
	} else if time.Duration(*c.ExpireAfter).Truncate(time.Second) != time.Duration(*c2.ExpireAfter) {
		t.Errorf("Expected ExpireAfter to be truncated to seconds, but it wasn't. got %s, want %s", (*time.Duration)(c2.ExpireAfter), (*time.Duration)(c.ExpireAfter))
	}
}
