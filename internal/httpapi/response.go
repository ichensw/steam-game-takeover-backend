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
	c.JSON(status, APIResponse{Success: false, Code: code, Message: friendlyMessage(code, message), Data: nil})
}

func friendlyMessage(code string, message string) string {
	if translated, ok := messageTranslations[message]; ok {
		return translated
	}
	if translated, ok := codeTranslations[code]; ok {
		return translated
	}
	if message == "" {
		return "操作失败，请稍后再试"
	}
	return message
}

var codeTranslations = map[string]string{
	CodeParamInvalid:         "参数不正确，请检查后再试",
	CodeUnauthorized:         "登录状态已失效，请重新进入小程序",
	CodeProfileIncomplete:    "请先完善个人资料",
	CodeUserBlocked:          "您已被管理员拉黑",
	CodeTakeoverNotFound:     "接龙不存在或已被删除",
	CodeTakeoverFull:         "接龙人数已满",
	CodeAlreadyJoined:        "您已经加入过这个接龙",
	CodeAdminUnauthorized:    "管理员登录已失效，请重新登录",
	CodeAdminPasswordInvalid: "管理员密码不正确",
	CodeSystemError:          "系统开小差了，请稍后再试",
}

var messageTranslations = map[string]string{
	"admin password is not configured":           "管理员密码尚未配置",
	"admin unauthorized":                         "当前账号暂无管理员权限",
	"already joined":                             "您已经加入过这个接龙",
	"avatarUrl must be at most 255 characters":   "头像地址过长，请重新上传",
	"block failed":                               "拉黑失败，请稍后再试",
	"code is required":                           "登录凭证缺失，请重新进入小程序",
	"create failed":                              "创建接龙失败，请稍后再试",
	"credit restore failed":                      "信誉分恢复失败，请稍后再试",
	"credit too low for create":                  "当前信誉分过低，暂无法发起接龙，请联系管理员处理",
	"credit too low for join":                    "当前信誉分低于 70，暂无法参与接龙，请联系管理员处理",
	"creator cannot leave takeover":              "创建人不能退出自己创建的接龙",
	"delete failed":                              "删除失败，请稍后再试",
	"file is required":                           "请选择要上传的图片",
	"gender must be 1 or 2":                      "请选择性别",
	"handle note must be at most 500 characters": "处理说明不能超过 500 个字",
	"image must be between 1 byte and 5 MB":      "图片大小不能超过 5MB",
	"invalid admin password":                     "管理员密码不正确",
	"invalid admin token":                        "管理员登录已失效，请重新登录",
	"invalid request":                            "请求内容不正确，请检查后再试",
	"invalid takeover id":                        "接龙信息不正确，请刷新后再试",
	"invalid user id":                            "用户信息不正确，请刷新后再试",
	"join failed":                                "加入接龙失败，请稍后再试",
	"leave failed":                               "退出接龙失败，请稍后再试",
	"login failed":                               "登录失败，请稍后再试",
	"nickname is required and must be at most 32 characters": "请输入 32 个字以内的昵称",
	"not joined": "您还没有加入这个接龙",
	"only jpg, png, gif, and webp images are allowed":               "仅支持上传 JPG、PNG、GIF 或 WebP 图片",
	"open upload failed":                                            "读取图片失败，请重新选择",
	"oss not configured":                                            "图片上传服务暂不可用",
	"participantLimit cannot be lower than joinedCount":             "人数上限不能小于已加入人数",
	"password is required":                                          "请输入管理员密码",
	"penalty score must be 0, 5, 10, or 20":                         "扣分只能选择 0、5、10 或 20",
	"profile incomplete":                                            "请先完善个人资料",
	"publish takeover disabled":                                     "暂未开放发起接龙",
	"query failed":                                                  "获取数据失败，请稍后再试",
	"reason must be at most 255 characters":                         "拉黑原因不能超过 255 个字",
	"ended takeover cannot be deleted":                              "已结束的接龙不可删除",
	"ended takeover cannot be modified":                             "已结束的接龙不可编辑",
	"report content is required and must be at most 500 characters": "请填写 500 个字以内的举报内容",
	"report failed":                                                 "举报提交失败，请稍后再试",
	"report already handled":                                        "该举报已处理，请勿重复操作",
	"report handle failed":                                          "举报处理失败，请稍后再试",
	"report image url must be at most 255 characters":               "举报截图地址过长，请重新上传",
	"report not found":                                              "举报记录不存在",
	"reported user is not in takeover":                              "只能举报当前接龙内的队友",
	"save failed":                                                   "保存失败，请稍后再试",
	"steamId cannot be changed":                                     "SteamID 只能填写一次，填错了请联系管理员",
	"steamId is required and must be at most 64 characters":         "请输入 64 个字符以内的 SteamID",
	"takeover full":                                                 "接龙人数已满",
	"takeover not found":                                            "接龙不存在或已被删除",
	"token sign failed":                                             "登录状态生成失败，请稍后再试",
	"unauthorized":                                                  "登录状态已失效，请重新进入小程序",
	"unblock failed":                                                "解除拉黑失败，请稍后再试",
	"upload failed":                                                 "图片上传失败，请稍后再试",
	"user blocked":                                                  "您已被管理员拉黑",
	"user not found":                                                "用户不存在",
	"wechat login failed":                                           "微信登录失败，请稍后再试",
}
