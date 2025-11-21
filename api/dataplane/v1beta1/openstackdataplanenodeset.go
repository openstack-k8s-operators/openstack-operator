/*
Copyright 2023.

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

package v1beta1

import (
	"regexp"
)

// NodeHostNameIsFQDN Helper to check if a hostname is fqdn
func NodeHostNameIsFQDN(hostname string) bool {
	// Regular expression to match a valid FQDN
	// This regex assumes that the hostname and domain name segments only contain letters, digits, hyphens, and periods.
	regex := `^([a-zA-Z0-9-]+\.)*[a-zA-Z0-9-]+\.[a-zA-Z]{2,}$`

	match, _ := regexp.MatchString(regex, hostname)
	return match
}
