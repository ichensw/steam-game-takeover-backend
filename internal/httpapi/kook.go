package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

const kookChannelCacheTTL = 24 * time.Hour

type kookChannelDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	ParentID string `json:"parentId"`
	Level    int    `json:"level"`
	KookSort int    `json:"kookSort"`
}

type kookChannelTreeDTO struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Type     int                  `json:"type"`
	ParentID string               `json:"parentId"`
	Level    int                  `json:"level"`
	KookSort int                  `json:"kookSort"`
	Children []kookChannelTreeDTO `json:"children"`
}

type kookChannelListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     int    `json:"type"`
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
	ok(c, "success", gin.H{"list": filterKookVoiceChannels(channels), "meta": meta})
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
	h.kookMu.Lock()
	if time.Now().Before(h.kookUntil) && h.kookChannels != nil {
		channels := cloneKookChannels(h.kookChannels)
		meta := cloneGinH(h.kookMeta)
		h.kookMu.Unlock()
		return channels, meta, nil
	}
	h.kookMu.Unlock()

	token := h.kookBotToken()
	guildID := h.kookGuildID()
	if token == "" || guildID == "" {
		return nil, nil, fmt.Errorf("kook not configured")
	}

	client := resty.New()
	var all []kookChannelDTO
	var meta gin.H
	for page := 1; ; page++ {
		result, err := fetchKookChannelPage(client, token, guildID, page)
		if err != nil {
			return nil, nil, err
		}
		list, pageMeta := toKookChannelList(result)
		all = append(all, list...)
		meta = pageMeta
		if result.Data.Meta.Page >= result.Data.Meta.PageTotal {
			break
		}
	}

	h.kookMu.Lock()
	h.kookChannels = cloneKookChannels(all)
	h.kookMeta = cloneGinH(meta)
	h.kookUntil = time.Now().Add(kookChannelCacheTTL)
	h.kookMu.Unlock()
	return all, meta, nil
}

func cloneKookChannels(channels []kookChannelDTO) []kookChannelDTO {
	if channels == nil {
		return nil
	}
	return append([]kookChannelDTO(nil), channels...)
}

func cloneGinH(value gin.H) gin.H {
	if value == nil {
		return nil
	}
	cloned := make(gin.H, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func fetchKookChannelPage(client *resty.Client, token, guildID string, page int) (kookChannelListResponse, error) {
	var result kookChannelListResponse
	resp, err := client.R().
		SetHeader("Authorization", "Bot "+token).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"guild_id":  guildID,
			"page":      fmt.Sprintf("%d", page),
			"page_size": "50",
		}).
		SetResult(&result).
		Get("https://www.kookapp.cn/api/v3/channel/list")
	if err != nil {
		return result, err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return result, fmt.Errorf("kook channel list failed: http=%d code=%d", resp.StatusCode(), result.Code)
	}
	return result, nil
}

func toKookChannelList(result kookChannelListResponse) ([]kookChannelDTO, gin.H) {
	list := make([]kookChannelDTO, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		list = append(list, kookChannelDTO{
			ID:       item.ID,
			Name:     item.Name,
			Type:     item.Type,
			ParentID: item.ParentID,
			Level:    1,
			KookSort: item.Level,
		})
	}
	return list, gin.H{
		"page":      result.Data.Meta.Page,
		"pageTotal": result.Data.Meta.PageTotal,
		"pageSize":  result.Data.Meta.PageSize,
		"total":     result.Data.Meta.Total,
	}
}

func filterKookVoiceChannels(channels []kookChannelDTO) []kookChannelDTO {
	list := make([]kookChannelDTO, 0, len(channels))
	for _, channel := range channels {
		if channel.Type == 0 || channel.Type == 2 {
			list = append(list, channel)
		}
	}
	return list
}

func toKookChannelTree(channels []kookChannelDTO) []kookChannelTreeDTO {
	nodes := make(map[string]kookChannelTreeDTO, len(channels))
	order := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel.Type != 0 && channel.Type != 2 {
			continue
		}
		node := kookChannelTreeDTO{
			ID:       channel.ID,
			Name:     channel.Name,
			Type:     channel.Type,
			ParentID: channel.ParentID,
			KookSort: channel.KookSort,
			Children: []kookChannelTreeDTO{},
		}
		nodes[channel.ID] = node
		order = append(order, channel.ID)
	}

	childrenByParent := make(map[string][]string, len(nodes))
	for _, id := range order {
		node := nodes[id]
		if _, ok := nodes[node.ParentID]; ok {
			childrenByParent[node.ParentID] = append(childrenByParent[node.ParentID], id)
		}
	}

	tree := make([]kookChannelTreeDTO, 0, len(order))
	for _, id := range order {
		node := nodes[id]
		if _, ok := nodes[node.ParentID]; ok {
			continue
		}
		if kept, ok := buildKookChannelTreeNode(id, 1, nodes, childrenByParent); ok {
			tree = append(tree, kept)
		}
	}
	return tree
}

func buildKookChannelTreeNode(id string, level int, nodes map[string]kookChannelTreeDTO, childrenByParent map[string][]string) (kookChannelTreeDTO, bool) {
	node := nodes[id]
	node.Level = level
	children := make([]kookChannelTreeDTO, 0, len(node.Children))
	for _, childID := range childrenByParent[id] {
		if kept, ok := buildKookChannelTreeNode(childID, level+1, nodes, childrenByParent); ok {
			children = append(children, kept)
		}
	}
	node.Children = children
	return node, node.Type == 2 || len(node.Children) > 0
}
