package SparkServer

import (
	"encoding/hex"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
)

type BedStatus struct {
	HeatTime        int `json:"heat_time"`
	HeatLevel       int `json:"heat_level"`
	TargetHeatLevel int `json:"target_heat_level"`
}

type BedSide int

const (
	BedSideLeft  BedSide = 0
	BedSideRight BedSide = 1
)

type PodStatus struct {
	Priming    bool `json:"priming"`
	WaterLevel bool `json:"water_level"`
	Updating   bool `json:"updating"`

	LeftBed  BedStatus `json:"left_side"`
	RightBed BedStatus `json:"right_side"`

	SensorLabel    string `json:"sensor_label"`
	Ssid           string `json:"ssid"`
	HubInfo        string `json:"hub_info"`
	MacAddress     string `json:"mac_address"`
	IpAddress      string `json:"ip_address"`
	SignalStrength string `json:"signal_strength"`
	Settings       string `json:"settings"`
}

func (c *PodConnection) GetStatus() (PodStatus, error) {
	status := PodStatus{
		LeftBed:  BedStatus{},
		RightBed: BedStatus{},
	}

	heatLevelLeft := getData(c, "heatLevelL")

	hlL, err := strconv.Atoi(string(heatLevelLeft[:]))
	if err != nil {
		println("Error parsing heat level left:", err)
	} else {
		status.LeftBed.HeatLevel = hlL
	}

	heatLevelRight := getData(c, "heatLevelR")
	hlR, err := strconv.Atoi(string(heatLevelRight[:]))
	if err != nil {
		println("Error parsing heat level right:", err)
	} else {
		status.RightBed.HeatLevel = hlR
	}

	targetHeatLevelLeft := getData(c, "tgHeatLevelL")
	thlL, err := strconv.Atoi(string(targetHeatLevelLeft[:]))
	if err != nil {
		println("Error parsing target heat level left:", err)
	} else {
		status.LeftBed.TargetHeatLevel = thlL
	}

	targetHeatLevelRight := getData(c, "tgHeatLevelR")
	thlR, err := strconv.Atoi(string(targetHeatLevelRight[:]))
	if err != nil {
		println("Error parsing target heat level right:", err)
	} else {
		status.RightBed.TargetHeatLevel = thlR
	}

	heatTimeLeft := getData(c, "heatTimeL")
	htL, err := strconv.Atoi(string(heatTimeLeft[:]))
	if err != nil {
		println("Error parsing heat time left:", err)
	} else {
		status.LeftBed.HeatTime = htL
	}
	heatTimeRight := getData(c, "heatTimeR")
	htR, err := strconv.Atoi(string(heatTimeRight[:]))
	if err != nil {
		println("Error parsing heat time right:", err)
	} else {
		status.RightBed.HeatTime = htR
	}

	priming := getData(c, "priming")
	if string(priming[:]) == "true" {
		status.Priming = true
	} else {
		status.Priming = false
	}

	waterLevel := getData(c, "waterLevel")
	if string(waterLevel[:]) == "true" {
		status.WaterLevel = true
	} else {
		status.WaterLevel = false
	}
	updating := getData(c, "updating")
	if string(updating[:]) == "true" {
		status.Updating = true
	} else {
		status.Updating = false
	}

	sensorLabel := getData(c, "sensorLabel")
	status.SensorLabel = unwrapQuotes(string(sensorLabel[:]))

	ssid := getData(c, "ssid")
	status.Ssid = unwrapQuotes(string(ssid[:]))

	hubInfo := getData(c, "hubInfo")
	status.HubInfo = unwrapQuotes(string(hubInfo[:]))

	macAddress := getData(c, "macAddr")
	status.MacAddress = unwrapQuotes(string(macAddress[:]))

	ipAddress := getData(c, "ipaddr")
	status.IpAddress = unwrapQuotes(string(ipAddress[:]))

	signalStrength := getData(c, "sigstr")
	status.SignalStrength = unwrapQuotes(string(signalStrength[:]))

	settings := getData(c, "settings")
	status.Settings = unwrapQuotes(string(settings[:]))

	return status, nil
}

func getData(c *PodConnection, verb string) []byte {
	msg := message.Message{
		Options: message.Options{{ID: message.URIPath, Value: []byte("v")}, {ID: message.URIPath, Value: []byte(verb)}},
		Code:    codes.GET,
		Type:    message.Confirmable,
	}
	podReq := NewPodRequest(&msg)
	c.RequestPipe <- podReq
	<-podReq.Ready
	return podReq.Response
}

func unwrapQuotes(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	return s
}

func (c *PodConnection) SetTime(seconds int, side BedSide) {
	path := "leftHeat"
	if side == BedSideRight {
		path = "rightHeat"
	}
	value := strconv.Itoa(seconds)
	c.SetValue(path, value)
}

func (c *PodConnection) SetLevel(level int, side BedSide) {
	path := "leftLevel"
	if side == BedSideRight {
		path = "rightLevel"
	}
	value := strconv.Itoa(level)
	c.SetValue(path, value)
}

func (c *PodConnection) SetValue(path string, value string) {
	msg := message.Message{
		Options: message.Options{
			{ID: message.URIPath, Value: []byte("f")},
			{ID: message.URIPath, Value: []byte(path)},
			{ID: message.URIQuery, Value: []byte(value)},
		},
		Code: codes.POST,
		Type: message.Confirmable,
	}
	podReq := NewPodRequest(&msg)
	c.RequestPipe <- podReq
	<-podReq.Ready
	println("done setting", path, "to", value)
}

type AlarmParams struct {
	Intensity int    `cbor:"pl"`
	Duration  int    `cbor:"du"`
	Time      uint64 `cbor:"tt"`
	Pattern   string `cbor:"pi"`
}

func (c *PodConnection) SetAlarm(side BedSide, input string) {
	// need to verify the pattern field, pod 2 has double or single, other versions have double/rise
	// so we need to swap any "rise" to "single" or it'll error out
	data, err := hex.DecodeString(input)
	if err != nil {
		println("Error decoding alarm params hex:", err)
		return
	}
	var alarmParams AlarmParams
	err = cbor.Unmarshal(data, &alarmParams)
	if err != nil {
		println("Error unmarshalling alarm params:", err)
		return
	}

	if alarmParams.Pattern == "rise" {
		alarmParams.Pattern = "single"
	}

	marshalled, err := cbor.Marshal(alarmParams)
	if err != nil {
		println("Error marshalling alarm params:", err)
		return
	}

	path := "alarmL"
	if side == BedSideRight {
		path = "alarmR"
	}

	hexStr := hex.EncodeToString(marshalled)

	c.SetValue(path, hexStr)
}
