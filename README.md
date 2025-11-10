# Fuxi 短URL系统 - 本地验证版

## 项目简介

这是基于李智慧《[高并发架构实战课](blog.md)》中短URL系统设计的**本地验证实现**。

本项目通过简化的本地实现来验证原文中的核心技术点，用代码和数据说明架构设计的合理性。

## 核心技术验证

本项目重点验证以下技术点：

### 1. 预生成短URL策略
- ✅ 随机生成算法
- ✅ 布隆过滤器去重
- ✅ 文件存储管理
- ✅ 与Hash/自增方案的性能对比

### 2. 预加载链表管理
- ✅ 内存链表实现
- ✅ 头指针消费模式
- ✅ 异步加载机制
- ✅ 偏移量文件互斥

### 3. 分层缓存策略
- ✅ 内存缓存（模拟Redis）
- ✅ SQLite数据库（模拟HBase）
- ✅ 文件存储（模拟HDFS）
- ✅ 缓存命中率测试

### 4. 性能测试
- ✅ 并发访问测试
- ✅ 响应时间统计
- ✅ 吞吐量测量
- ✅ 不同方案对比

## 架构简化对照

| 原架构（生产环境） | 本地验证方案 | 验证目标 |
|------------------|------------|---------|
| HDFS (86.4GB) | 本地文件 (100MB) | 偏移量互斥机制 |
| HBase (12TB) | SQLite (1GB) | 查询性能 |
| Redis (100GB) | 内存缓存 (100MB) | 缓存命中率 |
| 144亿条短URL | 100万条短URL | 算法正确性 |
| 4万QPS | 1000 QPS | 性能瓶颈 |

## 项目结构

```
fuxi/
├── cmd/
│   ├── generator/          # 预生成短URL工具
│   ├── api/               # API服务器
│   └── benchmark/         # 性能测试工具
├── internal/
│   ├── generator/         # 生成器实现
│   ├── preload/          # 预加载链表
│   ├── storage/          # 存储层
│   └── shorturl/         # 核心业务逻辑
├── test/                 # 测试代码
├── scripts/              # 脚本工具
├── data/                 # 数据文件
└── docs/                 # 文档
```

## 快速开始

### 1. 预生成短URL

```bash
# 生成100万条短URL（约6MB文件）
go run cmd/generator/main.go -count 1000000
```

输出：
- `data/shorturls.dat` - 短URL数据文件
- `data/offset.dat` - 偏移量文件

### 2. 启动API服务

```bash
# 启动API服务器（端口8080）
go run cmd/api/main.go
```

API接口：
- `POST /api/shorten` - 生成短URL
- `GET /:code` - 短URL重定向
- `GET /api/stats` - 统计信息

### 3. 运行性能测试

```bash
# 运行基准测试
go test -bench=. ./test/

# 运行压力测试
./scripts/load_test.sh
```

## 真实测试结果

> **重要说明**: 
> - 真实的测试数据请查看 [REAL_TEST_RESULTS.md](REAL_TEST_RESULTS.md)
> - TECH_BLOG.md 中的部分数据是为了说明概念的示例
> - 运行 `make test` 可以获取你本机的真实测试数据

### 快速测试结果预览

| 测试项 | 结果 |
|-------|------|
| 随机生成速度 | 753万 URLs/秒 |
| 序列生成速度 | 2049万 URLs/秒（但可预测⚠️）|
| 缓存性能提升 | **168倍** |
| 缓存延迟 | 0.105μs |
| 数据库延迟 | 12.8μs |
| 预加载延迟 | 1.2μs |

详细结果请运行：
```bash
make test  # 运行所有测试
go test -v ./test/ -run TestGenerationMethods  # 生成算法对比
go test -v ./test/ -run TestCacheDemo  # 缓存效果演示
go test -bench=. ./test/  # 基准测试
```

## 技术博客

详细的技术分析和验证过程请查看：[TECH_BLOG.md](TECH_BLOG.md)

博客内容包括：
1. 为什么采用预生成策略？
2. 链表预加载机制详解
3. 偏移量文件的互斥实现
4. 缓存策略的性能影响
5. 从本地到分布式的扩展思路

## 关键代码示例

### 预生成短URL

```go
// 使用随机数生成短URL
func generateShortURL() string {
    const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
    result := make([]byte, 6)
    for i := range result {
        result[i] = charset[rand.Intn(64)]
    }
    return string(result)
}
```

### 预加载链表

```go
type URLNode struct {
    Code string
    Next *URLNode
}

type LinkedURL struct {
    head   *URLNode
    count  int
    loader *FileLoader
}

func (l *LinkedURL) Acquire() string {
    // 获取头节点的短URL
    code := l.head.Code
    l.head = l.head.Next
    l.count--
    
    // 剩余不足阈值时触发异步加载
    if l.count < 2000 {
        go l.loadMore()
    }
    
    return code
}
```

### 偏移量互斥

```go
func (f *FileLoader) LoadBatch() ([]string, error) {
    // 写打开偏移量文件（获取锁）
    offsetFile, err := os.OpenFile("offset.dat", os.O_RDWR, 0644)
    if err != nil {
        return nil, err
    }
    defer offsetFile.Unlock() // 释放锁
    
    // 读取当前偏移量
    offset := readOffset(offsetFile)
    
    // 读取短URL数据
    urls := readURLsFromFile(offset, 10000)
    
    // 更新偏移量
    writeOffset(offsetFile, offset + 60000)
    
    return urls, nil
}
```

## 扩展思考

### 如何扩展到生产环境？

1. **存储层升级**
   - 文件存储 → HDFS集群
   - SQLite → HBase集群
   - 内存缓存 → Redis集群

2. **服务层扩展**
   - 单实例 → 多实例负载均衡
   - 本地锁 → 分布式锁（ZooKeeper）
   - 同步操作 → 消息队列（Kafka）

3. **容量扩展**
   - 100万条 → 100亿条
   - 100MB → 10TB
   - 垂直扩展 + 水平扩展

### 性能优化方向

1. **缓存优化**：多级缓存、预热策略
2. **数据库优化**：索引优化、分区分表
3. **网络优化**：CDN加速、连接池
4. **并发优化**：协程池、批量操作

## 参考资料

- [李智慧 - 高并发架构实战课](blog.md)
- [原始设计文档](.backup/blog.md)

## GitHub仓库

本项目源码：[https://github.com/d60-Lab/shorturl](https://github.com/d60-Lab/shorturl)

欢迎Star⭐、Fork和提Issue！

## License

MIT License
