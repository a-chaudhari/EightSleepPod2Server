package SparkServer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
	"go.uber.org/zap"
)

type PodConnection struct {
	conn             *net.Conn
	serverPrivateKey *rsa.PrivateKey
	aesCipher        cipher.Block
	incomingIv       [16]byte
	outgoingIv       [16]byte
	deviceId         [12]byte
	messageId        uint8 // outgoing mid can only be 0-255, loop back to 0 after that
	currentRequest   *PodRequest
	RequestPipe      chan *PodRequest
	sendMutex        sync.Mutex
	socketPath       string
	logger           *zap.Logger
}

func NewPodConnection(conn *net.Conn, serverPublicKey *rsa.PrivateKey, socketPath string) *PodConnection {
	logger, _ := zap.NewProduction()
	return &PodConnection{conn: conn, serverPrivateKey: serverPublicKey, messageId: 0,
		RequestPipe: make(chan *PodRequest, 100),
		socketPath:  socketPath,
		logger:      logger,
	}
}

func (c *PodConnection) HandleConnection() {
	err := c.performHandshake()
	if err != nil {
		c.logger.Error("Error performing handshake", zap.Error(err))
		return
	}

	c.logger.Info("Handshake successful, Ready for further communication")
	go c.podRequestHandler()

	defer c.logger.Info("Exiting client connection handler", zap.String("remote_addr", (*c.conn).RemoteAddr().String()))

	buf := make([]byte, 2048)
	for {
		n, err := (*c.conn).Read(buf)
		if err != nil {
			c.logger.Info("Client disconnected", zap.String("remote_addr", (*c.conn).RemoteAddr().String()))
			return
		}
		data := buf[:n]
		// split up payload into msgs
		messages, err := c.SplitMessages(data)
		if err != nil {
			c.logger.Error("Error splitting messages", zap.Error(err))
			return
		}

		// process each msg
		for _, msg := range messages {
			coapmsg := pool.NewMessage(context.Background())
			_, err := coapmsg.UnmarshalWithDecoder(coder.DefaultCoder, msg)
			if err != nil {
				return
			}

			url, err := coapmsg.Path()
			if err != nil {
				url = "/"
			}
			if url == "/" && coapmsg.Type() == message.Confirmable {
				err := c.handleKeepAlive(coapmsg)
				if err != nil {
					c.logger.Error("Error handling ping like", zap.Error(err))
					return
				}
				continue
			}
			if coapmsg.Type() == message.Acknowledgement {
				if c.currentRequest != nil && coapmsg.MessageID() == c.currentRequest.message.MessageID {
					body, err := coapmsg.ReadBody()
					if err != nil {
						c.logger.Error("Error reading body of pod Response", zap.Error(err))
						continue
					}
					cr := c.currentRequest
					c.currentRequest = nil
					cr.SetResponse(body)
					continue
				} else {
					c.logger.Info("Received acknowledgement for unknown request, ignoring")
					continue
				}
			}

			switch url {
			case "/h":
				c.logger.Info("Hello received")
				err := c.handleHello()
				if err != nil {
					c.logger.Error("Error when sending hello Response", zap.Error(err))
					return
				}
				go c.connectToUnixSocket()
			case "/E/spark/device/claim/code":
				// noop
			case "/E/spark/hardware/max_binary":
				// noop
			case "/E/spark/hardware/ota_chunk_size":
				// noop
			case "/e/spark":
				err := c.handleESpark(coapmsg)
				if err != nil {
					c.logger.Error("Error handling espark", zap.Error(err))
					return
				}
			case "/t":
				err := c.handleTimestamp(coapmsg)
				if err != nil {
					c.logger.Error("Error handling timestamp", zap.Error(err))
					return
				}
			case "/E/tracing/rat":
				// noop
			}
		}
	}
}

func (c *PodConnection) SplitMessages(data []byte) ([][]byte, error) {
	// a single tcp payload may contain multiple messages
	var messages [][]byte
	offset := 0
	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		// First 2 bytes = payload length (big-endian)
		payloadLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+payloadLen > len(data) {
			c.logger.Debug("Incomplete message", zap.Int("expected_bytes", payloadLen))
			break
		}

		ciphertext := data[offset : offset+payloadLen]
		offset += payloadLen

		plaintext, err := c.decrypt(ciphertext)
		if err != nil {
			c.logger.Error("Error decrypting message", zap.Error(err))
			return nil, err
		}
		messages = append(messages, plaintext)
	}
	return messages, nil
}

func (c *PodConnection) decrypt(input []byte) ([]byte, error) {
	plaintext := make([]byte, len(input))
	cipher.NewCBCDecrypter(c.aesCipher, c.incomingIv[:]).CryptBlocks(plaintext, input)
	// Remove PKCS7 padding

	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		return nil, errors.New("invalid padding")
	}
	c.incomingIv = [16]byte(input[:16])
	return plaintext[:len(plaintext)-padLen], nil
}

func (c *PodConnection) encrypt(plaintext []byte) ([]byte, error) {
	// PKCS7 padding
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(c.aesCipher, c.outgoingIv[:])
	mode.CryptBlocks(ciphertext, padded)

	c.outgoingIv = [16]byte(ciphertext[:16])
	return ciphertext, nil
}

func (c *PodConnection) sendMessage(msg *message.Message) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	response := pool.Message{}
	if msg.Type != message.Acknowledgement {
		msg.MessageID = int32(c.messageId)
		msg.Token = []byte{byte(c.messageId)}
		c.messageId++
	}

	response.SetMessage(*msg)
	output, err := response.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		return err
	}
	//fmt.Printf("message outgoing: %x\n", output)

	encryptedPayload, err := c.encrypt(output)
	if err != nil {
		return err
	}
	final := make([]byte, 2+len(encryptedPayload))
	// first two bytes is the length
	binary.BigEndian.PutUint16(final, uint16(len(encryptedPayload)))
	copy(final[2:], encryptedPayload)

	_, err = (*c.conn).Write(final)
	if err != nil {
		return err
	}

	return nil
}

func (c *PodConnection) handleKeepAlive(incoming *pool.Message) error {
	msg := message.Message{
		MessageID: incoming.MessageID(),
		Type:      message.Acknowledgement,
		Code:      codes.Empty,
		Token:     incoming.Token(),
	}
	return c.sendMessage(&msg)
}

func (c *PodConnection) handleHello() error {
	msg := message.Message{
		Options: message.Options{{ID: message.URIPath, Value: []byte("h")}},
		Code:    codes.POST,
		Payload: nil,
		Type:    message.NonConfirmable,
	}
	return c.sendMessage(&msg)
}

func (c *PodConnection) handleESpark(incoming *pool.Message) error {
	c.logger.Info("Handling e/spark")
	msg := message.Message{
		Type:      message.Acknowledgement,
		MessageID: incoming.MessageID(),
		Code:      codes.Empty,
		Token:     incoming.Token(),
	}
	return c.sendMessage(&msg)
}

func (c *PodConnection) handleTimestamp(incoming *pool.Message) error {
	c.logger.Info("Handling timestamp")
	now := time.Now().Unix()
	nowbytes := make([]byte, 4)
	binary.BigEndian.PutUint32(nowbytes, uint32(now))
	msg := message.Message{
		Type:      message.Acknowledgement,
		Code:      codes.Content,
		MessageID: incoming.MessageID(),
		Token:     incoming.Token(),
		Payload:   nowbytes,
	}
	return c.sendMessage(&msg)
}

func (c *PodConnection) podRequestHandler() {
	for {
		select {
		case req := <-c.RequestPipe:
			//c.logger.Debug("Received pod request")
			c.currentRequest = req
			err := c.sendMessage(req.message)
			if err != nil {
				c.logger.Error("Error sending pod request Response", zap.Error(err))
				continue
			}
			<-req.Ready // blocks until Response is Ready
			//c.logger.Debug("Pod request Response Ready")
		}
	}
}
