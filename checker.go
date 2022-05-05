package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/sgmitchell/dsmcert/api"
	"github.com/sgmitchell/go-common/log"
	"sync"
	"time"
)

const (
	maxUpdateFreq = 30 * time.Second
)

type CertCfg struct {
	Id   string // the cert id to look for
	Desc string // the cert description to look for
	Cert string // the path to the cert file
	Key  string // the path to the key file
}

func CheckForever(ctx context.Context, client *api.Client, cfg *CertCfg, freq time.Duration) error {
	if cfg == nil {
		return fmt.Errorf("got nil config")
	}

	ticker := time.NewTicker(freq)

	// trigger when files change
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher. %w", err)
	}
	defer watcher.Close()
	for _, f := range []string{cfg.Cert, cfg.Key} {
		if err := watcher.Add(f); err != nil {
			return fmt.Errorf("failed to watch %q. %w", f, err)
		}
		log.Debugf(ctx, "will recheck if file %q changes", f)
	}

	var last time.Time
	var mu sync.Mutex
	doCheck := func() error {
		mu.Lock()
		defer mu.Unlock()
		if dur := time.Since(last); dur < maxUpdateFreq {
			log.Warnf(ctx, "skipping check since we ran %s ago", dur)
			return nil
		}
		last = time.Now()
		return CheckOnce(ctx, client, cfg)
	}

	log.Debugf(ctx, "checking cert every %s", freq)
	if err = doCheck(); err != nil {
		return err
	}
	for {
		select {
		case <-ticker.C:
			if err = doCheck(); err != nil {
				return err
			}
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if err = doCheck(); err != nil {
				return err
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Errorf(ctx, "file watcher had an error. %s", err)
		}
	}

}

func CheckOnce(ctx context.Context, client *api.Client, cfg *CertCfg) error {
	if client == nil {
		return fmt.Errorf("got nil client")
	}
	if cfg == nil {
		return fmt.Errorf("got nil config")
	}

	ourCert, err := FirstCert(cfg.Cert, cfg.Key)
	if err != nil || ourCert == nil {
		return fmt.Errorf("failed to load local cert. %w", err)
	}
	localValidTill := ourCert.NotAfter
	log.Debugf(ctx, "we have a cert that should be valid until %s", localValidTill)

	if err := client.Login(); err != nil {
		return fmt.Errorf("failed to login. %w", err)
	}
	var existing api.Cert
	existingCerts, err := client.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certs. %w", err)
	}

	for _, c := range existingCerts {
		if cfg.Id != "" {
			if cfg.Id == c.Id {
				existing = c
			}
		} else if cfg.Desc != "" && cfg.Desc == c.Description {
			existing = c
		}
		if existing.Id != "" {
			break
		}
	}

	id := existing.Id
	if id == "" {
		if cfg.Id != "" {
			return fmt.Errorf("no cert found with id %q", cfg.Id)
		}
		log.Infof(ctx, "uploading new cert with description %q", cfg.Desc)
		id, err = client.UploadNewCert(cfg.Desc, cfg.Cert, cfg.Key)
	} else if existing.ValidTill.Equal(localValidTill) {
		log.Debugf(ctx, "noop. up to date cert found. %s", existing.String())
		return nil
	} else {
		log.Infof(ctx, "replacing cert %s with one that expires on %s", existing.String(), localValidTill)
		err = client.ReUploadCert(id, cfg.Cert, cfg.Key)
	}

	if err != nil {
		return fmt.Errorf("failed to put cert. %w", err)
	}
	log.Infof(ctx, "cert %q should be up to date", existing.Id)
	return nil
}

func FirstCert(cert, key string) (*x509.Certificate, error) {
	certs, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	if len(certs.Certificate) == 0 {
		return nil, fmt.Errorf("no certs in file")
	}
	return x509.ParseCertificate(certs.Certificate[0])
}
