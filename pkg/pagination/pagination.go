// Package pagination provides utilities for paginating query results.
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// Params holds parsed pagination parameters.
type Params struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"-"`
}

// Parse extracts pagination params from the Gin context query string.
// Supports ?page=1&limit=20. Enforces MaxLimit.
func Parse(c *gin.Context) Params {
	page := parseIntDefault(c.Query("page"), DefaultPage)
	limit := parseIntDefault(c.Query("limit"), DefaultLimit)

	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return Params{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

// Response wraps paginated data with metadata.
type Response struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalCount int         `json:"total_count"`
	TotalPages int         `json:"total_pages"`
	HasMore    bool        `json:"has_more"`
}

// NewResponse creates a paginated response from the given data and total count.
func NewResponse(data interface{}, params Params, totalCount int) Response {
	totalPages := totalCount / params.Limit
	if totalCount%params.Limit > 0 {
		totalPages++
	}

	return Response{
		Data:       data,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalCount: totalCount,
		TotalPages: totalPages,
		HasMore:    params.Page < totalPages,
	}
}

func parseIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
