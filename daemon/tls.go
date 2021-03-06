/*
 * Copyright 2017 Manuel Gauto (github.com/twa16)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package userspaced

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"

	"github.com/spf13/viper"
	"path/filepath"
)

//pemBlockForKey Gets blocks from PrivateKey
func pemBlockForKey(priv *rsa.PrivateKey) *pem.Block {
	return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
}

//CreateSelfSignedCertificate Creates a self-signed certificate for a hostname
func CreateSelfSignedCertificate(host string) (*rsa.PrivateKey, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	notBefore := time.Now()

	notAfter := notBefore.Add(time.Duration(viper.GetInt("CertValidity")) * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		//log.Fatalf("failed to generate serial number: %s", err)
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{viper.GetString("CertOrganization")},
			OrganizationalUnit: []string{viper.GetString("CertOrganizationalUnit")},
			Locality:           []string{viper.GetString("CertLocality")},
			Province:           []string{viper.GetString("CertProvince")},
			Country:            []string{viper.GetString("CertCountry")},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	template.DNSNames = append(template.DNSNames, host)

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		//log.Fatalf("Failed to create certificate: %s", err)
		return nil, nil, err
	}
	return priv, derBytes, nil
}

//WriteCertificateToFile Writes a certificate to a file
func WriteCertificateToFile(certificate []byte, filePath string) error {
	certPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Criticalf("Failed to expand path: %s", filePath)
		return err
	}
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	certOut.Close()
	return nil
}

//WritePrivateKeyToFile Writes a private key to a file
func WritePrivateKeyToFile(key *rsa.PrivateKey, filePath string) error {
	certPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Criticalf("Failed to expand path: %s", filePath)
		return err
	}
	keyOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	pem.Encode(keyOut, pemBlockForKey(key))
	keyOut.Close()
	return nil
}
