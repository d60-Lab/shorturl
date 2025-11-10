package preload

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

// URLNode 短URL链表节点
type URLNode struct {
	Code string   // 短URL代码
	Next *URLNode // 下一个节点
}

// LinkedURL 短URL链表管理器
type LinkedURL struct {
	head      *URLNode    // 链表头指针
	tail      *URLNode    // 链表尾指针
	count     int         // 当前节点数量
	threshold int         // 触发加载的阈值
	batchSize int         // 每次加载的数量
	loader    *FileLoader // 文件加载器
	mu        sync.Mutex  // 互斥锁
	loading   bool        // 是否正在加载
}

// FileLoader 文件加载器
type FileLoader struct {
	urlFilePath    string // 短URL文件路径
	offsetFilePath string // 偏移量文件路径
	mu             sync.Mutex
}

// NewFileLoader 创建文件加载器
func NewFileLoader(urlFile, offsetFile string) *FileLoader {
	return &FileLoader{
		urlFilePath:    urlFile,
		offsetFilePath: offsetFile,
	}
}

// LoadBatch 加载一批短URL（带文件锁互斥）
func (f *FileLoader) LoadBatch(count int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 1. 写打开偏移量文件（获取独占锁）
	offsetFile, err := os.OpenFile(f.offsetFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open offset file: %w", err)
	}
	defer offsetFile.Close()

	// 2. 对偏移量文件加独占锁
	err = syscall.Flock(int(offsetFile.Fd()), syscall.LOCK_EX)
	if err != nil {
		return nil, fmt.Errorf("failed to lock offset file: %w", err)
	}
	defer syscall.Flock(int(offsetFile.Fd()), syscall.LOCK_UN)

	// 3. 读取当前偏移量
	offset, err := readOffset(offsetFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read offset: %w", err)
	}

	// 4. 读打开短URL文件
	urlFile, err := os.Open(f.urlFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open url file: %w", err)
	}
	defer urlFile.Close()

	// 5. 从偏移量位置读取数据
	urls, bytesRead, err := readURLsFromFile(urlFile, offset, count)
	if err != nil {
		return nil, fmt.Errorf("failed to read urls: %w", err)
	}

	// 6. 更新偏移量
	newOffset := offset + int64(bytesRead)
	err = writeOffset(offsetFile, newOffset)
	if err != nil {
		return nil, fmt.Errorf("failed to write offset: %w", err)
	}

	return urls, nil
}

// readOffset 读取偏移量
func readOffset(file *os.File) (int64, error) {
	file.Seek(0, 0)
	var offset int64
	_, err := fmt.Fscanf(file, "%d", &offset)
	if err != nil {
		// 如果文件为空，返回0
		return 0, nil
	}
	return offset, nil
}

// writeOffset 写入偏移量
func writeOffset(file *os.File, offset int64) error {
	file.Seek(0, 0)
	file.Truncate(0)
	_, err := fmt.Fprintf(file, "%d", offset)
	return err
}

// readURLsFromFile 从文件读取短URL
func readURLsFromFile(file *os.File, offset int64, count int) ([]string, int, error) {
	const urlLength = 6
	bytesToRead := count * urlLength

	buffer := make([]byte, bytesToRead)
	file.Seek(offset, 0)

	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return nil, 0, err
	}

	// 解析短URL
	urls := make([]string, 0, n/urlLength)
	for i := 0; i+urlLength <= n; i += urlLength {
		urls = append(urls, string(buffer[i:i+urlLength]))
	}

	return urls, n, nil
}

// NewLinkedURL 创建链表管理器
func NewLinkedURL(loader *FileLoader, threshold, batchSize int) *LinkedURL {
	return &LinkedURL{
		threshold: threshold,
		batchSize: batchSize,
		loader:    loader,
	}
}

// Init 初始化链表，预加载第一批数据
func (l *LinkedURL) Init() error {
	return l.loadMore()
}

// Acquire 获取一个短URL
func (l *LinkedURL) Acquire() (string, error) {
	l.mu.Lock()

	if l.head == nil {
		l.mu.Unlock()
		return "", fmt.Errorf("no URLs available")
	}

	// 获取头节点的短URL
	code := l.head.Code

	// 移动头指针
	oldHead := l.head
	l.head = l.head.Next
	oldHead.Next = nil // 帮助GC回收
	l.count--

	// 如果链表为空，更新tail
	if l.head == nil {
		l.tail = nil
	}

	// 检查是否需要触发加载
	needLoad := l.count < l.threshold && !l.loading
	l.mu.Unlock()

	// 异步加载更多数据
	if needLoad {
		go l.loadMore()
	}

	return code, nil
}

// loadMore 加载更多短URL到链表
func (l *LinkedURL) loadMore() error {
	l.mu.Lock()

	// 检查是否已经在加载
	if l.loading {
		l.mu.Unlock()
		return nil
	}

	l.loading = true
	l.mu.Unlock()

	// 从文件加载
	urls, err := l.loader.LoadBatch(l.batchSize)
	if err != nil {
		l.mu.Lock()
		l.loading = false
		l.mu.Unlock()
		return err
	}

	// 构建链表节点
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, code := range urls {
		node := &URLNode{Code: code}

		if l.tail == nil {
			// 链表为空
			l.head = node
			l.tail = node
		} else {
			// 追加到尾部
			l.tail.Next = node
			l.tail = node
		}

		l.count++
	}

	l.loading = false
	return nil
}

// Count 返回当前链表中的节点数量
func (l *LinkedURL) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// IsLoading 返回是否正在加载
func (l *LinkedURL) IsLoading() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loading
}
