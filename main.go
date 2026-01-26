package main

import (
	"EightSleepServer/LogBlackHole"
	"EightSleepServer/SparkServer"
	"os"
)

func main() {
	println("Starting server...")
	// start the fake logging server
	logBlackHole := LogBlackHole.LogBlackHole{}
	go logBlackHole.StartServer()

	keyPath := os.Getenv("KEY_PATH")
	socketPath := os.Getenv("SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/deviceinfo/dac.sock"
	}

	server := SparkServer.NewServer(
		keyPath,
		5683,
		socketPath)
	go server.StartServer()

	// block forever
	select {}
}
