package SparkServer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
)

type Server struct {
	serverPrivateKey *rsa.PrivateKey
	port             int
	socketPath       string
}

func NewServer(publicKeyPath string, port int, socketPath string) *Server {
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
		serverPrivateKey: cert.(*rsa.PrivateKey),
		port:             port,
		socketPath:       socketPath,
	}
}

func (s *Server) StartServer() {
	println("Starting SparkServer on port ", s.port)
	portString := fmt.Sprintf(":%d", s.port)
	l, err := net.Listen("tcp4", portString)
	if err != nil {
		panic(err)
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
		s.handleConnection(c)
	}
}

func (s *Server) handleConnection(c net.Conn) {
	println("Client connected:", c.RemoteAddr().String())
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	client := NewPodConnection(&c, s.serverPrivateKey, s.socketPath)
	client.HandleConnection() // blocking call
	println("Client disconnected:", c.RemoteAddr().String())
}
