package main

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const appPath = "/apps/simpledms_integration"

type config struct {
	OpenCloudURL *url.URL
	PublicURL    *url.URL
	ConnectAddr  string
	CAFile       string
	ListenAddr   string
	TokenTTL     time.Duration
}

func parseOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("expected an absolute origin without credentials, path, query, or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback())) {
		return nil, errors.New("HTTPS is required except on loopback development origins")
	}
	u.Path = ""
	return u, nil
}

func loadConfig() (config, error) {
	var c config
	var err error
	c.OpenCloudURL, err = parseOrigin(os.Getenv("OPENCLOUD_URL"))
	if err != nil {
		return c, errors.New("invalid OPENCLOUD_URL")
	}
	c.PublicURL, err = parseOrigin(os.Getenv("INTEGRATION_PUBLIC_ORIGIN"))
	if err != nil {
		return c, errors.New("invalid INTEGRATION_PUBLIC_ORIGIN")
	}
	c.ConnectAddr = os.Getenv("OPENCLOUD_CONNECT_ADDRESS")
	if c.ConnectAddr != "" {
		if _, _, err := net.SplitHostPort(c.ConnectAddr); err != nil {
			return c, errors.New("OPENCLOUD_CONNECT_ADDRESS must be host:port")
		}
	}
	c.CAFile = os.Getenv("OPENCLOUD_CA_FILE")
	c.ListenAddr = envDefault("INTEGRATION_LISTEN_ADDRESS", ":8080")
	seconds, err := strconv.Atoi(envDefault("INTEGRATION_TOKEN_TTL_SECONDS", "600"))
	if err != nil || seconds < 1 || seconds > 600 {
		return c, errors.New("INTEGRATION_TOKEN_TTL_SECONDS must be between 1 and 600")
	}
	c.TokenTTL = time.Duration(seconds) * time.Second
	return c, nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
