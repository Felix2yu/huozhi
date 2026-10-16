package handlers_test

import (
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

func TestListCategories(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	cases := []string{
		"/api/categories?book_id=" + itoa(bookID),
		"/api/categories?book_id=" + itoa(bookID) + "&kind=expense",
		"/api/categories?book_id=" + itoa(bookID) + "&kind=system",
		"/api/categories?book_id=" + itoa(bookID) + "&kind=all&include_archived=1",
		"/api/categories",
	}
	for _, p := range cases {
		w := do(authReq("GET", p, tok, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s -> %d %s", p, w.Code, w.Body.String())
		}
	}
}

func TestCreateCategory(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	w := do(authReq("POST", "/api/categories", tok, map[string]interface{}{
		"book_id": bookID, "name": "测试分类", "kind": "expense", "icon": "x",
	}))
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	// bad body
	w = do(authReq("POST", "/api/categories", tok, map[string]interface{}{
		"book_id": bookID, "kind": "expense",
	}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestUpdateDeleteCategory(t *testing.T) {
	uid, tok, bookID := registerRealUser(t)
	catID := seedCategory(t, uid, bookID, models.KindExpense, "可改分类")
	w := do(authReq("PUT", "/api/categories/"+itoa(catID), tok, map[string]interface{}{
		"name": "改名后", "kind": "expense", "icon": "y",
	}))
	if w.Code != 200 {
		t.Fatalf("update %d %s", w.Code, w.Body.String())
	}
	// bad body
	w = do(authReq("PUT", "/api/categories/"+itoa(catID), tok, map[string]interface{}{}))
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
	// delete normal category (archives)
	w = do(authReq("DELETE", "/api/categories/"+itoa(catID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}

	// system category cannot be modified/deleted
	var sys models.Category
	database.DB.Where("user_id = ? AND is_system = ?", uid, true).First(&sys)
	w = do(authReq("PUT", "/api/categories/"+itoa(sys.ID), tok, map[string]interface{}{
		"name": "x", "kind": "expense",
	}))
	if int(decode(t, w)["code"].(float64)) != 3001 {
		t.Fatalf("expected 3001 got %v", decode(t, w)["code"])
	}
	w = do(authReq("DELETE", "/api/categories/"+itoa(sys.ID), tok, nil))
	if int(decode(t, w)["code"].(float64)) != 3001 {
		t.Fatalf("expected 3001 delete got %v", decode(t, w)["code"])
	}
}

func TestTagsCRUD(t *testing.T) {
	_, tok, bookID := registerRealUser(t)
	w := do(authReq("GET", "/api/tags", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list tags %d", w.Code)
	}
	w = do(authReq("POST", "/api/tags", tok, map[string]interface{}{
		"book_id": bookID, "name": "商务", "color": "#fff",
	}))
	if w.Code != 201 {
		t.Fatalf("create tag %d %s", w.Code, w.Body.String())
	}
	m := decode(t, w)
	tagID := uint(m["data"].(map[string]interface{})["id"].(float64))
	w = do(authReq("PUT", "/api/tags/"+itoa(tagID), tok, map[string]interface{}{
		"name": "差旅", "color": "#000",
	}))
	if w.Code != 200 {
		t.Fatalf("update tag %d %s", w.Code, w.Body.String())
	}
	w = do(authReq("DELETE", "/api/tags/"+itoa(tagID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete tag %d", w.Code)
	}
}
