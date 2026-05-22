# Changelog

## [1.0.2](https://github.com/amiwrpremium/macontrol/compare/v1.0.1...v1.0.2) (2026-05-22)


### Bug Fixes

* **ci:** repair Renovate semantic-commit type + per-PR labeler concurrency ([a5c92bf](https://github.com/amiwrpremium/macontrol/commit/a5c92bf347495a7f04af22c9436fdf1692b09fd7))
* **deps:** update module github.com/go-telegram/bot to v1.21.0 ([#7](https://github.com/amiwrpremium/macontrol/issues/7)) ([e7423e3](https://github.com/amiwrpremium/macontrol/commit/e7423e32fcb99c7f4f1ef3964f0ccf27567b816d))

## [1.0.1](https://github.com/amiwrpremium/macontrol/compare/v1.0.0...v1.0.1) (2026-05-21)


### Bug Fixes

* **release:** use cosign --bundle for cosign v3+ compatibility ([772f7b7](https://github.com/amiwrpremium/macontrol/commit/772f7b7306036bccbaa11e75cfbb2ab80c232714))
* **security:** drop redundant nolint:gosec comment ([0fb8aab](https://github.com/amiwrpremium/macontrol/commit/0fb8aab15b1fd4bbf639e4c9d4678f02f7f76e6e))
* **security:** silence gosec G204/G304 false-positives ([2a849d3](https://github.com/amiwrpremium/macontrol/commit/2a849d3bd5933b2316098e67bf68a05260428d6f))

## 1.0.0 (2026-05-21)


### Features

* **apps:** add domain Service ([935c5a4](https://github.com/amiwrpremium/macontrol/commit/935c5a44f7f269324d6afb0231c649e4c168305c))
* **battery:** add domain Service ([05bc6ad](https://github.com/amiwrpremium/macontrol/commit/05bc6ad2cd60144a8830e33846b9e6c782a8afe9))
* **bluetooth:** add domain Service ([a6ea6dd](https://github.com/amiwrpremium/macontrol/commit/a6ea6ddc27747b7be99396f64b91da6ac2554341))
* **bot:** add Deps + dispatcher + Whitelist ([7b862fd](https://github.com/amiwrpremium/macontrol/commit/7b862fdf1589a1b35c3280eff267e52a1ee3b7c9))
* **bot:** add httptest-based fake Telegram server ([6c0ea66](https://github.com/amiwrpremium/macontrol/commit/6c0ea6609e1eb9f67e1dc5e5b9046475a9194ff8))
* **callbacks:** add callback protocol + ShortMap ([94772e3](https://github.com/amiwrpremium/macontrol/commit/94772e326cb5be844ceff40ddb92bdd71a5d6066))
* **capability:** add macOS feature-detection report ([2896573](https://github.com/amiwrpremium/macontrol/commit/2896573d3b6e00b1512b1e5b46a0abee42e284fc))
* **config:** add Keychain-backed runtime config loader ([f237eef](https://github.com/amiwrpremium/macontrol/commit/f237eef63905945cbc3f35d28fc83e2a23e05c56))
* **daemon:** add macontrol CLI entrypoint and main ([281e454](https://github.com/amiwrpremium/macontrol/commit/281e4541d5113b75b7c7c42f1fc4bca2537b16f2))
* **display:** add domain Service ([5a8f160](https://github.com/amiwrpremium/macontrol/commit/5a8f160df5114ee43d188704078959728627bf90))
* **doctor:** add doctor + service subcommands ([dfbc9ef](https://github.com/amiwrpremium/macontrol/commit/dfbc9ef4a21efc499de0e56b0094f1fa0a691254))
* **flows:** add multi-step flow registry ([19e24c9](https://github.com/amiwrpremium/macontrol/commit/19e24c91276f5b5bd969857d398e0addf752dbd8))
* **flows:** add per-domain flow implementations ([8df013a](https://github.com/amiwrpremium/macontrol/commit/8df013a772c17128185de919a05f81c4093518bd))
* **handlers:** add callback router + per-domain handlers + nav ([1501048](https://github.com/amiwrpremium/macontrol/commit/1501048cce53e3bb64cf326a363ae3faab00ba8a))
* **keyboards:** add common keyboards + per-domain keyboards ([99ce1dc](https://github.com/amiwrpremium/macontrol/commit/99ce1dc766db4940c992eab87c33791d2aa13bf9))
* **keyboards:** add home grid ([97df1c7](https://github.com/amiwrpremium/macontrol/commit/97df1c74b27b0287ae8f6d29083bd23c1f5546a8))
* **keychain:** add macOS Keychain wrapper ([9174634](https://github.com/amiwrpremium/macontrol/commit/9174634b52ee756c711adbadc1c567f045cc42bb))
* **media:** add domain Service ([9f5cd20](https://github.com/amiwrpremium/macontrol/commit/9f5cd2014bd3f615053016e457859f0b7dea3261))
* **music:** add domain Service ([562a6ce](https://github.com/amiwrpremium/macontrol/commit/562a6ce5c5bed53039dd199fb4672cea4c2cc399))
* **music:** add per-chat live-refresh manager ([e659210](https://github.com/amiwrpremium/macontrol/commit/e6592104e8fd0f5d070acef1612badd08d9932b4))
* **notify:** add domain Service ([6d6731d](https://github.com/amiwrpremium/macontrol/commit/6d6731d64864686ad34b957d0846e5fab15de98f))
* **power:** add domain Service ([627ffbb](https://github.com/amiwrpremium/macontrol/commit/627ffbb08bb74b969419899f8156add90b8788e3))
* **runner:** add subprocess runner abstraction ([0600ca2](https://github.com/amiwrpremium/macontrol/commit/0600ca25755151ca9803698ab1cb4d768b8f7dea))
* **setup:** add interactive first-run wizard + token/whitelist subcommands ([46b824d](https://github.com/amiwrpremium/macontrol/commit/46b824d17e50495ab31cb3c021792093b73a8bfb))
* **sound:** add domain Service ([01e57e2](https://github.com/amiwrpremium/macontrol/commit/01e57e2b66c7a1219ddff49541e9ce737a0a8e7c))
* **status:** add aggregator Service ([35e9a8b](https://github.com/amiwrpremium/macontrol/commit/35e9a8b3333f7f7b8965dcabf546e3d6c5680027))
* **system:** add domain Service ([b2023f0](https://github.com/amiwrpremium/macontrol/commit/b2023f0acac176fd2d015c277117f40994706a78))
* **tools:** add domain Service ([bd272b4](https://github.com/amiwrpremium/macontrol/commit/bd272b42c7029f486ece6a28fd89974c757ae6c4))
* **version:** add version info package ([3c6e5aa](https://github.com/amiwrpremium/macontrol/commit/3c6e5aa272e1bd86ad674b7204e0881b73a57623))
* **wifi:** add domain Service ([1a30899](https://github.com/amiwrpremium/macontrol/commit/1a30899d8d70d89eda0a91e972e4f6d3fd6eb5b8))
