package main

import (
	"flag"
	"fuxi/internal/generator"
	"log"
	"os"
	"time"
)

func main() {
	// 解析命令行参数
	count := flag.Int("count", 1000000, "生成短URL的数量")
	output := flag.String("output", "data/shorturls.dat", "输出文件路径")
	bloomSize := flag.Int("bloom", 10000000, "布隆过滤器大小")
	flag.Parse()

	log.Printf("开始生成 %d 条短URL...\n", *count)
	log.Printf("布隆过滤器大小: %d\n", *bloomSize)

	startTime := time.Now()

	// 创建生成器
	gen := generator.NewGenerator(*bloomSize)

	// 生成短URL
	urls, err := gen.Generate(*count)
	if err != nil {
		log.Fatalf("生成失败: %v", err)
	}

	generateTime := time.Since(startTime)
	log.Printf("生成完成，耗时: %v\n", generateTime)
	log.Printf("平均速度: %.0f URLs/秒\n", float64(*count)/generateTime.Seconds())

	// 确保输出目录存在
	os.MkdirAll("data", 0755)

	// 写入文件
	log.Printf("写入文件: %s\n", *output)
	file, err := os.Create(*output)
	if err != nil {
		log.Fatalf("创建文件失败: %v", err)
	}
	defer file.Close()

	writeStartTime := time.Now()
	for _, url := range urls {
		file.WriteString(url)
	}

	writeTime := time.Since(writeStartTime)
	log.Printf("写入完成，耗时: %v\n", writeTime)

	// 获取文件大小
	info, _ := file.Stat()
	sizeMB := float64(info.Size()) / 1024 / 1024
	log.Printf("文件大小: %.2f MB\n", sizeMB)

	// 创建偏移量文件
	offsetFile := "data/offset.dat"
	offset, err := os.Create(offsetFile)
	if err != nil {
		log.Fatalf("创建偏移量文件失败: %v", err)
	}
	offset.WriteString("0")
	offset.Close()

	log.Printf("偏移量文件: %s\n", offsetFile)

	totalTime := time.Since(startTime)
	log.Printf("\n=== 总结 ===")
	log.Printf("总耗时: %v\n", totalTime)
	log.Printf("生成数量: %d\n", len(urls))
	log.Printf("文件大小: %.2f MB\n", sizeMB)
	log.Printf("生成速度: %.0f URLs/秒\n", float64(*count)/generateTime.Seconds())

	// 显示示例
	log.Printf("\n=== 示例短URL (前10个) ===")
	for i := 0; i < 10 && i < len(urls); i++ {
		log.Printf("%d: %s\n", i+1, urls[i])
	}
}
