package ApiServer

import (
	"EightSleepServer/Common"

	"github.com/gin-gonic/gin"
)

type ApiServer struct {
	connRequestPipe chan *Common.ConnectionRequest
}

func NewApiServer(connReqPipe chan *Common.ConnectionRequest) *ApiServer {
	return &ApiServer{
		connRequestPipe: connReqPipe,
	}
}

func (s *ApiServer) StartServer() {
	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.GET("/devices", s.listAllDevices)
	router.GET("/device/:id/status/", s.statusHandler)

	router.Run("0.0.0.0:1115")
}

func (s *ApiServer) statusHandler(c *gin.Context) {
	id := c.Param("id")
	req := &Common.ConnectionRequest{
		DeviceId: id,
		IsReady:  make(chan bool),
	}
	s.connRequestPipe <- req
	<-req.IsReady
	res, err := Common.GetStatus(req.Connection)
	if err != nil {
		println(err.Error())
		c.JSON(500, gin.H{
			"error": "cannot get status",
		})
		return
	}

	c.JSON(200, gin.H{
		"status": res,
	})
}

func (s *ApiServer) listAllDevices(c *gin.Context) {
	req := &Common.ConnectionRequest{
		ListAll: true,
		IsReady: make(chan bool),
	}
	s.connRequestPipe <- req
	<-req.IsReady

	var deviceIds = []string{}
	for _, id := range req.DeviceIds {
		deviceIds = append(deviceIds, id)
	}

	c.JSON(200, gin.H{
		"devices": deviceIds,
	})
}
