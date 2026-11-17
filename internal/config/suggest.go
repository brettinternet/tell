package config

import "fmt"

// DidYouMean returns `; did you mean "X"?` for the Closest candidate, or "".
func DidYouMean(typed string, candidates ...string) string {
	if best := Closest(typed, candidates...); best != "" {
		return fmt.Sprintf("; did you mean %q?", best)
	}
	return ""
}

// Closest returns the only candidate a small edit away from typed, or "" when
// none or several are. It only ever returns a candidate, so typed values never
// enter diagnostics through a suggestion.
func Closest(typed string, candidates ...string) string {
	best, bestDistance, ties := "", -1, 0
	for _, candidate := range candidates {
		// Allow one edit per three runes, at most two, and never a full rewrite.
		size := len([]rune(candidate))
		limit := min(max(1, size/3), 2, size-1)
		if gap := len([]rune(typed)) - size; gap > limit || -gap > limit {
			continue
		}
		distance := editDistance(typed, candidate)
		if distance > limit {
			continue
		}
		switch {
		case bestDistance < 0 || distance < bestDistance:
			best, bestDistance, ties = candidate, distance, 1
		case distance == bestDistance && candidate != best:
			ties++
		}
	}
	if ties != 1 {
		return ""
	}
	return best
}

// editDistance is the optimal string alignment distance, so a transposition
// such as "vai" for "via" counts as one edit.
func editDistance(a, b string) int {
	s, t := []rune(a), []rune(b)
	rows := make([][]int, len(s)+1)
	for i := range rows {
		rows[i] = make([]int, len(t)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			cost := 1
			if s[i-1] == t[j-1] {
				cost = 0
			}
			rows[i][j] = min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && s[i-1] == t[j-2] && s[i-2] == t[j-1] {
				rows[i][j] = min(rows[i][j], rows[i-2][j-2]+1)
			}
		}
	}
	return rows[len(s)][len(t)]
}
