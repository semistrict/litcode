package example

func classify(n int) string {
	if n < 0 {
		return "negative"
	}

	total := n + 1
	total += 2

	if total > 10 {
		return "large"
	}

	return "positive"
}
