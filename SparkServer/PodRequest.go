package SparkServer

import "github.com/plgd-dev/go-coap/v3/message"

type PodRequest struct {
	message  *message.Message
	Response []byte
	Ready    chan bool
}

func NewPodRequest(msg *message.Message) *PodRequest {
	return &PodRequest{
		message: msg,
		Ready:   make(chan bool, 1),
	}
}

func (pr *PodRequest) SetResponse(resp []byte) {
	pr.Response = resp
	close(pr.Ready)
}
