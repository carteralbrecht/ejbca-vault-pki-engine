package ejbca_vault_pki_engine

import (
	"context"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPathFetchCa(t *testing.T) {
	t.Skip()
	b, reqStorage := getTestBackend(t)

	err := testConfigCreate(t, b, reqStorage, map[string]interface{}{
		"client_cert":                 clientCert,
		"client_key":                  clientKey,
		"hostname":                    hostname,
		"default_ca":                  _defaultCaName,
		"default_end_entity_profile":  defaultEndEntityProfile,
		"default_certificate_profile": defaultCertificateProfile,
	})

	assert.NoError(t, err)

	t.Run("Test /ca", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "ca")
		assert.NoError(t, err)
	})

	t.Run("Test ca/pem", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "ca/pem")
		assert.NoError(t, err)
	})

	t.Run("Test cert/ca", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "cert/ca")
		assert.NoError(t, err)
	})

	t.Run("Test cert/ca/raw", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "cert/ca/raw")
		assert.NoError(t, err)
	})

	t.Run("Test cert/ca/raw/pem", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "cert/ca/raw/pem")
		assert.NoError(t, err)
	})

	t.Run("Test ca_chain", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "ca_chain")
		assert.NoError(t, err)
	})

	t.Run("Test cert/ca_chain", func(t *testing.T) {
		err = testFetchCa(b, reqStorage, "cert/ca_chain")
		assert.NoError(t, err)
	})
}

func testFetchCa(b logical.Backend, s logical.Storage, path string) error {
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      path,
		Storage:   s,
	})

	if err != nil {
		return err
	}

	if resp != nil && resp.IsError() {
		return resp.Error()
	}

	return nil
}
