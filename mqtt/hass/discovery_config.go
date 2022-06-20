package hass

import (
	"encoding/json"
	"time"
)

type common struct {
	BaseTopic       string `json:"~"`
	StateTopic      string `json:"state_topic"`
	ValueTemplate   string `json:"value_template"`
	AttributesTopic string `json:"json_attributes_topic"`

	Device   deviceConfig `json:"device"`
	UniqueID string       `json:"unique_id"`
	ObjectID string       `json:"object_id"`
	Name     string       `json:"name"`

	DeviceClass string `json:"device_class"`
	Icon        string `json:"icon"`

	ExpireAfter *seconds `json:"expire_after"`
}

// seconds is a time.Duration with only second granularity when marshalling to JSON.
type seconds time.Duration

func (s seconds) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(time.Duration(s).Seconds()))
}
func (s *seconds) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*s = seconds(time.Duration(i) * time.Second)
	return nil
}

type deviceConfig struct {
	Name        string   `json:"name"`
	Identifiers []string `json:"identifiers"`
}

type binarySensor struct {
	DeviceClass binarySensorDeviceClass `json:"device_class"`
	common

	PayloadOn  string `json:"payload_on"`
	PayloadOff string `json:"payload_off"`
}

func (s binarySensor) MarshalJSON() ([]byte, error) {
	type alias binarySensor
	return marshalAbbreviatedJSON(alias(s))
}
func (s *binarySensor) UnmarshalJSON(b []byte) error {
	type alias binarySensor
	return unmarshalAbbreviatedJSON(b, (*alias)(s))
}

type binarySensorDeviceClass string

var (
	binarySensorDeviceClasses = struct {
		battery         binarySensorDeviceClass
		batteryCharging binarySensorDeviceClass
		carbonMonoxide  binarySensorDeviceClass
		cold            binarySensorDeviceClass
		connectivity    binarySensorDeviceClass
		door            binarySensorDeviceClass
		garageDoor      binarySensorDeviceClass
		gas             binarySensorDeviceClass
		heat            binarySensorDeviceClass
		light           binarySensorDeviceClass
		lock            binarySensorDeviceClass
		moisture        binarySensorDeviceClass
		motion          binarySensorDeviceClass
		moving          binarySensorDeviceClass
		occupancy       binarySensorDeviceClass
		opening         binarySensorDeviceClass
		plug            binarySensorDeviceClass
		power           binarySensorDeviceClass
		presence        binarySensorDeviceClass
		problem         binarySensorDeviceClass
		running         binarySensorDeviceClass
		safety          binarySensorDeviceClass
		smoke           binarySensorDeviceClass
		sound           binarySensorDeviceClass
		tamper          binarySensorDeviceClass
		update          binarySensorDeviceClass
		vibration       binarySensorDeviceClass
		window          binarySensorDeviceClass
	}{
		battery:         "battery",
		batteryCharging: "battery_charging",
		carbonMonoxide:  "carbon_monoxide",
		cold:            "cold",
		connectivity:    "connectivity",
		door:            "door",
		garageDoor:      "garage_door",
		gas:             "gas",
		heat:            "heat",
		light:           "light",
		lock:            "lock",
		moisture:        "moisture",
		motion:          "motion",
		moving:          "moving",
		occupancy:       "occupancy",
		opening:         "opening",
		plug:            "plug",
		power:           "power",
		presence:        "presence",
		problem:         "problem",
		running:         "running",
		safety:          "safety",
		smoke:           "smoke",
		sound:           "sound",
		tamper:          "tamper",
		update:          "update",
		vibration:       "vibration",
		window:          "window",
	}
)

func marshalAbbreviatedJSON(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	abbreviateConfig(m)

	return json.Marshal(m)
}

func unmarshalAbbreviatedJSON(b []byte, v interface{}) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	expandConfig(m)

	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, v)
}
