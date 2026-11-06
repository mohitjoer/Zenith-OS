package main

import (
	"testing"
)

func TestCatalogRevisionStableAndSensitive(t *testing.T) {
	a := Catalog{Items: []Item{
		{Category: "rices", ID: "nord", Version: "1.0"},
		{Category: "plugins", ID: "market", Version: "2.0"},
	}}
	b := Catalog{Items: []Item{
		{Category: "plugins", ID: "market", Version: "2.0"},
		{Category: "rices", ID: "nord", Version: "1.0"},
	}}
	if catalogRevision(a) != catalogRevision(b) {
		t.Fatal("revision must be order-independent")
	}
	c := Catalog{Items: []Item{
		{Category: "rices", ID: "nord", Version: "1.1"},
		{Category: "plugins", ID: "market", Version: "2.0"},
	}}
	if catalogRevision(a) == catalogRevision(c) {
		t.Fatal("revision must change on a version bump")
	}
	d := Catalog{Items: []Item{
		{Category: "rices", ID: "nord", Version: "1.0", Upstream: "https://example.com/nord"},
		{Category: "plugins", ID: "market", Version: "2.0"},
	}}
	if catalogRevision(a) == catalogRevision(d) {
		t.Fatal("revision must change when upstream provenance appears")
	}
}
