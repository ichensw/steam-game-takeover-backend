package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func loadKookChannelSortLocation() (*time.Location, error) {
	return time.LoadLocation("Asia/Shanghai")
}

func (h *Handler) AdminPreviewKookChannelSort(c *gin.Context) {
	plan, start, end, err := h.planKookChannelSort(c.Request.Context(), kookHTTPGateway{h: h})
	if err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, err.Error())
		return
	}
	ok(c, "success", kookChannelSortPreview(plan, start, end))
}

func (h *Handler) AdminRunKookChannelSort(c *gin.Context) {
	run, err := h.executeKookChannelSort(c.Request.Context(), "manual", nil)
	if err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, err.Error())
		return
	}
	ok(c, "success", run)
}

func (h *Handler) AdminMoveKookChannel(c *gin.Context) {
	var input kookChannelMoveRequest
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.TargetParentID) == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid move request")
		return
	}
	owner := "move-" + c.Param("channelId") + "-" + time.Now().Format("150405.000000000")
	acquired, err := h.acquireKookChannelSortLease(owner, kookChannelSortLeaseTTL)
	if err != nil || !acquired {
		fail(c, http.StatusConflict, CodeKookOperationFailed, "another channel movement is running")
		return
	}
	defer h.releaseKookChannelSortLease(owner) //nolint:errcheck
	gateway := kookHTTPGateway{h: h}
	channels, err := gateway.ListChannels(context.Background())
	if err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, err.Error())
		return
	}
	position, err := resolveKookManualPosition(channels, c.Param("channelId"), input)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	if err := updateKookChannelWithRetry(c.Request.Context(), gateway, position, nil); err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, err.Error())
		return
	}
	ok(c, "success", position)
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
