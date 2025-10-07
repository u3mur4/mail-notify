package mailwatch

import (
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v3"
)

func configDir() string {
	cacheDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	return path.Join(cacheDir, "mail-notify")
}

type Account struct {
	Proivider string `yaml:"provider"`
	Username  string `yaml:"username"`
	Exec      string `yaml:"exec"`
}

func (a *Account) TokenFilePath() string {
	return path.Join(configDir(), fmt.Sprintf("%s_token.json", a.Username))
}

func GetAccounts() []Account {
	data, err := os.ReadFile(path.Join(configDir(), "config.yml"))
	if err != nil {
		// log.WithError(err).Debug("cannot read config file")
		return []Account{}
	}

	yamlAccounts := struct {
		Accounts []Account `yaml:"accounts"`
	}{}

	err = yaml.Unmarshal(data, &yamlAccounts)
	if err != nil {
		log.WithError(err).Error("cannot unmarshal config file")
		return []Account{}
	}

	return yamlAccounts.Accounts
}

func GetAccount(username string) (Account, error) {
	for _, account := range GetAccounts() {
		if account.Username == username {
			return account, nil
		}
	}
	return Account{}, fmt.Errorf("account '%s' not found", username)
}

func RemoveAccount(username string) error {
	account, err := GetAccount(username)
	if err != nil {
		return err
	}

	err = os.Remove(account.TokenFilePath())
	if err != nil {
		log.WithError(err).Errorf("cannot remove token file '%s'", account.TokenFilePath())
	}

	accounts := slices.DeleteFunc(GetAccounts(), func(a Account) bool {
		return a.Username == account.Username
	})

	return saveAccounts(accounts)
}

func saveAccount(account Account) error {
	accounts := GetAccounts()

	found := false
	for index, a := range accounts {
		if a.Username == account.Username {
			accounts[index] = account
			found = true
			break
		}
	}

	if !found {
		accounts = append(accounts, account)
	}

	return saveAccounts(accounts)
}

func saveAccounts(accounts []Account) error {
	yamlAccounts := struct {
		Accounts []Account `yaml:"accounts"`
	}{}
	yamlAccounts.Accounts = accounts

	data, err := yaml.Marshal(yamlAccounts)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join(configDir(), "config.yml"), data, 0644)
}

func NewAccountFromCli() (Account, error) {
	account := Account{}

	providerSelect := promptui.Select{
		Label: "Select Provider",
		Items: []string{"Gmail"},
	}

	_, provider, err := providerSelect.Run()

	if err != nil {
		return Account{}, err
	}
	account.Proivider = provider

	usernamePrompt := promptui.Prompt{
		Label: "Enter Username",
		Validate: func(input string) error {
			if input == "" {
				return fmt.Errorf("username cannot be empty")
			}
			return nil
		},
	}

	username, err := usernamePrompt.Run()

	if err != nil {
		return Account{}, err
	}
	account.Username = username

	execPrompt := promptui.Prompt{
		Label: "Enter executable path",
	}

	execCmd, err := execPrompt.Run()

	if err != nil {
		return Account{}, err
	}
	account.Exec = execCmd

	gmailToken := newOAuth2GmailToken(&account)
	token := gmailToken.Token()
	if token == nil {
		return Account{}, fmt.Errorf("failed to get auth2 token")
	}

	err = saveAccount(account)
	if err != nil {
		return Account{}, err
	}

	return account, nil
}
