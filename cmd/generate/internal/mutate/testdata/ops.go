package ops

func compare(a, b int) bool {
	return a == b || a != b && a < b
}

func bounds(a, b int) bool {
	return a <= b && a > b || a >= b
}

func step(i int) int {
	return i + 1 - 1 + 2
}

func flags() (bool, bool) {
	return true, false
}

func check(ok bool) error {
	if ok {
		return nil
	}
	f := func() error { return nil }
	g := func() *int { return nil }
	_ = g
	return f()
}

func notError() *int {
	return nil
}

func loop(xs []int) {
	for _, x := range xs {
		if x == 0 {
			continue
		}
		if x == 1 {
			break
		}
	}
outer:
	for range xs {
		continue outer
	}
}
