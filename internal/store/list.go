package store

func (m *Memory) LPush(key string, values []string) (int, error) {
	return m.pushList(key, values, true)
}

func (m *Memory) RPush(key string, values []string) (int, error) {
	return m.pushList(key, values, false)
}

func (m *Memory) pushList(key string, values []string, left bool) (int, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if ok && e.typ != TypeList {
		return 0, ErrWrongType
	}
	if !ok {
		e = entry{typ: TypeList}
	}
	e.typ = TypeList
	e.str = ""
	e.hash = nil
	e.fields = nil
	e.list = appendValues(e.list, values, left)
	s.data[key] = e
	return len(e.list), nil
}

func appendValues(list, values []string, left bool) []string {
	if len(values) == 0 {
		return list
	}
	if !left {
		out := make([]string, len(list), len(list)+len(values))
		copy(out, list)
		return append(out, values...)
	}
	out := make([]string, len(list)+len(values))
	for i, v := range values {
		out[len(values)-1-i] = v
	}
	copy(out[len(values):], list)
	return out
}

func (m *Memory) LPop(key string) (string, bool, error) {
	return m.popList(key, true)
}

func (m *Memory) RPop(key string) (string, bool, error) {
	return m.popList(key, false)
}

func (m *Memory) popList(key string, left bool) (string, bool, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return "", false, nil
	}
	if e.typ != TypeList {
		return "", false, ErrWrongType
	}
	if len(e.list) == 0 {
		s.remove(key)
		return "", false, nil
	}
	var value string
	if left {
		value = e.list[0]
		e.list = e.list[1:]
	} else {
		last := len(e.list) - 1
		value = e.list[last]
		e.list = e.list[:last]
	}
	if len(e.list) == 0 {
		s.remove(key)
		return value, true, nil
	}
	s.data[key] = e
	return value, true, nil
}

func (m *Memory) LLen(key string) (int, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return 0, nil
	}
	if e.typ != TypeList {
		return 0, ErrWrongType
	}
	return len(e.list), nil
}

func (m *Memory) LRange(key string, start, stop int64) ([]string, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return []string{}, nil
	}
	if e.typ != TypeList {
		return nil, ErrWrongType
	}
	from, to, ok := listBounds(len(e.list), start, stop)
	if !ok {
		return []string{}, nil
	}
	out := make([]string, to-from)
	copy(out, e.list[from:to])
	return out, nil
}

// listBounds returns a half-open interval. Negative indexes count from the tail.
func listBounds(n int, start, stop int64) (int, int, bool) {
	if n == 0 {
		return 0, 0, false
	}
	if start < 0 {
		start += int64(n)
		if start < 0 {
			start = 0
		}
	}
	if stop < 0 {
		stop += int64(n)
		if stop < 0 {
			return 0, 0, false
		}
	}
	if start >= int64(n) || start > stop {
		return 0, 0, false
	}
	if stop >= int64(n) {
		stop = int64(n) - 1
	}
	return int(start), int(stop) + 1, true
}

func (s *shard) remove(key string) {
	delete(s.data, key)
	delete(s.expires, key)
}
