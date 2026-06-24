package cache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
)

// withTimerCache overrides the timer package's interface cache map for
// the duration of t.
func withTimerCache(t *testing.T, m map[int]*timer.InterfaceTrafficCache) {
	t.Helper()
	prev := timer.InterfaceCaches
	timer.InterfaceCaches = m
	t.Cleanup(func() { timer.InterfaceCaches = prev })
}

// withClientSession registers a client session under id.
func withClientSession(t *testing.T, id string) {
	t.Helper()
	client.AddClient(id, &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	})
	t.Cleanup(func() { client.RemoveClient(id) })
	_ = context.TODO()
}

func TestUpdateInterfaceCacheSendsToChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Seed the timer cache with one entry.
	withTimerCache(t, map[int]*timer.InterfaceTrafficCache{
		1: {
			InterfaceName: "eth0",
			LastCacheTime: time.Unix(1700000000, 0),
			Caches: [][3]uint64{
				{1700000000, 1024, 2048},
			},
		},
	})

	withClientSession(t, "test-session")

	r := gin.New()
	r.GET("/cache/interfaces", func(c *gin.Context) {
		s, _ := client.GetClient("test-session")
		c.Set("clientSession", s)
		UpdateInterfaceCache(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/cache/interfaces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json body: %v", err)
	}
	if body["success"] != true {
		t.Errorf("success = %v; want true", body["success"])
	}

	// The handler must also have pushed an InterfaceCache message
	// onto the session channel.
	session, ok := client.GetClient("test-session")
	if !ok {
		t.Fatal("test-session not in client map")
	}
	select {
	case msg := <-session.Channel:
		if msg.Name != "InterfaceCache" {
			t.Errorf("message name = %q; want InterfaceCache", msg.Name)
		}
		if msg.Content == "" {
			t.Error("InterfaceCache message content is empty")
		}
	case <-time.After(time.Second):
		t.Fatal("InterfaceCache message not delivered")
	}
}

func TestUpdateInterfaceCacheEmptyCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withTimerCache(t, map[int]*timer.InterfaceTrafficCache{})
	withClientSession(t, "empty-session")

	r := gin.New()
	r.GET("/cache/interfaces", func(c *gin.Context) {
		s, _ := client.GetClient("empty-session")
		c.Set("clientSession", s)
		UpdateInterfaceCache(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/cache/interfaces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}

	// Even with empty cache, an empty InterfaceCache message must be
	// delivered so the client can re-render with no data.
	session, ok := client.GetClient("empty-session")
	if !ok {
		t.Fatal("empty-session not in client map")
	}
	select {
	case msg := <-session.Channel:
		if msg.Name != "InterfaceCache" {
			t.Errorf("name = %q", msg.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("InterfaceCache message not delivered for empty cache")
	}
}
