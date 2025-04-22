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

package genssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/pkg/errors"
)

// CertificateBundle holds the certificate and private key data for a service.
type CertificateBundle struct {
	CACert            []byte
	ServicePrivateKey []byte
	ServiceCert       []byte
}

// GenerateServiceCertBundle creates a certificate bundle for a service in a specific namespace.
func GenerateServiceCertBundle(service, namespace string) (*CertificateBundle, error) {
	commonName := service + "." + namespace + ".svc"
	// When using clientConfig.service, the server cert must be valid for <svc_name>.<svc_namespace>.svc.
	//	see https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers for more details.
	dnsNames := []string{service, service + "." + namespace, commonName}

	caPrivateKey, caCert, err := SelfSignedCertificate(commonName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate CA certificate")
	}

	svcPrivateKey, svcCert, err := SignCertificate(caPrivateKey, caCert, commonName, dnsNames)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate service certificate")
	}

	encodedPrivateKey, err := EncodePrivateKeyToPEM(svcPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode service private key")
	}

	return &CertificateBundle{
		CACert:            EncodeCertificateToPEM(caCert),
		ServicePrivateKey: encodedPrivateKey,
		ServiceCert:       EncodeCertificateToPEM(svcCert),
	}, nil
}

// SelfSignedCertificate generates a self-signed certificate for a given common name.
func SelfSignedCertificate(commonName string) (crypto.Signer, *x509.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key")
	}

	serialNumber, err := randomSerialNumber()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate serial number")
	}

	skid, err := subjectKeyId(privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate subject key id")
	}

	template := &x509.Certificate{
		SerialNumber:   serialNumber,
		Subject:        pkix.Name{CommonName: commonName},
		SubjectKeyId:   skid,
		AuthorityKeyId: skid,

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(100, 0, 0),

		KeyUsage: x509.KeyUsageCertSign,

		// Mark as CA certificate
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create certificate")
	}

	cert, err := x509.ParseCertificate(der)
	return privateKey, cert, err
}

// SignCertificate creates a signed certificate using a given CA key and certificate.
func SignCertificate(caKey crypto.Signer, caCert *x509.Certificate, commonName string, dnsNames []string) (crypto.Signer, *x509.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key")
	}

	serialNumber, err := randomSerialNumber()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate serial number")
	}

	skid, err := subjectKeyId(privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate subject key id")
	}

	template := &x509.Certificate{
		SerialNumber:   serialNumber,
		Subject:        pkix.Name{CommonName: commonName},
		SubjectKeyId:   skid,
		AuthorityKeyId: skid,

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(100, 0, 0),

		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageContentCommitment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, privateKey.Public(), caKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create certificate")
	}

	cert, err := x509.ParseCertificate(der)
	return privateKey, cert, err
}

// EncodeCertificateToPEM encodes an x509 certificate into PEM format.
func EncodeCertificateToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// EncodePrivateKeyToPEM encodes a private key into PEM format.
func EncodePrivateKeyToPEM(privateKey crypto.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal private key")
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// randomSerialNumber generates a random serial number for certificate issuance.
func randomSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

// SubjectPublicKeyInfo represents ASN.1 structure for public key info.
type SubjectPublicKeyInfo struct {
	Algorithm        pkix.AlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// subjectKeyId generates a subject key identifier from a given private key.
func subjectKeyId(privateKey crypto.Signer) ([]byte, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, err
	}

	var subjectPublicKeyInfo SubjectPublicKeyInfo
	if _, err = asn1.Unmarshal(pubASN1, &subjectPublicKeyInfo); err != nil {
		return nil, err
	}

	skid := sha1.Sum(subjectPublicKeyInfo.SubjectPublicKey.Bytes)
	return skid[:], nil
}
