package LogBlackHole

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

type LogBlackHole struct{}

func (b LogBlackHole) StartServer() {
	PORT := ":1337"
	l, err := net.Listen("tcp4", PORT)
	if err != nil {
		log.Fatal(err)
	}
	defer func(l net.Listener) {
		_ = l.Close()
	}(l)

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go b.handleConnection(c)
	}
}

func (b LogBlackHole) handleConnection(c net.Conn) {
	println("Client connected:", c.RemoteAddr().String())
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	buf := make([]byte, 4096)
	for {
		n, err := c.Read(buf)
		if err != nil {
			fmt.Println("Client disconnected:", c.RemoteAddr().String())
			return
		}
		data := buf[:n]
		//fmt.Printf("Received from %s: %s", c.RemoteAddr().String(), string(data))
		//fmt.Printf("Length: %s\n", len(data))
		stringVersion := string(data)

		// can either be a handshake, a file start header, or file end header
		if len(data) == 64 && strings.Contains(stringVersion, "eprotocrawdpartgsessioncdevx") {
			var req WelcomeMessage
			err := cbor.Unmarshal(data, &req)
			res := WelcomeResponse{
				Proto: "raw",
				Part:  "session",
			}
			handshakeResponse, err := cbor.Marshal(res)
			// print hex response
			fmt.Printf("%x\n", handshakeResponse)
			if err != nil {
				fmt.Println("Error marshalling handshake response:", err)
				return
			}
			_, err = c.Write(handshakeResponse)
			if err != nil {
				fmt.Println("Error sending handshake response:", err)
				return
			}
			deviceId := req.Dev
			fmt.Printf("Device Connected, ID: %s\n", deviceId)
		} else if len(data) == 38 && strings.Contains(stringVersion, "eprotocrawdpartebatchbid") {
			// this is the start of a new file, need to get the file id and return it in the response
			//var req BatchStart
			//decoder := cbor.NewDecoder(bytes.NewReader(data))
			//
			//err := decoder.Decode(&req)
			//err := cbor.Unmarshal(data, &req)
			//if err != nil {
			//	fmt.Println("Error unmarshalling batch start:", err)
			//	continue
			//}
			batchId := data[0x1a:0x1e]
			fmt.Printf("Batch Start ID: %x\n", batchId)
			response := b.getStartFileResponse(batchId)
			_, err = c.Write(response)
			if err != nil {
				fmt.Println("Error sending start file response:", err)
			}
		} else if len(data) == 1 && data[0] == 0xFF {
			// this is end of file
			println("Received end of file packet")
		} else {
			//fmt.Println("Received data of len:", len(data))
		}
	}
}

func (b LogBlackHole) getStartFileResponse(batchId []byte) []byte {
	template := []byte{0xa3, 0x65, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x72, 0x61, 0x77, 0x64, 0x70, 0x61, 0x72, 0x74,
		0x65, 0x62, 0x61, 0x74, 0x63, 0x68, 0x62, 0x69, 0x64, 0x1a}
	template = append(template, batchId...)
	return template
}
