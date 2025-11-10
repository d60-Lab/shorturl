package main

import (
	"flag"
	"fmt"
	"fuxi/internal/preload"
	"fuxi/internal/storage"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	linkedURL *preload.LinkedURL
	store     storage.Storage
)

func main() {
	// 解析命令行参数
	port := flag.Int("port", 8080, "服务端口")
	dbPath := flag.String("db", "data/fuxi.db", "数据库文件路径")
	urlFile := flag.String("urls", "data/shorturls.dat", "短URL文件路径")
	offsetFile := flag.String("offset", "data/offset.dat", "偏移量文件路径")
	cacheSize := flag.Int("cache", 100000, "缓存大小")
	flag.Parse()

	log.Printf("初始化Fuxi短URL服务...")

	// 初始化存储
	var err error
	store, err = storage.NewLayeredStorage(*dbPath, *cacheSize)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}
	defer store.Close()

	log.Printf("数据库: %s", *dbPath)
	log.Printf("缓存大小: %d", *cacheSize)

	// 初始化预加载链表
	loader := preload.NewFileLoader(*urlFile, *offsetFile)
	linkedURL = preload.NewLinkedURL(loader, 2000, 10000)

	err = linkedURL.Init()
	if err != nil {
		log.Fatalf("初始化预加载链表失败: %v", err)
	}

	log.Printf("预加载链表初始化完成，当前数量: %d", linkedURL.Count())

	// 启动定期日志
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			count := linkedURL.Count()
			loading := linkedURL.IsLoading()
			log.Printf("[状态] 链表数量: %d, 加载中: %v", count, loading)
		}
	}()

	// 设置Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"time":   time.Now(),
		})
	})

	// API路由
	api := r.Group("/api")
	{
		api.POST("/shorten", handleShorten)
		api.GET("/stats", handleStats)
	}

	// 短URL重定向
	r.GET("/:code", handleRedirect)

	// 启动服务器
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("服务器启动在 http://localhost%s", addr)
	log.Printf("API文档:")
	log.Printf("  POST http://localhost%s/api/shorten - 生成短URL", addr)
	log.Printf("  GET  http://localhost%s/api/stats   - 统计信息", addr)
	log.Printf("  GET  http://localhost%s/:code       - 短URL重定向", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// handleShorten 生成短URL
func handleShorten(c *gin.Context) {
	var req struct {
		LongURL string `json:"long_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "long_url is required"})
		return
	}

	// 从预加载链表获取短URL
	code, err := linkedURL.Acquire()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate short URL"})
		return
	}

	// 保存到存储
	err = store.Save(code, req.LongURL)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to save mapping"})
		return
	}

	shortURL := fmt.Sprintf("http://%s/%s", c.Request.Host, code)

	c.JSON(200, gin.H{
		"short_code": code,
		"short_url":  shortURL,
		"long_url":   req.LongURL,
	})
}

// handleRedirect 短URL重定向
func handleRedirect(c *gin.Context) {
	code := c.Param("code")

	// 查询长URL
	longURL, err := store.Get(code)
	if err != nil {
		c.JSON(404, gin.H{"error": "short URL not found"})
		return
	}

	// 增加访问计数（异步）
	go store.IncrementAccess(code)

	// 302重定向
	c.Redirect(http.StatusFound, longURL)
}

// handleStats 统计信息
func handleStats(c *gin.Context) {
	stats, err := store.GetStats()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(200, gin.H{
		"total_urls":     stats.TotalURLs,
		"active_urls":    stats.ActiveURLs,
		"expired_urls":   stats.ExpiredURLs,
		"total_access":   stats.TotalAccess,
		"cache_hit_rate": fmt.Sprintf("%.2f%%", stats.CacheHitRate*100),
		"preload_count":  linkedURL.Count(),
	})
}
