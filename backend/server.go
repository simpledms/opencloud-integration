package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var tokenPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type service struct {
	config   config
	store    *tokenStore
	cloud    *openCloud
	requests chan struct{}
}

func newService(c config, store *tokenStore, cloud *openCloud) *service {
	return &service{config: c, store: store, cloud: cloud, requests: make(chan struct{}, 32)}
}

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	// Exact routing avoids ServeMux's path-cleaning redirects carrying tokens.
	switch {
	case r.URL.Path == "/healthz" && r.Method == http.MethodGet:
		w.WriteHeader(http.StatusOK)
		return
	case r.URL.Path == appPath+"/api/create-signed-url":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, "Only POST is allowed.")
			return
		}
	case strings.HasPrefix(r.URL.Path, appPath+"/download/"):
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "Only GET is allowed.")
			return
		}
	default:
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeError(w, http.StatusBadRequest, "Query parameters are not accepted.")
		return
	}
	select {
	case s.requests <- struct{}{}:
		defer func() { <-s.requests }()
	default:
		writeError(w, http.StatusServiceUnavailable, "The integration is busy. Please try again.")
		return
	}
	if r.Method == http.MethodPost {
		s.create(w, r)
	} else {
		s.download(w, r)
	}
}

func (s *service) create(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && origin != s.config.PublicURL.String() {
		writeError(w, http.StatusForbidden, "Cross-origin requests are not accepted.")
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeError(w, http.StatusForbidden, "Cross-site requests are not accepted.")
		return
	}
	// Cookies and trusted-proxy user headers are deliberately not authentication.
	auth := strings.Fields(r.Header.Get("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") || len(auth[1]) > 16384 {
		writeError(w, http.StatusUnauthorized, "An OpenCloud bearer token is required.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Expected application/json.")
		return
	}
	var input struct {
		FileID string `json:"fileId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.FileID) > 1024 || !fileIDPattern.MatchString(input.FileID) || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "A valid OpenCloud file ID is required.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	owner, err := s.cloud.userID(ctx, auth[1])
	if err != nil {
		s.upstreamFailure(w, err)
		return
	}
	d, err := s.cloud.file(ctx, auth[1], input.FileID)
	if err != nil {
		s.upstreamFailure(w, err)
		return
	}
	d.ExpiresAt = time.Now().Add(s.config.TokenTTL).Unix()
	token, err := s.store.create(owner, d)
	if err != nil {
		if errors.Is(err, errCapacity) {
			writeError(w, http.StatusTooManyRequests, "Too many pending downloads. Please wait for them to expire.")
		} else {
			writeError(w, http.StatusInternalServerError, "Could not create a download token.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"downloadUrl": s.config.PublicURL.String() + appPath + "/download/" + token,
		"expiresAt":   d.ExpiresAt,
	})
}

func (s *service) download(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, appPath+"/download/")
	if !tokenPattern.MatchString(token) {
		writeError(w, http.StatusNotFound, "Download not found, expired, or already used.")
		return
	}
	d, ok := s.store.consume(token)
	if !ok {
		writeError(w, http.StatusNotFound, "Download not found, expired, or already used.")
		return
	}
	if !s.cloud.validDownloadURL(d.URL) {
		writeError(w, http.StatusBadGateway, "The source file is unavailable. Start a new export.")
		return
	}
	// Incoming Authorization, Cookie, Range and other headers are not forwarded.
	// Only GET is ever sent to OpenCloud. Redirects are never followed.
	resp, err := s.cloud.request(r.Context(), http.MethodGet, d.URL, "", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "The source file is unavailable. Start a new export.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "The source file is unavailable. Start a new export.")
		return
	}
	name := d.Name
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		name = params["filename"]
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	// Keep active content inert even if the download URL is opened in a browser.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Warn("download stream interrupted; token remains consumed")
		// Abort the connection so a truncated unknown-length body is not accepted
		// as a complete successful download by the recipient.
		panic(http.ErrAbortHandler)
	}
}

func (s *service) upstreamFailure(w http.ResponseWriter, err error) {
	var failure upstreamError
	status := http.StatusBadGateway
	if errors.As(err, &failure) {
		status = failure.status
	}
	writeError(w, status, "OpenCloud could not authorize this file. Check your session and file permissions.")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": message})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("response write failed")
	}
}
