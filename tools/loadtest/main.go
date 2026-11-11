// Command loadtest 是一个最小化的 HTTP 压测工具。
//
// 为什么不用 wrk / hey / ab：
//   部署机出站 DNS 不可用，装不了任何外部工具；Go 交叉编译是唯一零依赖的路径。
//
// 设计取舍：
//   - 只统计"客户端看到的真实延迟"，不假设服务端指标，黑盒测量。
//   - 每个 worker 各自收集延迟样本，结束后合并排序算分位数，
//     避免所有 goroutine 抢同一把锁把工具自己变成瓶颈。
//   - 不重试、不对失败请求重放：失败就是失败，重试会掩盖真实错误率。
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// headerFlags 支持多次传入 -H。
type headerFlags []string

func (h *headerFlags) String() string { return strings.Join(*h, "; ") }
func (h *headerFlags) Set(v string) error {
	*h = append(*h, v)
	return nil
}

func main() {
	var (
		target = flag.String("url", "", "目标 URL（必填）")
		conc   = flag.Int("c", 10, "并发数")
		total  = flag.Int("n", 0, "总请求数，0 表示按 -d 运行")
		dur    = flag.Duration("d", 10*time.Second, "持续时间（-n 为 0 时生效）")
		method = flag.String("method", "GET", "HTTP 方法")
		body   = flag.String("body", "", "请求体")
		warmup = flag.Duration("warmup", 1*time.Second, "预热时长，用于建立连接与 JIT 预热")
	)
	var headers headerFlags
	flag.Var(&headers, "H", "附加请求头，格式 'Key: Value'，可重复")
	flag.Parse()

	if *target == "" {
		fmt.Fprintln(os.Stderr, "缺少 -url")
		os.Exit(2)
	}

	reqBody := []byte(*body)
	tr := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 2000,
		// 不设 MaxConnsPerHost：设了会把并发压回队列，测不到服务的真实上限。
		DisableKeepAlives: false,
		DisableCompression: true, // 压测不该把带宽浪费在解压上
		IdleConnTimeout:    90 * time.Second,
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
		// 不跟随重定向：重定向会让一次请求变成两次，扭曲 QPS。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	newReq := func() *http.Request {
		req, err := http.NewRequest(*method, *target, bytes.NewReader(reqBody))
		if err != nil {
			fmt.Fprintf(os.Stderr, "构造请求失败: %v\n", err)
			os.Exit(2)
		}
		for _, h := range headers {
			if k, v, ok := strings.Cut(h, ":"); ok {
				req.Header.Set(strings.TrimSpace(k), strings.TrimSpace(v))
			}
		}
		return req
	}

	// 预热：先把连接建起来，否则前几十个请求的延迟里混着 TCP 握手时间。
	if *warmup > 0 {
		warmCtx, cancel := context.WithTimeout(context.Background(), *warmup)
		var wg sync.WaitGroup
		for i := 0; i < *conc; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for warmCtx.Err() == nil {
					if resp, err := client.Do(newReq()); err == nil {
						_, _ = io.Copy(io.Discard, resp.Body)
						_ = resp.Body.Close()
					}
				}
			}()
		}
		wg.Wait()
		cancel()
	}

	var (
		okCount   atomic.Int64
		failCount atomic.Int64
		codeMu    sync.Mutex
		codes     = map[int]int64{}
		latBufs   = make([][]int64, *conc)
		stop      atomic.Bool
	)

	deadline := time.Now().Add(*dur)
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lat := make([]int64, 0, 4096)
			localCodes := map[int]int64{}

			for !stop.Load() {
				if *total > 0 {
					// 预分配模式下用全局计数切分。
					// 这里必须 break 而不是 return：return 会跳过下面的
					// latBufs[id] = lat，导致该 worker 的样本为空，
					// 后续 all[0] 直接 panic。
					if okCount.Load()+failCount.Load() >= int64(*total) {
						break
					}
				} else if time.Now().After(deadline) {
					break
				}

				t0 := time.Now()
				resp, err := client.Do(newReq())
				elapsed := time.Since(t0)

				if err != nil {
					failCount.Add(1)
					lat = append(lat, elapsed.Microseconds())
					localCodes[-1]++
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()

				lat = append(lat, elapsed.Microseconds())
				localCodes[resp.StatusCode]++
				// 2xx/3xx 视为成功；4xx/5xx 是服务端表达拒绝，必须计入失败。
				if resp.StatusCode >= 400 {
					failCount.Add(1)
				} else {
					okCount.Add(1)
				}
			}
			latBufs[id] = lat

			codeMu.Lock()
			for k, v := range localCodes {
				codes[k] += v
			}
			codeMu.Unlock()
		}(i)
	}

	// 只有"按时长跑"的模式才需要主 goroutine 到点叫停。
	// 固定请求数模式（-n）下，worker 自己达到总数就退出，
	// 这里如果也置 stop，worker 会在发出第一个请求之前就被终止，
	// 结果就是"没有采集到有效请求"。
	if *total == 0 {
		time.Sleep(time.Until(deadline))
		stop.Store(true)
	}
	wg.Wait()

	elapsed := time.Since(start)

	var all []int64
	for _, buf := range latBufs {
		all = append(all, buf...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	totalReq := okCount.Load() + failCount.Load()
	if totalReq == 0 || len(all) == 0 {
		fmt.Println("没有采集到有效请求")
		return
	}

	pct := func(p float64) float64 {
		if len(all) == 0 {
			return 0
		}
		idx := int(float64(len(all)-1) * p)
		return float64(all[idx]) / 1000.0
	}

	fmt.Println(strings.Repeat("=", 62))
	fmt.Printf("目标    %s %s\n", *method, *target)
	fmt.Printf("并发    %d    实测时长 %.2fs    预热 %.0fs\n", *conc, elapsed.Seconds(), warmup.Seconds())
	fmt.Println(strings.Repeat("-", 62))
	fmt.Printf("总请求  %d    成功 %d    失败 %d    错误率 %.2f%%\n",
		totalReq, okCount.Load(), failCount.Load(),
		float64(failCount.Load())/float64(totalReq)*100)
	fmt.Printf("QPS     %.1f\n", float64(totalReq)/elapsed.Seconds())
	fmt.Println(strings.Repeat("-", 62))
	fmt.Printf("延迟(ms)  min %.2f   p50 %.2f   p90 %.2f   p95 %.2f   p99 %.2f   max %.2f\n",
		float64(all[0])/1000.0, pct(0.50), pct(0.90), pct(0.95), pct(0.99),
		float64(all[len(all)-1])/1000.0)
	fmt.Println(strings.Repeat("-", 62))
	keys := make([]int, 0, len(codes))
	for k := range codes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var parts []string
	for _, k := range keys {
		label := fmt.Sprintf("%d", k)
		if k == -1 {
			label = "连接错误"
		}
		parts = append(parts, fmt.Sprintf("%s:%d", label, codes[k]))
	}
	fmt.Printf("状态码  %s\n", strings.Join(parts, "  "))
	fmt.Println(strings.Repeat("=", 62))
}
