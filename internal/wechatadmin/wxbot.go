package wechatadmin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type wxbotHeartbeatRequest struct {
	BotID         string          `json:"botId"`
	Name          string          `json:"name"`
	Wxid          string          `json:"wxid"`
	Status        string          `json:"status"`
	Version       string          `json:"version"`
	Host          string          `json:"host"`
	PID           int             `json:"pid"`
	StartedAt     string          `json:"startedAt"`
	CurrentConfig json.RawMessage `json:"currentConfig"`
}

type wxbotRecord struct {
	BotID           string          `json:"botId"`
	Name            string          `json:"name"`
	Wxid            string          `json:"wxid"`
	Status          string          `json:"status"`
	Version         string          `json:"version"`
	Host            string          `json:"host"`
	PID             int             `json:"pid"`
	Online          bool            `json:"online"`
	StartedAt       string          `json:"startedAt,omitempty"`
	LastSeenAt      string          `json:"lastSeenAt,omitempty"`
	Config          json.RawMessage `json:"config"`
	CurrentConfig   json.RawMessage `json:"currentConfig"`
	ConfigUpdatedAt string          `json:"configUpdatedAt,omitempty"`
	ConfigAppliedAt string          `json:"configAppliedAt,omitempty"`
	UpdatedAt       string          `json:"updatedAt,omitempty"`
}

type wxbotConfigUpdateRequest struct {
	Config json.RawMessage `json:"config"`
}

type wxbotConfigAppliedRequest struct {
	BotID string `json:"botId"`
}

func (s *Server) wxbotHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req wxbotHeartbeatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	req.BotID = cleanBotID(req.BotID)
	if req.BotID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "botId is required")
		return
	}
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}
	startedAt := nullTime(req.StartedAt)
	currentConfig := json.RawMessage([]byte("{}"))
	hasCurrentConfig := len(bytes.TrimSpace(req.CurrentConfig)) > 0
	if hasCurrentConfig {
		var err error
		currentConfig, err = normalizeWxbotConfig(req.CurrentConfig)
		if err != nil {
			fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
			return
		}
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO wxbot_agents
			(bot_id, name, wxid, status, version, host, pid, started_at, last_seen_at, config_json, current_config_json, config_updated_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), JSON_OBJECT(), ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			wxid = VALUES(wxid),
			status = VALUES(status),
			version = VALUES(version),
			host = VALUES(host),
			pid = VALUES(pid),
			started_at = COALESCE(VALUES(started_at), started_at),
			last_seen_at = NOW(),
			current_config_json = IF(? = 1, VALUES(current_config_json), current_config_json),
			updated_at = NOW()
	`, req.BotID, req.Name, req.Wxid, req.Status, req.Version, req.Host, req.PID, startedAt, string(currentConfig), boolInt(hasCurrentConfig))
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save wxbot heartbeat failed")
		return
	}
	record, err := s.wxbotByID(r.Context(), req.BotID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot failed")
		return
	}
	ok(w, record)
}

func (s *Server) wxbotConfigForBot(w http.ResponseWriter, r *http.Request) {
	botID := cleanBotID(r.URL.Query().Get("botId"))
	if botID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "botId is required")
		return
	}
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}
	record, err := s.wxbotByID(r.Context(), botID)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = s.db.ExecContext(r.Context(), `
			INSERT INTO wxbot_agents (bot_id, status, last_seen_at, config_json, config_updated_at, updated_at)
			VALUES (?, 'unknown', NOW(), JSON_OBJECT(), NOW(), NOW())
		`, botID)
		record, err = s.wxbotByID(r.Context(), botID)
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot config failed")
		return
	}
	ok(w, map[string]interface{}{
		"botId":           record.BotID,
		"config":          record.Config,
		"currentConfig":   record.CurrentConfig,
		"configUpdatedAt": record.ConfigUpdatedAt,
	})
}

func (s *Server) wxbotConfigApplied(w http.ResponseWriter, r *http.Request) {
	var req wxbotConfigAppliedRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	req.BotID = cleanBotID(req.BotID)
	if req.BotID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "botId is required")
		return
	}
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO wxbot_agents (bot_id, status, last_seen_at, config_json, config_updated_at, config_applied_at, updated_at)
		VALUES (?, 'unknown', NOW(), JSON_OBJECT(), NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			config_applied_at = NOW(),
			updated_at = NOW()
	`, req.BotID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save wxbot config applied failed")
		return
	}
	record, err := s.wxbotByID(r.Context(), req.BotID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot failed")
		return
	}
	ok(w, record)
}

func (s *Server) wxbotList(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT bot_id, name, wxid, status, version, host, pid, started_at, last_seen_at,
		       COALESCE(config_json, JSON_OBJECT()), COALESCE(current_config_json, JSON_OBJECT()), config_updated_at, config_applied_at, updated_at
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, bot_id
	`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot list failed")
		return
	}
	defer rows.Close()
	items := make([]wxbotRecord, 0)
	for rows.Next() {
		item, err := scanWxbot(rows)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan wxbot list failed")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot list failed")
		return
	}
	ok(w, map[string]interface{}{"list": items})
}

func (s *Server) wxbotConfigDetail(w http.ResponseWriter, r *http.Request) {
	botID := cleanBotID(r.PathValue("botID"))
	if botID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid bot id")
		return
	}
	record, err := s.wxbotByID(r.Context(), botID)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "wxbot not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot config failed")
		return
	}
	ok(w, map[string]interface{}{
		"botId":           record.BotID,
		"config":          record.Config,
		"currentConfig":   record.CurrentConfig,
		"configUpdatedAt": record.ConfigUpdatedAt,
	})
}

func (s *Server) wxbotUpdateConfig(w http.ResponseWriter, r *http.Request) {
	botID := cleanBotID(r.PathValue("botID"))
	if botID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid bot id")
		return
	}
	var req wxbotConfigUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	configJSON, err := normalizeWxbotConfig(req.Config)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO wxbot_agents (bot_id, status, last_seen_at, config_json, config_updated_at, updated_at)
		VALUES (?, 'unknown', NOW(), ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			config_json = VALUES(config_json),
			config_updated_at = NOW(),
			updated_at = NOW()
	`, botID, string(configJSON))
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save wxbot config failed")
		return
	}
	record, err := s.wxbotByID(r.Context(), botID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot config failed")
		return
	}
	ok(w, map[string]interface{}{
		"botId":           record.BotID,
		"config":          record.Config,
		"currentConfig":   record.CurrentConfig,
		"configUpdatedAt": record.ConfigUpdatedAt,
	})
}

func (s *Server) ensureWxbotSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, wxbotSchemaSQL); err != nil {
		return err
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'wxbot_agents'
		  AND column_name = 'current_config_json'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = s.db.ExecContext(ctx, `ALTER TABLE wxbot_agents ADD COLUMN current_config_json JSON NULL AFTER config_json`)
	}
	return err
}

const wxbotSchemaSQL = `
CREATE TABLE IF NOT EXISTS wxbot_agents (
  bot_id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL DEFAULT '',
  wxid VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  version VARCHAR(64) NOT NULL DEFAULT '',
  host VARCHAR(255) NOT NULL DEFAULT '',
  pid INT NOT NULL DEFAULT 0,
  started_at DATETIME NULL,
  last_seen_at DATETIME NULL,
  config_json JSON NOT NULL,
  current_config_json JSON NULL,
  config_updated_at DATETIME NULL,
  config_applied_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_last_seen (last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='微信机器人控制中心实例';
`

func (s *Server) wxbotByID(ctx context.Context, botID string) (wxbotRecord, error) {
	if err := s.ensureWxbotSchema(ctx); err != nil {
		return wxbotRecord{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT bot_id, name, wxid, status, version, host, pid, started_at, last_seen_at,
		       COALESCE(config_json, JSON_OBJECT()), COALESCE(current_config_json, JSON_OBJECT()), config_updated_at, config_applied_at, updated_at
		FROM wxbot_agents
		WHERE bot_id = ?
	`, botID)
	return scanWxbot(row)
}

type wxbotScanner interface {
	Scan(dest ...interface{}) error
}

func scanWxbot(scanner wxbotScanner) (wxbotRecord, error) {
	var item wxbotRecord
	var startedAt, lastSeenAt, configUpdatedAt, configAppliedAt, updatedAt sql.NullTime
	var configRaw, currentConfigRaw []byte
	if err := scanner.Scan(
		&item.BotID,
		&item.Name,
		&item.Wxid,
		&item.Status,
		&item.Version,
		&item.Host,
		&item.PID,
		&startedAt,
		&lastSeenAt,
		&configRaw,
		&currentConfigRaw,
		&configUpdatedAt,
		&configAppliedAt,
		&updatedAt,
	); err != nil {
		return wxbotRecord{}, err
	}
	item.StartedAt = formatNullTime(startedAt)
	item.LastSeenAt = formatNullTime(lastSeenAt)
	item.ConfigUpdatedAt = formatNullTime(configUpdatedAt)
	item.ConfigAppliedAt = formatNullTime(configAppliedAt)
	item.UpdatedAt = formatNullTime(updatedAt)
	item.Online = lastSeenAt.Valid && time.Since(lastSeenAt.Time) <= 90*time.Second
	if len(configRaw) == 0 {
		configRaw = []byte("{}")
	}
	if len(currentConfigRaw) == 0 {
		currentConfigRaw = []byte("{}")
	}
	item.Config = json.RawMessage(configRaw)
	item.CurrentConfig = json.RawMessage(currentConfigRaw)
	return item, nil
}

func cleanBotID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return ""
	}
	return value
}

func normalizeWxbotConfig(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("{}"), nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, errors.New("config must be a json object")
	}
	schema := wxbotConfigSchema()
	result := make(map[string]interface{}, len(cfg))
	for section, value := range cfg {
		fields, ok := schema[section]
		if !ok {
			return nil, errors.New("config contains unsupported section")
		}
		values, ok := value.(map[string]interface{})
		if !ok {
			return nil, errors.New("config section must be a json object")
		}
		normalized := make(map[string]interface{}, len(values))
		for field, fieldValue := range values {
			spec, ok := fields[field]
			if !ok {
				return nil, errors.New("config contains unsupported field")
			}
			next, err := normalizeWxbotConfigValue(fieldValue, spec)
			if err != nil {
				return nil, err
			}
			normalized[field] = next
		}
		for field, spec := range fields {
			if _, ok := normalized[field]; !ok && spec.defaultValue != nil {
				normalized[field] = spec.defaultValue
			}
		}
		result[section] = normalized
	}
	if err := validateWxbotConfig(result); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(result)
	return encoded, nil
}

type wxbotConfigFieldSpec struct {
	kind         string
	defaultValue interface{}
}

func wxbotConfigSchema() map[string]map[string]wxbotConfigFieldSpec {
	stringSpec := wxbotConfigFieldSpec{kind: "string"}
	stringListSpec := wxbotConfigFieldSpec{kind: "stringList"}
	summaryJobsSpec := wxbotConfigFieldSpec{kind: "summaryJobs", defaultValue: []map[string]string{}}
	stringDefaultSpec := func(defaultValue string) wxbotConfigFieldSpec {
		return wxbotConfigFieldSpec{kind: "stringDefault", defaultValue: defaultValue}
	}
	boolSpec := func(defaultValue bool) wxbotConfigFieldSpec {
		return wxbotConfigFieldSpec{kind: "bool", defaultValue: defaultValue}
	}
	intSpec := func(defaultValue int) wxbotConfigFieldSpec {
		return wxbotConfigFieldSpec{kind: "int", defaultValue: defaultValue}
	}
	positiveIntSpec := func(defaultValue int) wxbotConfigFieldSpec {
		return wxbotConfigFieldSpec{kind: "positiveInt", defaultValue: defaultValue}
	}
	return map[string]map[string]wxbotConfigFieldSpec{
		"bot": {
			"name":            stringSpec,
			"admin_wxids":     stringListSpec,
			"group_whitelist": stringListSpec,
			"command_prefix":  stringSpec,
			"at_me_required":  boolSpec(true),
		},
		"hook": {
			"dll_path":                 stringSpec,
			"inject_exe_path":          stringSpec,
			"http_server_port":         intSpec(19088),
			"receive_mode":             stringSpec,
			"tcp_ip":                   stringSpec,
			"tcp_port":                 intSpec(61108),
			"callback_url":             stringSpec,
			"usedefault":               boolSpec(false),
			"start_server_while_login": boolSpec(true),
			"force_reinject_on_start":  boolSpec(false),
		},
		"monitor": {
			"message":             boolSpec(true),
			"message_types":       stringListSpec,
			"alert_member_change": boolSpec(true),
			"group_cache_ttl":     intSpec(600),
		},
		"webhook": {
			"enabled":      boolSpec(true),
			"host":         stringSpec,
			"port":         intSpec(5000),
			"token":        stringSpec,
			"rate_limit":   intSpec(60),
			"cors_origins": stringListSpec,
		},
		"database": {
			"host":                 stringSpec,
			"port":                 intSpec(3306),
			"user":                 stringSpec,
			"password":             stringSpec,
			"name":                 stringSpec,
			"charset":              stringSpec,
			"connect_timeout":      intSpec(10),
			"read_timeout":         intSpec(10),
			"write_timeout":        intSpec(10),
			"batch_size":           intSpec(100),
			"batch_flush_interval": intSpec(10),
			"message_queue_size":   intSpec(5000),
		},
		"logging": {
			"level":        stringSpec,
			"file":         stringSpec,
			"max_size_mb":  intSpec(10),
			"backup_count": intSpec(5),
		},
		"welcome": {
			"enabled":     boolSpec(true),
			"default_msg": stringSpec,
		},
		"party_site": {
			"enabled":        boolSpec(true),
			"base_url":       stringSpec,
			"admin_username": stringSpec,
			"admin_password": stringSpec,
			"token":          stringSpec,
			"timeout":        intSpec(10),
		},
		"summary_reminder": {
			"enabled": boolSpec(true),
			"jobs":    summaryJobsSpec,
		},
		"ai": {
			"enabled":                 boolSpec(false),
			"group_whitelist":         stringListSpec,
			"auto_memory_enabled":     boolSpec(true),
			"reply_enabled":           boolSpec(true),
			"api_base_url":            stringSpec,
			"api_key":                 stringSpec,
			"reply_model":             stringDefaultSpec("5.4 Mini"),
			"summary_model":           stringDefaultSpec("5.4 Mini"),
			"merge_model":             stringDefaultSpec("5.5"),
			"manual_deep_model":       stringDefaultSpec("5.6 Luna"),
			"scan_interval_seconds":   positiveIntSpec(300),
			"segment_min_messages":    positiveIntSpec(30),
			"segment_quiet_seconds":   positiveIntSpec(600),
			"segment_stale_seconds":   positiveIntSpec(21600),
			"profile_min_segments":    positiveIntSpec(3),
			"max_segment_messages":    positiveIntSpec(800),
			"reply_context_messages":  positiveIntSpec(100),
			"worker_queue_size":       positiveIntSpec(200),
			"reply_timeout_seconds":   positiveIntSpec(20),
			"summary_timeout_seconds": positiveIntSpec(180),
			"merge_timeout_seconds":   positiveIntSpec(300),
		},
		"wxbot_control": {
			"enabled":              boolSpec(true),
			"base_url":             stringSpec,
			"token":                stringSpec,
			"bot_id":               stringSpec,
			"name":                 stringSpec,
			"heartbeat_interval":   intSpec(30),
			"config_pull_interval": intSpec(30),
			"request_timeout":      intSpec(10),
		},
		"oss": {
			"enabled":           boolSpec(false),
			"endpoint":          stringSpec,
			"bucket":            stringSpec,
			"access_key_id":     stringSpec,
			"access_key_secret": stringSpec,
			"public_base_url":   stringSpec,
			"object_prefix":     stringSpec,
			"keep_local":        boolSpec(false),
		},
	}
}

func normalizeWxbotConfigValue(value interface{}, spec wxbotConfigFieldSpec) (interface{}, error) {
	switch spec.kind {
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("config string field must be a string")
		}
		return text, nil
	case "stringDefault":
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("config string field must be a string")
		}
		if strings.TrimSpace(text) == "" {
			return spec.defaultValue, nil
		}
		return text, nil
	case "bool":
		return normalizeWxbotBool(value, spec.defaultValue.(bool))
	case "int":
		return normalizeWxbotInt(value, spec.defaultValue.(int))
	case "positiveInt":
		next, err := normalizeWxbotInt(value, spec.defaultValue.(int))
		if err != nil {
			return nil, err
		}
		if next <= 0 {
			return spec.defaultValue, nil
		}
		return next, nil
	case "stringList":
		return normalizeWxbotStringList(value)
	case "summaryJobs":
		return normalizeWxbotSummaryJobs(value)
	default:
		return nil, errors.New("unsupported config field type")
	}
}

func normalizeWxbotBool(value interface{}, defaultValue bool) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		text := strings.TrimSpace(strings.ToLower(v))
		if text == "" {
			return defaultValue, nil
		}
		if text == "true" || text == "1" || text == "yes" || text == "on" {
			return true, nil
		}
		if text == "false" || text == "0" || text == "no" || text == "off" {
			return false, nil
		}
	}
	return false, errors.New("config bool field must be a boolean")
}

func normalizeWxbotInt(value interface{}, defaultValue int) (int, error) {
	switch v := value.(type) {
	case float64:
		next := int(v)
		if float64(next) == v {
			return next, nil
		}
	case int:
		return v, nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return defaultValue, nil
		}
		next, err := strconv.Atoi(text)
		if err == nil {
			return next, nil
		}
	}
	return 0, errors.New("config int field must be an integer")
}

func normalizeWxbotStringList(value interface{}) ([]string, error) {
	items, ok := value.([]interface{})
	if !ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return []string{}, nil
		}
		return nil, errors.New("config list field must be an array")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result, nil
}

func normalizeWxbotSummaryJobs(value interface{}) ([]map[string]string, error) {
	items, ok := value.([]interface{})
	if !ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return []map[string]string{}, nil
		}
		return nil, errors.New("summary jobs must be an array")
	}
	result := make([]map[string]string, 0, len(items))
	for _, item := range items {
		job, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.New("summary job must be an object")
		}
		result = append(result, map[string]string{
			"room_id": strings.TrimSpace(toString(job["room_id"])),
			"time":    strings.TrimSpace(toString(job["time"])),
		})
	}
	return result, nil
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func validateWxbotConfig(cfg map[string]interface{}) error {
	if section := wxbotSection(cfg, "bot"); section != nil {
		if err := requireWxbotText(section, "name", "机器人名称"); err != nil {
			return err
		}
	}
	if section := wxbotSection(cfg, "hook"); section != nil {
		for _, item := range []struct{ key, label string }{
			{"dll_path", "DLL 路径"},
			{"inject_exe_path", "注入程序路径"},
			{"receive_mode", "接收模式"},
			{"callback_url", "回调地址"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		if err := requireWxbotInt(section, "http_server_port", "Hook HTTP 端口"); err != nil {
			return err
		}
		if strings.TrimSpace(toString(section["receive_mode"])) == "tcp" {
			if err := requireWxbotText(section, "tcp_ip", "TCP IP"); err != nil {
				return err
			}
			if err := requireWxbotInt(section, "tcp_port", "TCP 端口"); err != nil {
				return err
			}
		}
	}
	if section := wxbotSection(cfg, "database"); section != nil {
		for _, item := range []struct{ key, label string }{
			{"host", "MySQL 地址"},
			{"user", "数据库用户名"},
			{"password", "数据库密码"},
			{"name", "数据库名"},
			{"charset", "数据库字符集"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		for _, item := range []struct{ key, label string }{
			{"port", "MySQL 端口"},
			{"connect_timeout", "数据库连接超时"},
			{"read_timeout", "数据库读取超时"},
			{"write_timeout", "数据库写入超时"},
			{"batch_size", "批量入库条数"},
			{"batch_flush_interval", "批量刷新间隔"},
			{"message_queue_size", "消息队列容量"},
		} {
			if err := requireWxbotInt(section, item.key, item.label); err != nil {
				return err
			}
		}
	}
	if section := wxbotSection(cfg, "logging"); section != nil {
		if err := requireWxbotText(section, "level", "日志级别"); err != nil {
			return err
		}
		if err := requireWxbotText(section, "file", "日志文件"); err != nil {
			return err
		}
		if err := requireWxbotInt(section, "max_size_mb", "日志单文件大小"); err != nil {
			return err
		}
		if err := requireWxbotInt(section, "backup_count", "日志保留文件数"); err != nil {
			return err
		}
	}
	if section := wxbotSection(cfg, "webhook"); wxbotEnabled(section) {
		for _, item := range []struct{ key, label string }{
			{"host", "Webhook 监听地址"},
			{"token", "Webhook Token"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		if err := requireWxbotInt(section, "port", "Webhook 监听端口"); err != nil {
			return err
		}
	}
	if section := wxbotSection(cfg, "welcome"); wxbotEnabled(section) {
		if err := requireWxbotText(section, "default_msg", "默认欢迎词"); err != nil {
			return err
		}
	}
	if section := wxbotSection(cfg, "party_site"); wxbotEnabled(section) {
		for _, item := range []struct{ key, label string }{
			{"base_url", "接龙网站地址"},
			{"admin_username", "接龙网站管理员账号"},
			{"admin_password", "接龙网站管理员密码"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		if err := requireWxbotInt(section, "timeout", "接龙网站请求超时"); err != nil {
			return err
		}
	}
	if section := wxbotSection(cfg, "ai"); wxbotEnabled(section) {
		for _, item := range []struct{ key, label string }{
			{"api_base_url", "AI API Base URL"},
			{"reply_model", "回复模型"},
			{"summary_model", "总结模型"},
			{"merge_model", "画像与文化模型"},
			{"manual_deep_model", "手动深度模型"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		for _, item := range []struct{ key, label string }{
			{"scan_interval_seconds", "AI 扫描间隔"},
			{"segment_min_messages", "AI 分段最少消息数"},
			{"segment_quiet_seconds", "AI 安静阈值"},
			{"segment_stale_seconds", "AI 最长未总结时间"},
			{"profile_min_segments", "AI 画像最少片段数"},
			{"max_segment_messages", "AI 分段消息上限"},
			{"reply_context_messages", "AI 回复上下文消息数"},
			{"worker_queue_size", "AI 任务队列容量"},
			{"reply_timeout_seconds", "AI 回复超时"},
			{"summary_timeout_seconds", "AI 总结超时"},
			{"merge_timeout_seconds", "AI 画像与人格超时"},
		} {
			if err := requireWxbotInt(section, item.key, item.label); err != nil {
				return err
			}
		}
	}
	if section := wxbotSection(cfg, "wxbot_control"); wxbotEnabled(section) {
		for _, item := range []struct{ key, label string }{
			{"base_url", "控制中心 Base URL"},
			{"token", "控制中心 Token"},
			{"bot_id", "机器人 ID"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
		for _, item := range []struct{ key, label string }{
			{"heartbeat_interval", "心跳间隔"},
			{"config_pull_interval", "配置拉取间隔"},
			{"request_timeout", "控制中心请求超时"},
		} {
			if err := requireWxbotInt(section, item.key, item.label); err != nil {
				return err
			}
		}
	}
	if section := wxbotSection(cfg, "oss"); wxbotEnabled(section) {
		for _, item := range []struct{ key, label string }{
			{"endpoint", "OSS Endpoint"},
			{"bucket", "OSS Bucket"},
			{"access_key_id", "OSS AccessKey ID"},
			{"access_key_secret", "OSS AccessKey Secret"},
			{"public_base_url", "OSS 公开访问地址"},
			{"object_prefix", "OSS 对象前缀"},
		} {
			if err := requireWxbotText(section, item.key, item.label); err != nil {
				return err
			}
		}
	}
	return nil
}

func wxbotSection(cfg map[string]interface{}, name string) map[string]interface{} {
	section, _ := cfg[name].(map[string]interface{})
	return section
}

func wxbotEnabled(section map[string]interface{}) bool {
	if section == nil {
		return false
	}
	enabled, _ := section["enabled"].(bool)
	return enabled
}

func requireWxbotText(section map[string]interface{}, key string, label string) error {
	if strings.TrimSpace(toString(section[key])) == "" {
		return fmt.Errorf("%s不能为空", label)
	}
	return nil
}

func requireWxbotInt(section map[string]interface{}, key string, label string) error {
	value, ok := section[key].(int)
	if !ok || value <= 0 {
		return fmt.Errorf("%s必须大于 0", label)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullTime(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return nil
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format(time.RFC3339)
}
