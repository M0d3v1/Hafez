package store

// globMatch reports whether s matches a Redis KEYS pattern.
// * is any sequence, ? is one byte, \ escapes the next byte, and [...] is a class.
// An unclosed [ is a literal bracket.
func globMatch(pattern, s string) bool {
	return matchGlob([]byte(pattern), []byte(s))
}

func matchGlob(p, s []byte) bool {
	for {
		if len(p) == 0 {
			return len(s) == 0
		}
		switch p[0] {
		case '*':
			p = p[1:]
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if matchGlob(p, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			p, s = p[1:], s[1:]
		case '[':
			if len(s) == 0 {
				return false
			}
			matched, rest, ok := matchClass(p, s[0])
			if !ok {
				if s[0] != '[' {
					return false
				}
				p, s = p[1:], s[1:]
				continue
			}
			if !matched {
				return false
			}
			p = rest
			s = s[1:]
		case '\\':
			if len(p) == 1 {
				return len(s) == 1 && s[0] == '\\'
			}
			if len(s) == 0 || s[0] != p[1] {
				return false
			}
			p, s = p[2:], s[1:]
		default:
			if len(s) == 0 || s[0] != p[0] {
				return false
			}
			p, s = p[1:], s[1:]
		}
	}
}

// matchClass parses a class at p[0] == '['. ok is false when the class is unclosed.
func matchClass(p []byte, c byte) (matched bool, rest []byte, ok bool) {
	i := 1
	neg := false
	if i < len(p) && p[i] == '^' {
		neg = true
		i++
	}
	if i >= len(p) {
		return false, nil, false
	}
	closeAt := -1
	for j := i; j < len(p); j++ {
		if p[j] == ']' && j != i {
			closeAt = j
			break
		}
	}
	if closeAt < 0 {
		return false, nil, false
	}
	hit := classContains(p[i:closeAt], c)
	if neg {
		hit = !hit
	}
	return hit, p[closeAt+1:], true
}

func classContains(body []byte, c byte) bool {
	for i := 0; i < len(body); i++ {
		if i+2 < len(body) && body[i+1] == '-' {
			lo, hi := body[i], body[i+2]
			if lo > hi {
				lo, hi = hi, lo
			}
			if c >= lo && c <= hi {
				return true
			}
			i += 2
			continue
		}
		if body[i] == c {
			return true
		}
	}
	return false
}
