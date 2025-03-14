package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/martinlindhe/notify"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var log = logrus.New()
var configPaths = []string{"/etc/mail-notify/", "$HOME/.config/mail-notify/", "$HOME/.mail-notify/", "$HOME/.i3/mail-notify"}

// getAccount loads an account identified by the name from the config file
func getAccount(name string) (Account, error) {
	var account []Account
	err := viper.UnmarshalKey("accounts."+name, &account)
	if err != nil {
		return Account{}, err
	}
	if len(account) == 0 {
		return Account{}, fmt.Errorf("account not found")
	}
	return account[0], nil
}

func setupConfig() {
	viper.SetConfigName("config")
	for _, configPath := range configPaths {
		viper.AddConfigPath(configPath)
	}
}

func getRootCmd() *cobra.Command {
	var accountName string
	var debugMode bool
	var format string
	var rootCmd = &cobra.Command{
		Use:   "mail-notify",
		Short: "monitoring mailbox events",
		Long:  `Continuously listen for mailbox events and inform the user of unseen email numbers when a new mailbox event detected.`,
		Example: `mail-notify --account account_name

Config paths: ` + strings.Join(configPaths, ", ") + `
Config name: config.yml

The config file format is the following:
accounts:
    account_name:
        - username: testUser
		  password: testPassword
		  # called when the user left clicks on the mail icon
		  exec: chromium --new-tab https://mail.google.com/mail/u/0 
    other_account:
		- username: otherUser
		  password: otherPassword
		  exec: chromium --new-tab https://mail.google.com/mail/u/1 

The i3blocks format is the following:
[mail-account_name]
command=mail-notify --account account_name
markup=pango
interval=persist
`,
		Run: func(cmd *cobra.Command, args []string) {
			format = strings.ToLower(format)
			log.SetOutput(ioutil.Discard)
			logPath := fmt.Sprintf("/tmp/%s.mail.log", accountName)
			f, err := os.Create(logPath)
			defer f.Close()
			if debugMode {
				if err != nil {
					fmt.Fprintf(os.Stderr, "cannot create log file: %v\n", err)
					return
				}
				log.SetOutput(os.Stderr)
				log.SetLevel(logrus.DebugLevel)
				// fmt.Fprintf(os.Stderr, "start logging to %s\n", logPath)
			}

			setupConfig()
			err = viper.ReadInConfig()
			if err != nil {
				log.WithError(err).Fatal("cannot read config file")
			}

			account, err := getAccount(accountName)
			if err != nil {
				log.WithError(err).Fatal("cannot parse account from config file")
			}

			c, err := NewMailClient(account)
			if err != nil {
				log.WithError(err).Fatal("cannot create mail client")
			}

			// update loop
			go func() {
				for unseen := range c.Update {
					switch format {
					case "polybar":
						fmt.Fprintln(os.Stdout, formatMailAsPolybar(unseen, account))
					case "i3blocks":
						fmt.Fprintln(os.Stdout, formatMailAsPango(unseen))
					}
					if unseen > 0 {
						notify.Notify("mail-notify", "New Mail", fmt.Sprintf("You have %d email on %s!", unseen, account.Username), "")
						playNotificationSound()
					}
				}
			}()

			// handle user input
			if format == "i3blocks" {
				go func() {
					for {
						var input string
						_, err := fmt.Scanln(&input)
						if err != nil {
							break
						}
						// left click
						if input == "1" && account.Exec != "" {
							defaultProgram, err := shellwords.Parse(account.Exec)
							log.Debug(strings.Join(defaultProgram, "|"))
							if err != nil {
								log.WithError(err).Debug("cannot parse the command")
								continue
							}
							cmd := exec.Command(defaultProgram[0], defaultProgram[1:]...)
							cmd.Run()
						}
					}
				}()
			}

			err = c.Listen()
			if err != nil {
				// empty out stdout before exit
				fmt.Println()
				log.WithError(err).Fatal("mail client stopped")
			}
		},
	}

	rootCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "Enable logging")
	rootCmd.Flags().StringVar(&format, "format", "polybar", "Use i3blocks or polybar format")
	rootCmd.Flags().StringVarP(&accountName, "account", "a", "", "Load account from config file")
	rootCmd.MarkFlagRequired("account")

	return rootCmd
}
