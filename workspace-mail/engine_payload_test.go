//go:build !sharedengine

package main

import "testing"

func TestPortablePrivateEngineDefaults(t *testing.T) {
	t.Setenv("WORKSPACE_MAIL_BACKEND", "")
	if !privateEngineDefault() {
		t.Fatal("single-executable build requires private-engine default")
	}
	t.Setenv("WORKSPACE_MAIL_BACKEND", "shared")
	if privateEngineDefault() {
		t.Fatal("explicit shared mode ignored")
	}
	t.Setenv("WORKSPACE_MAIL_BACKEND", "private")
	if !privateEngineDefault() {
		t.Fatal("installed private backend ignored")
	}
}
