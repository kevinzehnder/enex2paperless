package paperless

import (
	"net/http"
	"sync"
	"time"
)

var (
	client *http.Client
	once   sync.Once
)

func getSharedClient() *http.Client {
	once.Do(func() {
		client = &http.Client{
			Timeout: time.Second * 100,
		}
	})
	return client
}
