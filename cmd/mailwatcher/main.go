package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/martinlindhe/notify"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/u3mur4/mail-notify/pkg/formatter"
	"github.com/u3mur4/mail-notify/pkg/i3lock"
	"github.com/u3mur4/mail-notify/pkg/mailwatch"
)

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	var debug string = ""

	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	var rootCmd = &cobra.Command{
		Use:   "mailwatch",
		Short: "monitoring mailbox events",
		Long:  `Continuously listen for mailbox events and inform the user of unseen email numbers when a new mailbox event detected.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug != "" {
				logger := mailwatch.GetLogger()
				level, err := logrus.ParseLevel(debug)
				if err != nil {
					level = logrus.DebugLevel
				}
				logger.SetLevel(level)
				log = logger
			}
		},
	}
	rootCmd.PersistentFlags().StringVarP(&debug, "debug", "d", "", "set debug level")

	// Watch command
	var format string
	var waybarExec bool
	var i3lockPlugin int
	var imapDebug bool
	var watchCmd = &cobra.Command{
		Use:   "watch [account]",
		Short: "Start watching an email account for new messages",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			account, err := mailwatch.GetAccount(args[0])
			exitIfErr(err)

			if waybarExec {
				err := mailwatch.CheckAuth(&account)
				if err == nil {
					cmd := exec.Command("bash", "-c", account.Exec)
					cmd.Run()
				} else {
					urlBytes, readErr := os.ReadFile(mailwatch.AuthURLPath(account))
					if readErr == nil {
						exec.Command("xdg-open", strings.TrimSpace(string(urlBytes))).Run()
					} else {
						_, authURL, tokenChan, err := mailwatch.StartAuthFlow(&account)
						if err != nil {
							log.WithError(err).Error("cannot start auth flow")
						} else {
							exec.Command("xdg-open", authURL).Run()
							token := <-tokenChan
							if token == nil {
								log.Error("authentication failed")
							}
						}
					}
				}
				return
			}

			needsAuth, authURL, tokenChan, err := mailwatch.StartAuthFlow(&account)
			if err != nil {
				log.WithError(err).Error("authentication pre-check failed")
			}
			if needsAuth {
				os.WriteFile(mailwatch.AuthURLPath(account), []byte(authURL), 0644)

				authEvent := mailwatch.UpdateEvent{Status: mailwatch.StatusNeedsAuth}
				switch format {
				case "waybar":
					fmt.Fprintln(os.Stdout, formatter.Waybar(authEvent))
				case "i3blocks":
					fmt.Fprintln(os.Stdout, formatter.Pango(authEvent))
				case "polybar":
					fmt.Fprintln(os.Stdout, formatter.Polybar(authEvent, account.Exec))
				}

				token := <-tokenChan
				if token == nil {
					exitIfErr(fmt.Errorf("authentication failed"))
				}

				os.Remove(mailwatch.AuthURLPath(account))
			}

			client, err := mailwatch.NewMailClient(account)
			exitIfErr(err)

			if imapDebug {
				client.SetDebug(os.Stderr)
			}

			// update loop
			go func() {
				for event := range client.Update {
					switch format {
					case "polybar":
						fmt.Fprintln(os.Stdout, formatter.Polybar(event, account.Exec))
					case "i3blocks":
						fmt.Fprintln(os.Stdout, formatter.Pango(event))
					case "waybar":
						fmt.Fprintln(os.Stdout, formatter.Waybar(event))
					}
					if event.Status == mailwatch.StatusConnected && event.Unseen > 0 {
						notify.Notify("mail-notify", "New Mail", fmt.Sprintf("You have %d email on %s!", event.Unseen, account.Username), "")
						playNotification()
					}
					if i3lockPlugin > 0 {
						i3lock.GenerateImage(account.Username, event.Unseen, i3lockPlugin, 50, true)
					}
				}
				fmt.Println("update loop stopped")
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

			err = client.Listen()
			if err != nil {
				exitIfErr(err)
			}

		},
	}
	watchCmd.Flags().StringVar(&format, "format", "waybar", "Set format to i3blocks, polybar or waybar format")
	watchCmd.Flags().BoolVar(&waybarExec, "waybar-exec", false, "Run profile exec command and exit")
	watchCmd.Flags().IntVar(&i3lockPlugin, "i3lock-plugin", 0, "Generate png image for i3lock plugin and set Y value")
	watchCmd.Flags().BoolVar(&imapDebug, "imap-debug", false, "Enable imap debug")

	// Add command
	var addCmd = &cobra.Command{
		Use:   "add",
		Short: "Add a new email account",
		Run: func(cmd *cobra.Command, args []string) {
			_, err := mailwatch.NewAccountFromCli()
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	// Secret command
	var importClientSecretCmd = &cobra.Command{
		Use:   "import-client-secret [secret-path]",
		Short: "Import client secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := mailwatch.ImportClientSecret(args[0])
			exitIfErr(err)
		},
	}

	// Remove command
	var rmCmd = &cobra.Command{
		Use:   "rm [account]",
		Short: "Remove an email account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := mailwatch.RemoveAccount(args[0])
			exitIfErr(err)
		},
	}

	// List command
	var lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all email accounts",
		Run: func(cmd *cobra.Command, args []string) {
			for _, account := range mailwatch.GetAccounts() {
				fmt.Println(account.Username)
			}
		},
	}

	// Adding the commands to the root command
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(importClientSecretCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
		os.Exit(1)
	}

}
