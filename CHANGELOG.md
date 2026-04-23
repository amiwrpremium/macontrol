# Changelog

## [0.6.0](https://github.com/amiwrpremium/macontrol/compare/v0.5.0...v0.6.0) (2026-04-23)


### Features

* **bot:** collapse wi-fi dns presets into a drill-down submenu ([#70](https://github.com/amiwrpremium/macontrol/issues/70)) ([98646fb](https://github.com/amiwrpremium/macontrol/commit/98646fbb9718d2908866eb36929b5fd5d6ff4172))
* **bot:** tappable Timezone picker (region → city, paginated, with flags) ([#67](https://github.com/amiwrpremium/macontrol/issues/67)) ([01b2ae5](https://github.com/amiwrpremium/macontrol/commit/01b2ae549db0c0fb35ad76eaa6b8ac4064201e1f))

## [0.5.0](https://github.com/amiwrpremium/macontrol/compare/v0.4.0...v0.5.0) (2026-04-23)


### Features

* **bot:** tappable Shortcuts list with pagination + search ([#60](https://github.com/amiwrpremium/macontrol/issues/60)) ([a1a2be1](https://github.com/amiwrpremium/macontrol/commit/a1a2be1bc56b8812a066fbc1743aeb8be06ff820))

## [0.4.0](https://github.com/amiwrpremium/macontrol/compare/v0.3.0...v0.4.0) (2026-04-23)


### Features

* **bot:** redesign Disks panel — tighter filter + per-disk drill-down ([#56](https://github.com/amiwrpremium/macontrol/issues/56)) ([#57](https://github.com/amiwrpremium/macontrol/issues/57)) ([ef77b3c](https://github.com/amiwrpremium/macontrol/commit/ef77b3c96bbbce95f50c5570390c45c97306b4bb))

## [0.3.0](https://github.com/amiwrpremium/macontrol/compare/v0.2.4...v0.3.0) (2026-04-23)


### Features

* **bot:** tappable top-N processes in CPU and Memory panels ([#54](https://github.com/amiwrpremium/macontrol/issues/54)) ([debcef5](https://github.com/amiwrpremium/macontrol/commit/debcef5b05e09681847d9a6df51371a0172e25e3))

## [0.2.4](https://github.com/amiwrpremium/macontrol/compare/v0.2.3...v0.2.4) (2026-04-23)


### Bug Fixes

* **display:** capture brightness CLI's stderr too ([#52](https://github.com/amiwrpremium/macontrol/issues/52)) ([24b0eec](https://github.com/amiwrpremium/macontrol/commit/24b0eecc76083fcdb7d3cafee65706c7808fd544))

## [0.2.3](https://github.com/amiwrpremium/macontrol/compare/v0.2.2...v0.2.3) (2026-04-23)


### Bug Fixes

* **wifi:** extend Speedtest timeout to 60s (networkQuality needs &gt; 15s) ([#50](https://github.com/amiwrpremium/macontrol/issues/50)) ([5a3dbd1](https://github.com/amiwrpremium/macontrol/commit/5a3dbd18f8d2a8b923e50225ad214aa3c0b5cc23))

## [0.2.2](https://github.com/amiwrpremium/macontrol/compare/v0.2.1...v0.2.2) (2026-04-22)


### Bug Fixes

* **bot:** clear stale reply keyboard on /start and boot ping ([#47](https://github.com/amiwrpremium/macontrol/issues/47)) ([9aad8ab](https://github.com/amiwrpremium/macontrol/commit/9aad8ab271c5bea0f728d76508c9c55618c6f39f))

## [0.2.1](https://github.com/amiwrpremium/macontrol/compare/v0.2.0...v0.2.1) (2026-04-22)


### Bug Fixes

* **bot:** back button on every nested menu + PowerConfirm cancel fix ([#45](https://github.com/amiwrpremium/macontrol/issues/45)) ([5a8cac8](https://github.com/amiwrpremium/macontrol/commit/5a8cac8eddc6cf7d73af56ac3da9e9d3b6c9165a))

## [0.2.0](https://github.com/amiwrpremium/macontrol/compare/v0.1.12...v0.2.0) (2026-04-22)


### Features

* **bot:** tappable processes in Top 10 with Kill / Force Kill ([#43](https://github.com/amiwrpremium/macontrol/issues/43)) ([dd508c9](https://github.com/amiwrpremium/macontrol/commit/dd508c96bd97d0603458900a198d7643e01000dc))

## [0.1.12](https://github.com/amiwrpremium/macontrol/compare/v0.1.11...v0.1.12) (2026-04-22)


### Bug Fixes

* **system:** parse CPU panel, label fields, add top-3 CPU hogs ([#41](https://github.com/amiwrpremium/macontrol/issues/41)) ([6f69a2d](https://github.com/amiwrpremium/macontrol/commit/6f69a2dcb3ae0ecea14741652f2abaf19225dba8))

## [0.1.11](https://github.com/amiwrpremium/macontrol/compare/v0.1.10...v0.1.11) (2026-04-22)


### Bug Fixes

* **system:** parse memory, label fields, add top-3 RAM hogs ([#39](https://github.com/amiwrpremium/macontrol/issues/39)) ([ca890e6](https://github.com/amiwrpremium/macontrol/commit/ca890e6c99c4de8b1f40977e160b82ea2a04ea31))

## [0.1.10](https://github.com/amiwrpremium/macontrol/compare/v0.1.9...v0.1.10) (2026-04-22)


### Bug Fixes

* **system:** parse uptime, label fields, show load avg per core ([#37](https://github.com/amiwrpremium/macontrol/issues/37)) ([24dac8c](https://github.com/amiwrpremium/macontrol/commit/24dac8c5bafab0f05da04aba0bfc88111179586b))

## [0.1.9](https://github.com/amiwrpremium/macontrol/compare/v0.1.8...v0.1.9) (2026-04-22)


### Bug Fixes

* **bot:** drill-down panels stop reusing the parent dashboard keyboard ([#35](https://github.com/amiwrpremium/macontrol/issues/35)) ([8eb174f](https://github.com/amiwrpremium/macontrol/commit/8eb174fe7fdcc5fced48c0965b7815d028d71ef3))

## [0.1.8](https://github.com/amiwrpremium/macontrol/compare/v0.1.7...v0.1.8) (2026-04-22)


### Bug Fixes

* **display:** surface real brightness CLI error instead of misleading install hint ([#33](https://github.com/amiwrpremium/macontrol/issues/33)) ([e8a0136](https://github.com/amiwrpremium/macontrol/commit/e8a01363f02c2f62eefe9c422e37bebb38eae58a))

## [0.1.7](https://github.com/amiwrpremium/macontrol/compare/v0.1.6...v0.1.7) (2026-04-22)


### Bug Fixes

* **bot:** omit reply_markup when nil to avoid Telegram rejection ([#31](https://github.com/amiwrpremium/macontrol/issues/31)) ([58bd0f3](https://github.com/amiwrpremium/macontrol/commit/58bd0f3e9538db94288424c097bc58891c7cb66c))

## [0.1.6](https://github.com/amiwrpremium/macontrol/compare/v0.1.5...v0.1.6) (2026-04-22)


### Bug Fixes

* **wifi:** read SSID from wdutil info, not broken networksetup ([#29](https://github.com/amiwrpremium/macontrol/issues/29)) ([1ff29a2](https://github.com/amiwrpremium/macontrol/commit/1ff29a2ded103dce370544354b323964e393db58))

## [0.1.5](https://github.com/amiwrpremium/macontrol/compare/v0.1.4...v0.1.5) (2026-04-22)


### Bug Fixes

* **release:** switch smctemp dep from narugit/tap to same-tap mirror ([#26](https://github.com/amiwrpremium/macontrol/issues/26)) ([a8db083](https://github.com/amiwrpremium/macontrol/commit/a8db0835634903cff815635ce79ebc9467af5eda))

## [0.1.4](https://github.com/amiwrpremium/macontrol/compare/v0.1.3...v0.1.4) (2026-04-22)


### Refactors

* **bot:** drop reply keyboard, navigate via inline keyboards only ([#22](https://github.com/amiwrpremium/macontrol/issues/22)) ([aab94f5](https://github.com/amiwrpremium/macontrol/commit/aab94f54308b4bd9a85b33fc26e8443c8f35f37e))

## [0.1.3](https://github.com/amiwrpremium/macontrol/compare/v0.1.2...v0.1.3) (2026-04-21)


### Bug Fixes

* reply-keyboard routing + /lock on macOS 26 ([#17](https://github.com/amiwrpremium/macontrol/issues/17)) ([43d8e52](https://github.com/amiwrpremium/macontrol/commit/43d8e5204454fa4c99e8a15a9cb96464f6d91ecc))

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
