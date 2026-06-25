package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/phngkhuongduy/shortlink/internal/shortener"
)

// Handler holds dependencies for the HTTP layer.
type Handler struct {
	svc     *shortener.Service
	baseURL string // e.g. "http://localhost:8080", used to build short URLs
}

// NewHandler creates a Handler. baseURL has any trailing slash trimmed.
func NewHandler(svc *shortener.Service, baseURL string) *Handler {
	return &Handler{svc: svc, baseURL: strings.TrimRight(baseURL, "/")}
}

// Router builds the gin engine with all routes registered.
func (h *Handler) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/encode", h.encode)
	r.POST("/decode", h.decode)
	r.GET("/healthz", h.health)
	// Convenience redirect endpoint so a short URL works in a browser.
	r.GET("/:code", h.redirect)
	return r
}

type encodeRequest struct {
	URL string `json:"url"`
}

type encodeResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (h *Handler) encode(c *gin.Context) {
	var req encodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	code, err := h.svc.Encode(c.Request.Context(), req.URL)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, encodeResponse{
		Code:     code,
		ShortURL: h.baseURL + "/" + code,
	})
}

type decodeRequest struct {
	// Accept either field name for ergonomics; short_url is the documented one.
	ShortURL string `json:"short_url"`
	URL      string `json:"url"`
}

type decodeResponse struct {
	URL string `json:"url"`
}

func (h *Handler) decode(c *gin.Context) {
	var req decodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	input := req.ShortURL
	if input == "" {
		input = req.URL
	}
	longURL, err := h.svc.Decode(c.Request.Context(), input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, decodeResponse{URL: longURL})
}

func (h *Handler) redirect(c *gin.Context) {
	longURL, err := h.svc.Decode(c.Request.Context(), c.Param("code"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Redirect(http.StatusFound, longURL)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// writeServiceError maps domain errors to HTTP status codes.
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, shortener.ErrInvalidURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, shortener.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
