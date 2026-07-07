package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess                   = "SUCCESS"
	CodeParamInvalid              = "PARAM_INVALID"
	CodeUnauthorized              = "UNAUTHORIZED"
	CodeProfileIncomplete         = "PROFILE_INCOMPLETE"
	CodeUserBanned                = "USER_BANNED"
	CodeTakeoverNotFound          = "TAKEOVER_NOT_FOUND"
	CodeTakeoverFull              = "TAKEOVER_FULL"
	CodeAlreadyJoined             = "ALREADY_JOINED"
	CodeTakeoverTimeConflict      = "TAKEOVER_TIME_CONFLICT"
	CodeReportAlreadyExists       = "REPORT_ALREADY_EXISTS"
	CodeCannotReportSelf          = "CANNOT_REPORT_SELF"
	CodeReportedUserNotInTakeover = "REPORTED_USER_NOT_IN_TAKEOVER"
	CodeSteamIDTaken              = "STEAM_ID_TAKEN"
	CodeAdminUnauthorized         = "ADMIN_UNAUTHORIZED"
	CodeSystemError               = "SYSTEM_ERROR"
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
	CodeParamInvalid:              "参数不正确，请检查后再试",
	CodeUnauthorized:              "登录状态已失效，请重新进入小程序",
	CodeProfileIncomplete:         "请先完善个人资料",
	CodeUserBanned:                "账号已被封禁，如有疑问请联系管理员",
	CodeTakeoverNotFound:          "接龙不存在或已被删除",
	CodeTakeoverFull:              "接龙人数已满",
	CodeAlreadyJoined:             "您已经加入过这个接龙",
	CodeTakeoverTimeConflict:      "同一时间你已经加入了其他接龙",
	CodeReportAlreadyExists:       "已举报过该用户",
	CodeCannotReportSelf:          "不能举报自己",
	CodeReportedUserNotInTakeover: "被举报用户不在该接龙中",
	CodeSteamIDTaken:              "SteamID 已被其他玩家绑定，请确认后再填写。",
	CodeAdminUnauthorized:         "当前账号暂无管理员权限",
	CodeSystemError:               "系统开小差了，请稍后再试",
}

var messageTranslations = map[string]string{
	"admin unauthorized":                         "当前账号暂无管理员权限",
	"already joined":                             "您已经加入过这个接龙",
	"avatarUrl must be at most 255 characters":   "头像地址过长，请重新上传",
	"cannot report yourself":                     "不能举报自己",
	"content security reject":                    "内容包含不合规信息，请修改后再提交",
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
	"invalid admin password":                     "新密码需为 6-64 个字符",
	"invalid admin profile":                      "管理员资料不正确，请检查后再试",
	"invalid request":                            "请求内容不正确，请检查后再试",
	"invalid username or password":               "用户名或密码不正确",
	"invalid takeover id":                        "接龙信息不正确，请刷新后再试",
	"invalid user id":                            "用户信息不正确，请刷新后再试",
	"join failed":                                "加入接龙失败，请稍后再试",
	"kook channel query failed":                  "获取 KOOK 频道失败，请稍后再试",
	"kook invite create failed":                  "KOOK 频道邀请链接生成失败，请稍后再试",
	"leave failed":                               "退出接龙失败，请稍后再试",
	"login failed":                               "登录失败，请稍后再试",
	"nickname is required and must be at most 32 characters": "请输入 32 个字以内的昵称",
	"nickname already taken":                                 "该昵称已被使用，请换一个昵称",
	"nickname must be between 2 and 12 characters":           "请输入 2-12 个字的昵称",
	"not joined": "您还没有加入这个接龙",
	"only jpg, png, gif, and webp images are allowed":               "仅支持上传 JPG、PNG、GIF 或 WebP 图片",
	"open upload failed":                                            "读取图片失败，请重新选择",
	"oss not configured":                                            "图片上传服务暂不可用",
	"old password invalid":                                          "原密码不正确",
	"participantLimit cannot be lower than joinedCount":             "人数上限不能小于已加入人数",
	"password is required":                                          "请输入管理员密码",
	"penalty score must be 0, 5, 10, or 20":                         "扣分只能选择 0、5、10 或 20",
	"profile incomplete":                                            "请先完善个人资料",
	"publish takeover disabled":                                     "暂未开放发起接龙",
	"query failed":                                                  "获取数据失败，请稍后再试",
	"ended takeover cannot be deleted":                              "已结束的接龙不可删除",
	"ended takeover cannot be joined":                               "已结束的接龙不可加入",
	"ended takeover cannot be modified":                             "已结束的接龙不可编辑",
	"report content is required and must be at most 500 characters": "请填写 500 个字以内的举报内容",
	"report failed":                                                 "举报提交失败，请稍后再试",
	"report already submitted":                                      "已举报过该用户，请勿重复提交",
	"report already handled":                                        "该举报已处理，请勿重复操作",
	"report handle failed":                                          "举报处理失败，请稍后再试",
	"report image url is required":                                  "举报截图地址不能为空",
	"report image url must be at most 512 characters":               "举报截图地址过长，请重新上传",
	"report image url must be at most 255 characters":               "举报截图地址过长，请重新上传",
	"report images must be at most 9":                               "举报截图最多上传 9 张",
	"report not found":                                              "举报记录不存在",
	"reported user is not in takeover":                              "只能举报当前接龙内的队友",
	"save failed":                                                   "保存失败，请稍后再试",
	"steamId already taken":                                         "SteamID 已被其他玩家绑定，请确认后再填写。",
	"steamId must be at most 64 characters":                         "SteamID 不能超过 64 个字符",
	"steamId must contain digits only":                              "SteamID 只能填写数字",
	"steam friend code invalid":                                     "Steam好友码错误，请填写正确的好友码。",
	"steam friend code check failed":                                "Steam好友码校验失败，请稍后再试",
	"takeover full":                                                 "接龙人数已满",
	"takeover time conflict":                                        "同一时间你已经加入了其他接龙",
	"takeover not found":                                            "接龙不存在或已被删除",
	"token sign failed":                                             "登录状态生成失败，请稍后再试",
	"unauthorized":                                                  "登录状态已失效，请重新进入小程序",
	"upload failed":                                                 "图片上传失败，请稍后再试",
	"user banned":                                                   "账号已被封禁，如有疑问请联系管理员",
	"user not found":                                                "用户不存在",
	"wechat login failed":                                           "微信登录失败，请稍后再试",
}
