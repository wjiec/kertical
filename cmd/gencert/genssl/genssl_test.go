/*
Copyright 2025 Jayson Wang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package genssl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/cmd/gencert/genssl"
)

const (
	Service     = "kertical-example-server"
	Namespace   = "kertical-system"
	CommandName = Service + "." + Namespace + ".svc"
)

func TestGenerateServiceCertBundle(t *testing.T) {
	certBundle, err := genssl.GenerateServiceCertBundle(Service, Namespace)
	if assert.NoError(t, err) && assert.NotNil(t, certBundle) {
		assert.NotNil(t, certBundle.CACert)
		assert.NotNil(t, certBundle.ServicePrivateKey)
		assert.NotNil(t, certBundle.ServiceCert)
	}
}

func TestSelfSignedCertificate(t *testing.T) {
	privateKey, certificate, err := genssl.SelfSignedCertificate(CommandName)
	if assert.NoError(t, err) {
		assert.NotNil(t, privateKey)
		if assert.NotNil(t, certificate) {
			assert.Equal(t, CommandName, certificate.Subject.CommonName)
		}
	}
}

func TestSignCertificate(t *testing.T) {
	caKey, caCertificate, err := genssl.SelfSignedCertificate(CommandName)
	if assert.NoError(t, err) && assert.NotEmpty(t, caCertificate) {
		dnsNames := []string{Service, Service + "." + Namespace, Service + "." + Namespace + ".svc"}
		privateKey, certificate, err := genssl.SignCertificate(caKey, caCertificate, CommandName, dnsNames)
		if assert.NoError(t, err) {
			assert.NotNil(t, privateKey)
			if assert.NotNil(t, certificate) {
				assert.Equal(t, CommandName, certificate.Subject.CommonName)
				assert.Equal(t, dnsNames, certificate.DNSNames)
			}
		}
	}
}

func TestEncodeCertificateToPEM(t *testing.T) {
	_, certificates, err := genssl.SelfSignedCertificate(CommandName)
	if assert.NoError(t, err) && assert.NotEmpty(t, certificates) {
		assert.NotNil(t, genssl.EncodeCertificateToPEM(certificates))
	}
}

func TestEncodePrivateKeyToPEM(t *testing.T) {
	privateKey, _, err := genssl.SelfSignedCertificate(CommandName)
	if assert.NoError(t, err) && assert.NotNil(t, privateKey) {
		encoded, err := genssl.EncodePrivateKeyToPEM(privateKey)
		if assert.NoError(t, err) {
			assert.NotNil(t, encoded)
		}
	}
}
