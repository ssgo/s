package base

import (
	"encoding/base64"
	"crypto/aes"
	"crypto/cipher"
	"bytes"
	"fmt"
)

func EncryptAes(origData string, key []byte, iv []byte) string {
	key, iv = makeKeyIv(key, iv)
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

func DecryptAes(crypted string, key []byte, iv []byte) string {
	key, iv = makeKeyIv(key, iv)
	cryptedBytes, err := base64.StdEncoding.DecodeString(crypted)
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(cryptedBytes))
	blockMode.CryptBlocks(origData, cryptedBytes)
	origData = pkcs5UnPadding(origData)
	return string(origData)
}

func pkcs5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func makeKeyIv(key []byte, iv []byte) ([]byte, []byte) {
	if len(key) >= 32 {
		key = key[0:32]
	}else if len(key) >= 16 {
		key = key[0:16]
	}else{
		for i:=len(key); i<16; i++ {
			key = append(key, 0)
		}
	}
	if len(iv) < len(key){
		for i:=len(iv); i<len(key); i++ {
			iv = append(iv, 0)
		}
	}
	return key, iv
}
