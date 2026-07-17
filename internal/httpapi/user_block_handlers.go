package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errBlockedUserNotFound  = errors.New("blocked user not found")
	errUserBlockExists      = errors.New("user block already exists")
	errUserBlockInvalidUser = errors.New("user not found")
)

type adminUserBlockInput struct {
	OwnerUserID   uint64 `json:"ownerUserId"`
	BlockedUserID uint64 `json:"blockedUserId"`
}

type adminUserBlockDTO struct {
	ID            uint64         `json:"id"`
	OwnerUserID   uint64         `json:"ownerUserId"`
	BlockedUserID uint64         `json:"blockedUserId"`
	OwnerUser     adminWXUserDTO `json:"ownerUser"`
	BlockedUser   adminWXUserDTO `json:"blockedUser"`
	CreatedAt     string         `json:"createdAt"`
	UpdatedAt     string         `json:"updatedAt"`
}

func (h *Handler) BlockUser(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	blockedUserID, okID := pathUint64(c, "userId")
	if !okID || blockedUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	if blockedUserID == user.ID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "cannot block yourself")
		return
	}

	var blockedUser model.User
	if err := h.db.Where("id = ? AND is_deleted = ?", blockedUserID, false).First(&blockedUser).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}

	block := model.UserBlock{OwnerUserID: user.ID, BlockedUserID: blockedUserID}
	if err := h.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&block).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}

	ok(c, "success", gin.H{"blocked": true, "userId": blockedUserID})
}

func (h *Handler) BlockTakeoverMember(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	takeoverID, okTakeoverID := pathUint64(c, "takeoverId")
	blockedUserID, okUserID := pathUint64(c, "userId")
	if !okTakeoverID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	if !okUserID || blockedUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}

	var kicked bool
	var joinedCount int64
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := syncExpiredTakeovers(tx, time.Now()); err != nil {
			return err
		}

		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ?", takeoverID, false).
			First(&takeover).Error; err != nil {
			return err
		}
		if !canBlockTakeoverMember(user, takeover) {
			return errAdminUnauthorized
		}
		if takeover.TakeoverState == model.TakeoverStateClosed {
			return errTakeoverEnded
		}
		if blockedUserID == user.ID {
			return errCannotKickSelf
		}
		if blockedUserID == takeover.CreatorUserID {
			return errCannotKickCreator
		}

		var blockedUser model.User
		if err := tx.Where("id = ? AND is_deleted = ?", blockedUserID, false).First(&blockedUser).Error; err != nil {
			if isNotFound(err) {
				return errBlockedUserNotFound
			}
			return err
		}

		block := model.UserBlock{OwnerUserID: user.ID, BlockedUserID: blockedUserID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&block).Error; err != nil {
			return err
		}

		var member model.TakeoverMember
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, blockedUserID, model.MemberStateJoined).
			First(&member).Error
		if err == nil {
			if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", member.ID).
				Update("member_state", model.MemberStateExited).Error; err != nil {
				return err
			}
			if err := recordTakeoverMemberActivity(tx, takeoverID, blockedUserID, model.MemberActionKick, nil); err != nil {
				return err
			}
			kicked = true
		} else if !isNotFound(err) {
			return err
		}

		count, err := countValidJoinedMembers(tx, takeoverID)
		if err != nil {
			return err
		}
		joinedCount = count
		return nil
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		case errors.Is(err, errBlockedUserNotFound):
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
		case errors.Is(err, errAdminUnauthorized):
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
		case errors.Is(err, errTakeoverEnded):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be modified")
		case errors.Is(err, errCannotKickSelf):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "cannot block yourself")
		case errors.Is(err, errCannotKickCreator):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "creator cannot be blocked")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		}
		return
	}

	ok(c, "success", gin.H{"blocked": true, "kicked": kicked, "joinedCount": joinedCount, "userId": blockedUserID})
}

func (h *Handler) UnblockUser(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	blockedUserID, okID := pathUint64(c, "userId")
	if !okID || blockedUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}

	if err := h.db.Where("owner_user_id = ? AND blocked_user_id = ?", user.ID, blockedUserID).
		Delete(&model.UserBlock{}).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}

	ok(c, "success", gin.H{"blocked": false, "userId": blockedUserID})
}

func (h *Handler) ListBlockedUsers(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	var users []model.User
	if err := h.db.Table("ttw_user_block AS b").
		Select("u.*").
		Joins("JOIN ttw_user AS u ON u.id = b.blocked_user_id").
		Where("b.owner_user_id = ? AND u.is_deleted = ?", user.ID, false).
		Order("b.gmt_create DESC").
		Limit(200).
		Scan(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]userDTO, 0, len(users))
	for _, blockedUser := range users {
		list = append(list, toUserDTO(blockedUser))
	}
	ok(c, "success", gin.H{"list": list, "total": len(list)})
}

func canBlockTakeoverMember(user model.User, takeover model.Takeover) bool {
	return isTakeoverCreator(user, takeover)
}

func (h *Handler) AdminListUserBlocks(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.db.Model(&model.UserBlock{}).
		Joins("JOIN ttw_user AS owner ON owner.id = ttw_user_block.owner_user_id").
		Joins("JOIN ttw_user AS blocked ON blocked.id = ttw_user_block.blocked_user_id").
		Where("owner.is_deleted = ? AND blocked.is_deleted = ?", false, false)
	if ownerUserID := strings.TrimSpace(c.Query("ownerUserId")); ownerUserID != "" {
		query = query.Where("ttw_user_block.owner_user_id = ?", ownerUserID)
	}
	if blockedUserID := strings.TrimSpace(c.Query("blockedUserId")); blockedUserID != "" {
		query = query.Where("ttw_user_block.blocked_user_id = ?", blockedUserID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(`
			owner.nickname LIKE ? OR owner.steam_id LIKE ? OR owner.openid LIKE ?
			OR blocked.nickname LIKE ? OR blocked.steam_id LIKE ? OR blocked.openid LIKE ?
		`, like, like, like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var blocks []model.UserBlock
	if err := query.Order("ttw_user_block.gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&blocks).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	list, err := h.adminUserBlockDTOs(blocks)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", gin.H{"page": page, "pageSize": pageSize, "total": total, "list": list})
}

func (h *Handler) AdminCreateUserBlock(c *gin.Context) {
	var req adminUserBlockInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	block := model.UserBlock{OwnerUserID: req.OwnerUserID, BlockedUserID: req.BlockedUserID}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := validateUserBlockPair(block.OwnerUserID, block.BlockedUserID); err != nil {
			return err
		}
		if err := ensureUserBlockUsers(tx, block.OwnerUserID, block.BlockedUserID); err != nil {
			return err
		}
		exists, err := userBlockPairExists(tx, 0, block.OwnerUserID, block.BlockedUserID)
		if err != nil || exists {
			if exists {
				return errUserBlockExists
			}
			return err
		}
		return tx.Create(&block).Error
	})
	if err != nil {
		h.failUserBlockWrite(c, err)
		return
	}
	dto, err := h.adminUserBlockDTO(block)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", dto)
}

func (h *Handler) AdminUpdateUserBlock(c *gin.Context) {
	blockID, okID := pathUint64(c, "blockId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	var req adminUserBlockInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	var block model.UserBlock
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&block, blockID).Error; err != nil {
			return err
		}
		if err := validateUserBlockPair(req.OwnerUserID, req.BlockedUserID); err != nil {
			return err
		}
		if err := ensureUserBlockUsers(tx, req.OwnerUserID, req.BlockedUserID); err != nil {
			return err
		}
		exists, err := userBlockPairExists(tx, blockID, req.OwnerUserID, req.BlockedUserID)
		if err != nil || exists {
			if exists {
				return errUserBlockExists
			}
			return err
		}
		block.OwnerUserID = req.OwnerUserID
		block.BlockedUserID = req.BlockedUserID
		return tx.Save(&block).Error
	})
	if err != nil {
		h.failUserBlockWrite(c, err)
		return
	}
	dto, err := h.adminUserBlockDTO(block)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", dto)
}

func (h *Handler) AdminDeleteUserBlock(c *gin.Context) {
	blockID, okID := pathUint64(c, "blockId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	result := h.db.Delete(&model.UserBlock{}, blockID)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "user block not found")
		return
	}
	ok(c, "success", nil)
}

func validateUserBlockPair(ownerUserID, blockedUserID uint64) error {
	if ownerUserID == 0 || blockedUserID == 0 {
		return errUserBlockInvalidUser
	}
	if ownerUserID == blockedUserID {
		return errCannotKickSelf
	}
	return nil
}

func ensureUserBlockUsers(db *gorm.DB, ownerUserID, blockedUserID uint64) error {
	var count int64
	err := db.Model(&model.User{}).
		Where("id IN ? AND is_deleted = ?", []uint64{ownerUserID, blockedUserID}, false).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 2 {
		return errUserBlockInvalidUser
	}
	return nil
}

func userBlockPairExists(db *gorm.DB, excludeID, ownerUserID, blockedUserID uint64) (bool, error) {
	query := db.Model(&model.UserBlock{}).Where("owner_user_id = ? AND blocked_user_id = ?", ownerUserID, blockedUserID)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Limit(1).Count(&count).Error
	return count > 0, err
}

func (h *Handler) adminUserBlockDTO(block model.UserBlock) (adminUserBlockDTO, error) {
	list, err := h.adminUserBlockDTOs([]model.UserBlock{block})
	if err != nil || len(list) == 0 {
		return adminUserBlockDTO{}, err
	}
	return list[0], nil
}

func (h *Handler) adminUserBlockDTOs(blocks []model.UserBlock) ([]adminUserBlockDTO, error) {
	ids := make([]uint64, 0, len(blocks)*2)
	for _, block := range blocks {
		ids = append(ids, block.OwnerUserID, block.BlockedUserID)
	}
	users := map[uint64]model.User{}
	if len(ids) > 0 {
		var rows []model.User
		if err := h.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, user := range rows {
			users[user.ID] = user
		}
	}
	list := make([]adminUserBlockDTO, 0, len(blocks))
	for _, block := range blocks {
		list = append(list, adminUserBlockDTO{
			ID:            block.ID,
			OwnerUserID:   block.OwnerUserID,
			BlockedUserID: block.BlockedUserID,
			OwnerUser:     toAdminWXUserDTO(users[block.OwnerUserID]),
			BlockedUser:   toAdminWXUserDTO(users[block.BlockedUserID]),
			CreatedAt:     timeString(&block.GmtCreate),
			UpdatedAt:     timeString(&block.GmtModified),
		})
	}
	return list, nil
}

func (h *Handler) failUserBlockWrite(c *gin.Context, err error) {
	switch {
	case isNotFound(err):
		fail(c, http.StatusNotFound, CodeParamInvalid, "user block not found")
	case errors.Is(err, errUserBlockInvalidUser):
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
	case errors.Is(err, errCannotKickSelf):
		fail(c, http.StatusBadRequest, CodeParamInvalid, "cannot block yourself")
	case errors.Is(err, errUserBlockExists):
		fail(c, http.StatusConflict, CodeParamInvalid, "user block already exists")
	default:
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
	}
}
