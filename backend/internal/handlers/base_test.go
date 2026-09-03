package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponseHelpers(t *testing.T) {
	cases := []struct {
		name string
		fn   func(c *gin.Context)
		code int
	}{
		{"OK", func(c *gin.Context) { OK(c, gin.H{"a": 1}) }, 200},
		{"Created", func(c *gin.Context) { Created(c, gin.H{}) }, 201},
		{"Fail", func(c *gin.Context) { Fail(c, 1, "f") }, 200},
		{"Bad", func(c *gin.Context) { Bad(c, "b") }, 400},
		{"Unauthorized", func(c *gin.Context) { Unauthorized(c, "u") }, 401},
		{"Forbidden", func(c *gin.Context) { Forbidden(c, "f") }, 403},
		{"NotFound", func(c *gin.Context) { NotFound(c, "n") }, 404},
		{"InternalErr", func(c *gin.Context) { InternalErr(c, "i") }, 500},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		tc.fn(c)
		if rec.Code != tc.code {
			t.Fatalf("%s: code %d", tc.name, rec.Code)
		}
	}
}

func TestPagedOK(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	PagedOK(c, []int{1, 2}, 1, 20, 2)
	if rec.Code != 200 {
		t.Fatal("code")
	}
}

func TestGetPageParams(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?page=3&page_size=50", nil)
	p, ps := GetPageParams(c)
	if p != 3 || ps != 50 {
		t.Fatalf("got %d %d", p, ps)
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("GET", "/x?page=0&page_size=999", nil)
	p2, ps2 := GetPageParams(c2)
	if p2 != 1 || ps2 != 20 {
		t.Fatalf("defaults got %d %d", p2, ps2)
	}
}

func TestBroadcastNoUser(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	Broadcast(c, "transactions", "create", 1) // uid 0 -> no-op
}
