package utils

import (
	"crypto/sha512"
	"fmt"
	"time"
)

func GenerateKey(originalToken string) string {
	sha := sha512.New()
	sha.Write([]byte(fmt.Sprintf("%s_%d", originalToken, time.Now().UnixMilli())))
	return fmt.Sprintf("%x", sha.Sum(nil))
}
