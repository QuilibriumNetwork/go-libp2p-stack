package multiaddr

import (
	"bytes"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"
)

// multiaddr is the data structure representing a Multiaddr
type multiaddr struct {
	bytes []byte
}

// NewMultiaddr parses and validates an input string, returning a *Multiaddr
func NewMultiaddr(s string) (a Multiaddr, err error) {
	b, err := stringToBytes(s)
	if err != nil {
		return nil, err
	}
	return &multiaddr{bytes: b}, nil
}

// NewMultiaddrBytes initializes a Multiaddr from a byte representation.
// It validates it as an input string.
func NewMultiaddrBytes(b []byte) (a Multiaddr, err error) {
	if err := validateBytes(b); err != nil {
		return nil, err
	}

	return &multiaddr{bytes: b}, nil
}

// Equal tests whether two multiaddrs are equal
func (m *multiaddr) Equal(m2 Multiaddr) bool {
	if m2 == nil {
		return false
	}
	return bytes.Equal(m.bytes, m2.Bytes())
}

// Bytes returns the []byte representation of this Multiaddr
//
// Do not modify the returned buffer, it may be shared.
func (m *multiaddr) Bytes() []byte {
	return m.bytes
}

// String returns the string representation of a Multiaddr
func (m *multiaddr) String() string {
	s, err := bytesToString(m.bytes)
	if err != nil {
		return ""
	}
	return s
}

func (m *multiaddr) MarshalBinary() ([]byte, error) {
	return m.Bytes(), nil
}

func (m *multiaddr) UnmarshalBinary(data []byte) error {
	new, err := NewMultiaddrBytes(data)
	if err != nil {
		return err
	}
	*m = *(new.(*multiaddr))
	return nil
}

func (m *multiaddr) MarshalText() ([]byte, error) {
	s, err := bytesToString(m.bytes)
	if err != nil {
		return nil, err
	}

	return []byte(s), nil
}

func (m *multiaddr) UnmarshalText(data []byte) error {
	new, err := NewMultiaddr(string(data))
	if err != nil {
		return err
	}
	*m = *(new.(*multiaddr))
	return nil
}

func (m *multiaddr) MarshalJSON() ([]byte, error) {
	s, err := bytesToString(m.bytes)
	if err != nil {
		return nil, err
	}

	return json.Marshal(s)
}

func (m *multiaddr) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	new, err := NewMultiaddr(v)
	*m = *(new.(*multiaddr))
	return err
}

// Protocols returns the list of protocols this Multiaddr has.
func (m *multiaddr) Protocols() []Protocol {
	ps := make([]Protocol, 0, 8)
	b := m.bytes
	for len(b) > 0 {
		code, n, err := ReadVarintCode(b)
		if err != nil {
			return []Protocol{}
		}

		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return []Protocol{}
		}
		ps = append(ps, p)
		b = b[n:]

		n, size, err := sizeForAddr(p, b)
		if err != nil {
			return []Protocol{}
		}

		b = b[n+size:]
	}
	return ps
}

// Encapsulate wraps a given Multiaddr, returning the resulting joined Multiaddr
func (m *multiaddr) Encapsulate(o Multiaddr) Multiaddr {
	if o == nil {
		return m
	}

	mb := m.bytes
	ob := o.Bytes()

	b := make([]byte, len(mb)+len(ob))
	copy(b, mb)
	copy(b[len(mb):], ob)
	return &multiaddr{bytes: b}
}

// Decapsulate unwraps Multiaddr up until the given Multiaddr is found.
func (m *multiaddr) Decapsulate(right Multiaddr) Multiaddr {
	if right == nil {
		return m
	}

	leftParts := Split(m)
	rightParts := Split(right)

	lastIndex := -1
	for i := range leftParts {
		foundMatch := false
		for j, rightC := range rightParts {
			if len(leftParts) <= i+j {
				foundMatch = false
				break
			}

			foundMatch = rightC.Equal(leftParts[i+j])
			if !foundMatch {
				break
			}
		}

		if foundMatch {
			lastIndex = i
		}
	}

	if lastIndex == 0 {
		return nil
	}

	if lastIndex < 0 {
		// if multiaddr not contained, returns a copy.
		cpy := make([]byte, len(m.bytes))
		copy(cpy, m.bytes)
		return &multiaddr{bytes: cpy}
	}

	return Join(leftParts[:lastIndex]...)
}

var ErrProtocolNotFound = fmt.Errorf("protocol not found in multiaddr")

func (m *multiaddr) ValueForProtocol(code int) (value string, err error) {
	err = ErrProtocolNotFound
	ForEach(m, func(c Component, e error) bool {
		if e != nil {
			err = e
			return false
		}
		if c.Protocol().Code == code {
			value = c.Value()
			err = nil
			return false
		}
		return true
	})
	return
}

// FilterAddrs is a filter that removes certain addresses, according to the given filters.
// If all filters return true, the address is kept.
func FilterAddrs(a []Multiaddr, filters ...func(Multiaddr) bool) []Multiaddr {
	b := make([]Multiaddr, 0, len(a))
addrloop:
	for _, addr := range a {
		for _, filter := range filters {
			if !filter(addr) {
				continue addrloop
			}
		}
		b = append(b, addr)
	}
	return b
}

// Contains reports whether addr is contained in addrs.
func Contains(addrs []Multiaddr, addr Multiaddr) bool {
	for _, a := range addrs {
		if addr.Equal(a) {
			return true
		}
	}
	return false
}

// Unique deduplicates addresses in place, leave only unique addresses.
// It doesn't allocate.
func Unique(addrs []Multiaddr) []Multiaddr {
	if len(addrs) == 0 {
		return addrs
	}
	// Use the new slices package here, as the sort function doesn't allocate (sort.Slice does).
	slices.SortFunc(addrs, func(a, b Multiaddr) int { return bytes.Compare(a.Bytes(), b.Bytes()) })
	idx := 1
	for i := 1; i < len(addrs); i++ {
		if !addrs[i-1].Equal(addrs[i]) {
			addrs[idx] = addrs[i]
			idx++
		}
	}
	for i := idx; i < len(addrs); i++ {
		addrs[i] = nil
	}
	return addrs[:idx]
}
