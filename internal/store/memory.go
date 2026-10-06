package store

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

type entry struct {
	typ    string
	str    string
	list   []string
	hash   map[string]string
	fields []string  // hash field order
	expire time.Time // zero means the key does not expire
}

type shard struct {
	mu      sync.RWMutex
	data    map[string]entry
	expires map[string]struct{} // keys that have a deadline
}

// Memory is a sharded keyspace. Each shard has its own mutex and map.
type Memory struct {
	shards [ShardCount]shard
	now    func() time.Time
}

// Option configures a Memory store.
type Option func(*Memory)

// WithClock sets the clock used for key expiry. It must be set before the
// store is shared. A nil clock is ignored.
func WithClock(now func() time.Time) Option {
	return func(m *Memory) {
		if now != nil {
			m.now = now
		}
	}
}

// New returns an empty keyspace.
func New(opts ...Option) *Memory {
	m := &Memory{now: time.Now}
	for i := range m.shards {
		m.shards[i].data = make(map[string]entry)
		m.shards[i].expires = make(map[string]struct{})
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Set stores a string. It returns false when NX or XX rejects the write.
// SET replaces any previous type and, unless HasTTL is set, clears expiry.
func (m *Memory) Set(key, value string, opt SetOptions) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.alive(key, m.now())
	if opt.NX && exists {
		return false
	}
	if opt.XX && !exists {
		return false
	}
	e := entry{typ: TypeString, str: value}
	if opt.HasTTL {
		e.expire = m.now().Add(opt.TTL)
		s.expires[key] = struct{}{}
	} else {
		delete(s.expires, key)
	}
	s.data[key] = e
	return true
}

// Get returns a string value. Missing keys report ok false.
func (m *Memory) Get(key string) (string, bool, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok {
		return "", false, nil
	}
	if e.typ != TypeString {
		return "", false, ErrWrongType
	}
	return e.str, true, nil
}

// Del removes keys and returns how many existed. A repeated name is removed once.
func (m *Memory) Del(keys []string) int {
	return m.eachKey(keys, func(s *shard, key string, now time.Time) int {
		if _, ok := s.alive(key, now); !ok {
			return 0
		}
		delete(s.data, key)
		delete(s.expires, key)
		return 1
	})
}

// Exists counts arguments that name a live key. Repeated names are counted again.
func (m *Memory) Exists(keys []string) int {
	return m.eachKey(keys, func(s *shard, key string, now time.Time) int {
		if _, ok := s.alive(key, now); !ok {
			return 0
		}
		return 1
	})
}

// MSet replaces each pair. The last pair wins when a key is repeated.
// Previous expiry is cleared.
func (m *Memory) MSet(pairs []Pair) {
	keys := make([]string, len(pairs))
	for i, p := range pairs {
		keys[i] = p.Key
	}
	unlock := m.lockShards(keys)
	defer unlock()
	for _, p := range pairs {
		s := m.shard(p.Key)
		delete(s.expires, p.Key)
		s.data[p.Key] = entry{typ: TypeString, str: p.Value}
	}
}

// MGet returns one item per key, in order. A non-string key fails the whole call.
func (m *Memory) MGet(keys []string) ([]Item, error) {
	unlock := m.lockShards(keys)
	defer unlock()
	now := m.now()
	out := make([]Item, len(keys))
	for i, key := range keys {
		e, ok := m.shard(key).alive(key, now)
		if !ok {
			out[i].Null = true
			continue
		}
		if e.typ != TypeString {
			return nil, ErrWrongType
		}
		out[i].Value = e.str
	}
	return out, nil
}

// IncrBy adds delta to an integer string. A missing key is treated as 0.
// Expiry on an existing key is left in place.
func (m *Memory) IncrBy(key string, delta int64) (int64, error) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := m.now()
	e, ok := s.alive(key, now)
	cur := int64(0)
	if ok {
		if e.typ != TypeString {
			return 0, ErrWrongType
		}
		n, err := strconv.ParseInt(e.str, 10, 64)
		if err != nil {
			return 0, ErrNotInteger
		}
		cur = n
	} else {
		e = entry{typ: TypeString}
	}
	next, ok := addInt64(cur, delta)
	if !ok {
		return 0, ErrNotInteger
	}
	e.typ = TypeString
	e.str = strconv.FormatInt(next, 10)
	s.data[key] = e
	return next, nil
}

// Keys returns live keys matching pattern. Expired keys found during the
// scan are removed. Order is not significant.
func (m *Memory) Keys(pattern string) []string {
	now := m.now()
	var out []string
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		for key, e := range s.data {
			if !e.alive(now) {
				delete(s.data, key)
				continue
			}
			if globMatch(pattern, key) {
				out = append(out, key)
			}
		}
		s.mu.Unlock()
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// Type reports the key type, or TypeNone when the key is missing.
func (m *Memory) Type(key string) string {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.alive(key, m.now())
	if !ok || e.typ == "" {
		return TypeNone
	}
	return e.typ
}

// FlushAll removes every key.
func (m *Memory) FlushAll() {
	for i := range m.shards {
		m.shards[i].mu.Lock()
	}
	defer func() {
		for i := len(m.shards) - 1; i >= 0; i-- {
			m.shards[i].mu.Unlock()
		}
	}()
	for i := range m.shards {
		m.shards[i].data = make(map[string]entry)
		m.shards[i].expires = make(map[string]struct{})
	}
}

// DBSize is the number of entries still in the maps. A key past its deadline
// stays in this count until a command touches it or the active expire pass
// samples it.
func (m *Memory) DBSize() int {
	n := 0
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.RLock()
		n += len(s.data)
		s.mu.RUnlock()
	}
	return n
}

func (m *Memory) shard(key string) *shard {
	return &m.shards[shardIndex(key)]
}

func shardIndex(key string) int {
	// FNV-1a, 32-bit.
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % ShardCount)
}

func (e entry) alive(now time.Time) bool {
	return e.expire.IsZero() || now.Before(e.expire)
}

func (s *shard) alive(key string, now time.Time) (entry, bool) {
	e, ok := s.data[key]
	if !ok {
		return entry{}, false
	}
	if !e.alive(now) {
		delete(s.data, key)
		delete(s.expires, key)
		return entry{}, false
	}
	return e, true
}

// eachKey locks the shards it needs, in index order, and sums fn.
func (m *Memory) eachKey(keys []string, fn func(s *shard, key string, now time.Time) int) int {
	unlock := m.lockShards(keys)
	defer unlock()
	now := m.now()
	n := 0
	for _, key := range keys {
		n += fn(m.shard(key), key, now)
	}
	return n
}

// lockShards locks each distinct shard that holds one of keys, low index first.
func (m *Memory) lockShards(keys []string) func() {
	if len(keys) == 0 {
		return func() {}
	}
	idx := make([]int, len(keys))
	for i, key := range keys {
		idx[i] = shardIndex(key)
	}
	sort.Ints(idx)
	n := 0
	for _, id := range idx {
		if n > 0 && idx[n-1] == id {
			continue
		}
		idx[n] = id
		n++
	}
	idx = idx[:n]
	for _, id := range idx {
		m.shards[id].mu.Lock()
	}
	return func() {
		for i := len(idx) - 1; i >= 0; i-- {
			m.shards[idx[i]].mu.Unlock()
		}
	}
}

func addInt64(a, b int64) (int64, bool) {
	c := a + b
	if (b > 0 && c < a) || (b < 0 && c > a) {
		return 0, false
	}
	return c, true
}
