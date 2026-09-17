package telemetry

import "testing"

func TestAcquisitionQueuePushWithinCapacity(t *testing.T) {
	q := NewAcquisitionQueue(3)
	for i, id := range []string{"a", "b", "c"} {
		if queued := q.Push(NewSample(id, fixedTime(i), nil)); !queued {
			t.Fatalf("Push(%s): queued = false, want true", id)
		}
	}
	if depth := q.Depth(); depth != 3 {
		t.Fatalf("Depth = %d, want 3", depth)
	}
	if overflow := q.OverflowCount(); overflow != 0 {
		t.Fatalf("OverflowCount = %d, want 0", overflow)
	}
}

func TestAcquisitionQueueDropsOldestOnOverflow(t *testing.T) {
	q := NewAcquisitionQueue(2)
	q.Push(NewSample("a", fixedTime(0), nil))
	q.Push(NewSample("b", fixedTime(1), nil))
	if queued := q.Push(NewSample("c", fixedTime(2), nil)); queued {
		t.Fatal("Push over capacity: queued = true, want false")
	}
	if overflow := q.OverflowCount(); overflow != 1 {
		t.Fatalf("OverflowCount = %d, want 1", overflow)
	}
	if depth := q.Depth(); depth != 2 {
		t.Fatalf("Depth = %d, want 2", depth)
	}

	first := <-q.C()
	if first.DeviceID != "b" {
		t.Fatalf("oldest queued sample = %q, want %q (a should have been dropped)", first.DeviceID, "b")
	}
	second := <-q.C()
	if second.DeviceID != "c" {
		t.Fatalf("next queued sample = %q, want %q", second.DeviceID, "c")
	}
}

func TestAcquisitionQueueDefaultCapacity(t *testing.T) {
	q := NewAcquisitionQueue(0)
	for i := 0; i < DefaultQueueCapacity; i++ {
		if queued := q.Push(NewSample("s", fixedTime(i), nil)); !queued {
			t.Fatalf("Push #%d: queued = false, want true (capacity should default to %d)", i, DefaultQueueCapacity)
		}
	}
	if queued := q.Push(NewSample("overflow", fixedTime(999), nil)); queued {
		t.Fatal("Push past default capacity: queued = true, want false")
	}
}
