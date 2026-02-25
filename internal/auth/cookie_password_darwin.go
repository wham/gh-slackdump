package auth

import (
	"errors"
	"fmt"

	"github.com/keybase/go-keychain"
)

func cookiePassword() ([]byte, error) {
	accountNames := []string{"Slack Key", "Slack", "Slack App Store Key"}
	var lastErr error
	for _, name := range accountNames {
		password, err := cookiePasswordFromKeychain(name)
		if err == nil {
			return password, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%w — make sure to allow access when prompted by the Keychain dialog", lastErr)
}

func cookiePasswordFromKeychain(accountName string) ([]byte, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService("Slack Safe Storage")
	query.SetAccount(accountName)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnAttributes(true)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, err
	}
	switch len(results) {
	case 0:
		return nil, errors.New("no matching keychain items found")
	case 1:
		return results[0].Data, nil
	default:
		return nil, errors.New("multiple keychain items found")
	}
}
