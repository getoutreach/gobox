package queue

import (
	"math/rand"
	"sort"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestPriorityQueue_MaxHeap(t *testing.T) {
	queue := NewPriorityQueue(WithMaxHeap())
	assert.Assert(t, queue.Len() == 0)
	assert.Assert(t, queue.Peek() == nil)
	assert.Assert(t, queue.Pop() == nil)

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		pushRandNumbersToPriorityQueue(t, queue, n)

		// list
		items := queue.List()
		assert.Assert(t, len(items) == n)

		// peek and pop numbers
		for i := 0; i < n; i++ {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			assert.Assert(t, popItem == peekItem)
			assert.Assert(t, popItem == items[i])
			assert.Assert(t, popItem.GetData() == int64(n-i-1))
			assert.Assert(t, popItem.GetPriority() == int64(n-i-1))
			assert.Assert(t, popItem.getIndex() == -1)
		}
		assert.Assert(t, queue.Len() == 0)
		assert.Assert(t, queue.Peek() == nil)
		assert.Assert(t, queue.Pop() == nil)
	}
}

func TestPriorityQueue_Push_Pop_Peek(t *testing.T) {
	queue := NewPriorityQueue()
	assert.Assert(t, queue.Len() == 0)
	assert.Assert(t, queue.Peek() == nil)
	assert.Assert(t, queue.Pop() == nil)

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		pushRandNumbersToPriorityQueue(t, queue, n)

		// list
		items := queue.List()
		assert.Assert(t, len(items) == n)

		// peek and pop numbers
		for i := 0; i < n; i++ {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			assert.Assert(t, popItem == peekItem)
			assert.Assert(t, popItem == items[i])
			assert.Assert(t, popItem.GetData() == int64(i))
			assert.Assert(t, popItem.GetPriority() == int64(i))
			assert.Assert(t, popItem.getIndex() == -1)
		}
		assert.Assert(t, queue.Len() == 0)
		assert.Assert(t, queue.Peek() == nil)
		assert.Assert(t, queue.Pop() == nil)
	}
}

func TestPriorityQueue_Update(t *testing.T) {
	queue := NewPriorityQueue()

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		items := pushRandNumbersToPriorityQueue(t, queue, n)

		// update priority
		for _, item := range items {
			item.SetPriority(-item.GetPriority())
			queue.Update(item)
		}

		// peek and pop numbers
		for i := n - 1; i >= 0; i-- {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			assert.Assert(t, peekItem == popItem)
			assert.Assert(t, popItem.GetData() == int64(i))
			assert.Assert(t, popItem.GetPriority() == int64(-i))
			assert.Assert(t, popItem.getIndex() == -1)
		}
		assert.Assert(t, queue.Len() == 0)
		assert.Assert(t, queue.Peek() == nil)
		assert.Assert(t, queue.Pop() == nil)
	}
}

func TestPriorityQueue_Remove(t *testing.T) {
	queue := NewPriorityQueue()

	n := 1000
	m := n / 2
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		items := pushRandNumbersToPriorityQueue(t, queue, n)
		itemMap := make(map[int64]*PriorityQueueItem)
		for _, item := range items {
			itemMap[item.GetPriority()] = item
		}

		// remove some items in queue
		for j := 0; j < m; j++ {
			removedItem := queue.Remove(items[j])

			assert.Assert(t, removedItem != nil)
			assert.Assert(t, removedItem.GetData() == items[j].GetData())
			assert.Assert(t, removedItem.GetPriority() == items[j].GetPriority())

			delete(itemMap, removedItem.GetPriority())
		}

		// sort the items in queue by priority
		itemsInQueue := items[m:]
		sort.SliceStable(itemsInQueue, func(i, j int) bool {
			return itemsInQueue[i].GetPriority() < itemsInQueue[j].GetPriority()
		})

		// peek and pop numbers
		popCnt := 0
		for {
			peekItem := queue.Peek()
			popItem := queue.Pop()
			assert.Assert(t, peekItem == popItem)
			if popItem == nil {
				break
			}

			assert.Assert(t, popItem.GetData() == itemsInQueue[popCnt].GetData())
			assert.Assert(t, popItem.GetPriority() == itemsInQueue[popCnt].GetPriority())
			assert.Assert(t, popItem.getIndex() == -1)
			popCnt++

			// pop item should be in map
			assert.Assert(t, cmp.Contains(itemMap, popItem.GetPriority()))
			delete(itemMap, popItem.GetPriority())
		}
		assert.Assert(t, queue.Len() == 0)
		assert.Assert(t, queue.Peek() == nil)
		assert.Assert(t, queue.Pop() == nil)
		assert.Assert(t, popCnt == n-m)
	}
}

func TestPriorityQueue_Contains(t *testing.T) {
	queue1 := NewPriorityQueue()
	queue2 := NewPriorityQueue()

	testContain := func(items []*PriorityQueueItem, inQueue1, inQueue2 bool) {
		for _, item := range items {
			assert.Assert(t, inQueue1 == queue1.Contains(item))
			assert.Assert(t, inQueue2 == queue2.Contains(item))
		}
	}

	n := 1000
	repeat := 3
	for i := 0; i < repeat; i++ {
		// push numbers in random order
		items1 := pushRandNumbersToPriorityQueue(t, queue1, n)
		items2 := pushRandNumbersToPriorityQueue(t, queue2, n)

		// test contains
		testContain(items1, true, false)
		testContain(items2, false, true)

		// remove all items
		for queue1.Pop() != nil {
		}
		for queue2.Pop() != nil {
		}

		// test contains
		testContain(items1, false, false)
		testContain(items2, false, false)
	}
}

func TestPriorityQueue_Clear(t *testing.T) {
	queue := NewPriorityQueue()
	assert.Assert(t, queue.Len() == 0)

	n := 100
	pushRandNumbersToPriorityQueue(t, queue, n)
	assert.Assert(t, queue.Len() == n)

	queue.Clear()
	assert.Assert(t, queue.Len() == 0)
}

func TestPriorityQueue_List(t *testing.T) {
	queue := NewPriorityQueue()
	assert.Assert(t, queue.Len() == 0)

	n := 100
	pushRandNumbersToPriorityQueue(t, queue, n)
	items := queue.List()
	assert.Assert(t, len(items) == n)
	for i := 0; i < n; i++ {
		assert.Assert(t, items[i].GetData() == int64(i))
		assert.Assert(t, items[i].GetPriority() == int64(i))
	}
	assert.Assert(t, queue.Len() == n)
}

func TestPriorityQueue_Push_Error(t *testing.T) {
	size := 3
	queue := NewPriorityQueue(WithCapacity(uint(size)))

	// push
	n := size * 2
	for i := 0; i < n; i++ {
		item, err := queue.Push(i, int64(i))
		if i < size {
			assert.Assert(t, err == nil)
			assert.Assert(t, item != nil)
			continue
		}
		assert.Assert(t, item == nil)
		assert.Assert(t, err != nil)
	}
	assert.Assert(t, queue.Len() == size)

	// pop
	for i := 0; i < size; i++ {
		assert.Assert(t, queue.Pop().GetData() == i)
	}
	assert.Assert(t, queue.Len() == 0)
	assert.Assert(t, queue.Peek() == nil)
	assert.Assert(t, queue.Pop() == nil)
}

func TestPriorityQueue_Update_Error(t *testing.T) {
	queue := NewPriorityQueue()

	// push
	item, err := queue.Push(1, 1)
	assert.Assert(t, err == nil)

	// update pass
	assert.Assert(t, queue.Update(item) == nil)

	// update nil
	assert.Assert(t, queue.Update(nil) != nil)

	// update invalid item
	assert.Assert(t, queue.Update(&PriorityQueueItem{}) != nil)

	// update removed item
	removedItem := queue.Remove(item)
	assert.Assert(t, removedItem != nil)
	assert.Assert(t, queue.Update(removedItem) != nil)
}

func TestPriorityQueue_Remove_Invalid_Item(t *testing.T) {
	queue := NewPriorityQueue()

	// push
	item, err := queue.Push(1, 1)
	assert.Assert(t, err == nil)

	// remove nil
	assert.Assert(t, queue.Remove(nil) == nil)
	assert.Assert(t, queue.Len() == 1)

	// remove invalid item
	assert.Assert(t, queue.Remove(&PriorityQueueItem{}) == nil)
	assert.Assert(t, queue.Len() == 1)

	// remove valid item
	assert.Assert(t, queue.Remove(item) == item)
	assert.Assert(t, queue.Len() == 0)

	// remove removed item
	assert.Assert(t, queue.Remove(item) == nil)
	assert.Assert(t, queue.Len() == 0)
}

// push N numbers into queue in random push order.
func pushRandNumbersToPriorityQueue(t *testing.T, queue *PriorityQueue, n int) []*PriorityQueueItem {
	nums := make([]int64, n)
	for i := 0; i < n; i++ {
		nums[i] = int64(i)
	}
	rand.Shuffle(n, func(i, j int) {
		nums[i], nums[j] = nums[j], nums[i]
	})

	items := []*PriorityQueueItem{}
	for _, v := range nums {
		item, err := queue.Push(v, v)
		items = append(items, item)
		assert.Assert(t, err == nil)
		assert.Assert(t, v == item.GetData())
		assert.Assert(t, v == item.GetPriority())
	}
	assert.Assert(t, queue.Len() == n)
	return items
}
