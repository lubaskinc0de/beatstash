package search_music

type OwnPage[T any] struct {
	Found []T
	More  bool
}

func page[T any](found []T, limit int) *OwnPage[T] {
	found, more := cut(found, limit)
	return &OwnPage[T]{Found: found, More: more}
}

// cut takes what was found by asking for limit+1.
func cut[T any](found []T, limit int) (page []T, more bool) {
	if len(found) > limit {
		return found[:limit], true
	}
	return found, false
}
