package router

import (
	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/handlers"
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
		// authRequired := v1.Group("/")
		// authRequired.Use(middleware.AuthRequired())
		{
			// SECTION: Users API Endpoints
			users := v1.Group("/users")
			{
				users.POST("", handlers.PostUserProfile(dbIns))

				users.GET("/me", handlers.GetUserProfile(dbIns))
				users.PATCH("/:userId", handlers.UpdateUserProfile(dbIns))
			}
			// !SECTION: Users API Endpoints

			// SECTION: Profiles
			profiles := v1.Group("/profiles")
			{
				profiles.GET("", handlers.GetProfiles(dbIns))
				profiles.POST("", handlers.CreateProfile(dbIns))

				profiles.GET("/:profileId", handlers.GetProfile(dbIns))
				profiles.PATCH("/:profileId", handlers.UpdateProfile(dbIns))
				profiles.DELETE("/:profileId", handlers.DeleteProfile(dbIns))
			}
			// !SECTION: Profiles

			// SECTION: Groups
			groups := v1.Group("/groups")
			{
				groups.GET("", handlers.GetAllGroupsInProfile(dbIns))
				groups.POST("", handlers.CreateGroup(dbIns))

				groups.GET("/:groupId", handlers.GetGroup(dbIns))
				groups.PATCH("/:groupId", handlers.UpdateGroup(dbIns))
				groups.DELETE("/:groupId", handlers.DeleteGroup(dbIns))
			}

			// !SECTION: Groups

			// SECTION: Vault Entries
			vaultEntries := v1.Group("/entries")
			{
				vaultEntries.GET("", handlers.GetEntries(dbIns))
				vaultEntries.POST("", handlers.CreateEntry(dbIns))

				vaultEntries.GET("/:entryId", handlers.GetEntry(dbIns))
				vaultEntries.PATCH("/:entryId", handlers.UpdateEntry(dbIns))
				vaultEntries.DELETE("/:entryId", handlers.DeleteEntry(dbIns))
			}
			// !SECTION: Vault Entries
		}
		// !SECTION: Authenticated Routes
		// !SECTION: Authentication
	}
}
