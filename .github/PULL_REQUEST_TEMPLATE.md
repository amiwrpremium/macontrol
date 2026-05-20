<!--
Title must follow Conventional Commits, e.g.:
  feat(sound): add inline keyboard for volume ± and mute toggle
  fix(wifi): handle missing en0 on Macs without built-in Wi-Fi
Scopes: sound, display, power, wifi, bt, battery, system, media, notify,
        tools, bot, flows, keyboards, callbacks, runner, config, ci
-->

## Summary

<!-- 1-3 bullets describing the change and *why* it's needed. -->

-
-

## Screenshots / GIFs

<!-- For UI-visible changes (new keyboards, new flows), include a short
     Telegram screen recording or screenshot. -->

## Test plan

- [ ] `make lint test` passes locally
- [ ] New/changed code has unit tests
- [ ] Verified on macOS (version / chip: … )
- [ ] If permissions changed: updated `docs/permissions.md`
- [ ] If new capability: added to README feature table

## Checklist

- [ ] PR title follows Conventional Commits
- [ ] No secrets, tokens, or personal user IDs in diff
- [ ] Breaking changes documented in the PR body + marked with `!`
