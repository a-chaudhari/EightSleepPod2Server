package SparkServer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"

	"go.uber.org/zap"
)

type Server struct {
	serverPrivateKey *rsa.PrivateKey
	port             int
	socketPath       string
	logger           *zap.Logger
}

func NewServer(publicKeyPath string, port int, socketPath string) *Server {
	dat, err := os.ReadFile(publicKeyPath)
	if err != nil {
		zap.L().Panic("Failed to read private key file", zap.String("path", publicKeyPath), zap.Error(err))
	}

	out, rest := pem.Decode(dat)
	if len(rest) > 0 {
		zap.L().Panic("Unknown bytes in PEM decode", zap.ByteString("rest", rest))
	}

	cert, err := x509.ParsePKCS8PrivateKey(out.Bytes)
	if err != nil {
		zap.L().Panic("Cannot parse key private bytes", zap.Error(err))
	}

	logger, _ := zap.NewProduction()

	return &Server{
		serverPrivateKey: cert.(*rsa.PrivateKey),
		port:             port,
		socketPath:       socketPath,
		logger:           logger,
	}
}

func (s *Server) StartServer() {
	s.logger.Info("Starting SparkServer", zap.Int("port", s.port))
	portString := fmt.Sprintf(":%d", s.port)
	l, err := net.Listen("tcp4", portString)
	if err != nil {
		s.logger.Panic("Failed to listen on port", zap.String("port", portString), zap.Error(err))
	}
	defer func(l net.Listener) {
		_ = l.Close()
	}(l)

	for {
		c, err := l.Accept()
		if err != nil {
			s.logger.Error("Failed to accept connection", zap.Error(err))
			return
		}
		s.handleConnection(c)
	}
}

func (s *Server) handleConnection(c net.Conn) {
	s.logger.Info("Client connected", zap.String("remote_addr", c.RemoteAddr().String()))
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)

	client := NewPodConnection(&c, s.serverPrivateKey, s.socketPath)
	client.HandleConnection() // blocking call
	s.logger.Info("Client disconnected", zap.String("remote_addr", c.RemoteAddr().String()))
}
