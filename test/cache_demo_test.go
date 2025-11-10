package test

import (
	"fmt"
	"fuxi/internal/storage"
	"os"
	"testing"
	"time"
)

// TestCacheDemo 演示缓存效果
func TestCacheDemo(t *testing.T) {
	dbPath := "/tmp/fuxi_cache_demo.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	t.Run("小缓存vs大缓存对比", func(t *testing.T) {
		// 测试1: 小缓存 (10个)
		t.Log("\n=== 测试1: 小缓存 (容量10) ===")
		store1, _ := storage.NewLayeredStorage(dbPath, 10)

		// 写入100条数据
		for i := 0; i < 100; i++ {
			code := fmt.Sprintf("url%03d", i)
			store1.Save(code, fmt.Sprintf("https://example.com/%d", i))
		}

		// 清空缓存统计，模拟重启
		store1.Close()
		store1, _ = storage.NewLayeredStorage(dbPath, 10)
		defer store1.Close()

		// 访问模式：重复访问前10条（应该能缓存）
		start := time.Now()
		for i := 0; i < 1000; i++ {
			code := fmt.Sprintf("url%03d", i%10)
			store1.Get(code)
		}
		elapsed1 := time.Since(start)
		stats1, _ := store1.GetStats()

		t.Logf("小缓存结果:")
		t.Logf("  耗时: %v", elapsed1)
		t.Logf("  平均延迟: %.3f ms", float64(elapsed1.Microseconds())/1000.0/1000)
		t.Logf("  缓存命中率: %.2f%%", stats1.CacheHitRate*100)

		// 测试2: 大缓存 (100个)
		t.Log("\n=== 测试2: 大缓存 (容量100) ===")
		store1.Close()
		store2, _ := storage.NewLayeredStorage(dbPath, 100)
		defer store2.Close()

		// 访问相同模式
		start = time.Now()
		for i := 0; i < 1000; i++ {
			code := fmt.Sprintf("url%03d", i%10)
			store2.Get(code)
		}
		elapsed2 := time.Since(start)
		stats2, _ := store2.GetStats()

		t.Logf("大缓存结果:")
		t.Logf("  耗时: %v", elapsed2)
		t.Logf("  平均延迟: %.3f ms", float64(elapsed2.Microseconds())/1000.0/1000)
		t.Logf("  缓存命中率: %.2f%%", stats2.CacheHitRate*100)
		t.Logf("\n性能提升: %.2fx", float64(elapsed1)/float64(elapsed2))
	})

	t.Run("缓存命中vs未命中对比", func(t *testing.T) {
		os.Remove(dbPath)

		t.Log("\n=== 缓存命中 vs 未命中对比 ===")
		store, _ := storage.NewLayeredStorage(dbPath, 100)
		defer store.Close()

		// 写入数据
		for i := 0; i < 100; i++ {
			code := fmt.Sprintf("test%03d", i)
			store.Save(code, fmt.Sprintf("https://example.com/%d", i))
		}

		// 清空缓存统计
		store.Close()
		store, _ = storage.NewLayeredStorage(dbPath, 100)
		defer store.Close()

		// 第一次访问（全部未命中）
		t.Log("\n第一轮访问（冷缓存）:")
		start := time.Now()
		for i := 0; i < 100; i++ {
			code := fmt.Sprintf("test%03d", i)
			store.Get(code)
		}
		coldTime := time.Since(start)

		// 第二次访问（全部命中）
		t.Log("第二轮访问（热缓存）:")
		start = time.Now()
		for i := 0; i < 100; i++ {
			code := fmt.Sprintf("test%03d", i)
			store.Get(code)
		}
		hotTime := time.Since(start)

		stats, _ := store.GetStats()

		t.Logf("\n冷缓存（第一次）:")
		t.Logf("  总耗时: %v", coldTime)
		t.Logf("  平均延迟: %.3f ms", float64(coldTime.Microseconds())/100.0/1000)

		t.Logf("\n热缓存（第二次）:")
		t.Logf("  总耗时: %v", hotTime)
		t.Logf("  平均延迟: %.3f μs", float64(hotTime.Microseconds())/100.0)
		t.Logf("  缓存命中率: %.2f%%", stats.CacheHitRate*100)
		t.Logf("\n性能提升: %.2fx", float64(coldTime)/float64(hotTime))
	})
}
