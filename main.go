package main

import (
	"EightSleepServer/LogBlackHole"
	"EightSleepServer/SparkServer"
)

func main() {
	// start the fake logging server
	logBlackHole := LogBlackHole.LogBlackHole{}
	go logBlackHole.StartServer()

	server := SparkServer.NewServer("/home/amitchaudhari/eight_sleep_project/modified_files/server_priv_key.pem")
	go server.StartServer()

	// block forever
	select {}
}
