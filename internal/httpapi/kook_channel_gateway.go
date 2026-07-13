package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type kookChannelPosition struct {
	ChannelID string `json:"channel_id"`
	ParentID  string `json:"parent_id"`
	Level     int    `json:"level"`
}

type kookChannelGateway interface {
	ListChannels(context.Context) ([]kookChannelDTO, error)
	UpdateChannel(context.Context, kookChannelPosition) error
}

type kookHTTPGateway struct{ h *Handler }

type kookChannelGatewayError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *kookChannelGatewayError) Error() string {
	return fmt.Sprintf("kook channel update failed: http=%d code=%d message=%s", e.StatusCode, e.Code, e.Message)
}

func (g kookHTTPGateway) ListChannels(context.Context) ([]kookChannelDTO, error) {
	channels, _, err := g.h.fetchKookChannels()
	return channels, err
}

func (g kookHTTPGateway) UpdateChannel(ctx context.Context, position kookChannelPosition) error {
	var result kookProxyResponse
	resp, err := resty.New().SetTimeout(20*time.Second).R().
		SetContext(ctx).
		SetHeader("Authorization", "Bot "+g.h.kookBotToken()).
		SetHeader("Content-Type", "application/json").
		SetBody(position).
		SetResult(&result).
		Post(kookAPIBaseURL + "/channel/update")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return &kookChannelGatewayError{StatusCode: resp.StatusCode(), Code: result.Code, Message: result.Message}
	}
	return nil
}
