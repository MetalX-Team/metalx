package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginAndMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storePath := t.TempDir() + "/auth.sqlite"
	manager, err := New(storePath, "admin", "secret")
	if err != nil {
		t.Fatalf("failed to create auth manager: %v", err)
	}
	defer func() {
		_ = manager.Close()
	}()

	token, ok := manager.Login("admin", "secret")
	if !ok || token == "" {
		t.Fatal("expected login to succeed")
	}

	router := gin.New()
	router.Use(manager.GinMiddleware())
	router.GET("/secure", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, resp.Code)
	}
}
