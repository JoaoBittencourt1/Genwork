package platform

import "math"

// EndOfList is an upper bound for open-ended position ranges.
const EndOfList = math.MaxInt32

// ClampPosition keeps a requested position inside [0, last]; a position past the end means "last".
func ClampPosition(position, last int) int {
	return max(0, min(position, last))
}

// ReorderWithin shifts the neighbours of an item that moves from one position to another of the same ordered list.
func ReorderWithin(from, to int, shift func(from, to, delta int) error) error {
	switch {
	case to > from:
		return shift(from+1, to, -1)
	case to < from:
		return shift(to, from-1, 1)
	default:
		return nil
	}
}
