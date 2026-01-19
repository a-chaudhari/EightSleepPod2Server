package Common

import (
	"EightSleepServer/SparkServer"
	"strconv"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
)

type parseMethod int

const (
	parseMethodInt parseMethod = iota
	parseMethodFloat
)

type bedStatusField struct {
	verb   string
	method parseMethod
	dest   string
}

func GetStatus(c *SparkServer.ClientConnection) (BedStatus, error) {
	status := BedStatus{}

	heatLevelLeft := getData(c, "heatLevelL")

	hlL, err := strconv.Atoi(string(heatLevelLeft[:]))
	if err != nil {
		println("Error parsing heat level left:", err)
	} else {
		status.HeatLevelLeft = hlL
	}

	heatLevelRight := getData(c, "heatLevelR")
	hlR, err := strconv.Atoi(string(heatLevelRight[:]))
	if err != nil {
		println("Error parsing heat level right:", err)
	} else {
		status.HeatLevelRight = hlR
	}

	targetHeatLevelLeft := getData(c, "tgHeatLevelL")
	thlL, err := strconv.Atoi(string(targetHeatLevelLeft[:]))
	if err != nil {
		println("Error parsing target heat level left:", err)
	} else {
		status.TargetHeatLevelLeft = thlL
	}

	targetHeatLevelRight := getData(c, "tgHeatLevelR")
	thlR, err := strconv.Atoi(string(targetHeatLevelRight[:]))
	if err != nil {
		println("Error parsing target heat level right:", err)
	} else {
		status.TargetHeatLevelRight = thlR
	}

	heatTimeLeft := getData(c, "heatTimeL")
	htL, err := strconv.Atoi(string(heatTimeLeft[:]))
	if err != nil {
		println("Error parsing heat time left:", err)
	} else {
		status.HeatTimeLeft = htL
	}
	heatTimeRight := getData(c, "heatTimeR")
	htR, err := strconv.Atoi(string(heatTimeRight[:]))
	if err != nil {
		println("Error parsing heat time right:", err)
	} else {
		status.HeatTimeRight = htR
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
	status.SensorLabel = string(sensorLabel[:])

	ssid := getData(c, "ssid")
	status.Ssid = string(ssid[:])

	hubInfo := getData(c, "hubInfo")
	status.HubInfo = string(hubInfo[:])

	macAddress := getData(c, "macAddr")
	status.MacAddress = string(macAddress[:])

	ipAddress := getData(c, "ipaddr")
	status.IpAddress = string(ipAddress[:])

	signalStrength := getData(c, "sigstr")
	status.SignalStrength = string(signalStrength[:])

	settings := getData(c, "settings")
	status.Settings = string(settings[:])

	return status, nil
}

func getData(c *SparkServer.ClientConnection, verb string) []byte {
	msg := message.Message{
		Options: message.Options{{ID: message.URIPath, Value: []byte("v")}, {ID: message.URIPath, Value: []byte(verb)}},
		Code:    codes.GET,
		Type:    message.Confirmable,
	}
	podReq := SparkServer.NewPodRequest(&msg)
	c.RequestPipe <- podReq
	<-podReq.Ready
	return podReq.Response
}
