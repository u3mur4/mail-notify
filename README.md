## mail-notify

# How to use it?

1. Install mail-notify
```
go get -u ...
```

1. Create a new project and enable gmail api https://console.cloud.google.com/projectcreate?

2. Configure an external app in the OAuth consent screen with Test users

3. Create a new OAuth 2.0 Client IDs with Authorized redirect URIs set to http://localhost:14000.

exec: firefox --new-tab https://mail.google.com/mail/u/0
