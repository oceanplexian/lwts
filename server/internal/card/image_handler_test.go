package card

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/repo"
)

// tinyPNG returns the bytes of a real 2x2 PNG so content-type sniffing
// (http.DetectContentType) classifies it as image/png.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// setupImageHandler builds a handler with a wired image repo plus one card.
func setupImageHandler(t *testing.T) (*Handler, repo.User, repo.Card, *repo.CardRepository) {
	t.Helper()
	ds, users, boards, cards, comments := setupTestWithDS(t)
	h := NewHandler(cards, boards, comments, nil)
	h.SetImages(repo.NewCardImageRepository(ds))

	ctx := context.Background()
	user, _ := users.Create(ctx, "User", "u@t.com", "h")
	board, _ := boards.Create(ctx, "B", "LWTS", user.ID)
	card, err := cards.Create(ctx, board.ID, repo.CardCreate{ColumnID: "todo", Title: "Has image"})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	return h, user, card, cards
}

func imageMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/cards/{id}/images", http.HandlerFunc(h.UploadImage))
	mux.Handle("GET /api/v1/cards/{id}/images", http.HandlerFunc(h.ListImages))
	mux.Handle("GET /api/v1/cards/{id}/images/{imageId}", http.HandlerFunc(h.GetImage))
	mux.Handle("DELETE /api/v1/cards/{id}/images/{imageId}", http.HandlerFunc(h.DeleteImage))
	mux.Handle("GET /api/v1/cards/{id}", http.HandlerFunc(h.Get))
	return mux
}

// multipartBody builds a multipart/form-data body with a single "file" part.
// CreateFormFile sets the part content-type to application/octet-stream, so
// this exercises the server's content sniffing fallback.
func multipartBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return &body, mw.FormDataContentType()
}

func TestUploadImageMultipart(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", body)
	req.Header.Set("Content-Type", ct)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var img repo.CardImage
	if err := json.Unmarshal(w.Body.Bytes(), &img); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.ID == "" {
		t.Error("image id empty")
	}
	if img.CardID != card.ID {
		t.Errorf("card_id = %q, want %q", img.CardID, card.ID)
	}
	if img.ContentType != "image/png" {
		t.Errorf("content_type = %q, want image/png", img.ContentType)
	}
	if img.SizeBytes != len(data) {
		t.Errorf("size_bytes = %d, want %d", img.SizeBytes, len(data))
	}
	if img.Filename != "shot.png" {
		t.Errorf("filename = %q, want shot.png", img.Filename)
	}
	wantURL := "/api/v1/cards/" + card.ID + "/images/" + img.ID
	if img.URL != wantURL {
		t.Errorf("url = %q, want %q", img.URL, wantURL)
	}
	// Image bytes must never be serialized in the metadata JSON.
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	if _, ok := raw["data"]; ok {
		t.Error("response leaked image data field")
	}
}

func TestUploadImageRawBody(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images?filename=raw.png", bytes.NewReader(data))
	req.Header.Set("Content-Type", "image/png")
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var img repo.CardImage
	_ = json.Unmarshal(w.Body.Bytes(), &img)
	if img.ContentType != "image/png" {
		t.Errorf("content_type = %q", img.ContentType)
	}
	if img.Filename != "raw.png" {
		t.Errorf("filename = %q, want raw.png (from query)", img.Filename)
	}
}

func TestUploadByCardKey(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	// card.Key is like "LWTS-1"; the path must resolve keys, not just UUIDs.
	body, ct := multipartBody(t, "file", "k.png", data)
	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.Key+"/images", body)
	req.Header.Set("Content-Type", ct)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var img repo.CardImage
	_ = json.Unmarshal(w.Body.Bytes(), &img)
	if img.CardID != card.ID {
		t.Errorf("card_id = %q, want %q (key resolution failed)", img.CardID, card.ID)
	}
}

func TestListImages(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	up := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", body)
	up.Header.Set("Content-Type", ct)
	up = withUser(up, user)
	mux.ServeHTTP(httptest.NewRecorder(), up)

	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID+"/images", nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Images []repo.CardImage `json:"images"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(resp.Images))
	}
	if resp.Images[0].ContentType != "image/png" || resp.Images[0].URL == "" {
		t.Errorf("listed image missing fields: %+v", resp.Images[0])
	}
}

func TestGetImageBytes(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	up := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", body)
	up.Header.Set("Content-Type", ct)
	up = withUser(up, user)
	upRec := httptest.NewRecorder()
	mux.ServeHTTP(upRec, up)
	var img repo.CardImage
	_ = json.Unmarshal(upRec.Body.Bytes(), &img)

	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID+"/images/"+img.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if !bytes.Equal(w.Body.Bytes(), data) {
		t.Errorf("served bytes differ from uploaded (%d vs %d)", w.Body.Len(), len(data))
	}
}

func TestCardDetailIncludesImages(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	up := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", body)
	up.Header.Set("Content-Type", ct)
	up = withUser(up, user)
	mux.ServeHTTP(httptest.NewRecorder(), up)

	req := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID, nil)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Images []repo.CardImage `json:"images"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Images) != 1 {
		t.Errorf("card detail images len = %d, want 1", len(resp.Images))
	}
}

func TestUploadNonImageRejected(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", bytes.NewReader([]byte("just some text, not an image")))
	req.Header.Set("Content-Type", "text/plain")
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body: %s", w.Code, w.Body.String())
	}
}

func TestUploadToMissingCard(t *testing.T) {
	h, user, _, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	req := httptest.NewRequest("POST", "/api/v1/cards/LWTS-9999/images", body)
	req.Header.Set("Content-Type", ct)
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestUploadEmptyBodyRejected(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)

	req := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "image/png")
	req = withUser(req, user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteImage(t *testing.T) {
	h, user, card, _ := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	body, ct := multipartBody(t, "file", "shot.png", data)
	up := httptest.NewRequest("POST", "/api/v1/cards/"+card.ID+"/images", body)
	up.Header.Set("Content-Type", ct)
	up = withUser(up, user)
	upRec := httptest.NewRecorder()
	mux.ServeHTTP(upRec, up)
	var img repo.CardImage
	_ = json.Unmarshal(upRec.Body.Bytes(), &img)

	del := httptest.NewRequest("DELETE", "/api/v1/cards/"+card.ID+"/images/"+img.ID, nil)
	del = withUser(del, user)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body: %s", delRec.Code, delRec.Body.String())
	}

	// Now the image bytes endpoint should 404.
	get := httptest.NewRequest("GET", "/api/v1/cards/"+card.ID+"/images/"+img.ID, nil)
	get = withUser(get, user)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("get after delete status = %d, want 404", getRec.Code)
	}
}

func TestGetImageWrongCardRejected(t *testing.T) {
	h, user, cardA, cards := setupImageHandler(t)
	mux := imageMux(h)
	data := tinyPNG(t)

	// A second card on the same board.
	cardB, err := cards.Create(context.Background(), cardA.BoardID, repo.CardCreate{ColumnID: "todo", Title: "Other"})
	if err != nil {
		t.Fatalf("create cardB: %v", err)
	}

	body, ct := multipartBody(t, "file", "shot.png", data)
	up := httptest.NewRequest("POST", "/api/v1/cards/"+cardA.ID+"/images", body)
	up.Header.Set("Content-Type", ct)
	up = withUser(up, user)
	upRec := httptest.NewRecorder()
	mux.ServeHTTP(upRec, up)
	var img repo.CardImage
	_ = json.Unmarshal(upRec.Body.Bytes(), &img)

	// Fetch image via the WRONG card's path → must not leak across cards.
	get := httptest.NewRequest("GET", "/api/v1/cards/"+cardB.ID+"/images/"+img.ID, nil)
	get = withUser(get, user)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("cross-card get status = %d, want 404", getRec.Code)
	}
}
