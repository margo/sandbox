package helpers

import (
	"crypto/rand"
	"math/big"
)

func NewSerialNumber() *big.Int {
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return serial
}
