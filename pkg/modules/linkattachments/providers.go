// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Added by mia·nube on 2026-08-03.
//
// Package linkattachments holds the registry of external systems a task
// attachment may reference instead of storing.
//
// It is deliberately provider-agnostic: which systems exist, what they are
// called, where their file picker lives and how a reference turns back into a
// download are all configuration. Nothing about any particular external system
// is compiled in, and Vikunja never talks to a provider itself — it only hands
// the user's browser a URL and lets the provider apply its own access rules to
// the user making the request.
package linkattachments

import (
	"net/url"
	"strings"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/log"
)

// RefPlaceholder is what a provider's resolveurl must contain: the spot where
// the reference is substituted. A resolveurl without it addresses the same
// object for every attachment, which is never what was meant, so such a
// provider is rejected at load time rather than silently serving the wrong file.
const RefPlaceholder = "{ref}"

// LinkProvider describes one external system that can hold linked files.
//
// It is NOT called `Provider`, and must not be renamed to it. huma keys its
// OpenAPI schema registry by a type's BASE NAME, ignoring the package path, so a
// second `Provider` reaching the API surface collides with `openid.Provider` and
// **panics at startup** while the v2 routes are registered:
//
//	panic: duplicate name: Provider, new type: linkattachments.Provider,
//	       existing type: openid.Provider
//
// Nothing before deployment catches it — the build, `go vet` and the binary's own
// `version` command are all clean, because that registry is built only when the
// router is.
type LinkProvider struct {
	// Key is the identifier stored in a link attachment's link_provider column.
	// It comes from the configuration map key.
	Key string `json:"key" doc:"The identifier stored on a link attachment, used to look this provider up again."`
	// Name is what the user sees, e.g. in the "attach from …" action.
	Name string `json:"name" doc:"The provider's display name, shown in the UI."`
	// PickerURL is the page the client embeds to let the user choose a file. It
	// is never fetched by the server.
	PickerURL string `json:"picker_url" doc:"The file-picker page the client embeds to select a file from this provider."`
	// ResolveURL is the template a reference is resolved through, containing
	// {ref}. It is never fetched by the server either: the user's browser is
	// redirected to it, so the provider sees the user's own request and applies
	// that user's permissions rather than Vikunja's.
	ResolveURL string `json:"-"`
}

// resolveOne substitutes ref into the provider's ResolveURL.
//
// The reference is path-escaped. It is opaque, provider-owned data that reaches
// us over the API, so it is treated as untrusted input: without escaping, a
// reference containing a slash or a query separator could steer the redirect
// somewhere other than the object it names.
func (p *LinkProvider) resolveOne(ref string) string {
	return strings.ReplaceAll(p.ResolveURL, RefPlaceholder, url.PathEscape(ref))
}

// ResolveURLFor returns the absolute URL the given reference resolves to, and
// whether the provider is configured at all. An unknown provider yields false:
// a link created while a provider existed must not resolve to nothing-in-
// particular after it is removed from the configuration.
func ResolveURLFor(providerKey, ref string) (string, bool) {
	p, has := Get(providerKey)
	if !has {
		return "", false
	}
	return p.resolveOne(ref), true
}

// Get returns one configured provider by key.
func Get(key string) (*LinkProvider, bool) {
	for _, p := range GetAll() {
		if p.Key == key {
			return p, true
		}
	}
	return nil, false
}

// GetAll returns every configured provider, in no particular order.
//
// Configuration that cannot be used is dropped with a log line rather than
// silently accepted: a provider whose resolveurl is missing, relative or has no
// {ref} placeholder cannot produce a working download, and one that is absent
// from this list simply never appears in the UI. Failing quietly at attach time
// — or worse, at download time, months later — is the outcome this avoids.
func GetAll() []*LinkProvider {
	raw := normalizeProviderConfig(config.LinkAttachmentsProviders.Get())

	providers := make([]*LinkProvider, 0, len(raw))
	for key, cfg := range raw {
		p := &LinkProvider{
			Key:        key,
			Name:       stringValue(cfg["name"]),
			PickerURL:  stringValue(cfg["pickerurl"]),
			ResolveURL: stringValue(cfg["resolveurl"]),
		}
		if p.Name == "" {
			p.Name = key
		}
		if !validResolveURL(p.ResolveURL) {
			log.Errorf("Link attachment provider %q has an unusable resolveurl and is disabled: it must be an absolute http(s) URL containing %s", key, RefPlaceholder)
			continue
		}
		providers = append(providers, p)
	}
	return providers
}

func validResolveURL(raw string) bool {
	if !strings.Contains(raw, RefPlaceholder) {
		return false
	}
	parsed, err := url.Parse(strings.ReplaceAll(raw, RefPlaceholder, "ref"))
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func stringValue(v interface{}) string {
	s, is := v.(string)
	if !is {
		return ""
	}
	return strings.TrimSpace(s)
}

// normalizeProviderConfig flattens the configured provider map.
//
// The same value arrives as map[string]interface{} from JSON and env-backed
// config and as map[interface{}]interface{} from YAML, so both shapes are
// handled — the openid provider loader in this codebase does exactly the same
// for the same reason.
func normalizeProviderConfig(rawProviders interface{}) map[string]map[string]interface{} {
	if rawProviders == nil {
		return nil
	}

	outer, is := rawProviders.(map[string]interface{})
	if !is {
		asInterfaceMap, ok := rawProviders.(map[interface{}]interface{})
		if !ok {
			log.Criticalf("The linkattachments.providers configuration is in the wrong format; expected a map of provider key to settings.")
			return nil
		}
		outer = make(map[string]interface{}, len(asInterfaceMap))
		for k, v := range asInterfaceMap {
			if key, keyOK := k.(string); keyOK {
				outer[key] = v
			}
		}
	}

	configs := make(map[string]map[string]interface{}, len(outer))
	for key, p := range outer {
		inner, is := p.(map[string]interface{})
		if !is {
			asInterfaceMap, ok := p.(map[interface{}]interface{})
			if !ok {
				log.Errorf("Link attachment provider %q has an invalid configuration format and is ignored.", key)
				continue
			}
			inner = make(map[string]interface{}, len(asInterfaceMap))
			for i, v := range asInterfaceMap {
				if k, keyOK := i.(string); keyOK {
					inner[k] = v
				}
			}
		}
		configs[key] = inner
	}
	return configs
}
