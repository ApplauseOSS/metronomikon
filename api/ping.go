package api

import (
	"github.com/gin-gonic/gin"
)

func handlePing(c *gin.Context) {
	c.String(200, "pong")
}
