package SparkServer

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (c *PodConnection) connectToUnixSocket() {
	for {
		c.logger.Debug("Connecting to unix socket")
		c.processUnixSocket()
		c.logger.Debug("Disconnected from unix socket, retrying in 5 seconds...")
		// wait 5 seconds before trying to reconnect
		time.After(5 * time.Second)
	}
}

func (c *PodConnection) processUnixSocket() {
	socket, err := net.Dial("unix", c.socketPath)
	if err != nil {
		c.logger.Error("Error connecting to FrankenSocket unix socket", zap.String("socketPath", c.socketPath), zap.Error(err))
		return
	}
	defer func() {
		err := socket.Close()
		if err != nil {
			c.logger.Error("Error closing FrankenSocket unix socket", zap.Error(err))
		}
	}()

	c.logger.Info("Connected to FrankenSocket unix socket", zap.String("socketPath", c.socketPath))
	buf := make([]byte, 4096)
	for {
		n, err := socket.Read(buf)
		if err != nil {
			c.logger.Error("Error reading from FrankenSocket unix socket", zap.Error(err))
			return
		}
		data := buf[:n]
		parts := strings.Split(string(data), "\n")
		strVersion := parts[0]
		intVersion, err := strconv.Atoi(strVersion)
		if err != nil {
			c.logger.Error("Error converting data to int", zap.String("data", strVersion), zap.Error(err))
			continue
		}
		cmd := FrankenCommand(intVersion)
		switch cmd {
		case FrankenCmdDeviceStatus:
			res, err := c.GetStatus()
			if err != nil {
				c.logger.Error("Error getting pod status", zap.Error(err))
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
				c.logger.Error("Error converting left temp duration arg to int", zap.String("arg", parts[1]), zap.Error(err))
				continue
			}
			c.SetTime(arg, BedSideLeft)
			_, _ = socket.Write([]byte("ok\n\n"))
		case FrankenCmdRightTempDur:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				c.logger.Error("Error converting right temp duration arg to int", zap.String("arg", parts[1]), zap.Error(err))
				continue
			}
			c.SetTime(arg, BedSideRight)
			_, _ = socket.Write([]byte("ok\n\n"))
		case FrankenCmdTempLevelLeft:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				c.logger.Error("Error converting left temp level arg to int", zap.String("arg", parts[1]), zap.Error(err))
				continue
			}
			c.SetLevel(arg, BedSideLeft)
			_, _ = socket.Write([]byte("ok\n\n"))

		case FrankenCmdTempLevelRight:
			arg, err := strconv.Atoi(parts[1])
			if err != nil {
				c.logger.Error("Error converting right temp level arg to int", zap.String("arg", parts[1]), zap.Error(err))
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
			c.logger.Warn("Unhandled FrankenCommand from unix socket", zap.Int("command", intVersion))
		}
	}
}
