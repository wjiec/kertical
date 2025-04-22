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

package main

import (
	"context"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/wjiec/kertical/cmd/gencert/genssl"
	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/webhook/controller/utils"
)

func main() {
	root := &cobra.Command{
		Use:           "gencert [service_name/secret_name]...",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer runtime.HandleCrash()

			return Run(cmd.Context(), kertical.GetNamespace(), args)
		},
	}

	if err := root.ExecuteContext(ctrl.SetupSignalHandler()); err != nil {
		klog.ErrorS(err, "failed to execute gencert")
		os.Exit(1)
	}
}

func Run(ctx context.Context, namespace string, certs []string) error {
	kube, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return err
	}

	for _, cert := range certs {
		parts := strings.Split(cert, "/")
		if len(parts) != 2 {
			return errors.Errorf("invalid certificate format: %s", cert)
		}

		if err = genCert(ctx, kube, namespace, parts[0], parts[1]); err != nil {
			return errors.Wrapf(err, "failed to generate certificate for %s", cert)
		}
	}

	return nil
}

func genCert(ctx context.Context, kube *kubernetes.Clientset, namespace string, svcName, secretName string) error {
	certBundle, err := genssl.GenerateServiceCertBundle(svcName, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to generate certificate bundle")
	}

	secret, err := kube.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("secret not found, create new one", "secret", secretName, "namespace", namespace)

			_, err = kube.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					utils.TLSCACertKey:      certBundle.CACert,
					corev1.TLSCertKey:       certBundle.ServiceCert,
					corev1.TLSPrivateKeyKey: certBundle.ServicePrivateKey,
				},
			}, metav1.CreateOptions{})
			return errors.Wrapf(err, "failed to create secret %s/%s", namespace, secretName)
		}
		return errors.Wrapf(err, "failed to get secret %s/%s", namespace, secretName)
	}

	secret.Data[utils.TLSCACertKey] = certBundle.CACert
	secret.Data[corev1.TLSCertKey] = certBundle.ServiceCert
	secret.Data[corev1.TLSPrivateKeyKey] = certBundle.ServicePrivateKey

	klog.InfoS("secret already exists, update certs", "secret", secretName, "namespace", namespace)
	_, err = kube.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return errors.Wrapf(err, "failed to update secret %s/%s", namespace, secretName)
}
