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

	ExpireAfter *seconds `json:"expire_after,omitempty"`
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

type sensor struct {
	DeviceClass sensorDeviceClass `json:"device_class"`
	common

	UnitOfMeasurement unit       `json:"unit_of_measurement"`
	StateClass        stateClass `json:"state_class"`
}

func (s sensor) MarshalJSON() ([]byte, error) {
	type alias sensor
	return marshalAbbreviatedJSON(alias(s))
}
func (s *sensor) UnmarshalJSON(b []byte) error {
	type alias sensor
	return unmarshalAbbreviatedJSON(b, (*alias)(s))
}

type binarySensorDeviceClass string
type sensorDeviceClass string
type unit string
type stateClass string

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
	sensorDeviceClasses = struct {
		apparentPower            sensorDeviceClass
		aqi                      sensorDeviceClass
		battery                  sensorDeviceClass
		carbonDioxide            sensorDeviceClass
		carbonMonoxide           sensorDeviceClass
		current                  sensorDeviceClass
		date                     sensorDeviceClass
		duration                 sensorDeviceClass
		energy                   sensorDeviceClass
		frequency                sensorDeviceClass
		gas                      sensorDeviceClass
		humidity                 sensorDeviceClass
		illuminance              sensorDeviceClass
		monetary                 sensorDeviceClass
		nitrogenDioxide          sensorDeviceClass
		nitrogenMonoxide         sensorDeviceClass
		nitrousOxide             sensorDeviceClass
		ozone                    sensorDeviceClass
		pm1                      sensorDeviceClass
		pm10                     sensorDeviceClass
		pm25                     sensorDeviceClass
		powerFactor              sensorDeviceClass
		power                    sensorDeviceClass
		pressure                 sensorDeviceClass
		reactivePower            sensorDeviceClass
		signalStrength           sensorDeviceClass
		sulphurDioxide           sensorDeviceClass
		temperature              sensorDeviceClass
		timestamp                sensorDeviceClass
		volatileOrganicCompounds sensorDeviceClass
		voltage                  sensorDeviceClass
	}{
		apparentPower:            "apparent_power",
		aqi:                      "aqi",
		battery:                  "battery",
		carbonDioxide:            "carbon_dioxide",
		carbonMonoxide:           "carbon_monoxide",
		current:                  "current",
		date:                     "date",
		duration:                 "duration",
		energy:                   "energy",
		frequency:                "frequency",
		gas:                      "gas",
		humidity:                 "humidity",
		illuminance:              "illuminance",
		monetary:                 "monetary",
		nitrogenDioxide:          "nitrogen_dioxide",
		nitrogenMonoxide:         "nitrogen_monoxide",
		nitrousOxide:             "nitrous_oxide",
		ozone:                    "ozone",
		pm1:                      "pm1",
		pm10:                     "pm10",
		pm25:                     "pm25",
		powerFactor:              "power_factor",
		power:                    "power",
		pressure:                 "pressure",
		reactivePower:            "reactive_power",
		signalStrength:           "signal_strength",
		sulphurDioxide:           "sulphur_dioxide",
		temperature:              "temperature",
		timestamp:                "timestamp",
		volatileOrganicCompounds: "volatile_organic_compounds",
		voltage:                  "voltage",
	}
	units = struct {
		watt         unit
		kilowatt     unit
		volt         unit
		wattHour     unit
		kilowattHour unit
		ampere       unit
		voltAmpere   unit

		degree unit

		euro   unit
		dollar unit
		cent   unit

		celsuis   unit
		farenheit unit
		kelvin    unit

		microseconds unit
		milliseconds unit
		seconds      unit
		minutes      unit
		hours        unit
		days         unit
		weeks        unit
		months       unit
		years        unit

		millimeters unit
		centimeters unit
		meters      unit
		kilometers  unit

		inches unit
		feet   unit
		yards  unit
		miles  unit

		liters      unit
		milliliters unit
		cubicMeters unit
		cubicFeet   unit

		gallons     unit
		fluidOunces unit

		squareMeters unit

		grams      unit
		kilograms  unit
		milligrams unit
		micrograms unit

		ounces unit
		pounds unit

		percentage unit

		millimetersPerDay unit
		inchesPerDay      unit
		metersPerSecond   unit
		inchesPerHour     unit
		kilometersPerHour unit
		milesPerHour      unit

		bits               unit
		kilobits           unit
		megabits           unit
		gigabits           unit
		bytes              unit
		kilobytes          unit
		megabytes          unit
		gigabytes          unit
		terabytes          unit
		petabytes          unit
		exabytes           unit
		zettabytes         unit
		yottabytes         unit
		kibibytes          unit
		mebibytes          unit
		gibibytes          unit
		tebibytes          unit
		pebibytes          unit
		exbibytes          unit
		zebibytes          unit
		yobibytes          unit
		bitsPerSecond      unit
		kilobitsPerSecond  unit
		megabitsPerSecond  unit
		gigabitsPerSecond  unit
		bytesPerSecond     unit
		kilobytesPerSecond unit
		megabytesPerSecond unit
		gigabytesPerSecond unit
		kibibytesPerSecond unit
		mebibytesPerSecond unit
		gibibytesPerSecond unit
	}{
		watt:         "W",
		kilowatt:     "kW",
		volt:         "V",
		wattHour:     "Wh",
		kilowattHour: "kWh",
		ampere:       "A",
		voltAmpere:   "VA",

		degree: "°",

		euro:   "€",
		dollar: "$",
		cent:   "¢",

		celsuis:   "°C",
		farenheit: "°F",
		kelvin:    "K",

		microseconds: "μs",
		milliseconds: "ms",
		seconds:      "s",
		minutes:      "min",
		hours:        "h",
		days:         "d",
		weeks:        "w",
		months:       "m",
		years:        "y",

		millimeters: "mm",
		centimeters: "cm",
		meters:      "m",
		kilometers:  "km",

		inches: "in",
		feet:   "ft",
		yards:  "yd",
		miles:  "mi",

		liters:      "L",
		milliliters: "mL",
		cubicMeters: "m³",
		cubicFeet:   "ft³",

		gallons:     "gal",
		fluidOunces: "fl. oz.",

		squareMeters: "m²",

		grams:      "g",
		kilograms:  "kg",
		milligrams: "mg",
		micrograms: "µg",

		ounces: "oz",
		pounds: "lb",

		percentage: "%",

		millimetersPerDay: "mm/d",
		inchesPerDay:      "in/d",
		metersPerSecond:   "m/s",
		inchesPerHour:     "in/h",
		kilometersPerHour: "km/h",
		milesPerHour:      "mi/h",

		bits:               "bit",
		kilobits:           "kbit",
		megabits:           "Mbit",
		gigabits:           "Gbit",
		bytes:              "B",
		kilobytes:          "kB",
		megabytes:          "MB",
		gigabytes:          "GB",
		terabytes:          "TB",
		petabytes:          "PB",
		exabytes:           "EB",
		zettabytes:         "ZB",
		yottabytes:         "YB",
		kibibytes:          "KiB",
		mebibytes:          "MiB",
		gibibytes:          "GiB",
		tebibytes:          "TiB",
		pebibytes:          "PiB",
		exbibytes:          "EiB",
		zebibytes:          "ZiB",
		yobibytes:          "YiB",
		bitsPerSecond:      "bit/s",
		kilobitsPerSecond:  "kbit/s",
		megabitsPerSecond:  "mbit/s",
		gigabitsPerSecond:  "gbit/s",
		bytesPerSecond:     "B/s",
		kilobytesPerSecond: "kB/s",
		megabytesPerSecond: "MB/s",
		gigabytesPerSecond: "GB/s",
		kibibytesPerSecond: "KiB/s",
		mebibytesPerSecond: "MiB/s",
		gibibytesPerSecond: "GiB/s",
	}
	stateClasses = struct {
		measurement     stateClass
		total           stateClass
		totalIncreasing stateClass
	}{
		measurement:     "measurement",
		total:           "total",
		totalIncreasing: "total_increasing",
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
