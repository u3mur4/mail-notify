# mail-notify

Monitor email accounts for new messages with desktop notifications and status bar integration.

## Features

- IMAP email monitoring with real-time notifications
- OAuth2 authentication support (Gmail)
- Status bar integrations: waybar, polybar, i3blocks
- i3lock plugin: display unread count on lock screen
- Desktop notifications with sound

## Installation

```bash
make build
```

## Setup

1. Create a Google Cloud project and enable the Gmail API
2. Configure OAuth consent screen with test users
3. Create OAuth 2.0 Client ID with redirect URI: `http://localhost:14000`
4. Import client secret: `mail-notify import-client-secret <path-to-secret.json>`
5. Add account: `mail-notify add`

## Usage

```bash
# Watch for new emails (default waybar format)
mail-notify watch <account>

# Watch with i3blocks format
mail-notify watch <account> --format i3blocks

# Watch with polybar format
mail-notify watch <account> --format polybar

# List accounts
mail-notify ls

# Remove account
mail-notify rm <account>
```

## Configuration

Accounts are stored in `~/.config/mail-notify/config.yml`:

```yaml
accounts:
  - provider: gmail
    username: your-email@gmail.com
    exec: firefox --new-tab https://mail.google.com/mail/u/0
```
