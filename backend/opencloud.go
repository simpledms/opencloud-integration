package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var fileIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9!$._-]*$`)

const metadataRequest = `<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns"><d:prop>
<d:resourcetype/><d:getcontentlength/><oc:permissions/><oc:downloadURL/>
</d:prop></d:propfind>`

type upstreamError struct {
	status int
}

func (e upstreamError) Error() string { return "OpenCloud request failed" }

type openCloud struct {
	origin *url.URL
	client *http.Client
}

func newOpenCloud(c config) (*openCloud, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Do not send bearer credentials through an ambient HTTP(S)_PROXY.
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, err
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("OPENCLOUD_CA_FILE contains no certificates")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	if c.ConnectAddr != "" {
		dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, c.ConnectAddr)
		}
	}
	return &openCloud{
		origin: c.OpenCloudURL,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (o *openCloud) request(ctx context.Context, method, target, bearer string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, upstreamError{http.StatusBadGateway}
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if method == "PROPFIND" {
		req.Header.Set("Depth", "0")
		req.Header.Set("Content-Type", "application/xml")
	}
	resp, err := o.client.Do(req)
	if err != nil {
		// net/http errors contain the URL, which may contain a bearer credential.
		return nil, upstreamError{http.StatusBadGateway}
	}
	return resp, nil
}

func readBounded(r io.Reader) ([]byte, error) {
	const limit = 1 << 20
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || len(b) > limit {
		return nil, upstreamError{http.StatusBadGateway}
	}
	return b, nil
}

func (o *openCloud) userID(ctx context.Context, bearer string) (string, error) {
	resp, err := o.request(ctx, http.MethodGet, o.origin.String()+"/graph/v1.0/me", bearer, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", mapUpstreamStatus(resp.StatusCode)
	}
	b, err := readBounded(resp.Body)
	if err != nil {
		return "", err
	}
	var user struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &user) != nil || user.ID == "" {
		return "", upstreamError{http.StatusBadGateway}
	}
	return user.ID, nil
}

type davProperties struct {
	ResourceType *struct {
		Collection *struct{} `xml:"DAV: collection"`
	} `xml:"DAV: resourcetype"`
	Length      *string `xml:"DAV: getcontentlength"`
	Permissions *string `xml:"http://owncloud.org/ns permissions"`
	DownloadURL string  `xml:"http://owncloud.org/ns downloadURL"`
}

func (o *openCloud) file(ctx context.Context, bearer, fileID string) (download, error) {
	resp, err := o.request(ctx, "PROPFIND", o.origin.String()+"/remote.php/dav/spaces/"+url.PathEscape(fileID), bearer, strings.NewReader(metadataRequest))
	if err != nil {
		return download{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		return download{}, mapUpstreamStatus(resp.StatusCode)
	}
	b, err := readBounded(resp.Body)
	if err != nil {
		return download{}, err
	}
	var multistatus struct {
		XMLName   xml.Name `xml:"DAV: multistatus"`
		Responses []struct {
			Href      string `xml:"DAV: href"`
			Propstats []struct {
				Status string        `xml:"DAV: status"`
				Props  davProperties `xml:"DAV: prop"`
			} `xml:"DAV: propstat"`
		} `xml:"DAV: response"`
	}
	if xml.Unmarshal(b, &multistatus) != nil || len(multistatus.Responses) != 1 {
		return download{}, upstreamError{http.StatusBadGateway}
	}
	var props davProperties
	for _, ps := range multistatus.Responses[0].Propstats {
		fields := strings.Fields(ps.Status)
		if len(fields) < 2 || fields[1] != "200" {
			continue
		}
		if ps.Props.ResourceType != nil {
			props.ResourceType = ps.Props.ResourceType
		}
		if ps.Props.Length != nil {
			props.Length = ps.Props.Length
		}
		if ps.Props.Permissions != nil {
			props.Permissions = ps.Props.Permissions
		}
		if ps.Props.DownloadURL != "" {
			props.DownloadURL = ps.Props.DownloadURL
		}
	}
	if props.ResourceType == nil || props.ResourceType.Collection != nil || props.Length == nil || props.Permissions == nil || strings.Contains(*props.Permissions, "X") {
		return download{}, upstreamError{http.StatusForbidden}
	}
	if length, err := strconv.ParseInt(*props.Length, 10, 64); err != nil || length < 0 {
		return download{}, upstreamError{http.StatusBadGateway}
	}
	if !o.validDownloadURL(props.DownloadURL) {
		return download{}, upstreamError{http.StatusBadGateway}
	}
	href, err := url.Parse(multistatus.Responses[0].Href)
	if err != nil {
		return download{}, upstreamError{http.StatusBadGateway}
	}
	return download{URL: props.DownloadURL, Name: path.Base(href.Path)}, nil
}

func (o *openCloud) validDownloadURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != o.origin.Scheme || u.Host != o.origin.Host || u.User != nil || u.Fragment != "" {
		return false
	}
	// Only the configured OpenCloud's regular-file DAV endpoint is allowed.
	// Public-share URLs and arbitrary remote URLs are never accepted.
	if !strings.HasPrefix(u.Path, "/remote.php/dav/spaces/") || path.Clean(u.Path) != u.Path || strings.Contains(u.Path, "\\") {
		return false
	}
	return u.Query().Get("oc-jwt-sig") != ""
}

func mapUpstreamStatus(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return upstreamError{status}
	default:
		return upstreamError{http.StatusBadGateway}
	}
}
