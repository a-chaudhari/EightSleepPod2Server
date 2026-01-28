package LogServer

type state int

const (
	StateClientHello state = iota
	StateWaitingForStreamStart
	StateReceivingStream
)

type WelcomeMessage struct {
	Proto    string `cbor:"proto" json:"proto"`
	Version  string `cbor:"version" json:"version"`
	Part     string `cbor:"part" json:"part"`
	DeviceId string `cbor:"dev" json:"dev"`
}

type WelcomeResponse struct {
	Proto string `cbor:"proto" json:"proto"`
	Part  string `cbor:"part" json:"part"`
}

type FileAckResponse struct {
	Proto string `cbor:"proto" json:"proto"`
	Part  string `cbor:"part" json:"part"`
	Id    uint32 `cbor:"id" json:"id"`
}
