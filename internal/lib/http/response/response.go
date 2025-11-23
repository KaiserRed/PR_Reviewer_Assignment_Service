package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

func NewErrorResponse(code, message string) gin.H {
	return gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	}
}

func NewSuccessResponse(data interface{}) gin.H {
	return gin.H{
		"data": data,
	}
}

func HealthResponse() gin.H {
	return gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	}
}
