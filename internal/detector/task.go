package detector

import "time"

// ProbeTask 表示一次 ping 探测任务。
type ProbeTask struct {
	Ip         string
	TimeoutMs  int
	UnstableMs int
}

// ProbeResult 表示一次 ping 探测的结果。
type ProbeResult struct {
	LatencyMs *int
	Status    int
	Time      time.Time
}

// 状态常量。
const (
	StatusOffline  = 0
	StatusUnstable = 1
	StatusOnline   = 2
)
