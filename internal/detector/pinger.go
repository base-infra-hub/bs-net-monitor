package detector

import (
	"log"
	"runtime"
	"time"

	"github.com/prometheus-community/pro-bing"
)

// runProbe 执行一次 ping 探测并返回结果。
func runProbe(task *ProbeTask) *ProbeResult {
	result := &ProbeResult{
		Time:   time.Now().UTC(),
		Status: StatusOffline,
	}

	pinger, err := probing.NewPinger(task.Ip)
	if err != nil {
		log.Printf("[detector] 创建 %s 的 pinger 失败: %v", task.Ip, err)
		return result
	}

	pinger.Count = 1
	pinger.Timeout = time.Duration(task.TimeoutMs) * time.Millisecond
	// 动态检测操作系统：
	// 1. 在 Windows 上，非特权模式 (udp4) 不受支持，必须使用 Raw Socket（即 Privileged = true）且用管理员权限运行。
	// 2. 在 Linux/macOS 上，可以使用非特权模式 (Privileged = false) 以便非 root 用户直接运行。
	if runtime.GOOS == "windows" {
		pinger.SetPrivileged(true)
	} else {
		pinger.SetPrivileged(false)
	}

	if err := pinger.Run(); err != nil {
		log.Printf("[detector] ping %s 失败: %v", task.Ip, err)
		return result
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return result
	}

	latency := int(stats.AvgRtt.Milliseconds())
	result.LatencyMs = &latency

	if task.UnstableMs > 0 && latency > task.UnstableMs {
		result.Status = StatusUnstable
	} else {
		result.Status = StatusOnline
	}

	return result
}
