package main

import (
	"EightSleepServer/LogServer"
	"EightSleepServer/SparkServer"
	"os"
	"strconv"
)

func main() {
	println("Starting server...")
	// start the logging server
	logServer := LogServer.LogServer{}

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./logs"
	}
	logPort := os.Getenv("LOG_PORT")
	if logPort == "" {
		logPort = "1337"
	}
	logPortInt, err := strconv.Atoi(logPort)
	if err != nil {
		panic(err)
	}
	logSaveFiles := os.Getenv("LOG_SAVE_FILES")
	logSaveBool := true
	if logSaveFiles == "true" {
		logSaveBool = true
	}

	go logServer.StartServer(logSaveBool, logPath, logPortInt)

	keyPath := os.Getenv("KEY_PATH")
	socketPath := os.Getenv("SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/deviceinfo/dac.sock"
	}

	sparkPort := os.Getenv("SPARK_PORT")
	if sparkPort == "" {
		sparkPort = "5683"
	}
	sparkPortInt, err := strconv.Atoi(sparkPort)
	if err != nil {
		panic(err)
	}

	server := SparkServer.NewServer(
		keyPath,
		sparkPortInt,
		socketPath)
	go server.StartServer()

	// block forever
	select {}
}
