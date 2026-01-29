package SparkServer

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func (c *PodConnection) connectToUnixSocket() {
	socket, err := net.Dial("unix", c.socketPath)
	if err != nil {
		panic(err)
	}
	defer socket.Close()

	println("Connected to FrankenSocket unix socket")
	buf := make([]byte, 4096)
	for {
		n, err := socket.Read(buf)
		if err != nil {
			println("Error reading from FrankenSocket unix socket:", err)
			return
		}
		data := buf[:n]
		parts := strings.Split(string(data), "\n")
		strVersion := parts[0]
		//fmt.Printf("%x\n", strVersion)
		intVersion, err := strconv.Atoi(strVersion)
		if err != nil {
			println("Error converting data to int:", err)
			continue
		}
		cmd := FrankenCommand(intVersion)
		switch cmd {
		case FrankenCmdDeviceStatus:
			res, err := c.GetStatus()
			if err != nil {
				println("Error getting pod status:", err)
				continue
			}
			output := fmt.Sprintf(
				"tgHeatLevelR = %d\ntgHeatLevelL = %d\nheatTimeR = %d\nheatTimeL = %d\nheatLevelR = %d\nheatLevelL = %d\nsensorLabel = %s\nwaterLevel = %t\npriming = %t\nsettings = %s\n\n",
				res.RightBed.TargetHeatLevel,
				res.LeftBed.TargetHeatLevel,
				res.RightBed.HeatTime,
				res.LeftBed.HeatTime,
				res.RightBed.HeatLevel,
				res.LeftBed.HeatLevel,
				res.SensorLabel,
				res.WaterLevel,
				res.Priming,
				res.Settings,
			)
			_, _ = socket.Write([]byte(output))

		case FrankenCmdLeftTempDur:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				println("Error converting left temp duration arg to int:", err)
				continue
			}
			c.SetTime(arg, BedSideLeft)
			_, _ = socket.Write([]byte("ok\n\n"))
		case FrankenCmdRightTempDur:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				println("Error converting right temp duration arg to int:", err)
				continue
			}
			c.SetTime(arg, BedSideRight)
			_, _ = socket.Write([]byte("ok\n\n"))
		case FrankenCmdTempLevelLeft:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				println("Error converting left temp level arg to int:", err)
				continue
			}
			c.SetLevel(arg, BedSideLeft)
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdTempLevelRight:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				println("Error converting right temp level arg to int:", err)
				continue
			}
			c.SetLevel(arg, BedSideRight)
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdPrime:
			c.SetValue("prime", "true")
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdAlarmLeft:
			c.SetAlarm(BedSideLeft, parts[1])
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdAlarmRight:
			c.SetAlarm(BedSideRight, parts[1])
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdAlarmClear:
			c.ClearAlarms()
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdSetSettings:
			c.SetValue("setsettings", parts[1])
			_, _ = socket.Write([]byte("ok\n\n"))

		default:
			println("Unhandled FrankenCommand from unix socket:", intVersion)
		}
	}
}
