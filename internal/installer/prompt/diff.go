package prompt

import "strings"

type diffLine struct {
	op   byte
	text string
}

// context is how many unchanged lines stay around each change.
const context = 2

// lineDiff is a longest-common-subsequence diff; the files it compares are
// a few hundred lines at most.
func lineDiff(before, after string) []diffLine {
	a := strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	b := strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	if before == "" {
		a = nil
	}
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var all []diffLine
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			all = append(all, diffLine{' ', a[i]})
			i++
			j++
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			all = append(all, diffLine{'+', b[j]})
			j++
		default:
			all = append(all, diffLine{'-', a[i]})
			i++
		}
	}
	var shown []diffLine
	for k, l := range all {
		if l.op != ' ' || near(all, k) {
			shown = append(shown, l)
		}
	}
	return shown
}

func near(all []diffLine, k int) bool {
	for d := max(0, k-context); d <= min(len(all)-1, k+context); d++ {
		if all[d].op != ' ' {
			return true
		}
	}
	return false
}
