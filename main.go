package main

import (
	"EightSleepServer/LogServer"
	"EightSleepServer/SparkServer"
	"go.uber.org/zap"
	"os"
	"strconv"
)

func main() {
	logger, _ := zap.NewProduction()
	logger.Info("Starting server...")
	// start the logging server
	logServer := LogServer.LogServer{}

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./logs"
	}
	logPort := os.Getenv("LOG_PORT")
	if logPort == "" {
		logPort = "1338"
	}
	logPortInt, err := strconv.Atoi(logPort)
	if err != nil {
		logger.Panic("Invalid LOG_PORT", zap.String("LOG_PORT", logPort), zap.Error(err))
	}
	logSaveFiles := os.Getenv("LOG_SAVE_FILES")
	logSaveBool := false
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
		logger.Panic("Invalid SPARK_PORT", zap.String("SPARK_PORT", sparkPort), zap.Error(err))
	}

	server := SparkServer.NewServer(
		keyPath,
		sparkPortInt,
		socketPath)
	go server.StartServer()

	// block forever
	select {}
}
