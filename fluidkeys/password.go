package main

import (
	"fmt"

	"github.com/fluidkeys/fluidkeys/pgpkey"
	"golang.org/x/crypto/ssh/terminal"
)

// getDecryptedPrivateKeyAndPassword prompts the user for a password, tests it
// and if successful, returns a decrypted private key and the password they
// provided.
// If the password is incorrect, it loops until they get it right.
func getDecryptedPrivateKeyAndPassword(publicKey *pgpkey.PgpKey, prompter promptForPasswordInterface) (*pgpkey.PgpKey, string, error) {
	shouldStore := Config.ShouldStorePasswordForKey(publicKey.Fingerprint())

	if shouldStore {
		if loadedPassword, gotPassword := Keyring.LoadPassword(publicKey.Fingerprint()); gotPassword == true {
			return tryPassword(loadedPassword, publicKey, prompter, shouldStore, 0)
		} // else fall-through to prompting
	} else {
		Keyring.PurgePassword(publicKey.Fingerprint())
	}

	return tryPassword(prompter.promptForPassword(publicKey), publicKey, prompter, shouldStore, 0)
}

func tryPassword(password string, publicKey *pgpkey.PgpKey, prompter promptForPasswordInterface, shouldStore bool, attempt int) (*pgpkey.PgpKey, string, error) {
	if privateKey, err := loadPrivateKey(publicKey.Fingerprint(), password, &gpg, &pgpkey.Loader{}); err == nil {
		if shouldStore {
			Keyring.SavePassword(publicKey.Fingerprint(), password)
		}
		return privateKey, password, nil

	} else if isBadPasswordError(err) {
		fmt.Printf("Password appeared to be incorrect.\n")

		if attempt < 5 {
			return tryPassword(prompter.promptForPassword(publicKey), publicKey, prompter, shouldStore, attempt+1)
		} else {
			return nil, "", fmt.Errorf("Giving up.")
		}
	} else {
		// different type of error
		return nil, "", fmt.Errorf("error loading private key: %v", err)
	}
}

func isBadPasswordError(err error) bool {
	switch err {
	case err.(*IncorrectPassword):
		return true
	}
	return false
}

type promptForPasswordInterface interface {
	promptForPassword(key *pgpkey.PgpKey) string
}

type interactivePasswordPrompter struct{}

// promptForPassword asks the user for a password and returns the result
func (p *interactivePasswordPrompter) promptForPassword(key *pgpkey.PgpKey) string {
	fmt.Printf("Enter password for %s: ", displayName(key))
	password, err := terminal.ReadPassword(0)
	if err != nil {
		panic(fmt.Sprintf("Error reading password: %v\n", err))
	} else {
		fmt.Print("\n\n")
	}
	return string(password)
}
