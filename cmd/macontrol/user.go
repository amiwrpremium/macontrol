package main

import (
	"os"
	"os/user"
)

// currentUser returns the Unix username for the process owner.
// Used as the macOS Keychain `account` argument and as the
// templated user in [sudoersBody].
//
// Behavior:
//   - First tries [user.Current], which on macOS reads
//     /etc/passwd via getpwuid (libc).
//   - Falls back to the $USER environment variable when
//     user.Current fails — typical in stripped containers
//     without /etc/passwd entries.
//   - Returns "" if both lookups fail. Callers downstream
//     (e.g. [keychain.Client.Set]) detect the empty case and
//     surface "config: loader has no keychain client or
//     account."
//
// Note: there's a peer copy of this function at
// internal/config.currentUser with the same semantics. They
// can't share an implementation without an import cycle (the
// cmd package imports config but config can't import cmd).
// Keep the two in sync by hand.
func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}
