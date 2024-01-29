package kmp

// Knuth-Morris-Pratt (KMP) algorithm
func BytePatternSearch(data []byte, search []byte, startOffset int64) int {
	if startOffset < 0 || startOffset >= int64(len(data)) || len(search) == 0 {
		return -1
	}

	lps := computeLPSArray(search)

	i, j := startOffset, 0

	for i < int64(len(data)) {
		if search[j] == data[i] {
			i++
			j++
		}

		if j == len(search) {
			return int(i) - j
		} else if i < int64(len(data)) && search[j] != data[i] {
			if j != 0 {
				j = lps[j-1]
			} else {
				i++
			}
		}
	}

	return -1
}

func computeLPSArray(pattern []byte) []int {
	length := 0
	lps := make([]int, len(pattern))
	lps[0] = 0
	i := 1

	for i < len(pattern) {
		if pattern[i] == pattern[length] {
			length++
			lps[i] = length
			i++
		} else {
			if length != 0 {
				length = lps[length-1]
			} else {
				lps[i] = 0
				i++
			}
		}
	}

	return lps
}
