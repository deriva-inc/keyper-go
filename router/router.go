package router

import (
	"github.com/gin-gonic/gin"
)

// TODO: Placeholder for an auth middleware we will create later
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, we'll just let all requests pass.
		// Later, this will check for a valid JWT.
		c.Next()
	}
}

func SetupRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		// --- Authentication ---
		auth := v1.Group("/auth")
		{
			// The handler functions (e.g., handlers.Register) will be created in the handlers files
			// auth.GET("/register", handlers.Register)
			// auth.POST("/login", handlers.Login)
			auth.GET("/users", func(c *gin.Context) {
				c.JSON(200, gin.H{"status": "UP"})
			})
		}

		// --- Authenticated Routes ---
		authRequired := v1.Group("/")
		authRequired.Use(AuthMiddleware())
		{
			// --- User & Profiles ---
			// authRequired.GET("/me", handlers.GetMe)
			// authRequired.GET("/profiles", handlers.GetProfiles)
			// authRequired.POST("/profiles", handlers.CreateProfile)
			// authRequired.PUT("/profiles/:profileId", handlers.UpdateProfile)
			// authRequired.DELETE("/profiles/:profileId", handlers.DeleteProfile)

			// --- Groups ---
			// authRequired.GET("/profiles/:profileId/groups", handlers.GetGroups)
			// authRequired.POST("/profiles/:profileId/groups", handlers.CreateGroup)
			// authRequired.PUT("/groups/:groupId", handlers.UpdateGroup)
			// authRequired.DELETE("/groups/:groupId", handlers.DeleteGroup)

			// --- Vault Entries ---
			// authRequired.GET("/entries", handlers.GetEntries)
			// authRequired.POST("/entries", handlers.CreateEntry)
			// authRequired.GET("/entries/:entryId", handlers.GetEntry)
			// authRequired.PUT("/entries/:entryId", handlers.UpdateEntry)
			// authRequired.DELETE("/entries/:entryId", handlers.DeleteEntry)
		}
	}
}
