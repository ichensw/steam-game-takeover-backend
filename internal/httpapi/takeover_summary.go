package httpapi

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"
)

var summaryTimePattern = regexp.MustCompile(`(?i)(\d{1,2}[:：点]\d{0,2}|\d{1,2}\s*月\s*\d{1,2}\s*日?|\d{1,2}[/-]\d{1,2})`)

const (
	summarySourceAI       = "ai"
	summarySourceManual   = "manual"
	summarySourceFallback = "fallback"
)

type takeoverSummaryResult struct {
	Name   string
	Source string
	Error  string
	Hash   string
}

func (h *Handler) refreshTakeoverSummary(t *model.Takeover) {
	hash := takeoverSummaryHash(*t)
	if stringValue(t.SummarySource) == summarySourceManual || stringValue(t.SummaryTitleHash) == hash {
		return
	}
	result := h.extractTakeoverSummary(*t, hash)
	now := time.Now()
	updates := map[string]interface{}{
		"summary_name":       optionalStringPtr(result.Name),
		"summary_source":     optionalStringPtr(result.Source),
		"summary_title_hash": optionalStringPtr(result.Hash),
		"summary_error":      optionalStringPtr(result.Error),
		"summary_updated_at": &now,
	}
	if err := h.db.Model(&model.Takeover{}).Where("id = ?", t.ID).Updates(updates).Error; err != nil {
		return
	}
	t.SummaryName = optionalStringPtr(result.Name)
	t.SummarySource = optionalStringPtr(result.Source)
	t.SummaryTitleHash = optionalStringPtr(result.Hash)
	t.SummaryError = optionalStringPtr(result.Error)
	t.SummaryUpdatedAt = &now
}

func (h *Handler) extractTakeoverSummary(t model.Takeover, hash string) takeoverSummaryResult {
	fallback := cleanSummaryName(t.Title)
	if !h.aiExtractEnabled() {
		return takeoverSummaryResult{Name: fallback, Source: summarySourceFallback, Hash: hash}
	}
	name, err := h.extractTakeoverSummaryWithAI(t)
	if err != nil {
		return takeoverSummaryResult{Name: fallback, Source: summarySourceFallback, Error: trimSummaryError(err), Hash: hash}
	}
	name = cleanSummaryName(name)
	if name == "" {
		return takeoverSummaryResult{Name: fallback, Source: summarySourceFallback, Error: "empty ai result", Hash: hash}
	}
	return takeoverSummaryResult{Name: name, Source: summarySourceAI, Hash: hash}
}

func (h *Handler) extractTakeoverSummaryWithAI(t model.Takeover) (string, error) {
	apiKey := h.aiExtractAPIKey()
	baseURL := h.aiExtractBaseURL()
	modelName := h.aiExtractModel()
	if apiKey == "" || baseURL == "" || modelName == "" {
		return "", errors.New("ai config missing")
	}
	prompt := fmt.Sprintf("请提取用于微信群接龙汇总的短展示词，只返回 JSON：{\"summaryName\":\"...\"}。\n标题：%s\n简介：%s\n时间：%s", t.Title, stringValue(t.Description), scheduleText(t))
	if isDigits(modelName) {
		return h.extractTakeoverSummaryWithDashScopeApp(apiKey, baseURL, modelName, prompt)
	}

	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": "你是活动标题信息提取器。只返回 JSON，不要解释。字段 summaryName 表示用于微信群汇总的短展示词，优先提取游戏名、KTV、桌游、活动名等核心词，最多 12 个中文字符。"},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	payload, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai http %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("ai choices empty")
	}
	return parseSummaryName(result.Choices[0].Message.Content)
}

func (h *Handler) extractTakeoverSummaryWithDashScopeApp(apiKey string, baseURL string, appID string, prompt string) (string, error) {
	baseURL = strings.TrimSuffix(strings.Replace(baseURL, "/compatible-mode/v1", "/api/v1", 1), "/")
	body := map[string]interface{}{
		"input": map[string]string{
			"prompt": prompt,
		},
		"parameters": map[string]interface{}{},
	}
	payload, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/apps/"+appID+"/completion", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai app http %d", resp.StatusCode)
	}

	var result struct {
		Output struct {
			Text string `json:"text"`
		} `json:"output"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Output.Text) == "" {
		if result.Message != "" {
			return "", errors.New(result.Message)
		}
		return "", errors.New("ai app output empty")
	}
	return parseSummaryName(result.Output.Text)
}

func parseSummaryName(value string) (string, error) {
	var content struct {
		SummaryName string `json:"summaryName"`
	}
	text := strings.TrimSpace(value)
	if start := strings.Index(text, "{"); start >= 0 {
		if end := strings.LastIndex(text, "}"); end >= start {
			text = text[start : end+1]
		}
	}
	if err := json.Unmarshal([]byte(text), &content); err != nil {
		return "", err
	}
	return content.SummaryName, nil
}

func applyManualTakeoverSummary(updates map[string]interface{}, t *model.Takeover, summaryName *string) error {
	if summaryName == nil {
		return nil
	}
	name := cleanSummaryName(*summaryName)
	if name == "" {
		return errors.New("summaryName is required")
	}
	now := time.Now()
	hash := takeoverSummaryHash(*t)
	updates["summary_name"] = name
	updates["summary_source"] = summarySourceManual
	updates["summary_title_hash"] = hash
	updates["summary_error"] = nil
	updates["summary_updated_at"] = &now
	t.SummaryName = &name
	t.SummarySource = optionalStringPtr(summarySourceManual)
	t.SummaryTitleHash = &hash
	t.SummaryError = nil
	t.SummaryUpdatedAt = &now
	return nil
}

func takeoverSummaryHash(t model.Takeover) string {
	sum := sha1.Sum([]byte(strings.Join([]string{
		t.Title,
		stringValue(t.Description),
		scheduleText(t),
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

func cleanSummaryName(value string) string {
	text := strings.TrimSpace(value)
	text = summaryTimePattern.ReplaceAllString(text, " ")
	replacer := strings.NewReplacer("今晚", "", "今天", "", "明天", "", "晚上", "", "下午", "", "上午", "", "来人", "", "缺人", "", "差人", "", "一起玩", "", "有没有人", "")
	text = strings.TrimSpace(replacer.Replace(text))
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, " -_，,。:：")
	runes := []rune(text)
	if len(runes) > 12 {
		text = string(runes[:12])
	}
	return strings.TrimSpace(text)
}

func trimSummaryError(err error) string {
	if err == nil {
		return ""
	}
	runes := []rune(err.Error())
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return string(runes)
}
