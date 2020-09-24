package s

import (
	"fmt"
	"github.com/ssgo/discover"
	"github.com/ssgo/log"
	"strings"
	"sync"
	"time"
)

type TimeStatistician struct {
	startTime    time.Time
	endTime      time.Time
	stepNames    []string
	stepTotals   []float64
	stepTimes    []int
	stepMinimums []float32
	stepMaxes    []float32
	total        float64
	times        int
	totalMinimum float32
	totalMax     float32
	lock         sync.Mutex
	logger       *log.Logger
}

type TimeCounter struct {
	start     int64
	last      int64
	stepNames []string
	stepTimes []float32
	total     float32
}

func StartTimeCounter() *TimeCounter {
	now := time.Now().UnixNano()
	return &TimeCounter{start: now, last: now}
}

func (t *TimeCounter) Add(name string) float32 {
	now := time.Now().UnixNano()
	used := float32(now-t.last) / float32(time.Millisecond)
	t.last = now
	t.stepNames = append(t.stepNames, name)
	t.stepTimes = append(t.stepTimes, used)
	return used
}

func (t *TimeCounter) Total() float32 {
	if t.total == 0 {
		t.total = float32(t.last-t.start) / float32(time.Millisecond)
	}
	return t.total
}

func (t *TimeCounter) Sprint() string {
	a := make([]string, 0)
	for i, name := range t.stepNames {
		a = append(a, fmt.Sprintf("%s: %.4f", name, t.stepTimes[i]))
	}
	a = append(a, fmt.Sprintf("Total: %.4f", t.Total()))
	return strings.Join(a, "\n")
}

func (t *TimeCounter) Print() {
	fmt.Println(t.Sprint())
}

func NewTimeStatistic(logger *log.Logger) *TimeStatistician {
	return &TimeStatistician{logger: logger, lock: sync.Mutex{}, startTime: time.Now(), stepNames: make([]string, 0), stepMinimums: make([]float32, 0), stepMaxes: make([]float32, 0), stepTotals: make([]float64, 0), stepTimes: make([]int, 0)}
}

func (t *TimeStatistician) Push(c *TimeCounter) string {
	out := ""
	t.lock.Lock()
	if len(c.stepNames) >= len(t.stepNames) {
		for i := len(t.stepNames); i < len(c.stepNames); i++ {
			t.stepNames = append(t.stepNames, c.stepNames[i])
			t.stepTimes = append(t.stepTimes, 0)
			t.stepTotals = append(t.stepTotals, 0)
			t.stepMinimums = append(t.stepMinimums, 0)
			t.stepMaxes = append(t.stepMaxes, 0)
		}
		//fmt.Println("  #######", u.JsonP(t.stepNames))
	}

	cTotal := c.Total()
	t.total += float64(cTotal)
	if cTotal < t.totalMinimum || t.totalMinimum == 0 {
		t.totalMinimum = cTotal
	}
	if cTotal > t.totalMax || t.totalMax == 0 {
		t.totalMax = cTotal
	}
	t.times++
	for i, tm := range c.stepTimes {
		t.stepTotals[i] += float64(tm)
		t.stepTimes[i]++
		if tm < t.stepMinimums[i] || t.stepMinimums[i] == 0 {
			t.stepMinimums[i] = tm
		}
		if tm > t.stepMaxes[i] || t.stepMaxes[i] == 0 {
			t.stepMaxes[i] = tm
		}
	}

	// 进行统计
	n := Config.StatisticTimeInterval
	if n == 0 {
		n = 10000
	}
	if t.times >= n {
		t.endTime = time.Now()
		t.Log()
		t.startTime = t.endTime
		t.total = 0
		t.totalMinimum = 0
		t.totalMax = 0
		t.times = 0
		for i := range t.stepTimes {
			t.stepTotals[i] = 0
			t.stepTimes[i] = 0
			t.stepMinimums[i] = 0
			t.stepMaxes[i] = 0
		}
	}
	t.lock.Unlock()
	return out
}

func (t *TimeStatistician) Log() {
	for i, name := range t.stepNames {
		avg := t.stepTotals[i] / float64(t.stepTimes[i])
		t.logger.Statistic(serverId, discover.Config.App, "s-request-"+name, t.startTime, t.endTime, uint(t.times), 0, float32(avg), t.stepMinimums[i], t.stepMaxes[i])
	}
	t.logger.Statistic(serverId, discover.Config.App, "s-request-Total", t.startTime, t.endTime, uint(t.times), 0, float32(t.total/float64(t.times)), t.totalMinimum, t.totalMax)
}
