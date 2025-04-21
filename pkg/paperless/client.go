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
			Timeout: time.Second * 10,
		}
	})
	return client
}
