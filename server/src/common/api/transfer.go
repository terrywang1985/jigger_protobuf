package api

// string to uint64

func StringToUint64(s string) uint64 {
	var result uint64
	for _, c := range s {
		result = result*10 + uint64(c-'0')
	}
	return result
}
