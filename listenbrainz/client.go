package listenbrainz

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://api.listenbrainz.org"

type Client struct {
	token    string
	username string
	http     *http.Client
}

func NewClient(token, username string) *Client {
	return &Client{
		token:    token,
		username: username,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) HasAuth() bool {
	return c.token != "" && c.username != ""
}

func (c *Client) HasUsername() bool {
	return c.username != ""
}

func (c *Client) Username() string {
	return c.username
}

func (c *Client) get(path string, query url.Values) ([]byte, error) {
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Token "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listenbrainz: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

