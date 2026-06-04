package sources

import (
	"crypto/rand"
	"encoding/hex"
)

func randomHex12() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(buf[:])
}
