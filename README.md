## mail-notify

# How to use it?

1. Install mail-notify
```
go get -u ...
```
2. Create a config file for mail-notify. Possible config paths: /etc/mail-notify, $HOME/.config/mail-notify, etc. Use `--help` flag to see all.
```
cat <<EOF >> $HOME/.config/mail-notify/config.yml
accounts:
    account_name:
        - username: testUser
		  password: testPassword
		  # called when the user clicks the mail icon
		  exec: chromium --new-tab https://mail.google.com/mail/u/0 
    other_account:
		- username: otherUser
		  password: otherPassword
		  exec: chromium --new-tab https://mail.google.com/mail/u/1 
EOF
```
3. Add a new block to i3blocks config
```bash
[mail-account_name]
command=mail-notify --account account_name
markup=pango
interval=persist
```
4. Restart i3 to apply your changes.
```
i3-msg restart
```