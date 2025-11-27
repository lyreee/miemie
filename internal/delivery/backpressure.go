package delivery

import (
	"miemie/internal/logger"
	"runtime"
	"time"
)

// NewBackpressureCtrl 创建背压控制器
func NewBackpressureCtrl() *BackpressureCtrl {
	return &BackpressureCtrl{
		windowSize: 60 * time.Second, // 1分钟窗口
		lastWindow: time.Now(),
	}
}

// ShouldAccept 是否接受任务（背压控制核心逻辑）
func (bp *BackpressureCtrl) ShouldAccept(task DeliveryTask) bool {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// 计算当前拒绝率
	total := bp.rejectionCount + bp.acceptanceCount
	if total == 0 {
		bp.acceptanceCount++
		return true
	}

	rejectionRate := float64(bp.rejectionCount) / float64(total)

	// 根据内存压力和拒绝率决定
	pressure := GetCurrentMemoryPressure()

	switch pressure {
	case MemoryPressureCritical:
		// 只接受高优先级任务
		if task.Priority < 8 {
			bp.rejectionCount++
			logger.Infof("Rejected task due to critical memory pressure (priority: %d)", task.Priority)
			return false
		}
	case MemoryPressureHigh:
		// 拒绝率超过30%时开始限流
		if rejectionRate > 0.3 && task.Priority < 6 {
			bp.rejectionCount++
			logger.Infof("Rejected task due to high memory pressure (rejection rate: %.2f, priority: %d)",
				rejectionRate, task.Priority)
			return false
		}
	case MemoryPressureMedium:
		// 拒绝率超过50%时限制低优先级
		if rejectionRate > 0.5 && task.Priority < 4 {
			bp.rejectionCount++
			logger.Infof("Rejected task due to medium memory pressure (rejection rate: %.2f, priority: %d)",
				rejectionRate, task.Priority)
			return false
		}
	}

	bp.acceptanceCount++
	return true
}

// ResetWindow 重置统计窗口
func (bp *BackpressureCtrl) ResetWindow() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// 如果窗口时间已过，重置计数器
	if time.Since(bp.lastWindow) > bp.windowSize {
		bp.rejectionCount = 0
		bp.acceptanceCount = 0
		bp.lastWindow = time.Now()
	}
}

// GetStats 获取背压统计
func (bp *BackpressureCtrl) GetStats() (rejectionCount, acceptanceCount int64, rejectionRate float64) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	rejectionCount = bp.rejectionCount
	acceptanceCount = bp.acceptanceCount
	total := rejectionCount + acceptanceCount
	if total > 0 {
		rejectionRate = float64(rejectionCount) / float64(total)
	}

	return
}

// GetCurrentMemoryPressure 获取当前内存压力
func GetCurrentMemoryPressure() MemoryPressure {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算内存使用率
	memoryUsageMB := m.Alloc / 1024 / 1024
	sysMemoryMB := m.Sys / 1024 / 1024

	// 简单的内存压力评估
	if sysMemoryMB > 0 {
		usageRatio := float64(memoryUsageMB) / float64(sysMemoryMB)

		switch {
		case usageRatio > 0.9:
			return MemoryPressureCritical
		case usageRatio > 0.7:
			return MemoryPressureHigh
		case usageRatio > 0.5:
			return MemoryPressureMedium
		default:
			return MemoryPressureLow
		}
	}

	return MemoryPressureLow
}

// GetDetailedMemoryStats 获取详细内存统计
func GetDetailedMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc_mb":        m.Alloc / 1024 / 1024,
		"total_alloc_mb":  m.TotalAlloc / 1024 / 1024,
		"sys_mb":          m.Sys / 1024 / 1024,
		"num_gc":          m.NumGC,
		"goroutines":      runtime.NumGoroutine(),
		"gc_cpu_fraction": m.GCCPUFraction,
		"heap_alloc_mb":   m.HeapAlloc / 1024 / 1024,
		"heap_sys_mb":     m.HeapSys / 1024 / 1024,
		"heap_objects":    m.HeapObjects,
	}
}