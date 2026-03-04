package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSlogMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(SlogMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSlogMiddleware_RecordsDuration(t *testing.T) {
	r := gin.New()
	r.Use(SlogMiddleware())
	r.GET("/slow", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/slow", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSlogMiddleware_404(t *testing.T) {
	r := gin.New()
	r.Use(SlogMiddleware())
	r.GET("/exists", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-found", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func setupAuthRouter(token string) *gin.Engine {
	r := gin.New()
	r.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	auth := r.Group("/")
	auth.Use(BearerAuthMiddleware(token))
	auth.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doAuthRequest(r *gin.Engine, path, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestBearerAuth_ValidToken(t *testing.T) {
	r := setupAuthRouter("my-secret")
	w := doAuthRequest(r, "/protected", "Bearer my-secret")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	r := setupAuthRouter("my-secret")
	w := doAuthRequest(r, "/protected", "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "missing authorization header" {
		t.Errorf("unexpected error: %s", body["error"])
	}
}

func TestBearerAuth_WrongToken(t *testing.T) {
	r := setupAuthRouter("my-secret")
	w := doAuthRequest(r, "/protected", "Bearer wrong-token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "invalid token" {
		t.Errorf("unexpected error: %s", body["error"])
	}
}

func TestBearerAuth_MalformedHeader(t *testing.T) {
	r := setupAuthRouter("my-secret")

	// No "Bearer " prefix
	w := doAuthRequest(r, "/protected", "my-secret")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing prefix, got %d", w.Code)
	}

	// Basic auth instead of Bearer
	w = doAuthRequest(r, "/protected", "Basic dXNlcjpwYXNz")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for Basic auth, got %d", w.Code)
	}
}

func TestBearerAuth_CaseInsensitiveScheme(t *testing.T) {
	r := setupAuthRouter("my-secret")
	w := doAuthRequest(r, "/protected", "bearer my-secret")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for lowercase 'bearer', got %d", w.Code)
	}
}

func TestBearerAuth_PublicRouteUnaffected(t *testing.T) {
	r := setupAuthRouter("my-secret")
	w := doAuthRequest(r, "/public", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for public route, got %d", w.Code)
	}
}
