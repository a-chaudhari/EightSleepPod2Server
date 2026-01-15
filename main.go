package main

import "EightSleepServer/LogBlackHole"

func main() {
	// start the fake logging server
	logBlackHole := LogBlackHole.LogBlackHole{}
	go logBlackHole.StartServer()

	// block forever
	select {}
}
