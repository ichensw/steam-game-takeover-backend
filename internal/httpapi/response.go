package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess              = "SUCCESS"
	CodeParamInvalid         = "PARAM_INVALID"
	CodeUnauthorized         = "UNAUTHORIZED"
	CodeProfileIncomplete    = "PROFILE_INCOMPLETE"
	CodeUserBlocked          = "USER_BLOCKED"
	CodeTakeoverNotFound     = "TAKEOVER_NOT_FOUND"
	CodeTakeoverFull         = "TAKEOVER_FULL"
	CodeAlreadyJoined        = "ALREADY_JOINED"
	CodeAdminUnauthorized    = "ADMIN_UNAUTHORIZED"
	CodeAdminPasswordInvalid = "ADMIN_PASSWORD_INVALID"
	CodeSystemError          = "SYSTEM_ERROR"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func ok(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Success: true, Code: CodeSuccess, Message: message, Data: data})
}

func fail(c *gin.Context, status int, code string, message string) {
	c.JSON(status, APIResponse{Success: false, Code: code, Message: message, Data: nil})
}
