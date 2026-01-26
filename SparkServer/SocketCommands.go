package SparkServer

type FrankenCommand int

const (
	FRANKEN_CMD_HELLO            FrankenCommand = 0
	FRANKEN_CMD_SET_TEMP         FrankenCommand = 1
	FRANKEN_CMD_SET_ALARM        FrankenCommand = 2
	FRANKEN_CMD_ALARM_LEFT       FrankenCommand = 5
	FRANKEN_CMD_ALARM_RIGHT      FrankenCommand = 6
	FRANKEN_CMD_SET_SETTINGS     FrankenCommand = 8
	FRANKEN_CMD_LEFT_TEMP_DUR    FrankenCommand = 9
	FRANKEN_CMD_RIGHT_TEMP_DUR   FrankenCommand = 10
	FRANKEN_CMD_TEMP_LEVEL_LEFT  FrankenCommand = 11
	FRANKEN_CMD_TEMP_LEVEL_RIGHT FrankenCommand = 12
	FRANKEN_CMD_PRIME            FrankenCommand = 13
	FRANKEN_CMD_DEVICE_STATUS    FrankenCommand = 14
	FRANKEN_CMD_ALARM_CLEAR      FrankenCommand = 16
)
