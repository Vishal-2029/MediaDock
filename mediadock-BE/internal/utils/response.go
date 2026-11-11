package utils

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standard JSON envelope for all REST endpoints.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *PageMeta   `json:"meta,omitempty"`
}

// APIError represents a sanitized, structured client error.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// PageMeta holds pagination metadata for list endpoints.
type PageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// PaginationQuery holds validated pagination parameters.
type PaginationQuery struct {
	Page    int
	PerPage int
	Offset  int
}

// ParsePagination extracts page and per_page query parameters with safe bounds.
func ParsePagination(c *fiber.Ctx) PaginationQuery {
	page := 1
	perPage := 20

	if p, err := strconv.Atoi(c.Query("page", "1")); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(c.Query("per_page", "20")); err == nil && pp > 0 {
		if pp > 100 {
			pp = 100
		}
		perPage = pp
	}

	return PaginationQuery{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

// NewPageMeta calculates pagination metadata from total record count.
func NewPageMeta(q PaginationQuery, total int64) *PageMeta {
	totalPages := 0
	if q.PerPage > 0 && total > 0 {
		totalPages = int((total + int64(q.PerPage) - 1) / int64(q.PerPage))
	}
	return &PageMeta{
		Page:       q.Page,
		PerPage:    q.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// JSONSuccess writes a successful JSON response with optional pagination metadata.
func JSONSuccess(c *fiber.Ctx, status int, data interface{}, meta ...*PageMeta) error {
	resp := APIResponse{
		Success: true,
		Data:    data,
	}
	if len(meta) > 0 && meta[0] != nil {
		resp.Meta = meta[0]
	}
	return c.Status(status).JSON(resp)
}

// JSONError writes a structured error JSON response without leaking internal details.
func JSONError(c *fiber.Ctx, status int, code, message string, details ...map[string]string) error {
	apiErr := &APIError{
		Code:    code,
		Message: message,
	}
	if len(details) > 0 && len(details[0]) > 0 {
		apiErr.Details = details[0]
	}
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error:   apiErr,
	})
}

