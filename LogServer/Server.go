package LogServer

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"go.uber.org/zap"
)

type LogServer struct {
	state     state
	saveFiles bool
	filePath  string
	logger    *zap.Logger
}

func (b LogServer) StartServer(saveFiles bool, filePath string, port int) {
	logger, _ := zap.NewProduction()
	b.logger = logger
	b.state = StateClientHello
	b.saveFiles = saveFiles
	b.filePath = filePath

	b.logger.Info("Starting LogBlackhole server", zap.Int("port", port))
	portStr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp4", portStr)
	if err != nil {
		b.logger.Panic("Failed to listen on port", zap.String("port", portStr), zap.Error(err))
	}
	defer func(l net.Listener) {
		_ = l.Close()
	}(l)

	for {
		c, err := l.Accept()
		if err != nil {
			b.logger.Error("Failed to accept connection", zap.Error(err))
			return
		}
		b.handleConnection(c)
		b.state = StateClientHello
		b.logger.Info("Client disconnected, waiting for new connection...")
	}
}

func (b LogServer) handleConnection(c net.Conn) {
	b.logger.Info("Client connected", zap.String("remote_addr", c.RemoteAddr().String()))
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	parser := StreamParser{}
	buf := make([]byte, 4096)
	var batchId uint32
	var osFile *os.File
	var file *bufio.Writer
	var counter uint64 = 0
	for {
		n, err := c.Read(buf)
		if err != nil {
			b.logger.Info("Client disconnected", zap.String("remote_addr", c.RemoteAddr().String()))
			if b.saveFiles && osFile != nil {
				err := file.Flush()
				if err != nil {
					b.logger.Error("Error flushing file", zap.Error(err))
				}
				err = osFile.Close()
				if err != nil {
					b.logger.Error("Error closing file", zap.Error(err))
				}
				b.logger.Info("Closed open file due to client disconnect.")
			}
			return
		}
		data := buf[:n]

		if b.state == StateClientHello {
			var req WelcomeMessage
			err := cbor.Unmarshal(data, &req)
			if err != nil {
				b.logger.Error("Error unmarshalling welcome message", zap.Error(err))
				continue
			}
			res := WelcomeResponse{
				Proto: "raw",
				Part:  "session",
			}
			handshakeResponse, err := cbor.Marshal(res)
			if err != nil {
				b.logger.Error("Error marshalling handshake response", zap.Error(err))
				return
			}
			_, err = c.Write(handshakeResponse)
			if err != nil {
				b.logger.Error("Error sending handshake response", zap.Error(err))
				return
			}
			b.logger.Info("Device Connected", zap.String("device_id", req.DeviceId))
			b.state = StateWaitingForStreamStart
			continue
		} else if b.state == StateWaitingForStreamStart {
			/*
				we parse this manually to avoid having to deal with indefinite cbor byte strings
			*/
			stringVersion := string(data)
			if len(data) != 38 || !strings.Contains(stringVersion, "eprotocrawdpartebatchbid") {
				b.logger.Error("Invalid batch start packet received")
				continue
			}
			batchIdBytes := data[0x1a:0x1e]
			batchId = binary.BigEndian.Uint32(batchIdBytes)
			batchIdHex := fmt.Sprintf("%08X", batchId)
			b.logger.Info("Batch Start", zap.String("batch_id", batchIdHex))
			fileName := fmt.Sprintf("%s/%08X.RAW", b.filePath, batchId)
			if b.saveFiles {
				// open file for writing
				osFile, err = os.Create(fileName)
				if err != nil {
					b.logger.Error("Error creating file", zap.Error(err))
					return
				}
				file = bufio.NewWriter(osFile)
				b.logger.Info("Receiving stream", zap.String("file", fileName))
			} else {
				b.logger.Info("Receiving stream (not saving)", zap.String("file", fileName))
			}

			b.state = StateReceivingStream
			continue
		} else if b.state == StateReceivingStream {
			parser.Insert(data)
			result := parser.ExtractCBORByteStrings()
			for _, record := range result.Data {
				counter += uint64(len(record))
				if b.saveFiles {
					// dump the data into the file
					//fmt.Printf("Adding %d bytes to file\n", len(record))
					_, err := file.Write(record)
					if err != nil {
						b.logger.Error("Error writing to file", zap.Error(err))
					}
					err = file.Flush()
					if err != nil {
						b.logger.Error("Error flushing file", zap.Error(err))
					}
				}
			}
			if result.ResetFound {
				if b.saveFiles {
					// close the file
					err := file.Flush()
					if err != nil {
						b.logger.Error("Error flushing file", zap.Error(err))
					}
					err = osFile.Close()
					if err != nil {
						b.logger.Error("Error closing file", zap.Error(err))
					}
				}

				ack := b.getFileAck(batchId)
				_, err = c.Write(ack)
				if err != nil {
					b.logger.Error("Error sending ack", zap.Error(err))
				}
				counter = 0
				b.state = StateWaitingForStreamStart
				// convert to hex string

				batchIdHex := fmt.Sprintf("%08X", batchId)
				b.logger.Info("Stream finished", zap.String("batch_id", batchIdHex), zap.Uint64("bytes_received", counter))
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
		b.logger.Error("Error marshalling file ack response", zap.Error(err))
		return nil
	}
	return template
}
