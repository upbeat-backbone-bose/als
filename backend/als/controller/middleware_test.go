package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func setupTestRouter(clientMgr *client.ClientManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MiddlewareSessionOnHeader(clientMgr))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})
	return router
}

func TestMiddlewareSessionOnHeader_ValidSession(t *testing.T) {
	clientMgr := client.NewClientManager()
	
	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 10),
		CreatedAt: time.Now(),
	}
	clientMgr.AddClient("valid-session-id", session)
	
	router := setupTestRouter(clientMgr)
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("session", "valid-session-id")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddlewareSessionOnHeader_InvalidSession(t *testing.T) {
	clientMgr := client.NewClientManager()
	router := setupTestRouter(clientMgr)
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("session", "invalid-session")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMiddlewareSessionOnHeader_MissingSession(t *testing.T) {
	clientMgr := client.NewClientManager()
	router := setupTestRouter(clientMgr)
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMiddlewareSessionOnUrl_ValidSession(t *testing.T) {
	clientMgr := client.NewClientManager()
	
	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 10),
		CreatedAt: time.Now(),
	}
	clientMgr.AddClient("valid-session-id", session)
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MiddlewareSessionOnUrl(clientMgr))
	router.GET("/test/:session", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test/valid-session-id", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddlewareSessionOnUrl_InvalidSession(t *testing.T) {
	clientMgr := client.NewClientManager()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MiddlewareSessionOnUrl(clientMgr))
	router.GET("/test/:session", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test/invalid-session", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
