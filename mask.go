// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import "fmt"

// maskType is a bitmask used to represent a set of component types.
type maskType [maskWords]uint64

// has checks if the mask has a specific component ID.
func (self maskType) has(id ComponentID) bool {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return false
	}
	bit := id % bitsPerWord
	return (self[word] & (1 << bit)) != 0
}

// setMask adds a component ID to the mask.
func setMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		panic(fmt.Sprintf("component ID %d exceeds maximum (%d)", id, maxComponentTypes))
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] |= (1 << bit)
	return nm
}

// unsetMask removes a component ID from the mask.
func unsetMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return m
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] &^= (1 << bit)
	return nm
}

// orMask performs a bitwise OR between two masks.
func orMask(m1, m2 maskType) maskType {
	var nm maskType
	for i := 0; i < maskWords; i++ {
		nm[i] = m1[i] | m2[i]
	}
	return nm
}

// andNotMask performs a bitwise AND NOT (m1 &^ m2) between two masks.
func andNotMask(m1, m2 maskType) maskType {
	var nm maskType
	for i := 0; i < maskWords; i++ {
		nm[i] = m1[i] &^ m2[i]
	}
	return nm
}

// makeMask creates a mask from a slice of component IDs.
func makeMask(ids []ComponentID) maskType {
	var m maskType
	for _, id := range ids {
		word := int(id / bitsPerWord)
		bit := id % bitsPerWord
		m[word] |= (1 << bit)
	}
	return m
}

// makeMask1 creates a mask for a single component ID.
func makeMask1(id1 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	return m
}

// makeMask2 creates a mask for two component IDs.
func makeMask2(id1, id2 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	return m
}

// makeMask3 creates a mask for three component IDs.
func makeMask3(id1, id2, id3 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	return m
}

// makeMask4 creates a mask for four component IDs.
func makeMask4(id1, id2, id3, id4 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	return m
}

// makeMask5 creates a mask for five component IDs.
func makeMask5(id1, id2, id3, id4, id5 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	word5 := int(id5 / bitsPerWord)
	bit5 := id5 % bitsPerWord
	m[word5] |= (1 << bit5)
	return m
}

// includesAll checks if a mask contains all the bits of another mask.
func includesAll(m, include maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & include[i]) != include[i] {
			return false
		}
	}
	return true
}

// intersects checks if a mask has any bits in common with another mask.
func intersects(m, exclude maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & exclude[i]) != 0 {
			return true
		}
	}
	return false
}
