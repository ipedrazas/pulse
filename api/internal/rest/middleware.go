package rest

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerAuthMiddleware returns a Gin middleware that validates the
// Authorization: Bearer <token> header against the expected token.
func BearerAuthMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Next()
	}
}
