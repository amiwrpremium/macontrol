# Reference

Lookup-style docs. Tables, definitions, exact field meanings.

## What's here

- **[CLI](cli.md)** — every subcommand and flag, with exit codes.
- **[Callback protocol](callback-protocol.md)** — `<ns>:<action>:<arg>`
  format, namespace constants, the shortmap.
- **[macOS CLI mapping](macos-cli-mapping.md)** — feature → backing
  command, the canonical table.
- **[Version gates](version-gates.md)** — which features need which
  macOS release; how detection works.

## When to use which

- "What does `macontrol service status` do?" → [CLI](cli.md)
- "What's the format of `callback_data`?" → [Callback protocol](callback-protocol.md)
- "What command runs when I tap MUTE?" → [macOS CLI mapping](macos-cli-mapping.md)
- "Why is the speedtest button hidden on my Mac?" → [Version gates](version-gates.md)
