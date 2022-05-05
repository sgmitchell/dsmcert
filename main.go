package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sgmitchell/dsmcert/api"
	"github.com/sgmitchell/go-common/log"
	"github.com/sgmitchell/go-common/skeleton"
	"os"
	"time"
)

const (
	passwordEnvVar = "DSM_PASSWORD"
)

var (
	fUrl      = flag.String("url", "", "the url that you use to access your DSM")
	fUser     = flag.String("user", "", "a username that can upload certs")
	fCertFile = flag.String("cert", "/cert/tls.crt", "the certificate file")
	fKeyFile  = flag.String("key", "/cert/tls.key", "the key file of the cert")
	fFreq     = flag.Duration("freq", time.Hour, "how often to check the cert file")
	fId       = flag.String("id", "", "the certificate id in your DSM")
	fDesc     = flag.String("desc", "", "the certificate description in your DSM")
)

func run(ctx context.Context) error {
	if *fDesc == "" {
		return fmt.Errorf("must specify -desc flag")
	}

	if *fUrl == "" {
		return fmt.Errorf("must supply -url flag")
	}
	if *fUser == "" {
		return fmt.Errorf("must specify -user flag")
	}
	password, pwExists := os.LookupEnv(passwordEnvVar)
	if !pwExists {
		return fmt.Errorf("must specify password via %q env var", passwordEnvVar)
	}

	client, err := api.NewClient(*fUrl, *fUser, password)
	if err != nil {
		return fmt.Errorf("failed to create client. %w", err)
	}
	if err := client.Login(); err != nil {
		return fmt.Errorf("failed to login. %w", err)
	}
	existing, err := client.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certs. %w", err)
	}
	log.Infof(ctx, "found %d existing certs", len(existing))
	for _, c := range existing {
		log.Infof(ctx, "found cert: %s", c.String())
	}
	cfg := &CertCfg{
		Id:   *fId,
		Desc: *fDesc,
		Cert: *fCertFile,
		Key:  *fKeyFile,
	}

	return CheckForever(ctx, client, cfg, *fFreq)
}

func main() {
	skeleton.RunCommand(run)
}
