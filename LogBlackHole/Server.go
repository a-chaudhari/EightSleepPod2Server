package LogBlackHole

import (
	"fmt"
	"log"
	"net"
	"strings"
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

	buf := make([]byte, 1024)
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
			// this is a handshake, we can send a fixed response
			handshakeResponse := b.getHandshakeResponse()
			_, err = c.Write(handshakeResponse)
			if err != nil {
				fmt.Println("Error sending handshake response:", err)
				return
			}
			deviceId := data[0x1e:0x36]
			fmt.Printf("Device Connected, ID: %s\n", deviceId)
			//fmt.Printf("Sent handshake response to %s: %x\n", c.RemoteAddr().String(), handshakeResponse)
		} else if len(data) == 38 && strings.Contains(stringVersion, "eprotocrawdpartebatchbid") {
			// this is the start of a new file, need to get the file id and return it in the response
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

func (b LogBlackHole) getHandshakeResponse() []byte {
	// fixed literal response
	return []byte{0xa2, 0x65, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x72, 0x61, 0x77, 0x64, 0x70, 0x61, 0x72, 0x74, 0x67,
		0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e}
}

func (b LogBlackHole) getStartFileResponse(batchId []byte) []byte {
	template := []byte{0xa3, 0x65, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x72, 0x61, 0x77, 0x64, 0x70, 0x61, 0x72, 0x74,
		0x65, 0x62, 0x61, 0x74, 0x63, 0x68, 0x62, 0x69, 0x64, 0x1a}
	template = append(template, batchId...)
	return template
}
