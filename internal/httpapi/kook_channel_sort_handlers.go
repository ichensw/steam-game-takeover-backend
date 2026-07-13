package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func loadKookChannelSortLocation() (*time.Location, error) {
	return time.LoadLocation("Asia/Shanghai")
}

func (h *Handler) AdminGetKookChannelSortConfig(c *gin.Context) {
	config, err := h.getKookChannelSortConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", config)
}

func (h *Handler) AdminUpdateKookChannelSortConfig(c *gin.Context) {
	var input kookChannelSortConfigDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	location, err := loadKookChannelSortLocation()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "load timezone failed")
		return
	}
	config, err := h.saveKookChannelSortConfig(input, time.Now(), location)
	if err != nil {
		if validationErr := validateKookChannelSortConfig(input); validationErr != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, validationErr.Error())
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "success", config)
}

func (h *Handler) AdminListKookChannelSortRuns(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	runs, total, err := h.listKookChannelSortRuns(page, pageSize)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", gin.H{"list": runs, "total": total, "page": page, "pageSize": pageSize})
}
