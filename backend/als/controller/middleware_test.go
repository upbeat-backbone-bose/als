package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func TestMiddlewareSessionOnHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clearTestClients(t)

	// Register a known session in the global map.
	id := "header-ok"
	client.AddClient(id, &client.ClientSession{
		Channel:   make(chan *client.Message, 1),
		CreatedAt: time.Now(),
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "valid session header lets request through", header: id, wantStatus: http.StatusOK},
		{name: "missing session header is rejected", header: "", wantStatus: http.StatusBadRequest},
		{name: "unknown session id is rejected", header: "bogus", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(MiddlewareSessionOnHeader())
			r.GET("/probe", func(c *gin.Context) {
				v, ok := c.Get("clientSession")
				if !ok {
					t.Errorf("clientSession not set on context")
				}
				if v == nil {
					t.Errorf("clientSession is nil")
				}
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
			if tt.header != "" {
				req.Header.Set("session", tt.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d; want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestMiddlewareSessionOnUrl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clearTestClients(t)

	id := "url-ok"
	client.AddClient(id, &client.ClientSession{
		Channel:   make(chan *client.Message, 1),
		CreatedAt: time.Now(),
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "valid session in URL passes", path: "/probe/" + id, wantStatus: http.StatusOK},
		{name: "unknown session in URL is rejected", path: "/probe/bogus", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/probe/:session", MiddlewareSessionOnUrl(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d; want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestMiddlewareSessionOnHeaderErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clearTestClients(t)

	r := gin.New()
	r.Use(MiddlewareSessionOnHeader())
	r.GET("/", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json body: %v", err)
	}
	if body["success"] != false {
		t.Errorf("success = %v; want false", body["success"])
	}
	if !strings.Contains(strings.ToLower(fmt.Sprint(body["error"])), "session") {
		t.Errorf("error = %v; want it to mention session", body["error"])
	}
}

func clearTestClients(t *testing.T) {
	t.Helper()
	client.RemoveAllClients()
	t.Cleanup(client.RemoveAllClients)
}
