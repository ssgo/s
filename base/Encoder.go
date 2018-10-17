package base

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
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
	} else if len(key) >= 16 {
		key = key[0:16]
	} else {
		for i := len(key); i < 16; i++ {
			key = append(key, 0)
		}
	}
	if len(iv) < len(key) {
		for i := len(iv); i < len(key); i++ {
			iv = append(iv, 0)
		}
	}
	return key, iv
}

const digits = "9ukH1grX75TQS6LzpFAjIivsdZoO0m_c8NBwnyYDhtMWEC2V3KaGxfJRPqe4lbU"

var Rander = rand.New(rand.NewSource(int64(time.Now().Nanosecond() * 217)))

func UniqueId() string {
	var a [64]byte
	i := len(a)
	rander2 := rand.New(rand.NewSource(int64(time.Now().Nanosecond() * 217)))
	appendInt(&a, &i, rander2.Uint64())
	appendByte(&a, &i, '-')

	ratio := int64(Rander.Intn(62) + 1)
	appendInt(&a, &i, uint64(time.Now().UnixNano()/1000*ratio))
	appendByte(&a, &i, digits[ratio])
	appendByte(&a, &i, '-')

	appendInt(&a, &i, Rander.Uint64())
	return string(a[i:])
}

func appendByte(a *[64]byte, i *int, b byte) {
	*i--
	if *i >= 0 {
		(*a)[*i] = b
	}
}
func appendInt(a *[64]byte, i *int, u uint64) {
	for u >= 63 {
		q := u / 63
		appendByte(a, i, digits[uint(u-q*63)])
		u = q
	}
	appendByte(a, i, digits[uint(u)])
}

func EncodeInt(u uint64) string {
	var a [64]byte
	i := len(a)
	appendInt(&a, &i, u)
	return string(a[i:])
}

func DecodeInt(s string) uint64 {
	var r uint64 = 0
	var ratio uint64 = 0
	for i := len(s) - 1; i >= 0; i-- {
		c := uint64(strings.IndexByte(digits, s[i]))
		if ratio == 0 {
			r += c
			ratio = 63
		} else {
			r += c * ratio
			ratio *= 63
		}
	}
	return r
}
