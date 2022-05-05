package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Cert struct {
	Description string      `json:"desc"`
	Id          string      `json:"id"`
	IsDefault   bool        `json:"is_default"`
	ValidFrom   synoWeirdTs `json:"valid_from"`
	ValidTill   synoWeirdTs `json:"valid_till"`
	Issuer      CertIssuer  `json:"issuer"`
	Subject     CertSubject `json:"subject"`
}

func (c *Cert) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s (%s) [%s - %s]", c.Id, c.Description, c.ValidFrom, c.ValidTill)
}

type synoWeirdTs struct {
	time.Time
}

func (ts *synoWeirdTs) UnmarshalJSON(data []byte) error {
	const layout = `"Jan _2 15:04:05 2006 MST"`
	t, err := time.Parse(layout, string(data))
	if err == nil {
		ts.Time = t
	}
	return err
}

type CertIssuer struct {
	CN      string `json:"common_name"`
	Country string `json:"country"`
	Org     string `json:"organization"`
}

type CertSubject struct {
	CommonName string   `json:"common_name"`
	AltNames   []string `json:"sub_alt_name"`
}

func certReq(method string) Request {
	base := "SYNO.Core.Certificate"
	if method != "import" {
		base += ".CRT"
	}
	return Request{ApiName: base, Method: method, Version: 1, Path: "entry.cgi", Params: map[string]string{}}
}

func (c *Client) ListCerts() ([]Cert, error) {
	req, err := c.GetRequest(certReq("list"))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Certs []Cert `json:"certificates"`
	}
	err = c.Do(req, &resp)
	return resp.Certs, err
}

func (c *Client) GetCert(id string) (json.RawMessage, error) {
	r := certReq("get")
	r.Params["id"] = id
	req, err := c.GetRequest(r)
	if err != nil {
		return nil, err
	}
	var resp json.RawMessage
	err = c.Do(req, &resp)
	return resp, err
}

func (c *Client) UploadNewCert(desc, certPath, keyPath string) (string, error) {
	attrs := map[string]string{"desc": desc}
	return c.setCert(certPath, keyPath, attrs)
}

func (c *Client) ReUploadCert(id, certPath, keyPath string) error {
	certs, err := c.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certs. %w", err)
	}
	var cert Cert
	for _, c := range certs {
		if c.Id == id {
			cert = c
			break
		}
	}
	if cert.Id == "" {
		return fmt.Errorf("failed to find cert with id %q", id)
	}

	attrs := map[string]string{
		"id":         cert.Id,
		"desc":       cert.Description,
		"as_default": fmt.Sprintf("%t", cert.IsDefault),
	}
	newId, err := c.setCert(certPath, keyPath, attrs)
	if err != nil {
		return fmt.Errorf("failed to reupload cert. %w", err)
	}
	if newId != id {
		return fmt.Errorf("the cert id has changed")
	}
	return nil
}

func (c *Client) setCert(certPath, keyPath string, attrs map[string]string) (string, error) {
	u, err := c.Url(certReq("import"))
	if err != nil {
		return "", err
	}

	// https://matt.aimonetti.net/posts/2013-07-golang-multipart-file-upload-example/
	body := &bytes.Buffer{}
	mp := multipart.NewWriter(body)

	addFile := func(name, fp string) error {
		f, err := os.Open(fp)
		if err != nil {
			return fmt.Errorf("failed to open file %q. %w", fp, err)
		}
		defer f.Close()
		if part, err := mp.CreateFormFile(name, filepath.Base(fp)); err != nil {
			return fmt.Errorf("failed to create form file %q. %w", name, err)
		} else if _, err = io.Copy(part, f); err != nil {
			return fmt.Errorf("failed to copy contents of %q to form. %w", fp, err)
		}
		return nil
	}

	//if _, err := mp.CreateFormFile("inter_cert", ""); err != nil {
	//	return "", fmt.Errorf("failed to create form file inter_cert. %w", err)
	//}

	if err := addFile("key", keyPath); err != nil {
		return "", fmt.Errorf("failed to add key file. %w", err)
	}
	if err := addFile("cert", certPath); err != nil {
		return "", fmt.Errorf("failed to add cert file. %w", err)
	}

	for k, v := range attrs {
		if err := mp.WriteField(k, v); err != nil {
			return "", fmt.Errorf("failed to add %s=%s field. %w", k, v, err)
		}
	}
	// TODO, setting this isn't enough to actually have it be used, We also need to call some set operations on SYNO.Core.Certificate.Service
	//if setDefault {
	//	if err := mp.WriteField("as_default", "true"); err != nil {
	//		return err
	//	}
	//}

	if err := mp.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer. %w", err)
	}

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return "", fmt.Errorf("failed to create request. %w", err)
	}
	req.Header.Set("Content-Type", mp.FormDataContentType())

	var resp struct {
		Id string `json:"id"`
	}
	if err := c.Do(req, &resp); err != nil {
		return "", fmt.Errorf("failed to import cert. %w", err)
	}
	return resp.Id, nil
}
