package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gravitl/netmaker/models"
)

// ReturnSuccessResponse - success api response
func ReturnSuccessResponse(c *gin.Context, message string, responseBody interface{}) {
	var httpResponse models.SuccessResponse
	httpResponse.Code = http.StatusOK
	httpResponse.Message = message
	httpResponse.Response = responseBody
	if httpResponse.Response == nil {
		httpResponse.Response = struct{}{}
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(http.StatusOK, httpResponse)
}

// ReturnErrorResponse - error api response
func ReturnErrorResponse(c *gin.Context, errorMessage models.ErrorResponse) {
	httpResponse := &models.ErrorResponse{Code: errorMessage.Code, Message: errorMessage.Message}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(errorMessage.Code, httpResponse)
}

// AbortWithError - abort api request with error
func AbortWithError(c *gin.Context, errorMessage models.ErrorResponse) {
	httpResponse := &models.ErrorResponse{Code: errorMessage.Code, Message: errorMessage.Message}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.AbortWithStatusJSON(errorMessage.Code, httpResponse)
}
