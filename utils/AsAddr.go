package utils

import "strconv"

func AsAddr(port uint16) string {
	return ":" + strconv.Itoa(int(port))
}
