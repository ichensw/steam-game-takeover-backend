package wechatadmin

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

const statsDateLayout = "2006-01-02"

type statsRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type statsTotals struct {
	MessageCount           int     `json:"messageCount"`
	ParticipantCount       int     `json:"participantCount"`
	GroupCount             int     `json:"groupCount"`
	MessagesPerParticipant float64 `json:"messagesPerParticipant"`
}

type dailyStat struct {
	Date             string `json:"date"`
	MessageCount     int    `json:"messageCount"`
	ParticipantCount int    `json:"participantCount"`
	GroupCount       int    `json:"groupCount"`
}

type participantStat struct {
	SenderWxid   string `json:"senderWxid"`
	SenderName   string `json:"senderName"`
	MessageCount int    `json:"messageCount"`
	ActiveDays   int    `json:"activeDays"`
	GroupCount   int    `json:"groupCount"`
}

func (s *Server) dailyStats(w http.ResponseWriter, r *http.Request) {
	loc := s.cfg.Location
	if loc == nil {
		loc = time.Local
	}
	start, end, err := parseStatsRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"), loc)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	where, args := statsWhere(start, end, roomID)

	var totals statsTotals
	totalQuery := "SELECT COUNT(*), COUNT(DISTINCT NULLIF(sender_wxid, '')), COUNT(DISTINCT room_id) FROM group_messages WHERE " + where
	if err := s.db.QueryRowContext(r.Context(), totalQuery, args...).Scan(
		&totals.MessageCount,
		&totals.ParticipantCount,
		&totals.GroupCount,
	); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query statistics totals failed")
		return
	}
	if totals.ParticipantCount > 0 {
		totals.MessagesPerParticipant = math.Round(float64(totals.MessageCount)/float64(totals.ParticipantCount)*100) / 100
	}

	offset := timezoneOffset(start, loc)
	daily, err := s.queryDailyStats(r, where, append([]interface{}{offset}, args...), start, end, loc)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query daily statistics failed")
		return
	}
	participants, err := s.queryParticipantStats(r, where, append([]interface{}{offset}, args...))
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query participant statistics failed")
		return
	}

	ok(w, map[string]interface{}{
		"range": statsRange{
			Start: start.In(loc).Format(statsDateLayout),
			End:   end.AddDate(0, 0, -1).In(loc).Format(statsDateLayout),
		},
		"roomId":       roomID,
		"totals":       totals,
		"daily":        daily,
		"participants": participants,
	})
}

func parseStatsRange(startRaw, endRaw string, loc *time.Location) (time.Time, time.Time, error) {
	startRaw = strings.TrimSpace(startRaw)
	endRaw = strings.TrimSpace(endRaw)
	if startRaw == "" && endRaw == "" {
		now := time.Now().In(loc)
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return start, start.AddDate(0, 0, 1), nil
	}
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, errors.New("start and end must be YYYY-MM-DD")
	}
	start, err := time.ParseInLocation(statsDateLayout, startRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("start and end must be YYYY-MM-DD")
	}
	lastDay, err := time.ParseInLocation(statsDateLayout, endRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("start and end must be YYYY-MM-DD")
	}
	if lastDay.Before(start) {
		return time.Time{}, time.Time{}, errors.New("end must not be before start")
	}

	days := 0
	for day := start; !day.After(lastDay); day = day.AddDate(0, 0, 1) {
		days++
		if days > 90 {
			return time.Time{}, time.Time{}, errors.New("date range must not exceed 90 days")
		}
	}
	return start, lastDay.AddDate(0, 0, 1), nil
}

func statsWhere(start, end time.Time, roomID string) (string, []interface{}) {
	where := "created_at >= ? AND created_at < ?"
	args := []interface{}{float64(start.Unix()), float64(end.Unix())}
	if roomID != "" {
		where += " AND room_id = ?"
		args = append(args, roomID)
	}
	return where, args
}

func timezoneOffset(at time.Time, loc *time.Location) string {
	_, seconds := at.In(loc).Zone()
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

func (s *Server) queryDailyStats(r *http.Request, where string, args []interface{}, start, end time.Time, loc *time.Location) ([]dailyStat, error) {
	query := "SELECT DATE_FORMAT(CONVERT_TZ(FROM_UNIXTIME(created_at), @@session.time_zone, ?), '%Y-%m-%d') AS message_date, COUNT(*) AS message_count, COUNT(DISTINCT NULLIF(sender_wxid, '')) AS participant_count, COUNT(DISTINCT room_id) AS group_count FROM group_messages WHERE " + where + " GROUP BY message_date ORDER BY message_date"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := make(map[string]dailyStat)
	for rows.Next() {
		var item dailyStat
		if err := rows.Scan(&item.Date, &item.MessageCount, &item.ParticipantCount, &item.GroupCount); err != nil {
			return nil, err
		}
		byDate[item.Date] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]dailyStat, 0, 90)
	for day := start.In(loc); day.Before(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(statsDateLayout)
		item, exists := byDate[date]
		if !exists {
			item = dailyStat{Date: date}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) queryParticipantStats(r *http.Request, where string, args []interface{}) ([]participantStat, error) {
	query := "SELECT sender_wxid, SUBSTRING_INDEX(GROUP_CONCAT(NULLIF(sender_name, '') ORDER BY created_at DESC SEPARATOR '|#|'), '|#|', 1) AS sender_name, COUNT(*) AS message_count, COUNT(DISTINCT DATE_FORMAT(CONVERT_TZ(FROM_UNIXTIME(created_at), @@session.time_zone, ?), '%Y-%m-%d')) AS active_days, COUNT(DISTINCT room_id) AS group_count FROM group_messages WHERE " + where + " AND sender_wxid <> '' GROUP BY sender_wxid ORDER BY message_count DESC, sender_wxid LIMIT 100"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]participantStat, 0)
	for rows.Next() {
		var item participantStat
		var name sql.NullString
		if err := rows.Scan(&item.SenderWxid, &name, &item.MessageCount, &item.ActiveDays, &item.GroupCount); err != nil {
			return nil, err
		}
		item.SenderName = strings.TrimSpace(name.String)
		if item.SenderName == "" {
			item.SenderName = item.SenderWxid
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
