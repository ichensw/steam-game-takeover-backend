package httpapi

import (
	"errors"
	"net/http"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errBlockedUserNotFound = errors.New("blocked user not found")

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
