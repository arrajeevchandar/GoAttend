package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DeviceAuth enforces bearer JWT tokens signed with HS256.
func DeviceAuth(signingKey, issuer string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		if authz == "" || !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tokenStr := strings.TrimSpace(authz[len("bearer "):])
		claims, err := Parse(tokenStr, signingKey, issuer)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}
