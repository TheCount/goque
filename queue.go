package goque

import (
	"os"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

// Queue is a standard FIFO (first in, first out) queue.
type Queue struct {
	sync.RWMutex
	DataDir string
	db      *leveldb.DB
	head    uint64
	tail    uint64
	isOpen  bool
}

// OpenQueue opens a queue if one exists at the given directory. If one
// does not already exist, a new queue is created.
func OpenQueue(dataDir string) (*Queue, error) {
	var err error

	// Create a new Queue.
	q := &Queue{
		DataDir: dataDir,
		db:      &leveldb.DB{},
		head:    0,
		tail:    0,
		isOpen:  false,
	}

	// Open database for the queue.
	q.db, err = leveldb.OpenFile(dataDir, nil)
	if err != nil {
		return q, err
	}

	// Set queue isOpen and return.
	q.isOpen = true
	return q, q.init()
}

// Enqueue adds an item to the queue.
func (q *Queue) Enqueue(item *Item) error {
	q.Lock()
	defer q.Unlock()

	// Set item ID and key.
	item.ID = q.tail + 1
	item.Key = idToKey(item.ID)

	// Add it to the queue.
	err := q.db.Put(item.Key, item.Value, nil)
	if err == nil {
		q.tail++
	}

	return err
}

// Dequeue removes the next item in the queue and returns it.
func (q *Queue) Dequeue() (*Item, error) {
	q.Lock()
	defer q.Unlock()

	// Try to get the next item in the queue.
	item, err := q.getItemByID(q.head + 1)
	if err != nil {
		return item, err
	}

	// Remove this item from the queue.
	if err := q.db.Delete(item.Key, nil); err != nil {
		return item, err
	}

	// Increment position.
	q.head++

	return item, nil
}

// Peek returns the next item in the queue without removing it.
func (q *Queue) Peek() (*Item, error) {
	q.RLock()
	defer q.RUnlock()
	return q.getItemByID(q.head + 1)
}

// PeekByOffset returns the item located at the given offset,
// starting from the head of the queue, without removing it.
func (q *Queue) PeekByOffset(offset uint64) (*Item, error) {
	q.RLock()
	defer q.RUnlock()
	return q.getItemByID(q.head + offset)
}

// PeekByID returns the item with the given ID without removing it.
func (q *Queue) PeekByID(id uint64) (*Item, error) {
	q.RLock()
	defer q.RUnlock()
	return q.getItemByID(id)
}

// Update updates an item in the queue without changing its position.
func (q *Queue) Update(item *Item, newValue []byte) error {
	q.Lock()
	defer q.Unlock()
	item.Value = newValue
	return q.db.Put(item.Key, item.Value, nil)
}

// UpdateString is a helper function for Update that accepts a value
// as a string rather than a byte slice.
func (q *Queue) UpdateString(item *Item, newValue string) error {
	return q.Update(item, []byte(newValue))
}

// Length returns the total number of items currently in the queue.
func (q *Queue) Length() uint64 {
	return q.tail - q.head
}

// Drop closes and deletes the LevelDB database of the queue.
func (q *Queue) Drop() {
	q.Close()
	os.RemoveAll(q.DataDir)
}

// Close closes the LevelDB database of the queue.
func (q *Queue) Close() {
	// If queue is already closed.
	if !q.isOpen {
		return
	}

	q.db.Close()
	q.isOpen = false
}

// getItemByID returns an item, if found, for the given ID.
func (q *Queue) getItemByID(id uint64) (*Item, error) {
	// Check if empty or out of bounds.
	if q.Length() < 1 {
		return nil, ErrEmpty
	} else if id <= q.head || id > q.tail {
		return nil, ErrOutOfBounds
	}

	var err error
	item := &Item{ID: id, Key: idToKey(id)}
	item.Value, err = q.db.Get(item.Key, nil)

	return item, err
}

// Initialize the queue data.
func (q *Queue) init() error {
	// Create a new LevelDB Iterator.
	iter := q.db.NewIterator(nil, nil)
	defer iter.Release()

	// Set queue head to the first item.
	if iter.First() {
		q.head = keyToID(iter.Key()) - 1
	} else {
		q.head = 0
	}

	// Set queue tail to the last item.
	if iter.Last() {
		q.tail = keyToID(iter.Key())
	} else {
		q.tail = 0
	}

	return iter.Error()
}