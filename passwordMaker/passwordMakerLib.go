package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"bytes"
)

func aesEncrypt(origData string, key []byte, iv []byte) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	origDataBytes := []byte(origData)
	blockSize := block.BlockSize()
	origDataBytes = pkcs5Padding(origDataBytes, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	crypted := make([]byte, len(origDataBytes))
	blockMode.CryptBlocks(crypted, origDataBytes)
	return base64.StdEncoding.EncodeToString(crypted)
}

func pkcs5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}
