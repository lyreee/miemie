package workspace

import (
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Workspace  *Workspace
	LastAccess time.Time
	CreatedAt  time.Time
}

// WorkspaceCache 带过期时间的工作空间缓存
type WorkspaceCache struct {
	entries    map[string]*CacheEntry
	maxSize    int
	ttl        time.Duration // 缓存过期时间
	mu         sync.RWMutex
	cleanup    chan struct{}
	stopCleanup chan struct{}
}

// NewWorkspaceCache 创建工作空间缓存
func NewWorkspaceCache(maxSize int, ttl time.Duration) *WorkspaceCache {
	wc := &WorkspaceCache{
		entries:     make(map[string]*CacheEntry),
		maxSize:     maxSize,
		ttl:         ttl,
		cleanup:     make(chan struct{}),
		stopCleanup: make(chan struct{}),
	}

	// 启动清理协程
	go wc.cleanupExpiredEntries()

	return wc
}

// Get 获取工作空间
func (wc *WorkspaceCache) Get(userID string) (*Workspace, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	entry, exists := wc.entries[userID]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.LastAccess) > wc.ttl {
		go wc.removeEntry(userID) // 异步移除过期条目
		return nil, false
	}

	// 更新最后访问时间
	entry.LastAccess = time.Now()
	return entry.Workspace, true
}

// Put 存储工作空间
func (wc *WorkspaceCache) Put(userID string, ws *Workspace) {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// 检查是否需要清理空间
	if len(wc.entries) >= wc.maxSize {
		wc.evictLRU()
	}

	wc.entries[userID] = &CacheEntry{
		Workspace:  ws,
		LastAccess: time.Now(),
		CreatedAt:  time.Now(),
	}
}

// Remove 移除工作空间
func (wc *WorkspaceCache) Remove(userID string) {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	wc.removeEntryUnsafe(userID)
}

// removeEntry 移除条目（内部方法，不加锁）
func (wc *WorkspaceCache) removeEntry(userID string) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.removeEntryUnsafe(userID)
}

// removeEntryUnsafe 移除条目（不加锁版本）
func (wc *WorkspaceCache) removeEntryUnsafe(userID string) {
	if entry, exists := wc.entries[userID]; exists {
		// 关闭数据库连接
		if entry.Workspace != nil {
			if entry.Workspace.MessagesDB != nil {
				entry.Workspace.MessagesDB.Close()
			}
			if entry.Workspace.ReadDB != nil {
				entry.Workspace.ReadDB.Close()
			}
		}
		delete(wc.entries, userID)
	}
}

// evictLRU 淘汰最少使用的条目
func (wc *WorkspaceCache) evictLRU() {
	var oldestUserID string
	var oldestTime time.Time

	for userID, entry := range wc.entries {
		if oldestUserID == "" || entry.LastAccess.Before(oldestTime) {
			oldestUserID = userID
			oldestTime = entry.LastAccess
		}
	}

	if oldestUserID != "" {
		wc.removeEntryUnsafe(oldestUserID)
	}
}

// cleanupExpiredEntries 定期清理过期条目
func (wc *WorkspaceCache) cleanupExpiredEntries() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wc.cleanupOnce()
		case <-wc.stopCleanup:
			return
		}
	}
}

// cleanupOnce 执行一次清理
func (wc *WorkspaceCache) cleanupOnce() {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	var toRemove []string
	now := time.Now()

	for userID, entry := range wc.entries {
		if now.Sub(entry.LastAccess) > wc.ttl {
			toRemove = append(toRemove, userID)
		}
	}

	// 移除过期条目
	for _, userID := range toRemove {
		wc.removeEntryUnsafe(userID)
	}
}

// Size 返回缓存大小
func (wc *WorkspaceCache) Size() int {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return len(wc.entries)
}

// Stats 返回缓存统计
func (wc *WorkspaceCache) Stats() map[string]interface{} {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	now := time.Now()
	expiredCount := 0

	for _, entry := range wc.entries {
		if now.Sub(entry.LastAccess) > wc.ttl {
			expiredCount++
		}
	}

	return map[string]interface{}{
		"total_entries":  len(wc.entries),
		"expired_entries": expiredCount,
		"max_size":       wc.maxSize,
		"ttl_minutes":    wc.ttl.Minutes(),
	}
}

// Close 关闭缓存，清理所有资源
func (wc *WorkspaceCache) Close() {
	close(wc.stopCleanup)

	wc.mu.Lock()
	defer wc.mu.Unlock()

	// 关闭所有数据库连接
	for userID := range wc.entries {
		wc.removeEntryUnsafe(userID)
	}
}

// ListActiveUsers 列出活跃用户
func (wc *WorkspaceCache) ListActiveUsers() []string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	now := time.Now()
	var activeUsers []string

	for userID, entry := range wc.entries {
		if now.Sub(entry.LastAccess) <= wc.ttl {
			activeUsers = append(activeUsers, userID)
		}
	}

	return activeUsers
}