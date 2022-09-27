package queue

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue_Push_Pop_Peek(t *testing.T) {
	ast := assert.New(t)
	queue := NewPriorityQueue()
	ast.Equal(0, queue.Len())
	ast.Nil(queue.Peek())
	ast.Nil(queue.Pop())

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		pushRandN2PiroirtyQueue(ast, queue, n)

		// peek and pop numbers
		for i := 0; i < n; i++ {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			ast.Equal(peekItem, popItem)

			ast.EqualValues(i, popItem.GetData())
			ast.EqualValues(i, popItem.GetPriority())
			ast.EqualValues(-1, popItem.getIndex())
		}
		ast.Equal(0, queue.Len())
		ast.Nil(queue.Peek())
		ast.Nil(queue.Pop())
	}
}

func TestPriorityQueue_Update(t *testing.T) {
	ast := assert.New(t)
	queue := NewPriorityQueue()
	ast.Equal(0, queue.Len())
	ast.Nil(queue.Peek())
	ast.Nil(queue.Pop())

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		items := pushRandN2PiroirtyQueue(ast, queue, n)

		// update priority
		for _, item := range items {
			item.SetPriority(-item.GetPriority())
			queue.Update(item)
		}

		// peek and pop numbers
		for i := n - 1; i >= 0; i-- {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			ast.Equal(peekItem, popItem)

			ast.EqualValues(i, popItem.GetData())
			ast.EqualValues(-i, popItem.GetPriority())
			ast.EqualValues(-1, popItem.getIndex())
		}
		ast.Equal(0, queue.Len())
		ast.Nil(queue.Peek())
		ast.Nil(queue.Pop())
	}
}

func TestPriorityQueue_InvalidSize(t *testing.T) {
	ast := assert.New(t)
	test := func(size, exp int, expPanic bool) {
		hasPanic := false
		defer func() {
			if err := recover(); err != nil {
				hasPanic = true
			}
			ast.Equal(expPanic, hasPanic)
		}()
		queue := NewPriorityQueue(WithCapacity(size))
		ast.Equal(exp, queue.(*priorityQueue).capacity)
	}
	test(-1, defPriorityQueueCapacity, true)
	test(0, defPriorityQueueCapacity, true)
	test(1, 1, false)
}

func TestPriorityQueue_Push_Error(t *testing.T) {
	ast := assert.New(t)
	size := 3
	queue := NewPriorityQueue(WithCapacity(size))

	// push
	n := size * 2
	for i := 0; i < n; i++ {
		item, err := queue.Push(NewPriorityQueueItem(i, int64(i)))
		if i < size {
			ast.Nil(err)
			ast.NotNil(item)
			continue
		}
		ast.Nil(item)
		ast.NotNil(err)
		ast.IsType(new(MaxCapacityError), err)
	}
	ast.Equal(size, queue.Len())

	// pop
	for i := 0; i < size; i++ {
		ast.EqualValues(i, queue.Pop().GetData())
	}
	ast.Equal(0, queue.Len())
	ast.Nil(queue.Peek())
	ast.Nil(queue.Pop())
}

// push N numbers into queue in random push order.
func pushRandN2PiroirtyQueue(ast *assert.Assertions, queue PriorityQueue, n int) []PriorityQueueIndexItem {
	nums := make([]int64, n)
	for i := 0; i < n; i++ {
		nums[i] = int64(i)
	}
	rand.Shuffle(n, func(i, j int) {
		nums[i], nums[j] = nums[j], nums[i]
	})

	items := []PriorityQueueIndexItem{}
	for _, v := range nums {
		item, err := queue.Push(NewPriorityQueueItem(v, v))
		items = append(items, item)
		ast.Nil(err)
		ast.NotNil(item)
		ast.EqualValues(v, item.GetData())
		ast.EqualValues(v, item.GetPriority())
	}
	ast.Equal(n, queue.Len())
	return items
}
