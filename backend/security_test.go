package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type upstreamFixture struct {
	mu          sync.Mutex
	permissions string
	folder      bool
	mode        string
	graphStatus int
	davStatus   int
	gets        atomic.Int32
	seenBearer  []string
	getAuth     string
	getCookie   string
}

func newTestService(t *testing.T) (*service, *httptest.Server, *upstreamFixture) {
	t.Helper()
	fixture := &upstreamFixture{permissions: "RDNVCKZ"}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		if r.Header.Get("Authorization") != "" {
			fixture.seenBearer = append(fixture.seenBearer, r.Header.Get("Authorization"))
		}
		switch {
		case r.URL.Path == "/graph/v1.0/me":
			if fixture.graphStatus != 0 {
				w.WriteHeader(fixture.graphStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"owner-1"}`)
		case r.Method == "PROPFIND":
			if fixture.davStatus != 0 {
				w.WriteHeader(fixture.davStatus)
				return
			}
			resourceType := ""
			if fixture.folder {
				resourceType = `<d:collection/>`
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns"><d:response><d:href>/remote.php/dav/spaces/fixture.pdf</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status><d:prop xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns"><d:resourcetype>`+resourceType+`</d:resourcetype><d:getcontentlength>4</d:getcontentlength><oc:permissions>`+fixture.permissions+`</oc:permissions><oc:downloadURL>`+upstreamURLPlaceholder(r)+`</oc:downloadURL></d:prop></d:propstat></d:response></d:multistatus>`)
		case r.Method == http.MethodGet:
			fixture.gets.Add(1)
			fixture.getAuth = r.Header.Get("Authorization")
			fixture.getCookie = r.Header.Get("Cookie")
			switch fixture.mode {
			case "redirect":
				http.Redirect(w, r, "https://evil.example/secret", http.StatusFound)
			case "error":
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "upstream secret details")
			default:
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", `attachment; filename="fixture.pdf"`)
				io.WriteString(w, "PDF!")
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(upstream.Close)

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store := newTokenStore()
	c := config{OpenCloudURL: u, PublicURL: u, TokenTTL: time.Minute}
	cloud, err := newOpenCloud(c)
	if err != nil {
		t.Fatal(err)
	}
	return newService(c, store, cloud), upstream, fixture
}

func upstreamURLPlaceholder(r *http.Request) string {
	return "http://" + r.Host + "/remote.php/dav/spaces/fixture.pdf?oc-jwt-sig=upstream-secret"
}

func issueToken(t *testing.T, s *service, fileID string) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, appPath+"/api/create-signed-url", strings.NewReader(`{"fileId":"`+fileID+`"}`))
	r.Header.Set("Authorization", "Bearer user-token")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("issue: got %d: %s", w.Code, w.Body.String())
	}
	var payload struct {
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(payload.DownloadURL)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(u.Path)
}

func TestIssuanceAuthenticatesAndBindsFileMetadata(t *testing.T) {
	s, _, fixture := newTestService(t)
	token := issueToken(t, s, "fixture.pdf")
	if len(token) != 64 || strings.Contains(token, "upstream-secret") {
		t.Fatalf("unexpected opaque token %q", token)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.seenBearer) != 2 || fixture.seenBearer[0] != "Bearer user-token" || fixture.seenBearer[1] != "Bearer user-token" {
		t.Errorf("upstream bearer forwarding = %#v", fixture.seenBearer)
	}
	var count int
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if _, ok := s.store.tokens[tokenHash(token)]; ok {
		count = 1
	}
	if count != 1 {
		t.Errorf("hashed token row count = %d", count)
	}
	if _, ok := s.store.tokens[token]; ok {
		t.Error("plaintext token stored")
	}
}

func TestIssuanceRejectsMissingAuthCrossOriginUnknownAndUnsafeInput(t *testing.T) {
	s, _, _ := newTestService(t)
	tests := []struct {
		name   string
		method string
		body   string
		setup  func(*http.Request)
		want   int
	}{
		{"no auth", http.MethodPost, `{"fileId":"fixture.pdf"}`, func(*http.Request) {}, http.StatusUnauthorized},
		{"cookie is not auth", http.MethodPost, `{"fileId":"fixture.pdf"}`, func(r *http.Request) { r.Header.Set("Cookie", "oc_session=secret") }, http.StatusUnauthorized},
		{"cross origin", http.MethodPost, `{"fileId":"fixture.pdf"}`, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer x")
			r.Header.Set("Origin", "https://evil.example")
		}, http.StatusForbidden},
		{"unknown field", http.MethodPost, `{"fileId":"fixture.pdf","url":"https://evil"}`, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }, http.StatusBadRequest},
		{"path traversal", http.MethodPost, `{"fileId":"../secret"}`, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }, http.StatusBadRequest},
		{"oversized file ID", http.MethodPost, `{"fileId":"` + strings.Repeat("a", 1025) + `"}`, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, appPath+"/api/create-signed-url", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			tt.setup(r)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestIssuanceDoesNotStoreTokenForUpstreamAuthorizationFailures(t *testing.T) {
	for _, test := range []struct {
		name        string
		graphStatus int
		davStatus   int
		want        int
	}{
		{"invalid bearer rejected by graph", http.StatusUnauthorized, 0, http.StatusUnauthorized},
		{"access denied by DAV", 0, http.StatusForbidden, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, fixture := newTestService(t)
			fixture.graphStatus = test.graphStatus
			fixture.davStatus = test.davStatus
			r := httptest.NewRequest(http.MethodPost, appPath+"/api/create-signed-url", strings.NewReader(`{"fileId":"fixture.pdf"}`))
			r.Header.Set("Authorization", "Bearer invalid-token")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != test.want {
				t.Fatalf("status = %d, want %d", w.Code, test.want)
			}
			s.store.mu.Lock()
			count := len(s.store.tokens)
			s.store.mu.Unlock()
			if count != 0 {
				t.Fatalf("stored %d token(s) after authorization failure", count)
			}
		})
	}
}

func TestIssuanceRejectsFoldersAndNoDownloadPermission(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*upstreamFixture)
	}{
		{"folder", func(f *upstreamFixture) { f.folder = true }},
		{"secure-view permission", func(f *upstreamFixture) { f.permissions = "RDNVCKX" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, fixture := newTestService(t)
			test.set(fixture)
			r := httptest.NewRequest(http.MethodPost, appPath+"/api/create-signed-url", strings.NewReader(`{"fileId":"fixture.pdf"}`))
			r.Header.Set("Authorization", "Bearer user-token")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", w.Code)
			}
		})
	}
}

func TestDownloadIsPublicNoStoreAndSingleUse(t *testing.T) {
	s, _, fixture := newTestService(t)
	token := issueToken(t, s, "fixture.pdf")
	get := func(method string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, appPath+"/download/"+token, nil)
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	w := get(http.MethodGet, map[string]string{"Cookie": "session=secret", "Authorization": "Bearer secret"})
	if w.Code != http.StatusOK || w.Body.String() != "PDF!" || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), "fixture.pdf") {
		t.Fatalf("download = %d, %q, headers=%v", w.Code, w.Body.String(), w.Header())
	}
	fixture.mu.Lock()
	if fixture.getAuth != "" || fixture.getCookie != "" {
		t.Errorf("recipient credentials forwarded upstream: authorization=%q cookie=%q", fixture.getAuth, fixture.getCookie)
	}
	fixture.mu.Unlock()
	if second := get(http.MethodGet, nil); second.Code != http.StatusNotFound {
		t.Errorf("second GET = %d, want 404", second.Code)
	}
}

func TestDownloadMethodsDoNotConsumeToken(t *testing.T) {
	s, _, _ := newTestService(t)
	token := issueToken(t, s, "fixture.pdf")
	for _, method := range []string{http.MethodHead, http.MethodPut, http.MethodDelete} {
		r := httptest.NewRequest(method, appPath+"/download/"+token, nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", method, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, appPath+"/download/"+token, nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("GET after rejected methods = %d, want 200", w.Code)
	}
}

func TestDownloadParallelConsumptionHasExactlyOneWinner(t *testing.T) {
	s, _, fixture := newTestService(t)
	token := issueToken(t, s, "fixture.pdf")
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	start := make(chan struct{})
	done := make(chan struct{}, 2)
	call := func(w *httptest.ResponseRecorder) {
		<-start
		r := httptest.NewRequest(http.MethodGet, appPath+"/download/"+token, nil)
		s.ServeHTTP(w, r)
		done <- struct{}{}
	}
	go call(first)
	go call(second)
	close(start)
	<-done
	<-done
	if !((first.Code == http.StatusOK && second.Code == http.StatusNotFound) || (first.Code == http.StatusNotFound && second.Code == http.StatusOK)) {
		t.Fatalf("statuses = %d and %d, want exactly one 200", first.Code, second.Code)
	}
	if fixture.gets.Load() != 1 {
		t.Errorf("upstream GETs = %d, want 1", fixture.gets.Load())
	}
}

func TestExpiredTokenAndNewStoreInvalidatesOldTokens(t *testing.T) {
	s, _, _ := newTestService(t)
	expired, err := s.store.create("owner", download{URL: s.cloud.origin.String() + "/remote.php/dav/spaces/fixture.pdf?oc-jwt-sig=x", Name: "x", ExpiresAt: time.Now().Add(-time.Second).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+expired, nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("expired token = %d, want 404", w.Code)
	}

	oldToken := issueToken(t, s, "fixture.pdf")
	store := newTokenStore()
	s.store = store
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+oldToken, nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("token from previous store = %d, want 404", w.Code)
	}
	newToken := issueToken(t, s, "fixture.pdf")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+newToken, nil))
	if w.Code != http.StatusOK {
		t.Errorf("token from new store = %d, want 200", w.Code)
	}
}

func TestTokenStoreCapacityIsBoundedUnderParallelIssuance(t *testing.T) {
	d := download{URL: "https://cloud.example/file", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store := newTokenStore()
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.create("same-owner", d); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != maxPendingPerUser {
		t.Fatalf("successful per-user issuances = %d, want %d", successes.Load(), maxPendingPerUser)
	}
	store.mu.Lock()
	got := len(store.tokens)
	store.mu.Unlock()
	if got != maxPendingPerUser {
		t.Fatalf("stored per-user tokens = %d, want %d", got, maxPendingPerUser)
	}

	store = newTokenStore()
	var globalSuccesses atomic.Int32
	for i := 0; i < maxPendingTokens*2; i++ {
		wg.Add(1)
		owner := "owner-" + strconv.Itoa(i%maxPendingTokens)
		go func() {
			defer wg.Done()
			if _, err := store.create(owner, d); err == nil {
				globalSuccesses.Add(1)
			}
		}()
	}
	wg.Wait()
	if globalSuccesses.Load() != maxPendingTokens {
		t.Fatalf("successful global issuances = %d, want %d", globalSuccesses.Load(), maxPendingTokens)
	}
	store.mu.Lock()
	got = len(store.tokens)
	store.mu.Unlock()
	if got != maxPendingTokens {
		t.Fatalf("stored global tokens = %d, want %d", got, maxPendingTokens)
	}
}

func TestTokenStoreExpiredRecordsAreReclaimed(t *testing.T) {
	store := newTokenStore()
	d := download{URL: "https://cloud.example/file", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if _, err := store.create("owner", download{URL: d.URL, ExpiresAt: time.Now().Add(-time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	store.purge()
	if _, err := store.create("owner", d); err != nil {
		t.Fatalf("issuance after purge: %v", err)
	}
	store.mu.Lock()
	got := len(store.tokens)
	store.mu.Unlock()
	if got != 1 {
		t.Fatalf("stored tokens after reclaim = %d, want 1", got)
	}
}

func TestTokenStoreConsumptionFreesPerUserSlot(t *testing.T) {
	store := newTokenStore()
	d := download{URL: "https://cloud.example/file", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	var tokens []string
	for i := 0; i < maxPendingPerUser; i++ {
		token, err := store.create("owner", d)
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	if _, ok := store.consume(tokens[0]); !ok {
		t.Fatal("consuming pending token failed")
	}
	if _, err := store.create("owner", d); err != nil {
		t.Fatalf("issuance after consumption: %v", err)
	}
}

func TestFailedOrMaliciousUpstreamIsGenericAndConsumed(t *testing.T) {
	for _, mode := range []string{"redirect", "error"} {
		t.Run(mode, func(t *testing.T) {
			s, upstream, fixture := newTestService(t)
			fixture.mode = mode
			token := issueToken(t, s, "fixture.pdf")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+token, nil))
			if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), upstream.URL) || strings.Contains(w.Body.String(), "upstream-secret") {
				t.Errorf("unsafe upstream response: %d %s", w.Code, w.Body.String())
			}
			w = httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+token, nil))
			if w.Code != http.StatusNotFound {
				t.Errorf("retry after failed transfer = %d, want 404", w.Code)
			}
		})
	}

	s, _, _ := newTestService(t)
	bad, err := s.store.create("owner", download{URL: "https://evil.example/secret?oc-jwt-sig=secret", Name: "x", ExpiresAt: time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appPath+"/download/"+bad, nil))
	if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "evil.example") {
		t.Errorf("malicious URL response = %d %s", w.Code, w.Body.String())
	}
}
