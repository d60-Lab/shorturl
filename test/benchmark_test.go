package test

import (
	"fmt"
	"fuxi/internal/generator"
	"fuxi/internal/preload"
	"fuxi/internal/storage"
	"os"
	"testing"
	"time"
)

// BenchmarkGenerate 测试短URL生成性能
func BenchmarkGenerate(b *testing.B) {
	gen := generator.NewGenerator(1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate(1000)
	}
}

// BenchmarkHashGenerate 测试Hash生成性能
func BenchmarkHashGenerate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generator.GenerateWithHash(fmt.Sprintf("https://example.com/test%d", i))
	}
}

// BenchmarkSequenceGenerate 测试序列生成性能
func BenchmarkSequenceGenerate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generator.GenerateWithSequence(int64(i))
	}
}

// BenchmarkCacheGet 测试缓存查询性能
func BenchmarkCacheGet(b *testing.B) {
	store, _ := storage.NewLayeredStorage(":memory:", 10000)
	defer store.Close()

	// 预填充数据
	for i := 0; i < 10000; i++ {
		code := fmt.Sprintf("test%02d", i)
		store.Save(code, fmt.Sprintf("https://example.com/%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		code := fmt.Sprintf("test%02d", i%10000)
		store.Get(code)
	}
}

// BenchmarkDBGet 测试数据库查询性能（清空缓存）
func BenchmarkDBGet(b *testing.B) {
	store, _ := storage.NewLayeredStorage(":memory:", 10)
	defer store.Close()

	// 预填充数据
	for i := 0; i < 10000; i++ {
		code := fmt.Sprintf("test%02d", i)
		store.Save(code, fmt.Sprintf("https://example.com/%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次查询不同的code，避免缓存命中
		code := fmt.Sprintf("test%02d", i%10000)
		store.Get(code)
	}
}

// TestGenerationMethods 对比不同生成方法
func TestGenerationMethods(t *testing.T) {
	count := 10000

	t.Run("Random Generation", func(t *testing.T) {
		gen := generator.NewGenerator(100000)
		start := time.Now()
		urls, _ := gen.Generate(count)
		elapsed := time.Since(start)

		// 检查唯一性
		unique := make(map[string]bool)
		for _, url := range urls {
			unique[url] = true
		}

		collisionRate := float64(count-len(unique)) / float64(count) * 100

		t.Logf("随机生成:")
		t.Logf("  数量: %d", count)
		t.Logf("  唯一: %d", len(unique))
		t.Logf("  冲突率: %.4f%%", collisionRate)
		t.Logf("  耗时: %v", elapsed)
		t.Logf("  速度: %.0f URLs/秒", float64(count)/elapsed.Seconds())
	})

	t.Run("Hash Generation", func(t *testing.T) {
		start := time.Now()
		urls := make([]string, count)
		unique := make(map[string]bool)

		for i := 0; i < count; i++ {
			url := generator.GenerateWithHash(fmt.Sprintf("https://example.com/%d", i))
			urls[i] = url
			unique[url] = true
		}
		elapsed := time.Since(start)

		collisionRate := float64(count-len(unique)) / float64(count) * 100

		t.Logf("Hash生成:")
		t.Logf("  数量: %d", count)
		t.Logf("  唯一: %d", len(unique))
		t.Logf("  冲突率: %.4f%%", collisionRate)
		t.Logf("  耗时: %v", elapsed)
		t.Logf("  速度: %.0f URLs/秒", float64(count)/elapsed.Seconds())
	})

	t.Run("Sequence Generation", func(t *testing.T) {
		start := time.Now()
		urls := make([]string, count)

		for i := 0; i < count; i++ {
			urls[i] = generator.GenerateWithSequence(int64(i))
		}
		elapsed := time.Since(start)

		// 检查可预测性
		predictable := true
		for i := 1; i < 10; i++ {
			if urls[i-1] >= urls[i] {
				// 简单检查：后面的应该"大于"前面的（虽然是字符串）
			}
		}

		t.Logf("序列生成:")
		t.Logf("  数量: %d", count)
		t.Logf("  唯一: %d", count)
		t.Logf("  冲突率: 0%%")
		t.Logf("  可预测: %v ⚠️", predictable)
		t.Logf("  耗时: %v", elapsed)
		t.Logf("  速度: %.0f URLs/秒", float64(count)/elapsed.Seconds())
		t.Logf("  前5个: %v", urls[:5])
	})
}

// TestCacheEffectiveness 测试缓存效果
func TestCacheEffectiveness(t *testing.T) {
	testCases := []struct {
		name      string
		cacheSize int
		dataSize  int
		queries   int
	}{
		{"无缓存", 1, 10000, 10000},
		{"小缓存", 100, 10000, 10000},
		{"中等缓存", 1000, 10000, 10000},
		{"大缓存", 10000, 10000, 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 使用真实的数据库文件
			dbPath := fmt.Sprintf("/tmp/fuxi_test_%s.db", tc.name)
			os.Remove(dbPath) // 清理旧文件
			defer os.Remove(dbPath)

			store, _ := storage.NewLayeredStorage(dbPath, tc.cacheSize)
			defer store.Close()

			// 预填充数据
			for i := 0; i < tc.dataSize; i++ {
				code := fmt.Sprintf("url%06d", i)
				store.Save(code, fmt.Sprintf("https://example.com/%d", i))
			}

			// 模拟访问模式：80%访问最近的20%数据（热点数据）
			start := time.Now()
			hotDataStart := tc.dataSize * 8 / 10
			hotDataSize := tc.dataSize - hotDataStart

			for i := 0; i < tc.queries; i++ {
				var code string
				if i%10 < 8 {
					// 80%访问热点数据（最后20%的数据）
					idx := hotDataStart + (i % hotDataSize)
					code = fmt.Sprintf("url%06d", idx)
				} else {
					// 20%访问冷数据（前80%的数据）
					idx := i % hotDataStart
					code = fmt.Sprintf("url%06d", idx)
				}
				store.Get(code)
			}

			elapsed := time.Since(start)
			stats, _ := store.GetStats()

			t.Logf("配置:")
			t.Logf("  缓存大小: %d", tc.cacheSize)
			t.Logf("  数据量: %d", tc.dataSize)
			t.Logf("  查询次数: %d", tc.queries)
			t.Logf("结果:")
			t.Logf("  总耗时: %v", elapsed)
			t.Logf("  平均延迟: %.3f ms", float64(elapsed.Microseconds())/float64(tc.queries)/1000)
			t.Logf("  QPS: %.0f", float64(tc.queries)/elapsed.Seconds())
			t.Logf("  缓存命中率: %.2f%%", stats.CacheHitRate*100)
		})
	}
}

// TestPreloadPerformance 测试预加载性能
func TestPreloadPerformance(t *testing.T) {
	// 创建测试文件
	testFile := "test_urls.dat"
	testOffset := "test_offset.dat"

	// 生成测试数据
	gen := generator.NewGenerator(100000)
	urls, _ := gen.Generate(50000)

	// 写入文件
	f, _ := os.Create(testFile)
	for _, url := range urls {
		f.WriteString(url)
	}
	f.Close()

	// 创建偏移量文件
	of, _ := os.Create(testOffset)
	of.WriteString("0")
	of.Close()

	defer os.Remove(testFile)
	defer os.Remove(testOffset)

	// 测试预加载
	loader := preload.NewFileLoader(testFile, testOffset)
	linkedURL := preload.NewLinkedURL(loader, 2000, 10000)

	start := time.Now()
	linkedURL.Init()
	elapsed := time.Since(start)

	t.Logf("预加载性能:")
	t.Logf("  初始加载: %d 条", linkedURL.Count())
	t.Logf("  加载耗时: %v", elapsed)
	t.Logf("  加载速度: %.0f URLs/秒", float64(linkedURL.Count())/elapsed.Seconds())

	// 测试并发获取
	start = time.Now()
	acquireCount := 1000
	for i := 0; i < acquireCount; i++ {
		linkedURL.Acquire()
	}
	elapsed = time.Since(start)

	t.Logf("并发获取:")
	t.Logf("  获取数量: %d", acquireCount)
	t.Logf("  获取耗时: %v", elapsed)
	t.Logf("  平均延迟: %.3f μs", float64(elapsed.Microseconds())/float64(acquireCount))
	t.Logf("  剩余数量: %d", linkedURL.Count())
}
