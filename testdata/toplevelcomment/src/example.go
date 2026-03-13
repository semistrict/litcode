package example

func first() string {
	return "first"
}

// second returns the other label.
func second() string {
	return "second"
}

func combined() string {
	value := first()
	// This nested comment belongs with the implementation.
	if value == "" {
		return second()
	}
	return value
}
