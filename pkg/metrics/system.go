package metrics

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

// collectSystemMetrics 收集系统指标
func collectSystemMetrics() SystemMetrics {
	metrics := SystemMetrics{}

	// 收集CPU使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		logrus.Errorf("获取CPU使用率失败: %v", err)
	} else if len(cpuPercent) > 0 {
		metrics.CPUUsage = cpuPercent[0]
	}

	// 收集内存使用率
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logrus.Errorf("获取内存信息失败: %v", err)
	} else {
		metrics.MemoryUsage = memInfo.UsedPercent
	}

	// 收集系统负载
	loadInfo, err := load.Avg()
	if err != nil {
		logrus.Errorf("获取系统负载失败: %v", err)
	} else {
		metrics.SystemLoad = loadInfo.Load1
	}

	// 收集Goroutine数量
	metrics.GoroutineCount = int64(runtime.NumGoroutine())

	return metrics
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// CPU信息
	cpuInfo, err := cpu.Info()
	if err != nil {
		logrus.Errorf("获取CPU信息失败: %v", err)
	} else {
		info["cpu"] = cpuInfo
	}

	// 内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logrus.Errorf("获取内存信息失败: %v", err)
	} else {
		info["memory"] = map[string]interface{}{
			"total":        memInfo.Total,
			"available":    memInfo.Available,
			"used":         memInfo.Used,
			"used_percent": memInfo.UsedPercent,
		}
	}

	// Go运行时信息
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	info["runtime"] = map[string]interface{}{
		"goroutines":     runtime.NumGoroutine(),
		"threads":        runtime.GOMAXPROCS(0),
		"alloc":          rtm.Alloc,
		"total_alloc":    rtm.TotalAlloc,
		"sys":            rtm.Sys,
		"num_gc":         rtm.NumGC,
		"pause_total_ns": rtm.PauseTotalNs,
	}

	// 系统负载
	loadInfo, err := load.Avg()
	if err != nil {
		logrus.Errorf("获取系统负载失败: %v", err)
	} else {
		info["load"] = map[string]interface{}{
			"load1":  loadInfo.Load1,
			"load5":  loadInfo.Load5,
			"load15": loadInfo.Load15,
		}
	}

	return info
}

// StartSystemMetricsCollection 启动系统指标收集
func StartSystemMetricsCollection(interval time.Duration) {
	collector := GetCollector()
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			metrics := collectSystemMetrics()
			collector.UpdateSystemMetrics(metrics)
		}
	}()

	logrus.Infof("系统指标收集已启动，间隔: %v", interval)
}
