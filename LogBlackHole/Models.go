package LogBlackHole

import "github.com/fxamacker/cbor/v2"

type WelcomeMessage struct {
	Proto   string `cbor:"proto" json:"proto"`
	Version string `cbor:"version" json:"version"`
	Part    string `cbor:"part" json:"part"`
	Dev     string `cbor:"dev" json:"dev"`
}

type WelcomeResponse struct {
	Proto string `cbor:"proto" json:"proto"`
	Part  string `cbor:"part" json:"part"`
}

type BatchStart struct {
	Proto  string          `cbor:"proto" json:"proto"`
	Part   string          `cbor:"part" json:"part"`
	Id     int32           `cbor:"id" json:"id"`
	Stream cbor.RawMessage `cbor:"stream" json:"stream"`
}
