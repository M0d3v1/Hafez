package store

import "time"

// KeyState is one live key copied out of the store for an AOF rewrite.
type KeyState struct {
	Key    string
	Type   string
	Value  string
	List   []string
	Fields []Field
	TTL    time.Duration
	HasTTL bool
}

// Snapshot copies every live key. Expired keys are left in place for the
// expire path to remove. The copy is safe to use after Snapshot returns.
func (m *Memory) Snapshot() []KeyState {
	now := m.now()
	var out []KeyState
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		for key, e := range s.data {
			if !e.alive(now) {
				continue
			}
			out = append(out, keyState(key, e, now))
		}
		s.mu.Unlock()
	}
	return out
}

func keyState(key string, e entry, now time.Time) KeyState {
	st := KeyState{Key: key, Type: e.typ}
	switch e.typ {
	case TypeString:
		st.Value = e.str
	case TypeList:
		st.List = append([]string(nil), e.list...)
	case TypeHash:
		st.Fields = make([]Field, len(e.fields))
		for i, name := range e.fields {
			st.Fields[i] = Field{Name: name, Value: e.hash[name]}
		}
	}
	if !e.expire.IsZero() {
		st.HasTTL = true
		st.TTL = e.expire.Sub(now)
	}
	return st
}
