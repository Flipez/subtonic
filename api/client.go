package api

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	clientName    = "subtonic"
	clientVersion = "1.0.0"
	apiVersion    = "1.16.1"
)

type ServerInfo struct {
	Version       string // API version (e.g. "1.16.1")
	ServerVersion string // Server software version (e.g. "0.53.3")
	Type          string // Server type (e.g. "navidrome")
}

type Client struct {
	baseURL    string
	username   string
	token      string
	salt       string
	http       *http.Client
	serverInfo ServerInfo
}

func NewClient(baseURL, username, password string) *Client {
	salt := randomSalt(8)
	hash := md5.Sum([]byte(password + salt))
	token := fmt.Sprintf("%x", hash)
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		token:    token,
		salt:     salt,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func randomSalt(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}

func (c *Client) params(extra url.Values) url.Values {
	v := url.Values{}
	v.Set("u", c.username)
	v.Set("t", c.token)
	v.Set("s", c.salt)
	v.Set("v", apiVersion)
	v.Set("c", clientName)
	v.Set("f", "json")
	for key, vals := range extra {
		for _, val := range vals {
			v.Add(key, val)
		}
	}
	return v
}

func (c *Client) get(endpoint string, extra url.Values) (*subsonicResponseBody, error) {
	u := fmt.Sprintf("%s/rest/%s", c.baseURL, endpoint)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = c.params(extra).Encode()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope subsonicResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	body := &envelope.SubsonicResponse
	if body.Version != "" {
		c.serverInfo = ServerInfo{
			Version:       body.Version,
			ServerVersion: body.ServerVersion,
			Type:          body.Type,
		}
	}
	if body.Status != "ok" {
		if body.Error != nil {
			return nil, fmt.Errorf("subsonic error %d: %s", body.Error.Code, body.Error.Message)
		}
		return nil, fmt.Errorf("subsonic status: %s", body.Status)
	}
	return body, nil
}

func (c *Client) ServerInfo() ServerInfo {
	return c.serverInfo
}

func (c *Client) StreamURL(id string) string {
	// Use format=raw so the server sends the original file without transcoding.
	// The native decoders (FLAC, OGG/Vorbis) are truly streaming and don't
	// require buffering the entire file before playback — unlike go-mp3 which
	// reads the whole file in New(). We omit f=json; stream is a binary endpoint.
	p := c.params(url.Values{"id": {id}, "format": {"raw"}})
	p.Del("f") // stream is binary, not JSON
	return fmt.Sprintf("%s/rest/stream?%s", c.baseURL, p.Encode())
}
