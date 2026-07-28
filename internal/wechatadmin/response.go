package wechatadmin

import (
	"encoding/json"
	"net/http"
)

type apiResponse struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Code: "SUCCESS", Message: "ok", Data: data})
}

func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiResponse{Success: false, Code: code, Message: message, Data: nil})
}

func writeJSON(w http.ResponseWriter, status int, body apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
