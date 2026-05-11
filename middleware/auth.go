package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

// AuthMiddleware creates a gin middleware for JWT authentication.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {

		// 1. Get the Authorization header.
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// 2. Validate the header format (should be "Bearer <token>").
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			return
		}

		tokenString := headerParts[1]

		// 3. Parse and validate the token.
		//    jwt.ParseWithClaims will do most of the work:
		//    - It decodes the token.
		//    - It verifies the signature using your secret key.
		//    - It validates standard claims like expiration (`exp`) and issuance (`iat`).
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Make sure the signing method is what you expect.
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Return the secret key for validation.
			jwtSecret := os.Getenv("JWT_SECRET")
			return []byte(jwtSecret), nil
		})

		// 4. Handle parsing errors.
		if err != nil {
			if err == jwt.ErrTokenExpired {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			}
			return
		}

		// 5. Check if the token is valid and claims are extracted.
		if !token.Valid || claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Optional: Check the issuer if you set one.
		if claims.Issuer != "keyper-api" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token issuer"})
			return
		}

		// 6. Token is valid! Attach the extracted userId to the Gin context.
		//    This makes it available to any subsequent handlers in the chain.
		c.Set("userId", claims.UserID.String())

		// 7. Call the next handler in the chain.
		c.Next()
	}
}

// Helper to extract userId from Gin context as uuid.UUID
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDStr := c.GetString("userId")
	if userIDStr == "" {
		return uuid.Nil, fmt.Errorf("userId missing from context")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid userId format: %w", err)
	}

	return userID, nil
}
