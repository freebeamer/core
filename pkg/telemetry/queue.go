package telemetry

import "sync/atomic"

// DefaultQueueCapacity is used when NewAcquisitionQueue is given a
// non-positive capacity.
const DefaultQueueCapacity = 32

// AcquisitionQueue is a small, bounded, in-memory handoff between sample
// acquisition (reading the adapter) and durable spooling (writing to disk).
// It exists only to absorb a momentary spool-write stall; the Spool, not
// this queue, is the actual durable backlog — see
// docs/live-data-and-vehicle-mapping-plan.md's Phase 3 section for why the
// plan calls out a bounded queue and a durable spool as two distinct
// mechanisms rather than one. Push never blocks: a full queue drops the
// oldest still-queued sample and counts the drop, so a stalled spool writer
// can never stall the acquisition loop that owns this queue.
//
// Push assumes a single producer (matching this package's actual usage —
// one acquisition loop). It is safe to call C(), Depth(), OverflowCount(),
// Sent(), and MarkHandled() from any goroutine, including a concurrent
// single consumer.
type AcquisitionQueue struct {
	ch       chan Sample
	overflow uint64
	sent     uint64 // every item that ever successfully entered ch
	handled  uint64 // every item that has since left ch, by any path (eviction here, or the consumer's MarkHandled)
}

// NewAcquisitionQueue returns a queue with the given capacity
// (DefaultQueueCapacity if capacity <= 0).
func NewAcquisitionQueue(capacity int) *AcquisitionQueue {
	if capacity <= 0 {
		capacity = DefaultQueueCapacity
	}
	return &AcquisitionQueue{ch: make(chan Sample, capacity)}
}

// Push adds sample to the queue. queued reports whether it was accepted
// without dropping anything; if the queue was full, the oldest queued
// sample was dropped (and OverflowCount incremented) to make room, and
// queued is false. Push never blocks on a consumer: if evicting the oldest
// item still leaves no room (only possible with a concurrent second
// producer, which this type doesn't support), the new sample is dropped
// instead and counted the same way.
func (q *AcquisitionQueue) Push(sample Sample) (queued bool) {
	select {
	case q.ch <- sample:
		atomic.AddUint64(&q.sent, 1)
		return true
	default:
	}
	select {
	case <-q.ch:
		atomic.AddUint64(&q.overflow, 1)
		atomic.AddUint64(&q.handled, 1) // evicted here; the consumer will never see it
	default:
	}
	select {
	case q.ch <- sample:
		atomic.AddUint64(&q.sent, 1)
	default:
		// Only reachable with a second concurrent producer, which this
		// type does not support; drop and count defensively rather than
		// block.
		atomic.AddUint64(&q.overflow, 1)
	}
	return false
}

// C returns the channel a spool-writer goroutine drains samples from. It
// should be read with a select alongside a cancellation channel/context so
// the reader can exit cleanly. Each value received must be followed by a
// call to MarkHandled once the consumer has finished acting on it (e.g.
// after a durable spool append completes), so Drained can tell a
// still-in-flight handoff apart from a genuinely empty queue.
func (q *AcquisitionQueue) C() <-chan Sample { return q.ch }

// MarkHandled records that the consumer has fully finished processing one
// item received from C() (durably appended, or otherwise disposed of).
func (q *AcquisitionQueue) MarkHandled() { atomic.AddUint64(&q.handled, 1) }

// Depth returns the number of samples currently queued (received or not).
func (q *AcquisitionQueue) Depth() int { return len(q.ch) }

// OverflowCount returns how many samples have been dropped because the
// queue was full.
func (q *AcquisitionQueue) OverflowCount() uint64 { return atomic.LoadUint64(&q.overflow) }

// Drained reports whether every sample ever pushed has since been fully
// accounted for — either evicted by a later Push, or received and marked
// handled by the consumer. Unlike Depth() == 0, this is race-free against
// the narrow window between a consumer receiving a value from C() and
// finishing whatever durable work it does with it: Depth() (a channel
// length read) can read 0 the instant the value is handed to a blocked
// receiver, well before that receiver's own processing (e.g. a disk write)
// completes, which is exactly the gap Drained closes by requiring an
// explicit MarkHandled.
func (q *AcquisitionQueue) Drained() bool {
	return atomic.LoadUint64(&q.handled) >= atomic.LoadUint64(&q.sent)
}
