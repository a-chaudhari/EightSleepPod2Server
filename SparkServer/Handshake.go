package SparkServer

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
)

type ClientResponse struct {
	Nonce           [40]byte
	ClientDeviceKey [12]byte
	ClientPublicKey *rsa.PublicKey
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
	// PKCS#1 v1.5 padding WITHOUT DigestInfo
	// Format: 0x00 0x01 [0xFF...] 0x00 [data]
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
	// Sign using PKCS#1 v1.5 padding WITHOUT DigestInfo
	// Compatible with TropicSSL rsa_pkcs1_verify(..., RSA_RAW, ...)
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
		println("Error writing encrypted payload:", err)
		return nil, err
	}
	hmacDigest := encrypter.Sum(nil)

	signature, err := rsaSignRawPKCS1(signingKey, hmacDigest)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[DEBUG] Signature: %d bytes\n", len(signature))

	return signature, nil
}
