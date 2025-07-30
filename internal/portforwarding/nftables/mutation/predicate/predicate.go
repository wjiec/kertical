package predicate

import "iter"

// PtrEquals compares two pointers of the same comparable type for equality.
//
// Returns true if both pointers are nil or if their values are equal.
func PtrEquals[T comparable](lhs, rhs *T) bool {
	if lhs == nil || rhs == nil {
		return lhs == rhs
	}
	return *lhs == *rhs
}

// Any determines if any element exists in the provided sequence.
//
// Returns true if at least one element is found, false if the sequence is empty.
func Any[T any](seq iter.Seq[T]) bool {
	next, stop := iter.Pull(seq)
	defer stop()

	_, found := next()
	return found
}

// Filter creates a new sequence containing only elements that satisfy the predicate.
//
// Returns a new sequence with only the elements for which the predicate returns true.
// The filtering happens lazily when the returned sequence is consumed.
func Filter[T any](seq iter.Seq[T], predicate func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for elem := range seq {
			if predicate(elem) {
				if !yield(elem) {
					return
				}
			}
		}
	}
}

// First finds the first element in a sequence that satisfies the predicate.
//
// Returns the first matching element and a sequence containing the remaining elements.
// If no element matches, returns the zero value of T and an empty sequence.
func First[T any](seq iter.Seq[T], predicate func(T) bool) (v T, _ iter.Seq[T]) {
	next, stop := iter.Pull(seq)
	for {
		if first, ok := next(); ok && predicate(first) {
			return first, func(yield func(T) bool) {
				defer stop()
				for elem := v; ok; {
					if elem, ok = next(); ok && predicate(elem) {
						if !yield(elem) {
							return
						}
					}
				}
			}
		} else if !ok {
			break
		}
	}

	stop()
	return v, func(func(T) bool) {}
}

// And creates a composite predicate function that performs a logical AND operation
// on all the provided predicates. It returns a new predicate that evaluates to true
// only if all the component predicates evaluate to true.
func And[T any](predicates ...func(T) bool) func(T) bool {
	return func(t T) bool {
		for _, predicate := range predicates {
			if !predicate(t) {
				return false
			}
		}
		return true
	}
}
