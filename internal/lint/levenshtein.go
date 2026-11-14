package lint

func distance(a, b string, max int) int {
	ar, br := []rune(a), []rune(b)
	if abs(len(ar)-len(br)) > max {
		return max + 1
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ra := range ar {
		rowMin := prev[0] + 1
		cur := make([]int, len(br)+1)
		cur[0] = i + 1
		for j, rb := range br {
			cost := 1
			if ra == rb {
				cost = 0
			}
			cur[j+1] = min3(cur[j]+1, prev[j+1]+1, prev[j]+cost)
			if cur[j+1] < rowMin {
				rowMin = cur[j+1]
			}
		}
		if rowMin > max {
			return max + 1
		}
		prev = cur
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
