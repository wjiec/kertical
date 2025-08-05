package predicate_test

import (
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation/predicate"
)

func TestPtrEquals(t *testing.T) {
	a, b, c := 1, 1, 2
	assert.True(t, predicate.PtrEquals[int](nil, nil))
	assert.True(t, predicate.PtrEquals[int](&a, &a))

	assert.False(t, predicate.PtrEquals[int](nil, &b))
	assert.False(t, predicate.PtrEquals[int](&b, &c))
}

func Numbers(limit int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < limit; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func TestAny(t *testing.T) {
	assert.True(t, predicate.Any(Numbers(10)))
	assert.False(t, predicate.Any[int](func(yield func(int) bool) {}))
}

func IsOdd(v int) bool      { return v&1 == 1 }
func IsNegative(v int) bool { return v < 0 }

func TestFilter(t *testing.T) {
	assert.True(t, predicate.Any(predicate.Filter(Numbers(10), IsOdd)))
	assert.False(t, predicate.Any(predicate.Filter(Numbers(10), IsNegative)))
}

func TestFirst(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		first, rest := predicate.First(Numbers(10), IsOdd)

		assert.Equal(t, 1, first)
		if assert.NotNil(t, rest) {
			var sum int
			for elem := range rest {
				sum += elem
			}
			assert.Equal(t, 3+5+7+9, sum)
		}
	})

	t.Run("not found", func(t *testing.T) {
		first, rest := predicate.First(Numbers(10), IsNegative)

		assert.Zero(t, first)
		assert.NotNil(t, rest)
	})
}

func TestAnd(t *testing.T) {
	if f := predicate.And(IsOdd, IsNegative); assert.NotNil(t, f) {
		first, rest := predicate.First(Numbers(10), f)

		assert.Zero(t, first)
		assert.NotNil(t, rest)
	}
}
