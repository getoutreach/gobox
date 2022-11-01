// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Package queue provides queue data struct.
//
// # Priority Queue
//
// PriorityQueue implements container/heap and enriches it with additional features,
// such as Peek, Update, Remove, Contains, List, Clear, etc.
//
// PriorityQueue provides a standard priority queue feature.
//
//	// create priority queue.
//	pq, _ := queue.NewPriorityQueue()
//
//	pq.Push(1, 1) 		// push
//	peekItem := pq.Peek() 	// peek
//	popItem := pq.Pop() 	// pop
//
// Item priority can be updated after it is pushed.
//
//	// change item priority
//	pushItem := pq.Push(1, 1)
//	pushItem.SetPriority(2)
//
//	// update item priority in queue
//	pq.Update(pushItem)
//
// Remove an item in the queue without pop.
//
//	// change item priority
//	pushItem := pq.Push(1, 1)
//
//	// remove item in queue
//	pq.Remove(pushItem)
//
// Default priority uses min heap, smaller value means higher priority. You can
// change the heap type to max heap via option. With max heap, a smaller value means
// lower priority.
//
//	pq, _ := queue.NewPriorityQueue()
//	pq.Push(1, 1)	// higher priority
//	pq.Push(2, 2)	// lower priority
//
//	pq, _ = queue.NewPriorityQueue(queue.WithMaxHeap())
//	pq.Push(1, 1)	// lower priority
//	pq.Push(2, 2)	// higher priority
package queue

import (
	"container/heap"
	"math"
	"sort"
	"sync"

	"github.com/pkg/errors"
)

// This section defines default configuration.
const (
	// DefaultPriorityQueueCapacity is the default priority queue capacity.
	DefaultPriorityQueueCapacity = math.MaxInt32
)

// PriorityQueueOption is used to change priority default configuration.
type PriorityQueueOption func(q *PriorityQueue)

// WithCapacity sets the queue capacity.
func WithCapacity(v int) PriorityQueueOption {
	return func(q *PriorityQueue) {
		q.capacity = v
	}
}

// WithMinHeap sets the priority queue to use min heap. A smaller value means higher priority
// with min heap.
func WithMinHeap() PriorityQueueOption {
	return func(q *PriorityQueue) {
		q.queue.isMinHeap = true
	}
}

// WithMaxHeap sets the priority queue to use max heap. A smaller value means lower priority
// with max heap.
func WithMaxHeap() PriorityQueueOption {
	return func(q *PriorityQueue) {
		q.queue.isMinHeap = false
	}
}

// NewPriorityQueue creates a new priority queue.
func NewPriorityQueue(opts ...PriorityQueueOption) (*PriorityQueue, error) {
	q := &PriorityQueue{
		capacity: DefaultPriorityQueueCapacity,
		queue:    newPriorityQueueInternal(true),
	}
	for _, opt := range opts {
		opt(q)
	}
	if q.capacity <= 0 {
		return nil, errors.Errorf("invalid capacity: %v", q.capacity)
	}
	return q, nil
}

// PriorityQueue implements a priority queue with a heap. By default, min heap is used with a
// default queue capacity. Items are sorted by priority, smaller value means higher priority.
// Queue capacity and heap type can be changed via options.
type PriorityQueue struct {
	lock     sync.RWMutex
	capacity int
	queue    *priorityQueueInternal
}

// Push an item into the queue. Items in the queue will be sorted by priority. Push returns
// a PriorityQueueItem that can be used for other operations, such as updating item priority
// or removing the item from the queue.
func (q *PriorityQueue) Push(data interface{}, priority int64) (*PriorityQueueItem, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.queue.Len() >= q.capacity {
		return nil, errors.New("queue is full")
	}
	item := newPriorityQueueItem(data, priority)
	heap.Push(q.queue, item)
	return item, nil
}

// Pop removes and returns the first item in the queue. Pop returns nil if the queue is empty.
func (q *PriorityQueue) Pop() *PriorityQueueItem {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.queue.Len() <= 0 {
		return nil
	}
	item := heap.Pop(q.queue)
	return item.(*PriorityQueueItem)
}

// Peek returns the first item in the queue. Peek returns nil if the queue is empty.
func (q *PriorityQueue) Peek() *PriorityQueueItem {
	q.lock.RLock()
	defer q.lock.RUnlock()
	if q.queue.Len() <= 0 {
		return nil
	}
	return q.queue.Peek()
}

// Update the item priority in the queue. Call this function when the item priority is
// changed. Return error if the item is not updated successfully.
func (q *PriorityQueue) Update(item *PriorityQueueItem) error {
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.contains(item) {
		return errors.New("item is not in queue")
	}
	heap.Fix(q.queue, item.getIndex())
	return nil
}

// Remove an item from the queue and return the removed item. If the item does not exist in
// queue, return nil.
func (q *PriorityQueue) Remove(item *PriorityQueueItem) *PriorityQueueItem {
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.contains(item) {
		return nil
	}
	index := item.getIndex()
	removedItem := q.queue.Remove(index)
	// Fix the index if removed item is not the last one.
	if index < q.queue.Len() {
		heap.Fix(q.queue, index)
	}
	return removedItem
}

// Contains returns true if the item is in the queue.
func (q *PriorityQueue) Contains(item *PriorityQueueItem) bool {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.contains(item)
}

func (q *PriorityQueue) contains(item *PriorityQueueItem) bool {
	if item == nil {
		return false
	}
	index := item.getIndex()
	if index <= -1 || index >= q.queue.Len() {
		return false
	}
	// Compare the memory address to ensure they are the same item.
	return item == q.queue.Get(index)
}

// Clear removes all items in the queue.
func (q *PriorityQueue) Clear() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue.Clear()
}

// List returns a list of items, sorted by priority.
func (q *PriorityQueue) List() []*PriorityQueueItem {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.queue.List()
}

// Len returns the number of items in the queue.
func (q *PriorityQueue) Len() int {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.queue.Len()
}

// PriorityQueueItem represents an item in queue.
type PriorityQueueItem struct {
	lock     sync.RWMutex
	data     interface{}
	priority int64
	index    int
}

// newPriorityQueueItem creates a new priority queue item.
func newPriorityQueueItem(data interface{}, priority int64) *PriorityQueueItem {
	return &PriorityQueueItem{
		data:     data,
		priority: priority,
	}
}

// GetData returns the data.
func (i *PriorityQueueItem) GetData() interface{} {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.data
}

// SetData sets the data.
func (i *PriorityQueueItem) SetData(v interface{}) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.data = v
}

// GetPriority returns the priority.
func (i *PriorityQueueItem) GetPriority() int64 {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.priority
}

// SetPriority sets priority.
func (i *PriorityQueueItem) SetPriority(v int64) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.priority = v
}

// getIndex returns the index.
func (i *PriorityQueueItem) getIndex() int {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.index
}

// setIndex sets the index.
func (i *PriorityQueueItem) setIndex(v int) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.index = v
}

// _ ensures priorityQueueInternal implements the heap interface.
var _ heap.Interface = (*priorityQueueInternal)(nil)

// priorityQueueInternal implements heap interface. priorityQueueInternal does not
// performs any sanity check. Any sanity check should be done by the caller to ensure
// input data is valid.
type priorityQueueInternal struct {
	isMinHeap bool
	items     []*PriorityQueueItem
}

// newPriorityQueueInternal creates a new priorityQueueInternal.
func newPriorityQueueInternal(isMinHeap bool) *priorityQueueInternal {
	return &priorityQueueInternal{
		isMinHeap: isMinHeap,
		items:     []*PriorityQueueItem{},
	}
}

// Push an item into queue.
func (q *priorityQueueInternal) Push(x interface{}) {
	item := x.(*PriorityQueueItem)
	item.setIndex(q.Len())
	q.items = append(q.items, item)
}

// Pop removes and returns the last item in the queue.
func (q *priorityQueueInternal) Pop() interface{} {
	return q.Remove(q.Len() - 1)
}

// Peek returns the first item in the queue.
func (q priorityQueueInternal) Peek() *PriorityQueueItem {
	return q.items[0]
}

// Remove an item at index i by swapping it with the last item.
func (q *priorityQueueInternal) Remove(i int) *PriorityQueueItem {
	n := q.Len()
	q.Swap(i, n-1)
	item := q.items[n-1]
	item.setIndex(-1)
	q.items[n-1] = nil
	q.items = q.items[0 : n-1]
	return item
}

// Clear emptys the queue.
func (q *priorityQueueInternal) Clear() {
	q.items = []*PriorityQueueItem{}
}

// Get returns an item at index i
func (q *priorityQueueInternal) Get(i int) *PriorityQueueItem {
	return q.items[i]
}

// List all items, sorted by priority.
func (q *priorityQueueInternal) List() []*PriorityQueueItem {
	items := make([]*PriorityQueueItem, q.Len())
	copy(items, q.items)
	sort.SliceStable(items, func(i, j int) bool {
		if q.isMinHeap {
			return items[i].GetPriority() < items[j].GetPriority()
		}
		return items[i].GetPriority() > items[j].GetPriority()
	})
	return items
}

// Len returns the queue size.
func (q priorityQueueInternal) Len() int {
	return len(q.items)
}

// Less compares items at index i and j.
func (q priorityQueueInternal) Less(i, j int) bool {
	if q.isMinHeap {
		return q.items[i].GetPriority() < q.items[j].GetPriority()
	}
	return q.items[i].GetPriority() > q.items[j].GetPriority()
}

// Swap items at index i and j.
func (q priorityQueueInternal) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	tmp := q.items[i].getIndex()
	q.items[i].setIndex(q.items[j].getIndex())
	q.items[j].setIndex(tmp)
}
