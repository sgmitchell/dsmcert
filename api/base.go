// Docs
// https://global.download.synology.com/download/Document/Software/DeveloperGuide/Package/FileStation/All/enu/Synology_File_Station_API_Guide.pdf
// tail /var/log/synoscgi.log for better error messages
package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type Client struct {
	BaseUrl  string
	User     string
	Password string

	client *http.Client

	authSid string
	authMu  sync.RWMutex
}

func NewClient(baseUrl, user, password string) (*Client, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("bad baseUrl. %w", err)
	}
	if u.Path == "" {
		u.Path = "/webapi"
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &Client{
		BaseUrl:  u.String(),
		User:     user,
		Password: password,
		client:   &http.Client{Transport: tr},
	}, nil
}

type Request struct {
	ApiName string            `json:"api"`
	Version int               `json:"version"`
	Path    string            `json:"path"`
	Method  string            `json:"method"`
	Params  map[string]string `json:"params,omitempty"`
}

func (c *Client) Url(r Request) (*url.URL, error) {
	if r.Path == "" {
		return nil, fmt.Errorf("must supply request path")
	}
	u, err := url.Parse(fmt.Sprintf("%s/%s", c.BaseUrl, r.Path))
	if err != nil {
		return nil, fmt.Errorf("failed to build base url. %w", err)
	}

	params := map[string]string{
		"api":     r.ApiName,
		"version": strconv.Itoa(r.Version),
		"method":  r.Method,
	}
	for k, v := range r.Params {
		params[k] = v
	}

	c.authMu.RLock()
	if c.authSid != "" {
		params["_sid"] = c.authSid
	} else {
		// TODO warn
	}
	c.authMu.RUnlock()

	qp := u.Query()
	for k, v := range params {
		if v == "" {
			return nil, fmt.Errorf("cannot have empty param %q", k)
		}
		qp.Add(k, v)
	}
	u.RawQuery = qp.Encode()
	return u, nil
}

func (c *Client) GetRequest(r Request) (*http.Request, error) {
	u, err := c.Url(r)
	if err != nil {
		return nil, err
	}
	return http.NewRequest("GET", u.String(), nil)
}

type Response struct {
	Data    json.RawMessage
	Success bool `json:"success"`
	Error   struct {
		Code   int         `json:"code"`
		Errors interface{} `json:"errors"`
	} `json:"error"`
}

func (c *Client) Do(r *http.Request, data interface{}) error {
	client := c.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var rspBody Response
	if b, err := ioutil.ReadAll(resp.Body); err != nil {
		return err
	} else if err = json.Unmarshal(b, &rspBody); err != nil {
		return err
	}

	if !rspBody.Success {
		return fmt.Errorf("did not get succss. got %+v", rspBody)
	}

	if d := rspBody.Data; d != nil {
		return json.Unmarshal(d, &data)
	}

	if data != nil {
		return fmt.Errorf("got nil data but expected %T", data)
	}
	return nil
}

func (c *Client) ListApis() ([]Request, error) {
	r := Request{ApiName: "SYNO.API.Info", Method: "query", Version: 1, Path: "query.cgi", Params: map[string]string{"query": "all"}}
	req, err := c.GetRequest(r)
	if err != nil {
		return nil, err
	}
	var resp map[string]Request
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}
	var out []Request
	for k, v := range resp {
		v.ApiName = k
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) Login() error {
	if c == nil {
		return fmt.Errorf("nil client when trying to login")
	}

	r := Request{
		ApiName: "SYNO.API.Auth",
		Path:    "auth.cgi",
		Version: 3,
		Method:  "login",
		Params: map[string]string{
			"account": c.User,
			"passwd":  c.Password,
			"format":  "sid",
		},
	}
	req, err := c.GetRequest(r)
	if err != nil {
		return err
	}
	var resp struct {
		Sid string `json:"sid"`
	}
	if err := c.Do(req, &resp); err != nil {
		return fmt.Errorf("failed to login. %w", err)
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	c.authSid = resp.Sid
	return nil
}
