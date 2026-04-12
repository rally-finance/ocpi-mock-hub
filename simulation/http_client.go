package simulation

import (
	"net/http"
	"sync"
	"time"
)

var (
	httpClientMu       sync.RWMutex
	outboundHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

func SetHTTPClient(client *http.Client) {
	httpClientMu.Lock()
	defer httpClientMu.Unlock()
	if client == nil {
		outboundHTTPClient = &http.Client{Timeout: 10 * time.Second}
		return
	}
	outboundHTTPClient = client
}

func httpClient() *http.Client {
	httpClientMu.RLock()
	defer httpClientMu.RUnlock()
	return outboundHTTPClient
}
