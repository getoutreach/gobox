// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file demostrates priority queue usage.
package queue_test

import (
	"fmt"

	"github.com/getoutreach/gobox/pkg/queue"
)

func ExamplePriorityQueue() {
	// Create priority queue.
	pq, err := queue.NewPriorityQueue()
	if err != nil {
		panic(err)
	}

	// Push items.
	pushItem := func(data interface{}, priority int64) *queue.PriorityQueueItem {
		item, err := pq.Push(data, priority)
		if err != nil {
			panic(err)
		}
		return item
	}
	item1 := pushItem(1, 1)
	item2 := pushItem(2, 2)
	item3 := pushItem(3, 3)

	// List items
	list := func(msg string) {
		fmt.Println(msg)
		for _, item := range pq.List() {
			fmt.Println(item.GetPriority(), item.GetData())
		}
	}
	list("list items after push")

	// Update item priority.
	item1.SetPriority(3)
	item2.SetPriority(1)
	item3.SetPriority(2)
	pq.Update(item1)
	pq.Update(item2)
	pq.Update(item3)
	list("list items after update")

	// Remove an item
	pq.Remove(item3)
	list("list items after remove")

	// Pop items.
	fmt.Println("pop items")
	for {
		item := pq.Pop()
		if item == nil {
			break
		}
		fmt.Println(item.GetPriority(), item.GetData())
	}
	list("list items after pop")

	// Output:
	// list items after push
	// 1 1
	// 2 2
	// 3 3
	// list items after update
	// 1 2
	// 2 3
	// 3 1
	// list items after remove
	// 1 2
	// 3 1
	// pop items
	// 1 2
	// 3 1
	// list items after pop
}
