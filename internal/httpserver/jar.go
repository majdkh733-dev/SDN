package httpserver

import (
	"net/http"
	"net/url"
	"sync"
)

type memoryJar struct {
	mu      sync.Mutex
	cookies map[string][]*http.Cookie
}

func newJar() http.CookieJar { return &memoryJar{cookies: map[string][]*http.Cookie{}} }

func (j *memoryJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cookies[u.Host] = append(j.cookies[u.Host], cookies...)
}

func (j *memoryJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cookies[u.Host]
}
