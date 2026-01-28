package LogServer

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

type LogServer struct {
	state     state
	saveFiles bool
	filePath  string
}

func (b LogServer) StartServer(saveFiles bool, filePath string, port int) {
	println("Starting LogBlackhole server...")
	b.state = StateClientHello
	b.saveFiles = saveFiles
	b.filePath = filePath

	portStr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp4", portStr)
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
		b.handleConnection(c)
		b.state = StateClientHello
		println("Client disconnected, waiting for new connection...")
	}
}

func (b LogServer) handleConnection(c net.Conn) {
	println("Client connected:", c.RemoteAddr().String())
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	cbuffer := CircularBuffer{}
	buf := make([]byte, 4096)
	var osFile *os.File
	var file *bufio.Writer
	var counter uint64 = 0
	for {
		n, err := c.Read(buf)
		if err != nil {
			fmt.Println("Client disconnected:", c.RemoteAddr().String())
			if b.saveFiles && osFile != nil {
				err := file.Flush()
				if err != nil {
					fmt.Println("Error flushing file:", err)
				}
				err = osFile.Close()
				if err != nil {
					fmt.Println("Error closing file:", err)
				}
				fmt.Println("Closed open file due to client disconnect.")
			}
			return
		}
		data := buf[:n]
		var batchId uint32

		if b.state == StateClientHello {
			var req WelcomeMessage
			err := cbor.Unmarshal(data, &req)
			if err != nil {
				fmt.Println("Error unmarshalling welcome message:", err)
				continue
			}
			res := WelcomeResponse{
				Proto: "raw",
				Part:  "session",
			}
			handshakeResponse, err := cbor.Marshal(res)
			// print hex response
			if err != nil {
				fmt.Println("Error marshalling handshake response:", err)
				return
			}
			_, err = c.Write(handshakeResponse)
			if err != nil {
				fmt.Println("Error sending handshake response:", err)
				return
			}
			fmt.Printf("Device Connected, ID: %s\n", req.DeviceId)
			b.state = StateWaitingForStreamStart
			continue
		} else if b.state == StateWaitingForStreamStart {
			/*
				we parse this manually to avoid having to deal with indefinite cbor byte strings
			*/
			stringVersion := string(data)
			if len(data) != 38 || !strings.Contains(stringVersion, "eprotocrawdpartebatchbid") {
				fmt.Println("Invalid batch start packet received")
				continue
			}
			batchIdBytes := data[0x1a:0x1e]
			batchId = binary.BigEndian.Uint32(batchIdBytes)
			fmt.Printf("Batch Start ID: %x\n", batchIdBytes)
			fileName := fmt.Sprintf("%s/%08X.RAW", b.filePath, batchId)
			if b.saveFiles {
				// open file for writing
				osFile, err = os.Create(fileName)
				if err != nil {
					fmt.Println("Error creating file:", err)
					return
				}
				file = bufio.NewWriter(osFile)
				fmt.Printf("Receiving stream: %s\n", fileName)
			} else {
				fmt.Printf("Receiving stream: %s. However not saving.\n", fileName)
			}

			b.state = StateReceivingStream
			continue
		} else if b.state == StateReceivingStream {
			cbuffer.Insert(data)
			result := cbuffer.ExtractCBORByteStrings()
			for _, record := range result.Data {
				counter += uint64(len(record))
				if b.saveFiles {
					// dump the data into the file
					fmt.Printf("Adding %d bytes to file\n", len(record))
					_, err := file.Write(record)
					if err != nil {
						fmt.Println("Error writing to file:", err)
					}
					err = file.Flush()
					if err != nil {
						fmt.Println("Error flushing file:", err)
					}
				}
			}
			if result.ResetFound {
				if b.saveFiles {
					// close the file
					err := file.Flush()
					if err != nil {
						fmt.Println("Error flushing file:", err)
					}
					err = osFile.Close()
					if err != nil {
						fmt.Println("Error closing file:", err)
					}
					fmt.Println("Received end of file packet, closed file.")
				}

				ack := b.getFileAck(batchId)
				_, err = c.Write(ack)
				if err != nil {
					fmt.Println("Error sending file ack response:", err)
				}
				fmt.Printf("Total bytes received for batch %08X: %d bytes\n", batchId, counter)
				counter = 0
				b.state = StateWaitingForStreamStart
			}
			continue
		}
	}
}

func (b LogServer) getFileAck(batchId uint32) []byte {
	res := FileAckResponse{
		Proto: "raw",
		Part:  "batch",
		Id:    batchId,
	}
	template, err := cbor.Marshal(res)
	if err != nil {
		fmt.Println("Error marshalling file ack response:", err)
		return nil
	}
	return template
}
