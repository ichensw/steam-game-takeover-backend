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

var (
	errAdminUnauthorized       = errors.New("admin unauthorized")
	errCannotKickCreator       = errors.New("creator cannot be kicked")
	errCannotKickSelf          = errors.New("cannot kick yourself")
	errTakeoverJoinUnavailable = errors.New("takeover join unavailable")
)

func (h *Handler) KickTakeoverMember(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	memberUserID, okUserID := pathUint64(c, "userId")
	if !okUserID || memberUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}

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
		if !canManageTakeover(user, takeover) {
			return errAdminUnauthorized
		}
		if takeover.TakeoverState == model.TakeoverStateClosed {
			return errTakeoverEnded
		}
		if memberUserID == user.ID {
			return errCannotKickSelf
		}
		if memberUserID == takeover.CreatorUserID {
			return errCannotKickCreator
		}

		var member model.TakeoverMember
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, memberUserID, model.MemberStateJoined).
			First(&member).Error; err != nil {
			if isNotFound(err) {
				return errNotJoined
			}
			return err
		}

		if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", member.ID).
			Update("member_state", model.MemberStateExited).Error; err != nil {
			return err
		}
		if err := recordTakeoverMemberActivity(tx, takeoverID, memberUserID, model.MemberActionKick, nil); err != nil {
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
		case errors.Is(err, errAdminUnauthorized):
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
		case errors.Is(err, errTakeoverEnded):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be modified")
		case errors.Is(err, errCannotKickSelf):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "cannot kick yourself")
		case errors.Is(err, errCannotKickCreator):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "creator cannot be kicked")
		case errors.Is(err, errNotJoined):
			fail(c, http.StatusConflict, CodeParamInvalid, "not joined")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		}
		return
	}

	ok(c, "success", gin.H{"kicked": true, "joinedCount": joinedCount})
}

func (h *Handler) ensureCanManageTakeover(c *gin.Context, user model.User, takeoverID uint64) (model.Takeover, bool) {
	if err := syncExpiredTakeovers(h.db, time.Now()); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return model.Takeover{}, false
	}
	var takeover model.Takeover
	if err := h.db.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
			return model.Takeover{}, false
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return model.Takeover{}, false
	}
	if !canManageTakeover(user, takeover) {
		fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
		return model.Takeover{}, false
	}
	return takeover, true
}

func isUserBlockedBy(db *gorm.DB, ownerUserID, blockedUserID uint64) (bool, error) {
	if ownerUserID == 0 || blockedUserID == 0 || ownerUserID == blockedUserID {
		return false, nil
	}
	var count int64
	err := db.Model(&model.UserBlock{}).
		Where("owner_user_id = ? AND blocked_user_id = ?", ownerUserID, blockedUserID).
		Count(&count).Error
	return count > 0, err
}

func activeTakeoverMemberBlockQuery(db *gorm.DB, takeoverID, blockedUserID uint64) *gorm.DB {
	return db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user_block AS b ON b.owner_user_id = m.user_id").
		Where("m.takeover_id = ? AND m.member_state = ? AND b.blocked_user_id = ?", takeoverID, model.MemberStateJoined, blockedUserID)
}

func isUserBlockedByActiveTakeoverMember(db *gorm.DB, takeoverID, blockedUserID uint64) (bool, error) {
	var count int64
	err := activeTakeoverMemberBlockQuery(db, takeoverID, blockedUserID).Limit(1).Count(&count).Error
	return count > 0, err
}

func currentTakeoverMemberState(db *gorm.DB, takeoverID, userID uint64) (uint8, error) {
	if userID == 0 {
		return 0, nil
	}
	var member model.TakeoverMember
	if err := db.Where("takeover_id = ? AND user_id = ?", takeoverID, userID).First(&member).Error; err != nil {
		if isNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return member.MemberState, nil
}
