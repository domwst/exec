package common

import (
	"math/rand"
	"strconv"
)

func GetRandomId() string {
	part := func() string {
		return strconv.FormatUint(rand.Uint64(), 16)
	}
	return part() + part()
}
