/*
Copyright 2014 The Kubernetes Authors.

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

package validation

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	netutils "k8s.io/utils/net"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

var (
	supportedPathTypes = sets.NewString(
		string(networkingv1.PathTypeExact),
		string(networkingv1.PathTypePrefix),
		string(networkingv1.PathTypeImplementationSpecific),
	)
	invalidPathSequences = []string{"//", "/./", "/../", "%2f", "%2F"}
	invalidPathSuffixes  = []string{"/..", "/."}
)

// ValidateServiceName can be used to check whether the given service name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidateServiceName = apimachineryvalidation.NameIsDNS1035Label

// ValidateIngressClassName validates that the given name can be used as an
// IngressClass name.
var ValidateIngressClassName = apimachineryvalidation.NameIsDNSSubdomain

// ValidateDNS1123Label validates if value conforms to the definition of a label in DNS (RFC 1123).
func ValidateDNS1123Label(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsDNS1123Label(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}
	return allErrs
}

// ValidateQualifiedName validates if name is what Kubernetes calls a "qualified name".
func ValidateQualifiedName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsQualifiedName(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}
	return allErrs
}

func ValidatePortNumOrName(port intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if port.Type == intstr.Int {
		for _, msg := range validation.IsValidPortNum(port.IntValue()) {
			allErrs = append(allErrs, field.Invalid(fldPath, port.IntValue(), msg))
		}
	} else if port.Type == intstr.String {
		for _, msg := range validation.IsValidPortName(port.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath, port.StrVal, msg))
		}
	} else {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("unknown type: %v", port.Type)))
	}
	return allErrs
}

// IngressValidationOptions cover beta to GA transitions for HTTP PathType
type IngressValidationOptions struct {
	// AllowInvalidSecretName indicates whether spec.tls[*].secretName values that are not valid Secret names should be allowed
	AllowInvalidSecretName bool

	// AllowInvalidWildcardHostRule indicates whether invalid rule values are allowed in rules with wildcard hostnames
	AllowInvalidWildcardHostRule bool
}

// ValidateIngressTLS tests if a given ingress TLS is valid.
func ValidateIngressTLS(ingressTLS []networkingv1.IngressTLS, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	// the wildcard spec from RFC 6125 into account.
	for tlsIndex, tls := range ingressTLS {
		for i, host := range tls.Hosts {
			if strings.Contains(host, "*") {
				for _, msg := range validation.IsWildcardDNS1123Subdomain(host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("hosts").Index(i), host, msg))
				}
				continue
			}
			for _, msg := range validation.IsDNS1123Subdomain(host) {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("hosts").Index(i), host, msg))
			}
		}

		if !opts.AllowInvalidSecretName {
			for _, msg := range validateTLSSecretName(tls.SecretName) {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("secretName"), tls.SecretName, msg))
			}
		}
	}

	return allErrs
}

func validateTLSSecretName(name string) []string {
	if len(name) == 0 {
		return nil
	}
	return apimachineryvalidation.NameIsDNSSubdomain(name, false)
}

// ValidateIngressBackend tests if a given backend is valid.
func ValidateIngressBackend(backend *networkingv1alpha1.ExternalProxyIngressBackend, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	hasPortName := len(backend.Port.Name) > 0
	hasPortNumber := backend.Port.Number != 0
	if hasPortName && hasPortNumber {
		allErrs = append(allErrs, field.Invalid(fldPath, "", "cannot set both port name & port number"))
	} else if hasPortName {
		for _, msg := range validation.IsValidPortName(backend.Port.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("service", "port", "name"), backend.Port.Name, msg))
		}
	} else if hasPortNumber {
		for _, msg := range validation.IsValidPortNum(int(backend.Port.Number)) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("service", "port", "number"), backend.Port.Number, msg))
		}
	} else {
		allErrs = append(allErrs, field.Required(fldPath, "port name or number is required"))
	}

	return allErrs
}

func ValidateIngressRules(rules []networkingv1alpha1.ExternalProxyIngressRule, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, rule := range rules {
		wildcardHost := false
		if len(rule.Host) > 0 {
			if isIP := netutils.ParseIPSloppy(rule.Host) != nil; isIP {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), rule.Host, "must be a DNS name, not an IP address"))
			}
			// TODO: Ports and ips are allowed in the host part of a url
			// according to RFC 3986, consider allowing them.
			if strings.Contains(rule.Host, "*") {
				for _, msg := range validation.IsWildcardDNS1123Subdomain(rule.Host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), rule.Host, msg))
				}
				wildcardHost = true
			} else {
				for _, msg := range validation.IsDNS1123Subdomain(rule.Host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), rule.Host, msg))
				}
			}
		}

		if !wildcardHost || !opts.AllowInvalidWildcardHostRule {
			allErrs = append(allErrs, validateHTTPIngressRuleValue(rule.HTTP, fldPath.Index(i))...)
		}
	}
	return allErrs
}

func validateHTTPIngressRuleValue(httpRule *networkingv1alpha1.ExternalProxyIngressHttpRuleValue, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(httpRule.Paths) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("paths"), ""))
	}
	for i, path := range httpRule.Paths {
		allErrs = append(allErrs, validateHTTPIngressPath(&path, fldPath.Child("paths").Index(i))...)
	}
	return allErrs
}

func validateHTTPIngressPath(path *networkingv1alpha1.ExternalProxyIngressHttpPath, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if path.PathType == nil {
		return append(allErrs, field.Required(fldPath.Child("pathType"), "pathType must be specified"))
	}

	switch *path.PathType {
	case networkingv1.PathTypeExact, networkingv1.PathTypePrefix:
		if !strings.HasPrefix(path.Path, "/") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, "must be an absolute path"))
		}
		if len(path.Path) > 0 {
			for _, invalidSeq := range invalidPathSequences {
				if strings.Contains(path.Path, invalidSeq) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, fmt.Sprintf("must not contain '%s'", invalidSeq)))
				}
			}

			for _, invalidSuf := range invalidPathSuffixes {
				if strings.HasSuffix(path.Path, invalidSuf) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, fmt.Sprintf("cannot end with '%s'", invalidSuf)))
				}
			}
		}
	case networkingv1.PathTypeImplementationSpecific:
		if len(path.Path) > 0 {
			if !strings.HasPrefix(path.Path, "/") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, "must be an absolute path"))
			}
		}
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("pathType"), *path.PathType, supportedPathTypes.List()))
	}

	allErrs = append(allErrs, ValidateIngressBackend(path.Backend, fldPath.Child("backend"))...)
	return allErrs
}

// ValidateEndpointIP is used to validate Endpoints and EndpointSlice addresses, and also
// (for historical reasons) external IPs. It disallows certain address types that don't
// make sense in those contexts. Note that this function is _almost_, but not exactly,
// equivalent to net.IP.IsGlobalUnicast(). (Unlike IsGlobalUnicast, it allows global
// multicast IPs, which is probably a bug.)
//
// This function should not be used for new validations; the exact set of IPs that do and
// don't make sense in a particular field is context-dependent (e.g., localhost makes
// sense in some places; unspecified IPs make sense in fields that are used as bind
// addresses rather than destination addresses).
func ValidateEndpointIP(ipAddress string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	ip := netutils.ParseIPSloppy(ipAddress)
	if ip == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ipAddress, "must be a valid IP address"))
		return allErrs
	}
	if ip.IsUnspecified() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipAddress, fmt.Sprintf("may not be unspecified (%v)", ipAddress)))
	}
	if ip.IsLoopback() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipAddress, "may not be in the loopback range (127.0.0.0/8, ::1/128)"))
	}
	if ip.IsLinkLocalUnicast() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipAddress, "may not be in the link-local range (169.254.0.0/16, fe80::/10)"))
	}
	if ip.IsLinkLocalMulticast() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipAddress, "may not be in the link-local multicast range (224.0.0.0/24, ff02::/10)"))
	}
	return allErrs
}
