package addrtrie

import (
	"errors"
	"strings"
)

type labelNode[T any] struct {
	children      map[string]*labelNode[T]
	exactValue    T
	hasExact      bool
	wildcardValue T
	hasWildcard   bool
}

type DomainMatcher[T any] struct {
	root *labelNode[T]
}

func NewDomainMatcher[T any]() *DomainMatcher[T] {
	return &DomainMatcher[T]{
		root: &labelNode[T]{children: make(map[string]*labelNode[T])},
	}
}

func (m *DomainMatcher[T]) insertPattern(pattern string, value T, isWildcard, isExact bool) {
	node := m.root
	for i := len(pattern) - 1; i >= 0; {
		j := strings.LastIndexByte(pattern[:i+1], '.')
		var label string
		if j == -1 {
			label = pattern[:i+1]
			i = -1
		} else {
			label = pattern[j+1 : i+1]
			i = j - 1
		}

		if node.children[label] == nil {
			node.children[label] = &labelNode[T]{children: make(map[string]*labelNode[T])}
		}
		node = node.children[label]
	}

	if isWildcard {
		node.wildcardValue = value
		node.hasWildcard = true
	}
	if isExact {
		node.exactValue = value
		node.hasExact = true
	}
}

func (m *DomainMatcher[T]) Add(pattern string, value T) error {
	if pattern == "*" {
		m.root.wildcardValue = value
		m.root.hasWildcard = true
		return nil
	}
	if !strings.Contains(pattern, ".") {
		return errors.New("invalid pattern: " + pattern)
	}

	switch {
	case strings.HasPrefix(pattern, "*."):
		m.insertPattern(pattern[2:], value, true, false)
	case strings.HasPrefix(pattern, "*"):
		m.insertPattern(pattern[1:], value, true, true)
	default:
		m.insertPattern(pattern, value, false, true)
	}
	return nil
}

func (m *DomainMatcher[T]) Find(domain string) (matched T, exists bool) {
	node := m.root
	var candidate T
	hasCandidate := false
	fullyMatched := true

	for i := len(domain) - 1; i >= 0; {
		j := strings.LastIndexByte(domain[:i+1], '.')
		var label string
		if j == -1 {
			label = domain[:i+1]
		} else {
			label = domain[j+1 : i+1]
		}

		child, ok := node.children[label]
		if !ok {
			fullyMatched = false
			break
		}
		node = child

		if j != -1 && node.hasWildcard {
			candidate = node.wildcardValue
			hasCandidate = true
		}

		if j == -1 {
			i = -1
		} else {
			i = j - 1
		}
	}

	if fullyMatched && node.hasExact {
		return node.exactValue, true
	}
	if hasCandidate {
		return candidate, true
	}
	if m.root.hasWildcard {
		return m.root.wildcardValue, true
	}
	return matched, false
}
