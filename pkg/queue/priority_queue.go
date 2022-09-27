// Copyright 2022 Outreach Corporation. All Rights Reserved.
//
// Description: Priority queue.
package queue

import (
	"container/heap"
	"math"

	"github.com/pkg/errors"
)

// const
const (
	// defPriorityQueueCapacity
	defPriorityQueueCapacity = math.MaxInt32
)

// PriorityQueue provides priority queue feature.
type PriorityQueue interface {
	Push(item PriorityQueueItem) (PriorityQueueIndexItem, error)
	Pop() PriorityQueueIndexItem
	Peek() PriorityQueueIndexItem
	Update(item PriorityQueueIndexItem)
	Len() int
}

// PriorityQueueItem is required to be implemented for the item that is push
// into the priority queue. Priority with smaller value has higher priority.
type PriorityQueueItem interface {
	GetData() interface{}
	SetPriority(v int64)
	GetPriority() int64
}

// PriorityQueueIndexItem provides additional access to the item index. It is used
// by the priority queue internally.
type PriorityQueueIndexItem interface {
	PriorityQueueItem

	// private method that is used by priority queue internally.
	getIndex() int
	setIndex(v int)
}

// PriorityQueueOption
type PriorityQueueOption func(q *priorityQueue)

// WithCapacity sets the queue capacity.
func WithCapacity(v int) PriorityQueueOption {
	return func(q *priorityQueue) {
		q.capacity = v
	}
}

// NewPriorityQueue creates a new priority queue.
func NewPriorityQueue(opts ...PriorityQueueOption) PriorityQueue {
	q := &priorityQueue{
		capacity: defPriorityQueueCapacity,
		queue:    new(priorityQueueInternal),
	}
	for _, opt := range opts {
		opt(q)
	}
	if q.capacity <= 0 {
		panic(errors.Errorf("invalid capacity: %v", q.capacity))
	}
	return q
}

var _ PriorityQueue = new(priorityQueue)

// priorityQueue implements priority queue.
type priorityQueue struct {
	capacity int
	queue    *priorityQueueInternal
}

// Push
func (q *priorityQueue) Push(item PriorityQueueItem) (PriorityQueueIndexItem, error) {
	if q.Len() >= q.capacity {
		return nil, NewMaxCapacityError(q.capacity)
	}
	indexItem := &priorityQueueIndexItem{
		PriorityQueueItem: item,
	}
	heap.Push(q.queue, indexItem)
	return indexItem, nil
}

// Pop
func (q *priorityQueue) Pop() PriorityQueueIndexItem {
	if q.Len() <= 0 {
		return nil
	}
	item := heap.Pop(q.queue)
	return item.(PriorityQueueIndexItem)
}

// Peek
func (q *priorityQueue) Peek() PriorityQueueIndexItem {
	if q.Len() <= 0 {
		return nil
	}
	return q.queue.Peek()
}

// Update
func (q *priorityQueue) Update(item PriorityQueueIndexItem) {
	heap.Fix(q.queue, item.getIndex())
}

// Len
func (q *priorityQueue) Len() int {
	return q.queue.Len()
}

var _ PriorityQueueItem = new(priorityQueueItem)

// priorityQueueItem implements PriorityQueueItem.
type priorityQueueItem struct {
	data     interface{}
	priority int64
}

// NewPriorityQueueItem creates a new priority queue item.
func NewPriorityQueueItem(data interface{}, priority int64) PriorityQueueItem {
	return &priorityQueueItem{
		data:     data,
		priority: priority,
	}
}

// GetData
func (i *priorityQueueItem) GetData() interface{} {
	return i.data
}

// SetPriority
func (i *priorityQueueItem) SetPriority(v int64) {
	i.priority = v
}

// GetPriority
func (i *priorityQueueItem) GetPriority() int64 {
	return i.priority
}

var _ PriorityQueueIndexItem = new(priorityQueueIndexItem)

// priorityQueueIndexItem is the actual item that stored in the priority queue.
type priorityQueueIndexItem struct {
	PriorityQueueItem
	index int
}

// getIndex
func (i *priorityQueueIndexItem) getIndex() int {
	return i.index
}

// setIndex
func (i *priorityQueueIndexItem) setIndex(v int) {
	i.index = v
}

var _ heap.Interface = new(priorityQueueInternal)

// priorityQueueInternal sorts items by the priority.
type priorityQueueInternal []PriorityQueueIndexItem

// Push
func (q *priorityQueueInternal) Push(x interface{}) {
	item := x.(PriorityQueueIndexItem)
	item.setIndex(q.Len())
	*q = append(*q, item)
}

// Pop
func (q *priorityQueueInternal) Pop() interface{} {
	n := q.Len()
	item := (*q)[n-1]
	item.setIndex(-1)
	(*q)[n-1] = nil
	*q = (*q)[0 : n-1]
	return item
}

// Peek
func (q priorityQueueInternal) Peek() PriorityQueueIndexItem {
	return q[0]
}

// Len
func (q priorityQueueInternal) Len() int {
	return len(q)
}

// Less
func (q priorityQueueInternal) Less(i, j int) bool {
	return q[i].GetPriority() < q[j].GetPriority()
}

// Swap
func (q priorityQueueInternal) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	tmp := q[i].getIndex()
	q[i].setIndex(q[j].getIndex())
	q[j].setIndex(tmp)
}
