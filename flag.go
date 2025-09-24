package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/fogleman/gg"
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

func sendSignalToProcess(name string) error {
	// Find the PID of the process with the given name
	pids, err := findPIDByName(name)
	if err != nil {
		return err
	}

	// Send the SIGUSR1 signal to the process
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		err = process.Signal(syscall.SIGUSR1)
		if err != nil {
			return err
		}
	}

	return nil
}

func findPIDByName(name string) (pids []int, err error) {
	// Run the pidof command to find the PID of the process with the given name
	cmd := exec.Command("pidof", name)
	output, err := cmd.Output()
	if err != nil {
		return pids, fmt.Errorf("error running pidof: %v", err)
	}

	// Parse the output of the command to find the PID
	nums := strings.Split(string(output), " ")
	for _, num := range nums {
		pid, err := strconv.Atoi(strings.TrimSpace(num))
		if err != nil {
			// fmt.Println(fmt.Errorf("error parsing PID: %v", err))
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

func generateImage(account string, unseen uint32, x, y int) {
	dc := gg.NewContext(80, 50)
	dc.SetRGBA(1, 1, 1, 0)
	dc.Clear()
	dc.LoadFontFace("/usr/share/fonts/TTF/Noto-Sans-Regular-Nerd-Font-Complete.ttf", 50)
	mailIcon := ""
	dc.SetRGB(1, 1, 1)
	dc.DrawStringAnchored(mailIcon, 5, 3, 0, 1)
	w, h := dc.MeasureString(mailIcon)

	if unseen > 0 {
		dc.SetRGB(1, 0, 0)
		dc.DrawCircle(w+10, h/2, h/2)
		dc.Fill()

		dc.SetRGB(1, 1, 1)
		dc.LoadFontFace("/usr/share/fonts/TTF/Noto-Sans-Regular-Nerd-Font-Complete.ttf", 30)
		dc.DrawStringAnchored(fmt.Sprintf("%d", unseen), w+10, h/2, 0.5, 0.5)
	}

	os.Mkdir("/tmp/i3lock", 0755)
	dc.SavePNG(fmt.Sprintf("/tmp/i3lock/%s-pos:%d-%d.png", account, x, y))
}

func getRootCmd() *cobra.Command {
	var accountName string
	var debugMode bool
	var format string
	var i3lockPlugin int
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
					if i3lockPlugin > 0 {
						generateImage(accountName, unseen, i3lockPlugin, 50)
						sendSignalToProcess("i3lock")
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
	rootCmd.Flags().IntVar(&i3lockPlugin, "i3lock-plugin", 0, "Generate png image for i3lock plugin and set Y value")
	rootCmd.MarkFlagRequired("account")

	return rootCmd
}
