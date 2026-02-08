package SparkServer

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"go.uber.org/zap"
	"math/big"
)

type ClientResponse struct {
	Nonce           [40]byte
	ClientDeviceKey [12]byte
	ClientPublicKey *rsa.PublicKey
}

func (c *PodConnection) performHandshake() error {
	logger := c.logger
	// create random 40 byte slice
	nonce, err := createNonce()
	if err != nil {
		logger.Error("Error creating nonce", zap.Error(err))
		return err
	}

	// send down wire
	_, err = (*c.conn).Write(nonce)
	if err != nil {
		logger.Error("Error sending nonce", zap.Error(err))
		return err
	}

	// wait for Response payload
	buf := make([]byte, 1024)
	n, err := (*c.conn).Read(buf)
	if err != nil {
		logger.Error("Error reading Response", zap.Error(err))
		return err
	}
	responsePayload := buf[:n]

	// try decrypting with private key
	decryptedPayload, err := decryptWithServerRSA(responsePayload, c.serverPrivateKey)
	if err != nil {
		logger.Error("Error decrypting payload", zap.Error(err))
		return err
	}

	response, err := parseClientHandshake(decryptedPayload)
	if err != nil {
		logger.Error("Error parsing client handshake", zap.Error(err))
		return err
	}
	c.deviceId = response.ClientDeviceKey

	logger.Info("Client handshake received")
	if !bytes.Equal(nonce, response.Nonce[:40]) {
		logger.Error("Nonce mismatch")
		return err
	}
	logger.Info("Nonce matched")

	// now need to create handshake Response
	keybuffer, err := createNonce()
	if err != nil {
		logger.Error("Error creating key block", zap.Error(err))
		return err
	}
	c.aesCipher, err = aes.NewCipher(keybuffer[:16])
	if err != nil {
		logger.Error("Error creating AES cipher", zap.Error(err))
		return err
	}
	c.incomingIv = [16]byte(keybuffer[16:32])
	c.outgoingIv = c.incomingIv

	cyphertext, err := encryptWithClientRSA(keybuffer, response.ClientPublicKey)
	if err != nil {
		logger.Error("Error encrypting payload", zap.Error(err))
		return err
	}

	secondResponse, err := createHmacSignature(cyphertext, keybuffer, c.serverPrivateKey)
	if err != nil {
		logger.Error("Cannot generate hmac", zap.Error(err))
		return err
	}

	// Combine: 128 bytes ciphertext + 256 bytes signature
	bigBlob := make([]byte, len(cyphertext)+len(secondResponse))
	copy(bigBlob, cyphertext)
	copy(bigBlob[len(cyphertext):], secondResponse)

	_, err = (*c.conn).Write(bigBlob)
	if err != nil {
		logger.Error("Error writing Response", zap.Error(err))
		return err
	}

	return nil
}

func createNonce() ([]byte, error) {
	buf := make([]byte, 40)
	_, err := rand.Read(buf)
	return buf, err
}

func decryptWithServerRSA(cyphertext []byte, key *rsa.PrivateKey) ([]byte, error) {
	output, err := rsa.DecryptPKCS1v15(nil, key, cyphertext)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func parseClientHandshake(data []byte) (*ClientResponse, error) {
	// first 40 bytes is nonce, next 12 is device key, rest is public key in der format
	var response ClientResponse
	copy(response.Nonce[:], data[0:40])
	copy(response.ClientDeviceKey[:], data[40:52])
	pubKeyData := data[52:]
	pubKey, err := x509.ParsePKIXPublicKey(pubKeyData)
	if err != nil {
		return nil, err
	}
	response.ClientPublicKey = pubKey.(*rsa.PublicKey)
	return &response, nil
}

func encryptWithClientRSA(data []byte, key *rsa.PublicKey) ([]byte, error) {
	cyphertext, err := rsa.EncryptPKCS1v15(rand.Reader, key, data)
	if err != nil {
		return nil, err
	}
	return cyphertext, nil
}

func pkcs1v15PadRaw(data []byte, keySize int) ([]byte, error) {
	paddingLen := keySize - len(data) - 3

	if paddingLen < 8 {
		return nil, errors.New("data too long for key size")
	}

	padded := make([]byte, keySize)
	padded[0] = 0x00
	padded[1] = 0x01
	for i := 2; i < 2+paddingLen; i++ {
		padded[i] = 0xFF
	}
	padded[2+paddingLen] = 0x00
	copy(padded[3+paddingLen:], data)

	return padded, nil
}

func rsaSignRawPKCS1(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	keySize := priv.Size()

	padded, err := pkcs1v15PadRaw(data, keySize)
	if err != nil {
		return nil, err
	}

	// Convert to big.Int
	m := new(big.Int).SetBytes(padded)

	// RSA private key operation: s = m^d mod n
	s := new(big.Int).Exp(m, priv.D, priv.N)

	// Convert back to bytes, left-pad to key size
	sig := s.Bytes()
	if len(sig) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(sig):], sig)
		sig = padded
	}

	return sig, nil
}

func createHmacSignature(payload []byte, hmacKey []byte, signingKey *rsa.PrivateKey) ([]byte, error) {
	encrypter := hmac.New(sha1.New, hmacKey)
	_, err := encrypter.Write(payload)
	if err != nil {
		zap.L().Error("Error writing encrypted payload", zap.Error(err))
		return nil, err
	}
	hmacDigest := encrypter.Sum(nil)

	signature, err := rsaSignRawPKCS1(signingKey, hmacDigest)
	if err != nil {
		return nil, err
	}

	return signature, nil
}
