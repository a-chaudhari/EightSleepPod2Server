package Common

import "EightSleepServer/SparkServer"

type IPCRequest struct {
	DeviceId    string
	RequestType RequestType
	Response    []byte
	IsReady     chan bool
}

type BedStatus struct {
	Priming    bool `json:"priming"`
	WaterLevel bool `json:"water_level"`
	Updating   bool `json:"updating"`

	HeatLevelLeft       int `json:"heat_level_left"`
	TargetHeatLevelLeft int `json:"target_heat_level_left"`
	HeatTimeLeft        int `json:"heat_time_left"`

	HeatLevelRight       int `json:"heat_level_right"`
	TargetHeatLevelRight int `json:"target_heat_level_right"`
	HeatTimeRight        int `json:"heat_time_right"`

	SensorLabel    string `json:"sensor_label"`
	Ssid           string `json:"ssid"`
	HubInfo        string `json:"hub_info"`
	MacAddress     string `json:"mac_address"`
	IpAddress      string `json:"ip_address"`
	SignalStrength string `json:"signal_strength"`
	Settings       string `json:"settings"`
}

// ConnectionRequest doubles as both list all and get connection.  ListAll bool defines mode.  assume variables will be empty when not used.
type ConnectionRequest struct {
	ListAll   bool
	DeviceIds []string
	IsReady   chan bool

	DeviceId   string
	Connection *SparkServer.ClientConnection
}
