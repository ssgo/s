package s

import (
	"math"
	"time"
)

type Counter struct {
	StartTime time.Time
	EndTime   time.Time
	Total     float64
	Times     uint
	Failed    uint
	Min       float64
	Max       float64
	Avg       float64
}

func NewCounter() *Counter {
	now := time.Now()
	return &Counter{StartTime: now, EndTime: now}
}

func (t *Counter) Add(v float64) {
	t.Total += v
	if t.Times == 0 || v > t.Max {
		t.Max = v
	}
	if t.Times == 0 || v < t.Min {
		t.Min = v
	}
	t.Times++
}

func (t *Counter) AddFailed(v float64) {
	t.Add(v)
	t.Failed++
}

func (t *Counter) Count() {
	if t.Total > 0 && t.Times > 0 {
		t.Avg = math.Round(t.Total/float64(t.Times)*10000) / 10000
	}
	t.EndTime = time.Now()
}

func (t *Counter) Reset() {
	now := time.Now()
	t.StartTime = now
	t.EndTime = now
	t.Total = 0
	t.Times = 0
	t.Failed = 0
	t.Min = 0
	t.Max = 0
	t.Avg = 0
}
