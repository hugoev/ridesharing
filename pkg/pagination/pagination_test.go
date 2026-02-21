package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestParse_Defaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	params := Parse(c)

	if params.Page != DefaultPage {
		t.Errorf("expected page %d, got %d", DefaultPage, params.Page)
	}
	if params.Limit != DefaultLimit {
		t.Errorf("expected limit %d, got %d", DefaultLimit, params.Limit)
	}
	if params.Offset != 0 {
		t.Errorf("expected offset 0, got %d", params.Offset)
	}
}

func TestParse_CustomValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=3&limit=10", nil)

	params := Parse(c)

	if params.Page != 3 {
		t.Errorf("expected page 3, got %d", params.Page)
	}
	if params.Limit != 10 {
		t.Errorf("expected limit 10, got %d", params.Limit)
	}
	if params.Offset != 20 {
		t.Errorf("expected offset 20, got %d", params.Offset)
	}
}

func TestParse_MaxLimitEnforced(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?limit=500", nil)

	params := Parse(c)

	if params.Limit != MaxLimit {
		t.Errorf("expected limit capped to %d, got %d", MaxLimit, params.Limit)
	}
}

func TestParse_InvalidValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=abc&limit=-5", nil)

	params := Parse(c)

	if params.Page != DefaultPage {
		t.Errorf("expected default page, got %d", params.Page)
	}
	if params.Limit != DefaultLimit {
		t.Errorf("expected default limit for negative, got %d", params.Limit)
	}
}

func TestParse_ZeroPage(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=0", nil)

	params := Parse(c)

	if params.Page != DefaultPage {
		t.Errorf("expected page to be clamped to %d, got %d", DefaultPage, params.Page)
	}
}

func TestNewResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	params := Params{Page: 2, Limit: 3, Offset: 3}

	resp := NewResponse(data, params, 10)

	if resp.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Page)
	}
	if resp.Limit != 3 {
		t.Errorf("expected limit 3, got %d", resp.Limit)
	}
	if resp.TotalCount != 10 {
		t.Errorf("expected total_count 10, got %d", resp.TotalCount)
	}
	if resp.TotalPages != 4 {
		t.Errorf("expected total_pages 4 (ceil(10/3)), got %d", resp.TotalPages)
	}
	if !resp.HasMore {
		t.Error("expected has_more to be true for page 2 of 4")
	}
}

func TestNewResponse_LastPage(t *testing.T) {
	data := []string{"x"}
	params := Params{Page: 4, Limit: 3, Offset: 9}

	resp := NewResponse(data, params, 10)

	if resp.HasMore {
		t.Error("expected has_more to be false on last page")
	}
}

func TestNewResponse_EmptyResult(t *testing.T) {
	data := []string{}
	params := Params{Page: 1, Limit: 20, Offset: 0}

	resp := NewResponse(data, params, 0)

	if resp.TotalCount != 0 {
		t.Errorf("expected total_count 0, got %d", resp.TotalCount)
	}
	if resp.TotalPages != 0 {
		t.Errorf("expected total_pages 0, got %d", resp.TotalPages)
	}
	if resp.HasMore {
		t.Error("expected has_more to be false for empty results")
	}
}
