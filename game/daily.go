package game

import (
	"crypto/sha256"
	"encoding/binary"
)

func DailySeed(date string) int64 {
	h := sha256.Sum256([]byte("catacombs:" + date))
	return int64(binary.BigEndian.Uint64(h[:8]))
}
