/*
Copyright 2026.

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
	"testing"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/gomega" //revive:disable:dot-imports
)

// TestSchemeHasCertManagerTypes guards against the manager's client scheme silently losing
// cert-manager types it needs at runtime: OpenStackReconciler.checkWebhookCertificates reads
// certmgrv1.Certificate objects, and a missing scheme registration fails every Get call with a
// scheme error rather than a usable apierrors.IsNotFound, which unit tests built on their own
// fake-client scheme would not catch.
func TestSchemeHasCertManagerTypes(t *testing.T) {
	g := NewWithT(t)
	g.Expect(scheme.Recognizes(certmgrv1.SchemeGroupVersion.WithKind("Certificate"))).To(BeTrue())
}
