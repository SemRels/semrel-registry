package handlers

import (
	"net/http"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

func (h *AdminHandler) Status(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", appErrors.ErrNotImplemented.Error(), nil)
}
