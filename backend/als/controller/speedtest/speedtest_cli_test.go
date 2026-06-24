package speedtest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandleSpeedtestDotNetMissingSession covers the 500 path when
// no clientSession is set on the gin context.
func TestHandleSpeedtestDotNetMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/probe", HandleSpeedtestDotNet)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500; body = %s", w.Code, w.Body.String())
	}
}
