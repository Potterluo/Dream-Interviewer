package server

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/events"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// handlers_files.go: multipart upload → bytes on disk + metadata row.
//
// Uploads land in <DataDir>/files/<id> (no user input in the path, so no
// traversal surface); the database keeps name/size/content-type. Swap the
// disk read/write pair for S3/MinIO later without touching the API shape.
//
// MaxUploadBytes is a DoS floor enforced with http.MaxBytesReader; raise
// per deployment if you need bigger files.

const MaxUploadBytes = 50 << 20 // 50 MiB

func (s *Server) filesDir() string {
	return filepath.Join(s.cfg.DataDir, "files")
}

func (s *Server) ensureFilesDir() error {
	return os.MkdirAll(s.filesDir(), 0o755)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	// Scope goes through the shared resolver (scope.go), like every other
	// list endpoint: an admin may widen with ?all=true / ?userId=, and a
	// non-admin asking for either is refused with 403 instead of silently
	// getting their own rows back. This used to widen inline, which made
	// files the one list where the documented rule did not hold.
	userID, ok := s.resolveScope(w, r)
	if !ok {
		return
	}
	recs, err := s.store.ListFileRecords(r.Context(), userID)
	if err != nil {
		storeError(w, err)
		return
	}
	out := make([]fileDTO, 0, len(recs))
	for i := range recs {
		out = append(out, toFileDTO(&recs[i]))
	}
	writeOK(w, map[string]any{"files": out})
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	if err := s.ensureFilesDir(); err != nil {
		storeError(w, err)
		return
	}

	// The reader refuses bodies past the cap; ParseMultipartForm then
	// returns an error mentioning the limit.
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil { // up to 8MB buffered in memory, rest spills to temp files
		writeError(w, http.StatusRequestEntityTooLarge,
			"upload too large or malformed (limit "+strconv.Itoa(MaxUploadBytes>>20)+" MiB)")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, `multipart field "file" is required`)
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename) // strip any client-side path bits
	if name == "" || name == "." || name == "/" {
		writeError(w, http.StatusBadRequest, "file has no usable name")
		return
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	id, err := randomID("f_")
	if err != nil {
		storeError(w, err)
		return
	}
	dstPath := filepath.Join(s.filesDir(), id)
	dst, err := os.Create(dstPath)
	if err != nil {
		storeError(w, err)
		return
	}
	size, err := io.Copy(dst, file)
	closeErr := dst.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(dstPath) // don't leave orphaned blobs on failure
		storeError(w, errors.Join(err, closeErr))
		return
	}

	rec := &store.FileRecord{
		ID:          id,
		UserID:      ident.EffectiveUserID(),
		Name:        name,
		Size:        size,
		ContentType: contentType,
	}
	if err := s.store.CreateFileRecord(r.Context(), rec); err != nil {
		_ = os.Remove(dstPath)
		storeError(w, err)
		return
	}
	s.hub.PublishTo(rec.UserID, events.Event{Type: "file.uploaded", Data: toFileDTO(rec)})
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "file": toFileDTO(rec)})
}

// handleDownloadFile streams the bytes back with the stored content type
// and original filename. API-key callers may pass ?token= (this endpoint
// is on the auth package's allow-list) so plain HTTP clients can use
// plain URLs.
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.loadOwnedFile(w, r)
	if !ok {
		return
	}
	path := filepath.Join(s.filesDir(), rec.ID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "file data missing")
			return
		}
		storeError(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", rec.ContentType)
	// FormatMediaType quotes/escapes the filename per RFC 6266 so names
	// with quotes/spaces/non-ASCII can't inject headers.
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": rec.Name}))
	http.ServeContent(w, r, rec.Name, rec.CreatedAt, f)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.loadOwnedFile(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteFileRecord(r.Context(), rec.ID); err != nil {
		storeError(w, err)
		return
	}
	// Best-effort blob removal; a leaked file with no row is a janitor
	// problem, a row with no file 404s cleanly on download.
	_ = os.Remove(filepath.Join(s.filesDir(), rec.ID))
	s.hub.PublishTo(rec.UserID, events.Event{Type: "file.deleted", Data: toFileDTO(rec)})
	writeOK(w, nil)
}

// loadOwnedFile enforces ownership with the same 404-for-strangers rule
// as items.
func (s *Server) loadOwnedFile(w http.ResponseWriter, r *http.Request) (*store.FileRecord, bool) {
	ident, _ := auth.FromContext(r.Context())
	rec, err := s.store.GetFileRecord(r.Context(), r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return nil, false
	}
	if rec.UserID != ident.UserID && !ident.IsAdmin() {
		writeError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return rec, true
}

// fileDTO is the wire shape (mirrors web/src/lib/api.ts FileItem).
type fileDTO struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toFileDTO(f *store.FileRecord) fileDTO {
	return fileDTO{
		ID: f.ID, UserID: f.UserID, Name: f.Name,
		Size: f.Size, ContentType: f.ContentType, CreatedAt: f.CreatedAt,
	}
}
