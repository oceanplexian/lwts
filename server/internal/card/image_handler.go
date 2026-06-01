package card

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

// maxImageUploadBytes bounds in-memory multipart buffering. The global
// body-limit middleware (MAX_UPLOAD_SIZE, default 10MiB) is the real ceiling;
// this just keeps ParseMultipartForm from spilling more than needed to disk.
const maxImageUploadBytes = 10 << 20

var errNoFile = errors.New("no file part in multipart upload")

// SetImages enables image attachment endpoints. Passing nil leaves them
// returning 404 / empty so the handler degrades gracefully when unwired.
func (h *Handler) SetImages(images *repo.CardImageRepository) { h.images = images }

func imageURL(cardID, imageID string) string {
	return "/api/v1/cards/" + cardID + "/images/" + imageID
}

// normalizeMediaType strips any parameters (e.g. "; charset=utf-8") and lowercases.
func normalizeMediaType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct
}

// isImageMediaType reports whether ct is an image media type we accept.
func isImageMediaType(ct string) bool {
	return strings.HasPrefix(normalizeMediaType(ct), "image/")
}

// resolveImageContentType trusts an explicitly-declared image media type
// (covers SVG, which content-sniffing classifies as XML); otherwise it sniffs
// from the bytes (handles multipart parts sent as application/octet-stream).
func resolveImageContentType(declared string, data []byte) string {
	if isImageMediaType(declared) {
		return normalizeMediaType(declared)
	}
	return normalizeMediaType(http.DetectContentType(data))
}

// readUpload extracts image bytes, declared content type, and filename from
// either a multipart/form-data body (field "file" or "image") or a raw binary
// body (Content-Type header + optional ?filename=).
func readUpload(r *http.Request) (data []byte, contentType, filename string, err error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err = r.ParseMultipartForm(maxImageUploadBytes); err != nil {
			return nil, "", "", err
		}
		file, header, ferr := r.FormFile("file")
		if ferr != nil {
			file, header, ferr = r.FormFile("image")
		}
		if ferr != nil {
			return nil, "", "", errNoFile
		}
		defer file.Close()
		data, err = io.ReadAll(file)
		if err != nil {
			return nil, "", "", err
		}
		return data, header.Header.Get("Content-Type"), cleanFilename(header.Filename), nil
	}

	data, err = io.ReadAll(r.Body)
	if err != nil {
		return nil, "", "", err
	}
	return data, r.Header.Get("Content-Type"), cleanFilename(r.URL.Query().Get("filename")), nil
}

func cleanFilename(name string) string {
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	if base == "." || base == string(filepath.Separator) || base == "/" {
		return ""
	}
	return base
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if h.images == nil {
		writeErr(w, http.StatusNotFound, "image storage not enabled")
		return
	}
	id, err := h.resolveExistingCard(r, r.PathValue("id"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	data, declaredCT, filename, err := readUpload(r)
	if err != nil {
		if errors.Is(err, errNoFile) {
			writeValidation(w, map[string]string{"file": "required"})
			return
		}
		writeErr(w, http.StatusBadRequest, "invalid upload")
		return
	}
	if len(data) == 0 {
		writeValidation(w, map[string]string{"file": "empty"})
		return
	}

	contentType := resolveImageContentType(declaredCT, data)
	if !isImageMediaType(contentType) {
		writeErr(w, http.StatusUnsupportedMediaType, "only image uploads are allowed")
		return
	}

	user := auth.UserFromContext(r.Context())
	var uploadedBy *string
	if user != nil {
		uploadedBy = &user.ID
	}

	img, err := h.images.Create(r.Context(), id, filename, contentType, data, uploadedBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	img.URL = imageURL(id, img.ID)

	if card, cerr := h.cards.GetByID(r.Context(), id); cerr == nil {
		senderID := ""
		if user != nil {
			senderID = user.ID
		}
		broadcast(h.hub, card.BoardID, "card_image_added", map[string]any{"card_id": id, "image": img}, senderID)
	}

	writeJSON(w, http.StatusCreated, img)
}

func (h *Handler) ListImages(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveExistingCard(r, r.PathValue("id"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	images := h.cardImages(r, id)
	writeJSON(w, http.StatusOK, map[string]any{"images": images})
}

func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	if h.images == nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	cardID, err := resolveID(r.Context(), h.cards, r.PathValue("id"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	img, data, err := h.images.GetData(r.Context(), r.PathValue("imageId"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	// Scope the image to its card so a valid id can't be fetched via another card.
	if img.CardID != cardID {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}

	ct := img.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	disposition := "inline"
	if img.Filename != "" {
		disposition += "; filename=" + strconv.Quote(img.Filename)
	}
	w.Header().Set("Content-Disposition", disposition)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if h.images == nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	cardID, err := resolveID(r.Context(), h.cards, r.PathValue("id"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	img, err := h.images.GetMeta(r.Context(), r.PathValue("imageId"))
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if img.CardID != cardID {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}

	if err := h.images.Delete(r.Context(), img.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := auth.UserFromContext(r.Context())
	senderID := ""
	if user != nil {
		senderID = user.ID
	}
	if card, cerr := h.cards.GetByID(r.Context(), cardID); cerr == nil {
		broadcast(h.hub, card.BoardID, "card_image_deleted",
			map[string]string{"card_id": cardID, "image_id": img.ID}, senderID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveExistingCard resolves a UUID-or-key to a card UUID and confirms the
// card exists. resolveID returns UUIDs unverified, so this guards both paths.
func (h *Handler) resolveExistingCard(r *http.Request, idOrKey string) (string, error) {
	id, err := resolveID(r.Context(), h.cards, idOrKey)
	if err != nil {
		return "", err
	}
	if _, err := h.cards.GetByID(r.Context(), id); err != nil {
		return "", err
	}
	return id, nil
}

// cardImages lists a card's images with URLs populated, tolerating a nil repo.
func (h *Handler) cardImages(r *http.Request, cardID string) []repo.CardImage {
	images := []repo.CardImage{}
	if h.images == nil {
		return images
	}
	listed, err := h.images.ListByCard(r.Context(), cardID)
	if err != nil {
		return images
	}
	for i := range listed {
		listed[i].URL = imageURL(listed[i].CardID, listed[i].ID)
	}
	if listed != nil {
		images = listed
	}
	return images
}
