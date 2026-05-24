package handlers

import (
	"errors"
	"net/http"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/gin-gonic/gin"
)

type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeError(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, gin.H{"error": ApiError{Code: code, Message: message, Details: details}})
}

func BadRequest(c *gin.Context, message string, details any) {
	writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", message, details)
}

func NotFound(c *gin.Context, message string, details any) {
	writeError(c, http.StatusNotFound, "NOT_FOUND", message, details)
}

func Conflict(c *gin.Context, message string, details any) {
	writeError(c, http.StatusConflict, "CONFLICT", message, details)
}

func Unauthorized(c *gin.Context, message string, details any) {
	writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", message, details)
}

func ServiceUnavailable(c *gin.Context, message string, details any) {
	writeError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", message, details)
}

func InternalServerError(c *gin.Context, message string, details any) {
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", message, details)
}

func HandleError(c *gin.Context, err error) {
	var validationErr *appErrors.ValidationError

	switch {
	case errors.As(err, &validationErr):
		BadRequest(c, "Invalid request parameters", gin.H{"field": validationErr.Field, "issue": validationErr.Issue})
	case errors.Is(err, appErrors.ErrPluginNotFound):
		NotFound(c, "Plugin not found", nil)
	case errors.Is(err, appErrors.ErrDuplicatePlugin):
		Conflict(c, "Plugin name already exists", gin.H{"field": "name", "issue": "duplicate"})
	case errors.Is(err, appErrors.ErrDatabaseUnavailable):
		InternalServerError(c, "Database unavailable", nil)
	default:
		InternalServerError(c, "Internal server error", nil)
	}
}
