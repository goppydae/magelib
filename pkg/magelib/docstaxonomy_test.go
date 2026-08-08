// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import "testing"

// A TERM IS A DIRECTORY; AN OUTPUT FORMAT IS A FILE. The first version
// of this counter took any path under the taxonomy as a term, so hugo's
// own /categories/index.xml and /categories/index.print.html made an
// index with no terms look populated - and the gate passed on precisely
// the tree it was written to fail. These are the real strings from that
// render.
func TestCountTermLinksIgnoresOutputFormats(t *testing.T) {
	for _, tt := range []struct {
		name string
		html string
		seg  string
		want int
	}{
		{
			name: "an empty index that renders its output formats",
			html: `<a href="/categories/index.xml">RSS</a>` +
				`<a href="/categories/index.print.html">print</a>`,
			seg:  "categories",
			want: 0,
		},
		{
			name: "real terms",
			html: `<a href="/tags/build/">build</a><a href="/tags/docs/">docs</a>`,
			seg:  "tags",
			want: 2,
		},
		{
			name: "minified, unquoted attributes",
			html: `<a href=/tags/build/>build</a>`,
			seg:  "tags",
			want: 1,
		},
		{
			name: "the index linking to itself is not a term",
			html: `<a href="/tags/">all tags</a>`,
			seg:  "tags",
			want: 0,
		},
		{
			name: "a page belonging to a term is not another term",
			html: `<a href="/tags/build/page/2/">next</a>`,
			seg:  "tags",
			want: 0,
		},
		{
			name: "the same term twice counts once",
			html: `<a href="/tags/build/">a</a><a href="/tags/build/">b</a>`,
			seg:  "tags",
			want: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := countTermLinks([]byte(tt.html), tt.seg); got != tt.want {
				t.Errorf("countTermLinks = %d, want %d", got, tt.want)
			}
		})
	}
}

// ONLY INTERNAL LINKS ARE THE SITE'S TO RENDER. The gate asserts a site
// renders what it advertises, and it cannot render github.com - so an
// external entry must not be reported as an unresolvable target.
func TestAdvertisedLinksExcludesExternalEntries(t *testing.T) {
	c := hugoConfig{Menus: map[string][]struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}{
		"sidebar": {
			{Name: "Tags", URL: "/tags/"},
			{Name: "GitHub", URL: "https://github.com/goppydae/magelib"},
			{Name: "Nothing", URL: ""},
		},
	}}
	got := c.advertisedLinks()
	if len(got) != 1 || got[0].URL != "/tags" {
		t.Fatalf("advertisedLinks = %v, want only the internal /tags entry", got)
	}
}
