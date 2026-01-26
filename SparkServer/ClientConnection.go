package SparkServer

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
)

type ClientConnection struct {
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
}

func NewClientConnection(conn *net.Conn, serverPublicKey *rsa.PrivateKey, socketPath string) *ClientConnection {
	return &ClientConnection{conn: conn, serverPrivateKey: serverPublicKey, messageId: 0,
		RequestPipe: make(chan *PodRequest, 100),
		socketPath:  socketPath,
	}
}

func (c *ClientConnection) HandleConnection() {
	err := c.performHandshake()
	if err != nil {
		println("Error performing handshake:", err)
		return
	}

	println("Handshake successful, Ready for further communication")
	go c.podRequestHandler()

	defer println("exiting client connection handler for", (*c.conn).RemoteAddr().String())

	buf := make([]byte, 2048)
	for {
		n, err := (*c.conn).Read(buf)
		if err != nil {
			fmt.Println("Client disconnected:", (*c.conn).RemoteAddr().String())
			return
		}
		data := buf[:n]
		fmt.Printf("Received %d bytes of data\n", len(data))
		// split up payload into msgs
		messages, err := c.SplitMessages(data)
		if err != nil {
			fmt.Println("Error splitting messages:", err)
			return
		}

		// process each msg
		for _, msg := range messages {
			fmt.Printf("Processing message: %x\n", msg)
			coapmsg := pool.NewMessage(context.Background())
			_, err := coapmsg.UnmarshalWithDecoder(coder.DefaultCoder, msg)
			println("CoAP Message:", coapmsg.String())
			if err != nil {
				return
			}

			url, err := coapmsg.Path()
			if err != nil {
				//println("Error getting path from message:", err)
				url = "/"
			}
			if url == "/" && coapmsg.Type() == message.Confirmable {
				err := c.handlePingLike(coapmsg)
				if err != nil {
					println("Error handling ping like:", err)
					return
				}
				continue
			}
			if coapmsg.Type() == message.Acknowledgement {
				// this is a Response to an earlier request
				if c.currentRequest != nil && coapmsg.MessageID() == c.currentRequest.message.MessageID {
					println("Received Response for current pod request")
					body, err := coapmsg.ReadBody()
					if err != nil {
						println("Error reading body of pod Response:", err)
						continue
					}
					cr := c.currentRequest
					c.currentRequest = nil
					cr.SetResponse(body)
					continue
				} else {
					println("Received acknowledgement for unknown request, ignoring")
					continue
				}
			}

			switch url {
			case "/h":
				println("H received")
				err := c.handleHello()
				if err != nil {
					println("error when sending hello Response", err)
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
					println("Error handling espark:", err)
					return
				}
			case "/t":
				err := c.handleTimestamp(coapmsg)
				if err != nil {
					println("Error handling timestamp:", err)
					return
				}
			case "/E/tracing/rat":
				// noop
			default:
				println("Unhandled message:", url)
			}
		}
	}
}

func (c *ClientConnection) SplitMessages(data []byte) ([][]byte, error) {
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
			fmt.Printf("[DEBUG] Incomplete message, expected %d bytes\n", payloadLen)
			break
		}

		ciphertext := data[offset : offset+payloadLen]
		offset += payloadLen

		plaintext, err := c.decrypt(ciphertext)
		if err != nil {
			println("Error decrypting message:", err)
			return nil, err
		}
		fmt.Printf("[DEBUG] Decrypted message: %d bytes\n", len(plaintext))
		messages = append(messages, plaintext)
	}
	return messages, nil
}

func (c *ClientConnection) connectToUnixSocket() {
	socket, err := net.Dial("unix", c.socketPath)
	if err != nil {
		panic(err)
	}
	defer socket.Close()

	println("Connected to FrankenSocket unix socket")
	buf := make([]byte, 4096)
	for {
		n, err := socket.Read(buf)
		if err != nil {
			println("Error reading from FrankenSocket unix socket:", err)
			return
		}
		data := buf[:n]
		fmt.Printf("Received %d bytes from FrankenSocket unix socket\n", len(data))
		strVersion := strings.TrimSpace(string(data))
		fmt.Printf("%x\n", strVersion)
		intVersion, err := strconv.Atoi(strVersion)
		if err != nil {
			println("Error converting data to int:", err)
			continue
		}
		println("Got int from unix socket:", intVersion)
		cmd := FrankenCommand(intVersion)
		switch cmd {
		case FRANKEN_CMD_DEVICE_STATUS:
			println("Got device status command")
			res, err := GetStatus(c)
			if err != nil {
				println("Error getting pod status:", err)
				continue
			}
			_, _ = socket.Write([]byte("tgHeatLevelR = " + strconv.Itoa(res.RightBed.TargetHeatLevel) + "\n"))
			_, _ = socket.Write([]byte("tgHeatLevelL = " + strconv.Itoa(res.LeftBed.TargetHeatLevel) + "\n"))
			_, _ = socket.Write([]byte("heatTimeR = " + strconv.Itoa(res.RightBed.HeatTime) + "\n"))
			_, _ = socket.Write([]byte("heatTimeL = " + strconv.Itoa(res.LeftBed.HeatTime) + "\n"))
			_, _ = socket.Write([]byte("heatLevelR = " + strconv.Itoa(res.RightBed.HeatLevel) + "\n"))
			_, _ = socket.Write([]byte("heatLevelL = " + strconv.Itoa(res.LeftBed.HeatLevel) + "\n"))
			_, _ = socket.Write([]byte("sensorLabel = " + res.SensorLabel + "\n"))
			_, _ = socket.Write([]byte("waterLevel = " + strconv.FormatBool(res.WaterLevel) + "\n"))
			_, _ = socket.Write([]byte("priming = " + strconv.FormatBool(res.Priming) + "\n"))
			_, _ = socket.Write([]byte("settings = " + res.Settings + "\n"))
			_, _ = socket.Write([]byte("\n"))
		}
	}
}

func (c *ClientConnection) performHandshake() error {
	// create random 40 byte slice
	nonce, err := createNonce()
	if err != nil {
		fmt.Println("Error creating nonce:", err)
		return err
	}

	// send down wire
	_, err = (*c.conn).Write(nonce)
	if err != nil {
		fmt.Println("Error sending nonce:", err)
		return err
	}

	// wait for Response payload
	buf := make([]byte, 1024)
	n, err := (*c.conn).Read(buf)
	if err != nil {
		fmt.Println("Error reading Response:", err)
		return err
	}
	responsePayload := buf[:n]

	// try decrypting with private key
	decryptedPayload, err := decryptWithServerRSA(responsePayload, c.serverPrivateKey)
	if err != nil {
		fmt.Println("Error decrypting payload:", err)
		return err
	}

	//fmt.Printf("Decrypted Payload: %x\n", decryptedPayload)
	response, err := parseClientHandshake(decryptedPayload)
	if err != nil {
		fmt.Println("Error parsing client handshake:", err)
		return err
	}
	c.deviceId = response.ClientDeviceKey

	println("Client handshake received")
	if !bytes.Equal(nonce, response.Nonce[:40]) {
		println("Nonce mismatch")
		return err
	}
	println("nonce matched")

	// now need to create handshake Response
	keybuffer, err := createNonce()
	if err != nil {
		println("Error creating key block:", err)
		return err
	}
	c.aesCipher, err = aes.NewCipher(keybuffer[:16])
	if err != nil {
		println("Error creating AES cipher:", err)
		return err
	}
	c.incomingIv = [16]byte(keybuffer[16:32])
	c.outgoingIv = c.incomingIv
	//fmt.Printf("aes key: %h  iv: %x\n", aesKey, outgoingIv)

	cyphertext, err := encryptWithClientRSA(keybuffer, response.ClientPublicKey)
	if err != nil {
		println("Error encrypting payload:", err)
		return err
	}

	secondResponse, err := createHmacSignature(cyphertext, keybuffer, c.serverPrivateKey)
	if err != nil {
		println("cannot generate hmac", err)
		return err
	}

	// Combine: 128 bytes ciphertext + 256 bytes signature
	bigBlob := make([]byte, len(cyphertext)+len(secondResponse))
	copy(bigBlob, cyphertext)
	copy(bigBlob[len(cyphertext):], secondResponse)

	_, err = (*c.conn).Write(bigBlob)
	if err != nil {
		fmt.Println("Error writing Response:", err)
		return err
	}

	return nil
}

func (c *ClientConnection) decrypt(input []byte) ([]byte, error) {
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

func (c *ClientConnection) encrypt(plaintext []byte) ([]byte, error) {
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

func intToMinBytes(n uint32) []byte {
	if n == 0 {
		return []byte{0}
	}

	buf := make([]byte, 1)
	binary.BigEndian.PutUint32(buf, n)

	// Find first non-zero byte
	for i, b := range buf {
		if b != 0 {
			return buf[i:]
		}
	}
	return buf
}

func (c *ClientConnection) sendMessaage(msg *message.Message) error {
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
	fmt.Printf("message outgoing: %x\n", output)

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

func (c *ClientConnection) handlePingLike(incoming *pool.Message) error {
	println("Handling ping-like")
	msg := message.Message{
		MessageID: incoming.MessageID(),
		Type:      message.Acknowledgement,
		Code:      codes.Empty,
		Token:     incoming.Token(),
	}
	return c.sendMessaage(&msg)
}

func (c *ClientConnection) handleHello() error {
	msg := message.Message{
		Options: message.Options{{ID: message.URIPath, Value: []byte("h")}},
		Code:    codes.POST,
		Payload: nil,
		Type:    message.NonConfirmable,
	}
	return c.sendMessaage(&msg)
}

func (c *ClientConnection) handleESpark(incoming *pool.Message) error {
	println("Handling e/spark")
	msg := message.Message{
		Type:      message.Acknowledgement,
		MessageID: incoming.MessageID(),
		Code:      codes.Empty,
		Token:     incoming.Token(),
	}
	return c.sendMessaage(&msg)
}

func (c *ClientConnection) handleTimestamp(incoming *pool.Message) error {
	println("Handling timestamp")
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
	return c.sendMessaage(&msg)
}

func (c *ClientConnection) podRequestHandler() {
	for {
		select {
		case req := <-c.RequestPipe:
			println("Received pod request")
			c.currentRequest = req
			err := c.sendMessaage(req.message)
			if err != nil {
				println("Error sending pod request Response:", err)
				continue
			}
			<-req.Ready // blocks until Response is Ready
			println("Pod request Response Ready")
		}
	}
}
