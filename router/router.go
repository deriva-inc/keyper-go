package router

import (
	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/handlers"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, dbIns *db.DB) {
	v1 := r.Group("/api/v1")
	{
		// SECTION: Authentication
		auth := v1.Group("/auth")
		{
			auth.GET("/users", func(c *gin.Context) {
				c.JSON(200, gin.H{"status": "UP"})
			})

		}

		// SECTION: Authenticated Routes
		authRequired := v1.Group("/")
		authRequired.Use(middleware.AuthRequired())
		{
			// SECTION: Users API Endpoints
			users := v1.Group("/users")
			users.GET("/me", handlers.GetUserProfile(dbIns))
			users.POST("/", handlers.PostUserProfile(dbIns))
			users.PATCH("/:userId", handlers.UpdateUserProfile(dbIns))
			// !SECTION: Users API Endpoints

			// SECTION: Profiles
			profiles := v1.Group("/profiles")
			profiles.GET("/", handlers.GetProfiles(dbIns))
			profiles.GET("/:profileId", handlers.GetProfile(dbIns))
			profiles.POST("/", handlers.CreateProfile(dbIns))
			profiles.PATCH("/:profileId", handlers.UpdateProfile(dbIns))
			profiles.DELETE("/:profileId", handlers.DeleteProfile(dbIns))
			// !SECTION: Profiles

			// SECTION: Groups
			authRequired.GET("/profiles/:profileId/groups", handlers.GetGroupsInProfile(dbIns))
			// authRequired.POST("/profiles/:profileId/groups", handlers.CreateGroup)
			// authRequired.PUT("/groups/:groupId", handlers.UpdateGroup)
			// authRequired.DELETE("/groups/:groupId", handlers.DeleteGroup)
			// !SECTION: Groups

			// SECTION: Vault Entries
			authRequired.GET("/entries", handlers.GetEntries(dbIns))
			authRequired.POST("/entries", handlers.CreateEntry(dbIns))
			authRequired.GET("/entries/:entryId", handlers.GetEntry(dbIns))
			// authRequired.PUT("/entries/:entryId", handlers.UpdateEntry)
			// authRequired.DELETE("/entries/:entryId", handlers.DeleteEntry)
			// !SECTION: Vault Entries
		}
		// !SECTION: Authenticated Routes
		// !SECTION: Authentication
	}
}
