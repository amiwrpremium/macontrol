package main

import "testing"

func TestCurrentUser_NotEmpty(t *testing.T) {
	// currentUser falls back to $USER when user.Current fails — in CI it
	// might be "root" or "runner", but it should never be empty.
	if got := currentUser(); got == "" {
		t.Fatal("currentUser returned empty string")
	}
}
