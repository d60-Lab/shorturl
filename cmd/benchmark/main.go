package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	TotalDuration time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	QPS           float64
}

func main() {
	apiURL := flag.String("url", "http://localhost:8080", "API服务地址")
	concurrent := flag.Int("c", 100, "并发数")
	requests := flag.Int("n", 10000, "总请求数")
	flag.Parse()

	log.Printf("=== Fuxi 性能测试 ===")
	log.Printf("目标: %s", *apiURL)
	log.Printf("并发: %d", *concurrent)
	log.Printf("请求: %d", *requests)
	log.Println()

	// 运行创建短URL测试
	log.Println(">>> 测试1: 创建短URL")
	createResult := benchmarkCreate(*apiURL, *concurrent, *requests)
	printResult(createResult)
	log.Println()

	// 运行重定向测试
	log.Println(">>> 测试2: 短URL重定向")
	redirectResult := benchmarkRedirect(*apiURL, *concurrent, *requests)
	printResult(redirectResult)
	log.Println()

	// 获取统计信息
	log.Println(">>> 服务器统计信息")
	printStats(*apiURL)
}

func benchmarkCreate(apiURL string, concurrent, requests int) *Result {
	var (
		successCount int64
		failureCount int64
		latencies    []time.Duration
		latenciesMu  sync.Mutex
		wg           sync.WaitGroup
	)

	requestsPerWorker := requests / concurrent
	startTime := time.Now()

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			for j := 0; j < requestsPerWorker; j++ {
				reqData := map[string]string{
					"long_url": fmt.Sprintf("https://example.com/test/%d/%d", workerID, j),
				}

				jsonData, _ := json.Marshal(reqData)
				reqStart := time.Now()

				resp, err := client.Post(
					apiURL+"/api/shorten",
					"application/json",
					bytes.NewBuffer(jsonData),
				)

				latency := time.Since(reqStart)

				if err != nil || resp.StatusCode != 200 {
					atomic.AddInt64(&failureCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
					latenciesMu.Lock()
					latencies = append(latencies, latency)
					latenciesMu.Unlock()
				}

				if resp != nil {
					resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	return calculateResult(successCount, failureCount, totalDuration, latencies)
}

func benchmarkRedirect(apiURL string, concurrent, requests int) *Result {
	// 先创建一些短URL
	codes := make([]string, 1000)
	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 1000; i++ {
		reqData := map[string]string{
			"long_url": fmt.Sprintf("https://example.com/redirect-test/%d", i),
		}
		jsonData, _ := json.Marshal(reqData)

		resp, err := client.Post(
			apiURL+"/api/shorten",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err == nil && resp.StatusCode == 200 {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if code, ok := result["short_code"].(string); ok {
				codes[i] = code
			}
			resp.Body.Close()
		}
	}

	log.Printf("预创建了 %d 个短URL", len(codes))

	// 开始重定向测试
	var (
		successCount int64
		failureCount int64
		latencies    []time.Duration
		latenciesMu  sync.Mutex
		wg           sync.WaitGroup
	)

	requestsPerWorker := requests / concurrent
	startTime := time.Now()

	// 禁用自动重定向
	noRedirectClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				code := codes[j%len(codes)]
				reqStart := time.Now()

				resp, err := noRedirectClient.Get(apiURL + "/" + code)
				latency := time.Since(reqStart)

				if err != nil || (resp.StatusCode != 302 && resp.StatusCode != 301) {
					atomic.AddInt64(&failureCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
					latenciesMu.Lock()
					latencies = append(latencies, latency)
					latenciesMu.Unlock()
				}

				if resp != nil {
					resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	return calculateResult(successCount, failureCount, totalDuration, latencies)
}

func calculateResult(successCount, failureCount int64, totalDuration time.Duration, latencies []time.Duration) *Result {
	result := &Result{
		TotalRequests: successCount + failureCount,
		SuccessCount:  successCount,
		FailureCount:  failureCount,
		TotalDuration: totalDuration,
	}

	if len(latencies) == 0 {
		return result
	}

	// 排序延迟
	sortLatencies(latencies)

	result.MinLatency = latencies[0]
	result.MaxLatency = latencies[len(latencies)-1]

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	result.AvgLatency = totalLatency / time.Duration(len(latencies))

	result.P50Latency = latencies[len(latencies)*50/100]
	result.P95Latency = latencies[len(latencies)*95/100]
	result.P99Latency = latencies[len(latencies)*99/100]

	result.QPS = float64(successCount) / totalDuration.Seconds()

	return result
}

func sortLatencies(latencies []time.Duration) {
	// 简单冒泡排序
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}
}

func printResult(result *Result) {
	log.Printf("总请求数: %d", result.TotalRequests)
	log.Printf("成功: %d, 失败: %d", result.SuccessCount, result.FailureCount)
	log.Printf("总耗时: %v", result.TotalDuration)
	log.Printf("QPS: %.2f", result.QPS)
	log.Printf("延迟统计:")
	log.Printf("  最小: %v", result.MinLatency)
	log.Printf("  平均: %v", result.AvgLatency)
	log.Printf("  P50:  %v", result.P50Latency)
	log.Printf("  P95:  %v", result.P95Latency)
	log.Printf("  P99:  %v", result.P99Latency)
	log.Printf("  最大: %v", result.MaxLatency)
}

func printStats(apiURL string) {
	resp, err := http.Get(apiURL + "/api/stats")
	if err != nil {
		log.Printf("获取统计失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)

	log.Printf("总URL数: %.0f", stats["total_urls"])
	log.Printf("活跃URL: %.0f", stats["active_urls"])
	log.Printf("总访问量: %.0f", stats["total_access"])
	log.Printf("缓存命中率: %v", stats["cache_hit_rate"])
	log.Printf("预加载数量: %.0f", stats["preload_count"])
}
