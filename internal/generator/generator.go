package generator

import (
	"crypto/rand"
	"fmt"
	"sync"
)

// Base64字符集（URL安全版本）
const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// BloomFilter 简单的布隆过滤器实现
type BloomFilter struct {
	bits []bool
	size int
	mu   sync.RWMutex
}

// NewBloomFilter 创建布隆过滤器
func NewBloomFilter(size int) *BloomFilter {
	return &BloomFilter{
		bits: make([]bool, size),
		size: size,
	}
}

// Add 添加元素
func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	hash1 := simpleHash(item, 0) % bf.size
	hash2 := simpleHash(item, 1) % bf.size
	hash3 := simpleHash(item, 2) % bf.size

	bf.bits[hash1] = true
	bf.bits[hash2] = true
	bf.bits[hash3] = true
}

// Contains 检查元素是否可能存在
func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	hash1 := simpleHash(item, 0) % bf.size
	hash2 := simpleHash(item, 1) % bf.size
	hash3 := simpleHash(item, 2) % bf.size

	return bf.bits[hash1] && bf.bits[hash2] && bf.bits[hash3]
}

// simpleHash 简单的哈希函数
func simpleHash(s string, seed int) int {
	hash := seed
	for i := 0; i < len(s); i++ {
		hash = hash*31 + int(s[i])
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// Generator 短URL生成器
type Generator struct {
	bf *BloomFilter
}

// NewGenerator 创建生成器
func NewGenerator(bloomSize int) *Generator {
	return &Generator{
		bf: NewBloomFilter(bloomSize),
	}
}

// Generate 生成指定数量的短URL
func (g *Generator) Generate(count int) ([]string, error) {
	urls := make([]string, 0, count)
	generated := 0
	attempts := 0
	maxAttempts := count * 2 // 最多尝试2倍次数

	for generated < count && attempts < maxAttempts {
		code := g.generateOne()
		attempts++

		// 使用布隆过滤器检查是否已存在
		if !g.bf.Contains(code) {
			g.bf.Add(code)
			urls = append(urls, code)
			generated++
		}
	}

	if generated < count {
		return urls, fmt.Errorf("only generated %d URLs out of %d requested", generated, count)
	}

	return urls, nil
}

// generateOne 生成单个短URL（6个字符）
func (g *Generator) generateOne() string {
	// 使用crypto/rand生成高质量随机数
	bytes := make([]byte, 6)
	rand.Read(bytes)

	result := make([]byte, 6)
	for i := 0; i < 6; i++ {
		result[i] = charset[int(bytes[i])%64]
	}

	return string(result)
}

// GenerateWithHash Hash截断方式生成（用于对比测试）
func GenerateWithHash(input string) string {
	// 简单的哈希实现
	hash := 0
	for i := 0; i < len(input); i++ {
		hash = hash*31 + int(input[i])
	}
	if hash < 0 {
		hash = -hash
	}

	// 转为Base64编码
	result := make([]byte, 6)
	for i := 0; i < 6; i++ {
		result[i] = charset[hash%64]
		hash = hash / 64
	}

	return string(result)
}

// GenerateWithSequence 自增序列方式生成（用于对比测试）
func GenerateWithSequence(seq int64) string {
	if seq < 0 {
		seq = -seq
	}

	result := make([]byte, 6)
	for i := 0; i < 6; i++ {
		result[5-i] = charset[seq%64]
		seq = seq / 64
	}

	return string(result)
}

// Stats 生成统计信息
type Stats struct {
	TotalGenerated int     // 总生成数量
	UniqueCount    int     // 唯一数量
	CollisionRate  float64 // 冲突率
	AvgTime        float64 // 平均生成时间（ms）
}
