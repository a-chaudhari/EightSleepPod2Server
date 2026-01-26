package SparkServer

type FrankenCommand int

const (
	FrankenCmdHello          FrankenCommand = 0
	FrankenCmdSetTemp        FrankenCommand = 1
	FrankenCmdSetAlarm       FrankenCommand = 2
	FrankenCmdAlarmLeft      FrankenCommand = 5
	FrankenCmdAlarmRight     FrankenCommand = 6
	FrankenCmdSetSettings    FrankenCommand = 8
	FrankenCmdLeftTempDur    FrankenCommand = 9
	FrankenCmdRightTempDur   FrankenCommand = 10
	FrankenCmdTempLevelLeft  FrankenCommand = 11
	FrankenCmdTempLevelRight FrankenCommand = 12
	FrankenCmdPrime          FrankenCommand = 13
	FrankenCmdDeviceStatus   FrankenCommand = 14
	FrankenCmdAlarmClear     FrankenCommand = 16
)
