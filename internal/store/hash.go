package store

func (m *Memory) HSet(key string, fields []Field) (int, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if ok && e.typ != TypeHash {
		return 0, ErrWrongType
	}
	if !ok {
		e = entry{typ: TypeHash, hash: make(map[string]string)}
	}
	if e.hash == nil {
		e.hash = make(map[string]string)
	}
	added := 0
	for _, f := range fields {
		if _, exists := e.hash[f.Name]; !exists {
			e.fields = append(e.fields, f.Name)
			added++
		}
		e.hash[f.Name] = f.Value
	}
	e.typ = TypeHash
	e.str = ""
	e.list = nil
	s.data[key] = e
	return added, nil
}

func (m *Memory) HGet(key, field string) (string, bool, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return "", false, nil
	}
	if e.typ != TypeHash {
		return "", false, ErrWrongType
	}
	v, exists := e.hash[field]
	return v, exists, nil
}

func (m *Memory) HDel(key string, fields []string) (int, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return 0, nil
	}
	if e.typ != TypeHash {
		return 0, ErrWrongType
	}
	removed := 0
	for _, name := range fields {
		if _, exists := e.hash[name]; !exists {
			continue
		}
		delete(e.hash, name)
		removed++
	}
	if removed == 0 {
		return 0, nil
	}
	if len(e.hash) == 0 {
		s.remove(key)
		return removed, nil
	}
	kept := make([]string, 0, len(e.fields))
	for _, name := range e.fields {
		if _, exists := e.hash[name]; exists {
			kept = append(kept, name)
		}
	}
	e.fields = kept
	s.data[key] = e
	return removed, nil
}

func (m *Memory) HGetAll(key string) ([]Field, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return []Field{}, nil
	}
	if e.typ != TypeHash {
		return nil, ErrWrongType
	}
	out := make([]Field, len(e.fields))
	for i, name := range e.fields {
		out[i] = Field{Name: name, Value: e.hash[name]}
	}
	return out, nil
}

func (m *Memory) HExists(key, field string) (bool, error) {
	_, ok, err := m.HGet(key, field)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (m *Memory) HLen(key string) (int, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return 0, nil
	}
	if e.typ != TypeHash {
		return 0, ErrWrongType
	}
	return len(e.hash), nil
}
