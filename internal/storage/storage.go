package storage

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// URLMapping 短URL映射模型
type URLMapping struct {
	ID          uint      `gorm:"primarykey"`
	ShortCode   string    `gorm:"uniqueIndex;size:20;not null"`
	LongURL     string    `gorm:"size:2048;not null"`
	AccessCount int64     `gorm:"default:0"`
	ExpiresAt   time.Time `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Storage 存储接口
type Storage interface {
	Save(code, longURL string) error
	Get(code string) (string, error)
	IncrementAccess(code string) error
	GetStats() (*Stats, error)
	Close() error
}

// Stats 统计信息
type Stats struct {
	TotalURLs    int64
	TotalAccess  int64
	ActiveURLs   int64
	ExpiredURLs  int64
	CacheHitRate float64
}

// LayeredStorage 分层存储实现
type LayeredStorage struct {
	db    *gorm.DB
	cache *LRUCache
}

// NewLayeredStorage 创建分层存储
func NewLayeredStorage(dbPath string, cacheSize int) (*LayeredStorage, error) {
	// 初始化SQLite数据库
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 自动迁移表结构
	err = db.AutoMigrate(&URLMapping{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	// 创建LRU缓存
	cache := NewLRUCache(cacheSize)

	return &LayeredStorage{
		db:    db,
		cache: cache,
	}, nil
}

// Save 保存短URL映射
func (s *LayeredStorage) Save(code, longURL string) error {
	mapping := &URLMapping{
		ShortCode: code,
		LongURL:   longURL,
		ExpiresAt: time.Now().Add(2 * 365 * 24 * time.Hour), // 2年有效期
	}

	result := s.db.Create(mapping)
	if result.Error != nil {
		return result.Error
	}

	// 写入缓存
	s.cache.Put(code, longURL)

	return nil
}

// Get 获取长URL
func (s *LayeredStorage) Get(code string) (string, error) {
	// 1. 先查缓存
	if longURL, ok := s.cache.Get(code); ok {
		return longURL, nil
	}

	// 2. 缓存未命中，查数据库
	var mapping URLMapping
	result := s.db.Where("short_code = ? AND expires_at > ?", code, time.Now()).First(&mapping)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("short URL not found or expired")
		}
		return "", result.Error
	}

	// 3. 写入缓存
	s.cache.Put(code, mapping.LongURL)

	return mapping.LongURL, nil
}

// IncrementAccess 增加访问计数
func (s *LayeredStorage) IncrementAccess(code string) error {
	return s.db.Model(&URLMapping{}).
		Where("short_code = ?", code).
		UpdateColumn("access_count", gorm.Expr("access_count + 1")).Error
}

// GetStats 获取统计信息
func (s *LayeredStorage) GetStats() (*Stats, error) {
	stats := &Stats{}

	// 总URL数量
	s.db.Model(&URLMapping{}).Count(&stats.TotalURLs)

	// 总访问量
	s.db.Model(&URLMapping{}).Select("SUM(access_count)").Scan(&stats.TotalAccess)

	// 活跃URL数量
	s.db.Model(&URLMapping{}).Where("expires_at > ?", time.Now()).Count(&stats.ActiveURLs)

	// 过期URL数量
	stats.ExpiredURLs = stats.TotalURLs - stats.ActiveURLs

	// 缓存命中率
	stats.CacheHitRate = s.cache.HitRate()

	return stats, nil
}

// Close 关闭存储
func (s *LayeredStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// LRUCache LRU缓存实现
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	lruList  *list.List
	mu       sync.RWMutex
	hits     int64
	misses   int64
}

type cacheEntry struct {
	key   string
	value string
}

// NewLRUCache 创建LRU缓存
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

// Get 获取缓存
func (c *LRUCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.hits++
		c.lruList.MoveToFront(elem)
		return elem.Value.(*cacheEntry).value, true
	}

	c.misses++
	return "", false
}

// Put 写入缓存
func (c *LRUCache) Put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新并移到前面
	if elem, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = value
		return
	}

	// 新增元素
	entry := &cacheEntry{key: key, value: value}
	elem := c.lruList.PushFront(entry)
	c.cache[key] = elem

	// 如果超过容量，删除最久未使用的
	if c.lruList.Len() > c.capacity {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.cache, oldest.Value.(*cacheEntry).key)
		}
	}
}

// HitRate 获取缓存命中率
func (c *LRUCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// Size 获取当前缓存大小
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}

// Clear 清空缓存
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lruList = list.New()
	c.hits = 0
	c.misses = 0
}
