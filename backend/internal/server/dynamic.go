package server

import (
	"net/http"
	"sync/atomic"
)

type dynamicHandler struct {
	current atomic.Value
}

func newDynamicHandler(handler http.Handler) *dynamicHandler {
	d := &dynamicHandler{}
	d.current.Store(handler)
	return d
}

func (d *dynamicHandler) Update(handler http.Handler) {
	if handler == nil {
		return
	}
	d.current.Store(handler)
}

func (d *dynamicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := d.current.Load()
	if handler, ok := h.(http.Handler); ok && handler != nil {
		handler.ServeHTTP(w, r)
		return
	}
	http.Error(w, "handler unavailable", http.StatusServiceUnavailable)
}
