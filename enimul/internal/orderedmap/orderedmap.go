package orderedmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"iter"

	E "github.com/lzpls/enimul/internal/errors"
)

type Map[T any] struct {
	keys   []string
	values map[string]T
}

func (m *Map[T]) Get(key string) (value T, ok bool) {
	if m == nil || m.values == nil {
		return
	}
	value, ok = m.values[key]
	return
}

func (m *Map[T]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

func (m *Map[T]) UnmarshalJSON(data []byte) error {
	if m == nil {
		return E.New("orderedmap: unmarshal nil Map")
	}

	keys := make([]string, 0)
	values := make(map[string]T)

	decoder := json.NewDecoder(bytes.NewReader(data))

	token, err := decoder.Token()
	if err != nil {
		return err
	}

	if token == nil {
		if _, err := decoder.Token(); err != io.EOF {
			if err == nil {
				return E.New("redundant data after JSON null")
			}
			return err
		}

		m.keys = keys
		m.values = values
		return nil
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("expected JSON object, got %v", token)
	}

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}

		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("expected string key, got number %v", keyToken)
		}

		var v T
		if err := decoder.Decode(&v); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}

		keys = append(keys, key)
		values[key] = v
	}

	if _, err := decoder.Token(); err != nil {
		return err
	}

	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return E.New("redundant data after JSON object")
		}
		return err
	}

	m.keys = keys
	m.values = values

	return nil
}

func (m *Map[T]) All() iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		if m == nil {
			return
		}
		for _, key := range m.keys {
			if !yield(key, m.values[key]) {
				return
			}
		}
	}
}
