package main

import (
	"EightSleepServer/ApiServer"
	"EightSleepServer/Common"
	"EightSleepServer/LogBlackHole"
	"EightSleepServer/SparkServer"
)

type MainContext struct {
	connectionMap       map[string]*SparkServer.ClientConnection
	connStateChangePipe chan *SparkServer.ConnectionNotification
	connRequestPipe     chan *Common.ConnectionRequest
}

func NewMainContext() *MainContext {
	return &MainContext{
		connectionMap:       make(map[string]*SparkServer.ClientConnection),
		connStateChangePipe: make(chan *SparkServer.ConnectionNotification, 10),
		connRequestPipe:     make(chan *Common.ConnectionRequest, 10),
	}
}

func (mc *MainContext) Start() {
	go mc.processStateChanges()
	go mc.processConnectionRequests()

	// start the fake logging server
	logBlackHole := LogBlackHole.LogBlackHole{}
	go logBlackHole.StartServer()

	server := SparkServer.NewServer("/home/amitchaudhari/eight_sleep_project/modified_files/server_priv_key.pem", mc.connStateChangePipe)
	go server.StartServer()

	apiServer := ApiServer.NewApiServer(mc.connRequestPipe)
	go apiServer.StartServer()

	// block forever
	select {}
}

func (mc *MainContext) processStateChanges() {
	for {
		select {
		case stateChange := <-mc.connStateChangePipe:
			if stateChange.IsConnected {
				mc.connectionMap[stateChange.DeviceId] = stateChange.Conn
			} else {
				delete(mc.connectionMap, stateChange.DeviceId)
			}
		}
	}
}

func (mc *MainContext) processConnectionRequests() {
	for {
		select {
		case connRequest := <-mc.connRequestPipe:
			if connRequest.ListAll {
				for deviceId, conn := range mc.connectionMap {
					connRequest.DeviceIds = append(connRequest.DeviceIds, deviceId)
					_ = conn // to avoid unused variable warning
				}
			} else {
				if conn, exists := mc.connectionMap[connRequest.DeviceId]; exists {
					connRequest.Connection = conn
				} else {
					connRequest.Connection = nil
				}
			}
			close(connRequest.IsReady)
		}
	}
}

func main() {
	ctx := NewMainContext()
	ctx.Start() // blocking call
}
