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
			auth.POST("/signup", handlers.RegisterUser(dbIns))
			auth.POST("/login", handlers.Login(dbIns))
			auth.POST("/refresh", handlers.RefreshToken(dbIns))

			// Recovery endpoints
			auth.POST("/recovery/setup", handlers.RecoverySetup(dbIns))
			auth.POST("/recovery/initiate", handlers.RecoveryInitiate(dbIns))
			auth.POST("/reset-password/confirm", handlers.ResetPasswordConfirm(dbIns))
			auth.POST("/recovery/complete", handlers.RecoveryComplete(dbIns))
		}
		users := v1.Group("/users")
		{
			users.GET("/salt", handlers.GetUserSalt(dbIns))
		}

		// SECTION: Health & Kubernetes Probes
		health := v1.Group("/health")
		{
			health.GET("", handlers.HealthCheck(dbIns))
			health.GET("/live", handlers.LivenessProbe())
			health.GET("/ready", handlers.ReadinessProbe(dbIns))
			health.GET("/startup", handlers.StartupProbe(dbIns))
		}

		// SECTION: Authenticated Routes
		authRequired := v1.Group("/")
		authRequired.Use(middleware.AuthRequired())
		{
			// Auth actions requiring active JWT
			authRequired.POST("/auth/logout", handlers.Logout(dbIns))
			authRequired.POST("/auth/logout-all", handlers.RevokeAllSessions(dbIns))

			// SECTION: Users API Endpoints
			users := authRequired.Group("/users")
			{
				users.GET("/", handlers.GetUserProfile(dbIns))
				users.DELETE("/", handlers.DeleteUserProfile(dbIns))
				users.PATCH("/", handlers.UpdateUserProfile(dbIns))
			}
			// !SECTION: Users API Endpoints

			// SECTION: Profiles
			profiles := authRequired.Group("/profiles")
			{
				profiles.GET("", handlers.GetProfiles(dbIns))
				profiles.POST("", handlers.CreateProfile(dbIns))

				profiles.GET("/:profileId", handlers.GetProfile(dbIns))
				profiles.PATCH("/:profileId", handlers.UpdateProfile(dbIns))
				profiles.DELETE("/:profileId", handlers.DeleteProfile(dbIns))

				profiles.GET("/:profileId/groups", handlers.GetProfileGroups(dbIns))
				profiles.GET("/:profileId/entries", handlers.GetProfileEntries(dbIns))
				profiles.GET("/:profileId/groups/count", handlers.GetProfileGroupCount(dbIns))
				profiles.GET("/:profileId/entries/count", handlers.GetProfileEntryCount(dbIns))
			}
			// !SECTION: Profiles

			// SECTION: Groups
			groups := authRequired.Group("/groups")
			{
				groups.GET("", handlers.GetAllGroupsInProfile(dbIns))
				groups.POST("", handlers.CreateGroup(dbIns))

				groups.GET("/:groupId", handlers.GetGroup(dbIns))
				groups.PATCH("/:groupId", handlers.UpdateGroup(dbIns))
				groups.DELETE("/:groupId", handlers.DeleteGroup(dbIns))

				groups.GET("/:groupId/entries", handlers.GetGroupEntries(dbIns))
				groups.GET("/:groupId/entries/count", handlers.GetGroupEntriesCount(dbIns))
			}

			// !SECTION: Groups

			// SECTION: Vault Entries
			vaultEntries := authRequired.Group("/entries")
			{
				vaultEntries.GET("", handlers.GetEntries(dbIns))
				vaultEntries.POST("", handlers.CreateEntry(dbIns))

				vaultEntries.GET("/favorites", handlers.GetFavoriteEntries(dbIns))
				vaultEntries.PATCH("/:entryId/favorite", handlers.FavoriteEntry(dbIns))

				vaultEntries.GET("/:entryId", handlers.GetEntry(dbIns))
				vaultEntries.PATCH("/:entryId", handlers.UpdateEntry(dbIns))
				vaultEntries.DELETE("/:entryId", handlers.DeleteEntry(dbIns))

			}
			// !SECTION: Vault Entries

			// SECTION: Dashboard
			dashboard := authRequired.Group("/dashboard")
			{
				dashboard.GET("", handlers.GetDashboardOverview(dbIns))
				dashboard.GET("/stats", handlers.GetDashboardStats(dbIns))
				dashboard.GET("/security-score", handlers.GetSecurityScore(dbIns))
				dashboard.GET("/distribution", handlers.GetEntryDistribution(dbIns))
				dashboard.GET("/activity", handlers.GetRecentActivity(dbIns))
			}
			// !SECTION: Dashboard
		}
		// !SECTION: Authenticated Routes
		// !SECTION: Authentication
	}
}
