package handlers_test

import (
	"testing"

	"huozhi/internal/database"
	"huozhi/internal/models"
)

func TestBooksCRUD(t *testing.T) {
	_, tok := newUser(t)

	// list (empty but ok)
	w := do(authReq("GET", "/api/books", tok, nil))
	if w.Code != 200 {
		t.Fatalf("list %d", w.Code)
	}

	// create
	w = do(authReq("POST", "/api/books", tok, map[string]interface{}{
		"name": "Travel", "icon": "✈️", "color": "#fff", "currency": "CNY",
	}))
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	created := decode(t, w)
	book := created["data"].(map[string]interface{})
	bookID := uint(book["id"].(float64))

	// get
	w = do(authReq("GET", "/api/books/"+itoa(bookID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("get %d", w.Code)
	}

	// update
	w = do(authReq("PUT", "/api/books/"+itoa(bookID), tok, map[string]interface{}{
		"name": "Travel2", "color": "#000",
	}))
	if w.Code != 200 {
		t.Fatalf("update %d", w.Code)
	}

	// list members (none yet)
	w = do(authReq("GET", "/api/books/"+itoa(bookID)+"/members", tok, nil))
	if w.Code != 200 {
		t.Fatalf("members %d", w.Code)
	}

	// delete
	w = do(authReq("DELETE", "/api/books/"+itoa(bookID), tok, nil))
	if w.Code != 200 {
		t.Fatalf("delete %d", w.Code)
	}
}

func TestDeleteDefaultBookFails(t *testing.T) {
	uid, tok := newUser(t)
	bookID := seedBook(t, uid) // is_default true

	w := do(authReq("DELETE", "/api/books/"+itoa(bookID), tok, nil))
	m := decode(t, w)
	if int(m["code"].(float64)) != 2001 {
		t.Fatalf("expected 2001, got %v", m["code"])
	}
}

func TestInviteBookMember(t *testing.T) {
	ownerID, ownerTok := newUser(t)
	ownerBook := seedBook(t, ownerID)

	_, otherTok := newUser(t)
	// need other user's username
	var other models.User
	database.DB.Order("id DESC").First(&other)
	otherName := other.Username

	// invite success
	w := do(authReq("POST", "/api/books/"+itoa(ownerBook)+"/members", ownerTok, map[string]string{
		"username": otherName, "role": "editor",
	}))
	if w.Code != 201 {
		t.Fatalf("invite %d %s", w.Code, w.Body.String())
	}

	// invite self fails
	w = do(authReq("POST", "/api/books/"+itoa(ownerBook)+"/members", ownerTok, map[string]string{
		"username": otherName, "role": "editor",
	}))
	// other already a member -> 2004
	m := decode(t, w)
	if int(m["code"].(float64)) != 2004 {
		t.Fatalf("expected 2004 got %v", m["code"])
	}

	// invite nonexistent
	w = do(authReq("POST", "/api/books/"+itoa(ownerBook)+"/members", ownerTok, map[string]string{
		"username": "nobody_xyz", "role": "editor",
	}))
	m = decode(t, w)
	if int(m["code"].(float64)) != 2002 {
		t.Fatalf("expected 2002 got %v", m["code"])
	}

	// owner invites self -> 2003
	var owner models.User
	database.DB.First(&owner, ownerID)
	w = do(authReq("POST", "/api/books/"+itoa(ownerBook)+"/members", ownerTok, map[string]string{
		"username": owner.Username, "role": "editor",
	}))
	m = decode(t, w)
	if int(m["code"].(float64)) != 2003 {
		t.Fatalf("expected 2003 got %v", m["code"])
	}

	// non-owner cannot invite -> 403 Forbidden
	w = do(authReq("POST", "/api/books/"+itoa(ownerBook)+"/members", otherTok, map[string]string{
		"username": otherName, "role": "editor",
	}))
	if w.Code != 403 {
		t.Fatalf("expected 403 got %d %s", w.Code, w.Body.String())
	}
}
