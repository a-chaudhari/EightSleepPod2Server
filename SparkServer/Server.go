package SparkServer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
)

type Server struct {
	serverPrivateKey    *rsa.PrivateKey
	ConnectionStatePipe chan *ConnectionNotification
}

type ConnectionNotification struct {
	DeviceId    string
	IsConnected bool
	Conn        *ClientConnection
}

func NewServer(publicKeyPath string, newConnPipe chan *ConnectionNotification) *Server {
	dat, err := os.ReadFile(publicKeyPath)
	if err != nil {
		panic(err)
	}

	out, rest := pem.Decode(dat)
	if len(rest) > 0 {
		panic("Unknown bytes")
	}

	cert, err := x509.ParsePKCS8PrivateKey(out.Bytes)
	if err != nil {
		panic("Cannot parse key private bytes")
	}

	return &Server{
		serverPrivateKey:    cert.(*rsa.PrivateKey),
		ConnectionStatePipe: newConnPipe,
	}
}

func (s *Server) StartServer() {
	PORT := ":5683"
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
		go s.handleConnection(c)
	}
}

func (s *Server) handleConnection(c net.Conn) {
	println("Client connected:", c.RemoteAddr().String())
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	client := NewClientConnection(&c, s.serverPrivateKey, s.ConnectionStatePipe)
	client.HandleConnection()
}
