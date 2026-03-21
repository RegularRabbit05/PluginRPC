package utils

import (
	"strconv"
)

func StrTPort(port string) uint16 {
	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0
	}

	if p < 0 || p > 65535 {
		return 0
	}

	return uint16(p)
}
