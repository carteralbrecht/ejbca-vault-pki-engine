package ejbca_vault_pki_engine

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/sdk/helper/certutil"
	"github.com/hashicorp/vault/sdk/helper/errutil"
	"github.com/hashicorp/vault/sdk/logical"
	"time"
)

const (
	revokedPath = "revoked/"
)

type storageContext struct {
	Context context.Context
	Storage logical.Storage
	Backend *ejbcaBackend
}

func (b *ejbcaBackend) makeStorageContext(ctx context.Context, s logical.Storage) *storageContext {
	return &storageContext{
		Context: ctx,
		Storage: s,
		Backend: b,
	}
}

// ====================== roleStorageContext ======================

type roleStorageContext struct {
	storageContext *storageContext
}

func (sc *storageContext) Role() *roleStorageContext {
	return &roleStorageContext{
		storageContext: sc,
	}
}

// ====================== caStorageContext ======================

type caStorageContext struct {
	storageContext *storageContext
}

func (sc *storageContext) CA() *caStorageContext {
	return &caStorageContext{
		storageContext: sc,
	}
}

// ====================== configStorageContext ======================

type configStorageContext struct {
	storageContext *storageContext
}

func (sc *storageContext) Config() *configStorageContext {
	return &configStorageContext{
		storageContext: sc,
	}
}

func (c *configStorageContext) putConfig(config *ejbcaConfig) error {
	entry, err := logical.StorageEntryJSON(configStoragePath, config)
	if err != nil {
		return err
	}
	return c.storageContext.Storage.Put(c.storageContext.Context, entry)
}

func (c *configStorageContext) getConfig() (*ejbcaConfig, error) {
	entry, err := c.storageContext.Storage.Get(c.storageContext.Context, configStoragePath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	config := new(ejbcaConfig)
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, fmt.Errorf("error reading root configuration: %w", err)
	}

	return config, nil
}

func (c *configStorageContext) deleteConfig() error {
	return c.storageContext.Storage.Delete(c.storageContext.Context, configStoragePath)
}

// ====================== certStorageContext ======================

type certStorageContext struct {
	storageContext *storageContext
}

func (sc *storageContext) Cert() *certStorageContext {
	return &certStorageContext{
		storageContext: sc,
	}
}

type certEntry struct {
	Certificate    string                  `json:"certificate"`      // The PEM encoded certificate
	SerialNumber   string                  `json:"serial_number"`    // The serial number of the certificate
	PrivateKeyType certutil.PrivateKeyType `json:"private_key_type"` // The type of the certificate's private key
	PrivateKey     string                  `json:"private_key"`      // The PEM encoded private key
	IssuerName     string                  `json:"issuer_name"`      // The issuer name of the certificate
}

type revokedCertEntry struct {
	Certificate       string    `json:"certificate"`   // The PEM encoded certificate
	SerialNumber      string    `json:"serial_number"` // The serial number of the certificate
	RevocationTime    int64     `json:"revocation_time"`
	RevocationTimeUTC time.Time `json:"revocation_time_utc"`
}

func (c *certStorageContext) putCertEntry(certEntry *certEntry) error {
	entry, err := logical.StorageEntryJSON("certs/"+normalizeSerial(certEntry.SerialNumber), certEntry)
	if err != nil {
		return err
	}
	return c.storageContext.Storage.Put(c.storageContext.Context, entry)
}

func (c *certStorageContext) putRevokedCertEntry(entry *revokedCertEntry) error {
	storageEntry, err := logical.StorageEntryJSON("revoked/"+normalizeSerial(entry.SerialNumber), entry)
	if err != nil {
		return err
	}
	return c.storageContext.Storage.Put(c.storageContext.Context, storageEntry)
}

func (c *certStorageContext) fetchCertBundleBySerial(serial string) (*certutil.ParsedCertBundle, error) {
	hyphenSerial := normalizeSerial(serial)
	path := "certs/" + hyphenSerial

	storageEntry, err := c.storageContext.Storage.Get(c.storageContext.Context, path)
	if err != nil {
		return nil, errutil.InternalError{Err: fmt.Sprintf("error fetching certificate %s: %s", path, err)}
	}

	var parsedStorageEntry certEntry

	if storageEntry != nil && storageEntry.Value != nil && len(storageEntry.Value) > 0 {
		err = storageEntry.DecodeJSON(&parsedStorageEntry)
		if err != nil {
			return nil, errutil.InternalError{Err: fmt.Sprintf("error decoding certificate entry with sn %s: %s", path, err)}
		}
	} else {
		// TODO get certificate from EJBCA
	}

	if parsedStorageEntry.Certificate == "" {
		return nil, errutil.InternalError{Err: fmt.Sprintf("returned certificate bytes were empty")}
	}

	var caCertBundle *certutil.CertBundle
	if parsedStorageEntry.IssuerName != "" {
		var caInfo *certutil.CAInfoBundle
		caInfo, err = c.storageContext.CA().fetchCaBundle(parsedStorageEntry.IssuerName)
		if err != nil {
			return nil, errutil.InternalError{Err: fmt.Sprintf("error fetching certificate bundle for certificate with sn %s: %s", path, err)}
		}
		caCertBundle, err = caInfo.ToCertBundle()
		if err != nil {
			return nil, errutil.InternalError{Err: fmt.Sprintf("error fetching certificate bundle for certificate with sn %s: %s", path, err)}
		}
	}

	cert := &certutil.CertBundle{
		PrivateKeyType: parsedStorageEntry.PrivateKeyType,
		Certificate:    parsedStorageEntry.Certificate,
		IssuingCA:      caCertBundle.CAChain[0],
		CAChain:        caCertBundle.CAChain,
		PrivateKey:     parsedStorageEntry.PrivateKey,
		SerialNumber:   parsedStorageEntry.SerialNumber,
	}

	bundle, err := cert.ToParsedCertBundle()
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (c *certStorageContext) fetchRevokedCertBySerial(serial string) (*revokedCertEntry, error) {
	path := revokedPath + normalizeSerial(serial)

	storageEntry, err := c.storageContext.Storage.Get(c.storageContext.Context, path)
	if err != nil {
		return nil, errutil.InternalError{Err: fmt.Sprintf("error fetching certificate %s: %s", path, err)}
	}

	var parsedStorageEntry revokedCertEntry

	if storageEntry != nil && storageEntry.Value != nil && len(storageEntry.Value) > 0 {
		err = storageEntry.DecodeJSON(&parsedStorageEntry)
		if err != nil {
			return nil, errutil.InternalError{Err: fmt.Sprintf("error decoding revoked certificate entry with sn %s: %s", path, err)}
		}
	} else {
		// TODO get certificate from EJBCA
	}

	return &parsedStorageEntry, nil
}

func (c *certStorageContext) deleteCert(path string) error {
	return c.storageContext.Storage.Delete(c.storageContext.Context, path)
}

func (c *certStorageContext) listRevokedCerts() ([]string, error) {
	list, err := c.storageContext.Storage.List(c.storageContext.Context, revokedPath)
	if err != nil {
		return nil, fmt.Errorf("failed listing revoked certs: %w", err)
	}

	return list, err
}
