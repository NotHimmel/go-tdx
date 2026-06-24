package transport

import (
	"sort"
	"sync"
	"time"
)

// PingResult 单台服务器的延迟测量结果。
type PingResult struct {
	Host    string
	Latency time.Duration
}

// Ping 连接并完成首条握手，返回耗时；失败返回 (0,false)。
func Ping(host string, port int, timeout time.Duration) (time.Duration, bool) {
	if port <= 0 {
		port = DefaultPort
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	t0 := time.Now()
	c := NewConn(host, port, timeout)
	if err := c.Connect(); err != nil {
		return 0, false
	}
	c.Close()
	return time.Since(t0), true
}

// PingAll 并发测量所有候选服务器，返回按延迟升序排序的可达列表。
func PingAll(hosts []string, port int, timeout time.Duration) []PingResult {
	if hosts == nil {
		hosts = FallbackHosts
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []PingResult
	)
	for _, h := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			if lat, ok := Ping(host, port, timeout); ok {
				mu.Lock()
				results = append(results, PingResult{host, lat})
				mu.Unlock()
			}
		}(h)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})
	return results
}

// BestHost 返回延迟最低的可达服务器；全部不可达返回空串。
func BestHost(hosts []string, port int, timeout time.Duration) string {
	r := PingAll(hosts, port, timeout)
	if len(r) == 0 {
		return ""
	}
	return r[0].Host
}
