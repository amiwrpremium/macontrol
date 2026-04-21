# Changelog

## [0.1.2](https://github.com/amiwrpremium/macontrol/compare/v0.1.1...v0.1.2) (2026-04-21)


### Bug Fixes

* **bot:** switch to HTML parse mode to stop Telegram rejecting messages ([#15](https://github.com/amiwrpremium/macontrol/issues/15)) ([d604d30](https://github.com/amiwrpremium/macontrol/commit/d604d305f3a8179bc440825498e568d123120fb2))

## [0.1.1](https://github.com/amiwrpremium/macontrol/compare/v0.1.0...v0.1.1) (2026-04-21)


### Bug Fixes

* **release:** qualify smctemp dependency as narugit/tap/smctemp ([#13](https://github.com/amiwrpremium/macontrol/issues/13)) ([5966bae](https://github.com/amiwrpremium/macontrol/commit/5966bae0b6bfcc6d5556579bf268d7d8916e7840))

## 0.1.0 (2026-04-21)


### ⚠ BREAKING CHANGES

* **config:** drop .env; Keychain-only secrets, CLI-flag runtime ([#11](https://github.com/amiwrpremium/macontrol/issues/11))
* **setup:** store token + whitelist in macOS Keychain, drop the .env ([#10](https://github.com/amiwrpremium/macontrol/issues/10))

### Features

* add core infrastructure packages ([3d1a45f](https://github.com/amiwrpremium/macontrol/commit/3d1a45f9e3df4d7d2317f3c64a5e50ff3cdff479))
* add macontrol CLI with setup wizard and service management ([c7e04b0](https://github.com/amiwrpremium/macontrol/commit/c7e04b0c7dd385fb3f030c4beee8f280b8813e08))
* add macOS domain control packages ([fe1fec9](https://github.com/amiwrpremium/macontrol/commit/fe1fec9b0bf176252ceebeb30642c10628eb904d))
* **bot:** add Telegram menu, callbacks, flows, and handlers ([0580316](https://github.com/amiwrpremium/macontrol/commit/058031642bf445036aaa28785dcb49ec93a2bfa1))
* **brew:** pull companion formulae as hard deps so everything works out of the box ([#9](https://github.com/amiwrpremium/macontrol/issues/9)) ([d960c8d](https://github.com/amiwrpremium/macontrol/commit/d960c8d0ae6da9ae99305a6b2e74b85cf871175e))
* **release:** add release tooling, Makefile, and install artifacts ([db1d457](https://github.com/amiwrpremium/macontrol/commit/db1d457b2bd69dd41bc062f3a66fe8ac015dcde4))
* **setup:** store token + whitelist in macOS Keychain, drop the .env ([#10](https://github.com/amiwrpremium/macontrol/issues/10)) ([bf132d6](https://github.com/amiwrpremium/macontrol/commit/bf132d6a86afa43bb11cedac7a1b6d0a1a7d6ca8))


### Bug Fixes

* **ci:** resolve 50 golangci-lint v2 findings ([caa55d6](https://github.com/amiwrpremium/macontrol/commit/caa55d6c0464f87cb6563ad78662598ba4c2411d))
* **ci:** upgrade lint action and Go toolchain to pass checks ([34182c8](https://github.com/amiwrpremium/macontrol/commit/34182c819ff52aadbec64efc95701db05cd37b90))


### Refactors

* **config:** drop .env; Keychain-only secrets, CLI-flag runtime ([#11](https://github.com/amiwrpremium/macontrol/issues/11)) ([994983e](https://github.com/amiwrpremium/macontrol/commit/994983e81c186fb101e21d91a401dc3837524151))

## Changelog

All notable changes to this project are documented here. This file is managed
by [release-please](https://github.com/googleapis/release-please) — do not
edit it by hand. Versions follow [SemVer](https://semver.org) and commits
follow [Conventional Commits](https://www.conventionalcommits.org).
