# VaultixIMQ

![GitHub Release](https://img.shields.io/github/v/release/remmody/VaultixIMQ)
![GitHub Downloads](https://img.shields.io/github/downloads/remmody/VaultixIMQ/total)

Secure, modern IMAP mail client designed for managing large numbers of connected mailboxes.
 It provides a secure environment for email synchronization and authentication management.

## Key Features

- High-density mailbox management: Efficient handling of multiple simultaneous IMAP connections.
- Secure Storage: All local data, including credentials and message headers, is protected using AES-256-GCM encryption.
- Authentication Management: Integrated TOTP (Time-based One-Time Password) generator.
- Privacy First: Local-only storage with no third-party cloud dependencies.
- Modern Interface: Responsive glassmorphic UI built for desktop performance.
- System Integration: Native platform notifications and background synchronization.

## Technology Stack

- Backend: Go with Wails framework.
- Frontend: HTML, Vanilla CSS, Javascript (Vite/Alpine.js).
- Security: AES-256-GCM encryption.
- Protocol: Standard IMAP over TLS.

## Requirements

- Go 1.21+
- Node.js & NPM
- Wails CLI

## Development

To run in development mode:
```bash
wails dev
```

To build a production package:
```bash
wails build
```
