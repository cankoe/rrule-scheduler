package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func APIKeyMiddleware(apiKey string, isAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		providedKey := c.GetHeader("X-API-KEY")
		if providedKey == "" || providedKey != apiKey {
			log.Warn().Str("middleware", "APIKeyMiddleware").Msg("Invalid or missing API key")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing API key"})
			c.Abort()
			return
		}

		c.Set("isAdmin", isAdmin)
		log.Debug().Bool("isAdmin", isAdmin).Msg("API key validated, proceeding with request")
		c.Next()
	}
}
