package connector

import (
	"net/http"
	"time"
)

func NewHTTPConnector(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}
