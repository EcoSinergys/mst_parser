package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// UserAgentPool содержит список современных User-Agent для ротации
var UserAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36 Edg/127.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36 OPR/113.0.0.0",
}

// ScraperClient обёртка над http.Client с защитой от блокировок
type ScraperClient struct {
	client      *http.Client
	baseReferer string
	minDelay    time.Duration
	maxDelay    time.Duration
	maxRetries  int
}

// NewScraperClient создаёт новый клиент для скрапинга
func NewScraperClient(minDelay, maxDelay time.Duration, maxRetries int) *ScraperClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// CookieJar для поддержания сессии
	jar, _ := cookiejar.New(nil)

	// Если задержки слишком маленькие — увеличиваем
	if minDelay < 2*time.Second {
		minDelay = 2 * time.Second
	}
	if maxDelay < 5*time.Second {
		maxDelay = 5 * time.Second
	}

	return &ScraperClient{
		client: &http.Client{
			Timeout:   60 * time.Second,
			Transport: transport,
			Jar:       jar,
		},
		baseReferer: "https://www.mstpumps.com/",
		minDelay:    minDelay,
		maxDelay:    maxDelay,
		maxRetries:  maxRetries,
	}
}

// randomUserAgent возвращает случайный User-Agent из пула
func (sc *ScraperClient) randomUserAgent() string {
	return UserAgentPool[rand.Intn(len(UserAgentPool))]
}

// setHeaders добавляет заголовки для защиты от блокировки
func (sc *ScraperClient) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", sc.randomUserAgent())
	req.Header.Set("Referer", sc.baseReferer)
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("DNT", "1")
}

// UpdateReferer обновляет referer на текущий URL (после перехода)
func (sc *ScraperClient) UpdateReferer(url string) {
	sc.baseReferer = url
}

// RandomDelay делает случайную паузу между запросами
func (sc *ScraperClient) RandomDelay() {
	delay := sc.minDelay + time.Duration(rand.Int63n(int64(sc.maxDelay-sc.minDelay)))
	fmt.Printf("⏳ Пауза %v...\n", delay)
	time.Sleep(delay)
}

// ReadBody читает всё тело ответа в []byte
func ReadBody(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Get выполняет HTTP GET с защитой и retry
func (sc *ScraperClient) Get(url string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= sc.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(2<<uint(attempt-1)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			sleepTime := backoff + jitter
			fmt.Printf("🔄 Попытка %d из %d, ожидание %v...\n", attempt+1, sc.maxRetries+1, sleepTime)
			time.Sleep(sleepTime)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			fmt.Printf("⚠️ Ошибка создания запроса: %v\n", err)
			continue
		}

		sc.setHeaders(req)
		resp, err := sc.client.Do(req)

		if err != nil {
			lastErr = err
			fmt.Printf("⚠️ Ошибка сети: %v\n", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			fmt.Printf("⚠️ Сервер вернул %d (попытка %d)\n", resp.StatusCode, attempt+1)
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			extraDelay := time.Duration(5+rand.Intn(10)) * time.Second
			fmt.Printf("⏳ Дополнительная пауза %v...\n", extraDelay)
			time.Sleep(extraDelay)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("⚠️ Сервер вернул %d для %s\n", resp.StatusCode, url)
			resp.Body.Close()
			if resp.StatusCode == 404 {
				return nil, fmt.Errorf("страница не найдена (404): %s", url)
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		sc.UpdateReferer(url)
		return resp, nil
	}

	return nil, fmt.Errorf("все попытки запроса исчерпаны для %s: %v", url, lastErr)
}
