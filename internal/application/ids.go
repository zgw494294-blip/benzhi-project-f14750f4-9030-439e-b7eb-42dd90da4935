package application

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic("secure random source unavailable: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(bytes)
}
