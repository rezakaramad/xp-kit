# Changelog

## [0.0.3](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.2...functions/xtenant-render/v0.0.3) (2026-06-14)


### Bug Fixes

* **ci:** escape Dockerfile shell variables with $$ to prevent build-arg expansion ([7a81ea0](https://github.com/rezakaramad/crossplane-toolkit/commit/7a81ea04ff5af5fa515a36554892364ba22a8e1e))
* **ci:** pass netrc build secret for private Go module auth ([2dcf917](https://github.com/rezakaramad/crossplane-toolkit/commit/2dcf9178c69cf2eb75752aaf39733237426e3f18))
* **ci:** use git config --global insteadOf for private module auth ([c588969](https://github.com/rezakaramad/crossplane-toolkit/commit/c5889690dfc8f0f6105d2bbfa0f5dd9fe6a874fb))
* **ci:** use GIT_CONFIG env vars to authenticate private Go modules in Docker build ([3a5074d](https://github.com/rezakaramad/crossplane-toolkit/commit/3a5074d1b2616631f02a969cc81a4d261d8ffb0f))
* **xtenant-render:** restore standard Dockerfile for public repo ([eecdb94](https://github.com/rezakaramad/crossplane-toolkit/commit/eecdb9484fb6bcd28a810ff7e13d3b6fa4e5447f))

## [0.0.2](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.1...functions/xtenant-render/v0.0.2) (2026-06-14)


### Bug Fixes

* update internal module versions to v0.0.1 ([a496173](https://github.com/rezakaramad/crossplane-toolkit/commit/a496173b74d880f2f8a65d751f464fb7f08931ac))

## 0.0.1 (2026-06-14)


### Features

* add xtenant-render and xtenant-validate functions ([75b7290](https://github.com/rezakaramad/crossplane-toolkit/commit/75b7290c268e401466a349592bbbd09c63e12227))
* initial release of crossplane-toolkit ([87c5351](https://github.com/rezakaramad/crossplane-toolkit/commit/87c5351145e0d5b14103f30764d6504d2a725050))

## [0.2.0](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.1.2...functions/xtenant-render/v0.2.0) (2026-06-14)


### Features

* add xdeployment function and types, migrate xtenant functions to typed providers ([1a6ebb2](https://github.com/rezakaramad/crossplane-toolkit/commit/1a6ebb2a5e0d515b0070c67d269cf47372be1b90))


### Bug Fixes

* **xtenant-render:** add lint comments and package docs for argocd stub and internal packages ([00d9ce5](https://github.com/rezakaramad/crossplane-toolkit/commit/00d9ce524cf261149bec2bbfae9764158bb90610))
* **xtenant-render:** add nolint explanation for gochecknoglobals ([4c104e7](https://github.com/rezakaramad/crossplane-toolkit/commit/4c104e7aad77256cae21b206a0ca98793190393a))
* **xtenant-render:** upgrade genproto to resolve ambiguous import after provider split ([8dacfa3](https://github.com/rezakaramad/crossplane-toolkit/commit/8dacfa39b3af0418291114d8469d91102fd1ce88))

## [0.1.2](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.1.1...functions/xtenant-render/v0.1.2) (2026-06-08)


### Bug Fixes

* **functions:** add go.sum entries for modules/nextinsight v0.1.2 ([2be8001](https://github.com/rezakaramad/crossplane-toolkit/commit/2be8001fc810c75d6c38aad79f33fdca98ad6838))
* **functions:** bump modules/nextinsight to v0.1.2 (fixed /groups/{id} endpoint) ([e56c496](https://github.com/rezakaramad/crossplane-toolkit/commit/e56c496c6455f52d324e6c4d04afb288012aaa71))

## [0.1.1](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.1.0...functions/xtenant-render/v0.1.1) (2026-06-07)


### Bug Fixes

* **functions/xtenant-render:** move passwordSecretRef to initProvider to stop reconcile loop ([6e94f9b](https://github.com/rezakaramad/crossplane-toolkit/commit/6e94f9b805221ce4f9a8caadb4e75f82cc8a5e26))

## [0.1.0](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.1...functions/xtenant-render/v0.1.0) (2026-05-30)


### Features

* **functions:** integrate Next-Insight tenant metadata enrichment and validation ([caa73db](https://github.com/rezakaramad/crossplane-toolkit/commit/caa73dbf138a03fcd52d5e3ed13e7e487fcd6e8d))


### Bug Fixes

* **functions:** bump nextinsight and xtenant deps to v0.1.1 ([41a31ab](https://github.com/rezakaramad/crossplane-toolkit/commit/41a31abfdf99e3212d24cbfc2598f1e9f224193c))

## [0.1.0](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.1...functions/xtenant-render/v0.1.0) (2026-05-29)


### Features

* **runner:** add ContextEnricher and StampMetadata for metadata enrichment ([09d9d2e](https://github.com/rezakaramad/crossplane-toolkit/commit/09d9d2e32c7fb492d0f1a09932eadc91a1f67574))
* **xtenant-render:** integrate Next-Insight metadata enrichment ([7189d8c](https://github.com/rezakaramad/crossplane-toolkit/commit/7189d8c839febfcad8f941a18e7a39461bffe880))

## [0.1.0](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.1...functions/xtenant-render/v0.1.0) (2026-05-29)


### Features

* **runner:** add ContextEnricher and StampMetadata for metadata enrichment ([09d9d2e](https://github.com/rezakaramad/crossplane-toolkit/commit/09d9d2e32c7fb492d0f1a09932eadc91a1f67574))
* **xtenant-render:** integrate Next-Insight metadata enrichment ([7189d8c](https://github.com/rezakaramad/crossplane-toolkit/commit/7189d8c839febfcad8f941a18e7a39461bffe880))

## [0.1.0](https://github.com/rezakaramad/crossplane-toolkit/compare/functions/xtenant-render/v0.0.1...functions/xtenant-render/v0.1.0) (2026-05-13)


### Features

* bootstrap clean history ([88c9ead](https://github.com/rezakaramad/crossplane-toolkit/commit/88c9eadf965df4af7455bb8eecd986eb6af4830b))

## 0.0.1 (2026-05-13)


### Features

* bootstrap clean history ([88c9ead](https://github.com/rezakaramad/crossplane-toolkit/commit/88c9eadf965df4af7455bb8eecd986eb6af4830b))

## Changelog
