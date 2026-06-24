package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
)

const maxUploadImageSize = 5 << 20

func (h *Handler) UploadImage(c *gin.Context) {
	user, _ := currentUser(c)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "file is required")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxUploadImageSize {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "image must be between 1 byte and 5 MB")
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	ext := imageExt(fileHeader, contentType)
	if ext == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "only jpg, png, gif, and webp images are allowed")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "open upload failed")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "open upload failed")
		return
	}
	if err := h.checkImageSecurity(contentSecurityTarget{
		User:        user,
		ContentType: "image",
		Scene:       contentSceneProfile,
	}, fileHeader.Filename, data); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
		return
	}

	bucket, err := h.ossBucket()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "oss not configured")
		return
	}

	objectKey := uploadObjectKey(user.ID, ext)
	if err := bucket.PutObject(objectKey, bytes.NewReader(data), oss.ContentType(contentType)); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "upload failed")
		return
	}

	ok(c, "uploaded", gin.H{"url": h.ossObjectURL(objectKey), "objectKey": objectKey})
}

func (h *Handler) ossBucket() (*oss.Bucket, error) {
	if h.cfg.OSSEndpoint == "" || h.cfg.OSSBucket == "" || h.cfg.OSSAccessKeyID == "" || h.cfg.OSSAccessKeySecret == "" {
		return nil, fmt.Errorf("oss config missing")
	}
	client, err := oss.New(h.cfg.OSSEndpoint, h.cfg.OSSAccessKeyID, h.cfg.OSSAccessKeySecret)
	if err != nil {
		return nil, err
	}
	return client.Bucket(h.cfg.OSSBucket)
}

func (h *Handler) ossObjectURL(objectKey string) string {
	baseURL := strings.TrimRight(h.cfg.OSSBaseURL, "/")
	if baseURL == "" && h.cfg.OSSBucket != "" && h.cfg.OSSEndpoint != "" {
		baseURL = "https://" + h.cfg.OSSBucket + "." + h.cfg.OSSEndpoint
	}
	return baseURL + "/" + objectKey
}

func imageExt(fileHeader *multipart.FileHeader, contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}

	switch strings.ToLower(filepath.Ext(fileHeader.Filename)) {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png", ".gif", ".webp":
		return strings.ToLower(filepath.Ext(fileHeader.Filename))
	default:
		return ""
	}
}

func uploadObjectKey(userID uint64, ext string) string {
	now := time.Now()
	return fmt.Sprintf("miniapp/uploads/%04d/%02d/%d-%d-%s%s", now.Year(), now.Month(), userID, now.UnixNano(), randomHex(6), ext)
}

func randomHex(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "random"
	}
	return hex.EncodeToString(value)
}
