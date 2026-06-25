package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type kookChannelDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	ParentID string `json:"parentId"`
	Level    int    `json:"level"`
}

type kookChannelTreeDTO struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Topic    string               `json:"topic"`
	ParentID string               `json:"parentId"`
	Level    int                  `json:"level"`
	Children []kookChannelTreeDTO `json:"children"`
}

type kookChannelListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Topic    string `json:"topic"`
			ParentID string `json:"parent_id"`
			Level    int    `json:"level"`
		} `json:"items"`
		Meta struct {
			Page      int `json:"page"`
			PageTotal int `json:"page_total"`
			PageSize  int `json:"page_size"`
			Total     int `json:"total"`
		} `json:"meta"`
	} `json:"data"`
}

func (h *Handler) ListKookChannels(c *gin.Context) {
	channels, meta, err := h.fetchKookChannels()
	if err != nil {
		fail(c, http.StatusBadGateway, CodeSystemError, "kook channel query failed")
		return
	}
	ok(c, "success", gin.H{"list": channels, "meta": meta})
}

func (h *Handler) ListKookChannelTree(c *gin.Context) {
	channels, meta, err := h.fetchKookChannels()
	if err != nil {
		fail(c, http.StatusBadGateway, CodeSystemError, "kook channel query failed")
		return
	}
	ok(c, "success", gin.H{"list": toKookChannelTree(channels), "meta": meta})
}

func (h *Handler) fetchKookChannels() ([]kookChannelDTO, gin.H, error) {
	token := h.kookBotToken()
	guildID := h.kookGuildID()
	if token == "" || guildID == "" {
		return nil, nil, fmt.Errorf("kook not configured")
	}

	var result kookChannelListResponse
	resp, err := resty.New().R().
		SetHeader("Authorization", "Bot "+token).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"guild_id":  guildID,
			"type":      "1",
			"page_size": "50",
		}).
		SetResult(&result).
		Get("https://www.kookapp.cn/api/v3/channel/list")
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return nil, nil, fmt.Errorf("kook channel list failed: http=%d code=%d", resp.StatusCode(), result.Code)
	}

	list, meta := toKookChannelList(result)
	return list, meta, nil
}

func toKookChannelList(result kookChannelListResponse) ([]kookChannelDTO, gin.H) {
	list := make([]kookChannelDTO, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		list = append(list, kookChannelDTO{
			ID:       item.ID,
			Name:     item.Name,
			Topic:    item.Topic,
			ParentID: item.ParentID,
			Level:    item.Level,
		})
	}
	return list, gin.H{
		"page":      result.Data.Meta.Page,
		"pageTotal": result.Data.Meta.PageTotal,
		"pageSize":  result.Data.Meta.PageSize,
		"total":     result.Data.Meta.Total,
	}
}

func toKookChannelTree(channels []kookChannelDTO) []kookChannelTreeDTO {
	nodes := make(map[string]*kookChannelTreeDTO, len(channels))
	order := make([]string, 0, len(channels))
	for _, channel := range channels {
		node := kookChannelTreeDTO{
			ID:       channel.ID,
			Name:     channel.Name,
			Topic:    channel.Topic,
			ParentID: channel.ParentID,
			Level:    channel.Level,
			Children: []kookChannelTreeDTO{},
		}
		nodes[channel.ID] = &node
		order = append(order, channel.ID)
	}

	tree := make([]kookChannelTreeDTO, 0, len(channels))
	for _, id := range order {
		node := nodes[id]
		if parent := nodes[node.ParentID]; parent != nil {
			parent.Children = append(parent.Children, *node)
		}
	}
	for _, id := range order {
		node := nodes[id]
		if nodes[node.ParentID] != nil {
			continue
		}
		tree = append(tree, *node)
	}
	return tree
}
