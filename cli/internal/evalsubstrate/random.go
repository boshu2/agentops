package evalsubstrate

import (
	"crypto/rand"
	"encoding/hex"
)

func randomSuffix(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "000000"[:n]
	}
	return hex.EncodeToString(b)[:n]
}
