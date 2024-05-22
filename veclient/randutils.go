package veclient

import (
	"encoding/hex"
	"io"
)

func randomHexString(rng io.Reader, length int) (string, error) {
	b := make([]byte, length)
	_, err := rng.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}