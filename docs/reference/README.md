# Reference

Lookup-style docs. Tables, definitions, exact field meanings.

## What's here

- **[CLI](cli.md)** — every `macontrol` subcommand and flag, with
  exit codes.
- **[Brew commands](brew.md)** — every `brew` / `brew services`
  command relevant to managing macontrol.
- **[Callback protocol](callback-protocol.md)** — `<ns>:<action>:<arg>`
  format, namespace constants, the shortmap.
- **[macOS CLI mapping](macos-cli-mapping.md)** — feature → backing
  command, the canonical table.
- **[Version gates](version-gates.md)** — which features need which
  macOS release; how detection works.

## When to use which

- "What does `macontrol service status` do?" → [CLI](cli.md)
- "How do I restart the daemon after an upgrade?" → [Brew commands](brew.md)
- "What's the format of `callback_data`?" → [Callback protocol](callback-protocol.md)
- "What command runs when I tap MUTE?" → [macOS CLI mapping](macos-cli-mapping.md)
- "Why is the speedtest button hidden on my Mac?" → [Version gates](version-gates.md)
