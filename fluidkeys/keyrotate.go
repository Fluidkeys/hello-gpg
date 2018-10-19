package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluidkeys/fluidkeys/archiver"
	"github.com/fluidkeys/fluidkeys/backupzip"
	"github.com/fluidkeys/fluidkeys/colour"
	"github.com/fluidkeys/fluidkeys/fingerprint"
	"github.com/fluidkeys/fluidkeys/humanize"
	"github.com/fluidkeys/fluidkeys/pgpkey"
	"github.com/fluidkeys/fluidkeys/scheduler"
	"github.com/fluidkeys/fluidkeys/status"
)

func keyRotate(dryRun bool, automatic bool, cronOutput bool) exitCode {
	keys, err := loadPgpKeys()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n")
	if dryRun {
		return runKeyRotateDryRun(keys)
	} else {
		var yesNoPrompter promptYesNoInterface
		var passwordPrompter promptForPasswordInterface

		if !automatic {
			yesNoPrompter = &interactiveYesNoPrompter{}
			passwordPrompter = &interactivePasswordPrompter{}
		} else {
			yesNoPrompter = &automaticResponder{}
			passwordPrompter = &alwaysFailPasswordPrompter{}
		}

		var outputter outputterInterface
		if cronOutput {
			outputter = &cronOutputter{}
		} else {
			outputter = &terminalOutputter{}
		}
		return runKeyRotate(keys, yesNoPrompter, passwordPrompter, outputter)
	}
}

var (
	nothingToDo        string = colour.Success("✔ All keys look good — nothing to do.\n")
	reviewTheseActions string = "Fluidkeys will perform the following actions.\n\n" +
		colour.Warning("Take time to review these actions.") + "\n\n"
)

func runKeyRotateDryRun(keys []pgpkey.PgpKey) exitCode {
	outputter := &terminalOutputter{}
	keyTasks := makeKeyTasks(keys)

	if len(keyTasks) == 0 {
		outputter.print(nothingToDo)
		return 0 // success! nothing to do
	}

	outputter.print(reviewTheseActions)

	for i := range keyTasks {
		var keyTask *keyTask = keyTasks[i]
		keyTask.actions = addImportExportActions(keyTask.actions, nil)
		printKeyWarningsAndActions(*keyTask, outputter)
	}

	outputter.print("To start run\n")
	outputter.print(" >   " + colour.CommandLineCode("fk key rotate") + "\n\n")
	outputter.print("You’ll be asked at each stage to confirm before making any changes.\n\n")
	return 0
}

type keyTask struct {
	key      *pgpkey.PgpKey
	warnings []status.KeyWarning
	actions  []status.KeyAction
	password string

	// err is used to record any errors encountered while running actions
	// if nil, no errors were encountered
	err error
}

const (
	promptBackupGpg           = "Automatically create backup now?"
	promptRunActions          = "     Run these actions?"
	promptRotateAutomatically = "Automatically rotate this key from now on?"
)

type promptYesNoInterface interface {
	promptYesNo(message string, defaultResponse string, key *pgpkey.PgpKey, outputter outputterInterface) bool
}

type interactiveYesNoPrompter struct{}

func (iP *interactiveYesNoPrompter) promptYesNo(message string, defaultInput string, key *pgpkey.PgpKey, outputter outputterInterface) bool {
	var options string
	switch strings.ToLower(defaultInput) {
	case "y":
		options = "[Y/n]"
	case "n":
		options = "[y/N]"
	default:
		options = "[y/n]"
	}
	messageWithOptions := message + " " + options + " "
	for {
		input := promptForInput(messageWithOptions)
		if input == "" {
			input = defaultInput
		}
		switch strings.ToLower(input) {
		case "y":
			return true
		case "n":
			return false
		default:
			outputter.print("Please select only Y or N.\n")
		}
	}
}

type automaticResponder struct{}

func (aR *automaticResponder) promptYesNo(message string, defaultResponse string, key *pgpkey.PgpKey, outputter outputterInterface) bool {
	switch message {

	case promptBackupGpg:
		return true

	case promptRunActions:
		if key == nil {
			panic("expected *key but got nil pointer")
		}
		return Config.ShouldStorePasswordForKey(key.Fingerprint()) &&
			Config.ShouldRotateAutomaticallyForKey(key.Fingerprint())

	case promptRotateAutomatically:
		panic("prompting to rotate key automatically, but it should be set and therefore not prompt")

	default:
		panic(fmt.Errorf("don't know how to automatically respond to : %s\n", message))
	}
}

// alwaysFailPasswordPrompter can be used for automatic running, where it's
// impossible to prompt for a password. If a password prompt is required
// (because we didn't get it from the keychain or config), it falls through to
// here, which fails.
type alwaysFailPasswordPrompter struct{}

// promptForPassword always returns an empty string
func (p *alwaysFailPasswordPrompter) promptForPassword(key *pgpkey.PgpKey) (string, error) {
	return "", fmt.Errorf("can't prompt for password when running unattended")
}

type outputterInterface interface {
	print(message string)
}

type terminalOutputter struct{}

func (o *terminalOutputter) print(message string) {
	fmt.Print(message)
}

func runKeyRotate(keys []pgpkey.PgpKey, prompter promptYesNoInterface, passwordPrompter promptForPasswordInterface, outputter outputterInterface) exitCode {
	keyTasks := makeKeyTasks(keys)

	if len(keyTasks) == 0 {
		outputter.print(nothingToDo)
		return 0 // success! nothing to do
	}

	outputter.print(reviewTheseActions)

	promptAndBackupGnupg(prompter, outputter)

	for i := range keyTasks {
		var keyTask *keyTask = keyTasks[i]
		keyTask.actions = addImportExportActions(keyTask.actions, passwordPrompter)
	}

	for i := range keyTasks {
		var keyTask *keyTask = keyTasks[i]

		printKeyWarningsAndActions(*keyTask, outputter)
		ranActionsSuccesfully := promptAndRunActions(prompter, keyTask, outputter)

		if ranActionsSuccesfully && !Config.ShouldRotateAutomaticallyForKey(keyTask.key.Fingerprint()) {
			promptAndTurnOnRotateAutomatically(prompter, *keyTask, outputter)
		}
	}

	if anyTasksHaveErrors(keyTasks) {
		outputter.print(colour.Error("Encountered errors while running rotate:\n"))

		for _, keyTask := range keyTasks {
			if keyTask.err != nil {
				outputter.print(displayName(keyTask.key) + ": " + colour.Error(keyTask.err.Error()) + "\n")
			}
		}
		return 1
	} else {
		outputter.print(colour.Success("Rotate complete") + "\n")
		return 0
	}
}

func addImportExportActions(actions []status.KeyAction, passwordPrompter promptForPasswordInterface) []status.KeyAction {
	actions = prepend(actions, LoadPrivateKeyFromGnupg{passwordGetter: passwordPrompter})
	actions = append(actions, PushIntoGnupg{})
	actions = append(actions, UpdateBackupZIP{})
	return actions
}

func prepend(actions []status.KeyAction, actionToPrepend status.KeyAction) []status.KeyAction {
	return append([]status.KeyAction{actionToPrepend}, actions...)
}

func anyTasksHaveErrors(keyTasks []*keyTask) bool {
	for _, keyTask := range keyTasks {
		if keyTask.err != nil {
			return true
		}
	}

	return false
}

func makeKeyTasks(keys []pgpkey.PgpKey) []*keyTask {
	var keyTasks []*keyTask

	for i := range keys {
		key := &keys[i] // get a pointer here, not in the `for` expression
		warnings := status.GetKeyWarnings(*key)
		actions := status.MakeActionsFromWarnings(warnings, time.Now())

		if len(actions) > 0 {
			keyTask := keyTask{
				key:      key,
				warnings: warnings,
				actions:  actions,
			}
			keyTasks = append(keyTasks, &keyTask)
		}
	}
	return keyTasks
}

func printKeyWarningsAndActions(keyTask keyTask, outputter outputterInterface) {
	outputter.print(formatKeyWarningsAndActions(keyTask))
}

func promptAndRunActions(prompter promptYesNoInterface, keyTask *keyTask, outputter outputterInterface) (ranActionsSuccessfully bool) {
	if prompter.promptYesNo(promptRunActions, "y", keyTask.key, outputter) == false {
		outputter.print(colour.Disabled(" ▸   OK, skipped.\n\n"))
		ranActionsSuccessfully = false
		return
	}

	if err := runActions(keyTask, outputter); err != nil {
		keyTask.err = err
		outputter.print("\n")
		outputter.print("     " + colour.Warning("Skipping remaining actions for") + " " + displayName(keyTask.key) + "\n\n")
		ranActionsSuccessfully = false
		return
	} else {
		outputter.print(colour.Success(" ▸   Successfully updated keys for " + displayName(keyTask.key) + "\n\n"))
		ranActionsSuccessfully = true
		return
	}
}

func promptAndTurnOnRotateAutomatically(prompter promptYesNoInterface, keyTask keyTask, outputter outputterInterface) {

	fmt.Print("Fluidkeys can configure a " + colour.CommandLineCode("cron") +
		" task to automatically rotate this key for you from now on ♻️\n")
	fmt.Print("To do this requires storing the key's password in your operating system's keyring.\n\n")

	if prompter.promptYesNo(promptRotateAutomatically, "", keyTask.key, outputter) == true {
		if err := tryEnableRotateAutomatically(keyTask.key, keyTask.password); err == nil {
			fmt.Print(colour.Success(" ▸   Successfully configured key to automatically rotate\n\n"))
		} else {
			fmt.Print(colour.Warning(" ▸   Failed to configure key to automatically rotate\n\n"))
		}
	} else {
		fmt.Print(colour.Disabled(" ▸   OK, skipped.\n\n"))
	}
}

func runActions(keyTask *keyTask, outputter outputterInterface) error {
	for _, action := range keyTask.actions {
		printCheckboxPending(action.String(), outputter)

		var err error
		err = action.Enact(keyTask.key, time.Now(), &keyTask.password)
		if err != nil {
			printCheckboxFailure(action.String(), err, outputter)
			return err // don't run any more actions

		} else {
			printCheckboxSuccess(action.String(), outputter)
		}
	}
	outputter.print("\n")
	return nil
}

func promptAndBackupGnupg(prompter promptYesNoInterface, outputter outputterInterface) {
	outputter.print("While fluidkeys is in alpha, it backs up GnuPG (~/.gnupg) each time.\n")

	action := "Backup GnuPG directory (~/.gnupg)"

	if prompter.promptYesNo(promptBackupGpg, "y", nil, outputter) == true {
		printCheckboxPending(action, outputter)
		filename, err := makeGnupgBackup()
		if err != nil {
			printCheckboxFailure(action, err, outputter)
			outputter.print("\n")
		} else {
			printCheckboxSuccess(fmt.Sprintf("GnuPG backed up to %v", filename), outputter)
			outputter.print("\n")
		}
	} else {
		printCheckboxSkipped(action, outputter)
	}
}

func makeGnupgBackup() (string, error) {
	directory := archiver.DateStampedDirectory(fluidkeysDirectory, time.Now())
	filepath := filepath.Join(directory, "gpghome.tgz")
	filename, err := gpg.BackupHomeDir(filepath, time.Now())
	return filename, err
}

func printCheckboxPending(actionText string, outputter outputterInterface) {
	outputter.print(fmt.Sprintf("     [.] %s\n", actionText))
	moveCursorUpLines(1, outputter)
}

func printCheckboxSuccess(actionText string, outputter outputterInterface) {
	outputter.print(fmt.Sprintf("     [%s] %s\n", colour.Success("✔"), actionText))
}

func printCheckboxSkipped(actionText string, outputter outputterInterface) {
	outputter.print(fmt.Sprintf("     [%s] %s\n", colour.Info("-"), actionText))
}

func printCheckboxFailure(actionText string, err error, outputter outputterInterface) {
	outputter.print(fmt.Sprintf("     %s %s\n", colour.Error("[!]"), actionText))
	outputter.print(fmt.Sprintf("         %s\n", colour.Error(fmt.Sprintf("%s", err))))
}

// formatKeyWarningsAndActions outputs a header for each key as follows:
//
// 2 issues for foo@example.com:
//
// ▸   Encryption subkey overdue for rotation, expires in 5 days
// ▸   Primary key set to expire too far in the future
//
//    [ ] Shorten the primary key expiry to 31 Oct 18
//    [ ] Expire the encryption subkey now (ID: 0xC52C5BD9719C9F00)
//    [ ] Create a new encryption subkey valid until 31 Oct 18
func formatKeyWarningsAndActions(keyTask keyTask) (header string) {
	if len(keyTask.actions) == 0 {
		return
	}

	header += humanize.Pluralize(len(keyTask.warnings), "warning", "warnings") + " for " +
		colour.Info(displayName(keyTask.key)) + "\n\n"

	for _, warning := range keyTask.warnings {
		header += fmt.Sprintf(" "+colour.Warning("▸")+"   %s\n", warning)
	}
	header += fmt.Sprintln()

	for _, action := range keyTask.actions {
		header += fmt.Sprintf("     [ ] %s\n", action)
	}
	header += "\n"

	return
}

func tryEnableRotateAutomatically(key *pgpkey.PgpKey, password string) (err error) {
	if err = Keyring.SavePassword(key.Fingerprint(), password); err != nil {
		return
	}

	if err = Config.SetStorePassword(key.Fingerprint(), true); err != nil {
		return
	}
	if err = Config.SetRotateAutomatically(key.Fingerprint(), true); err != nil {
		return
	}

	if _, err = scheduler.Enable(); err != nil {
		return
	}
	return nil
}

func moveCursorUpLines(numLines int, outputter outputterInterface) {
	for i := 0; i < numLines; i++ {
		outputter.print("\033[1A")
	}
}

type LoadPrivateKeyFromGnupg struct {
	passwordGetter promptForPasswordInterface
}

func (a LoadPrivateKeyFromGnupg) String() string {
	return "Load private key from GnuPG"
}

func (a LoadPrivateKeyFromGnupg) Enact(key *pgpkey.PgpKey, now time.Time, returnPassword *string) error {
	if returnPassword == nil {
		return fmt.Errorf("returnPassword was nil, but it's required")
	}

	if privateKey, password, err := getDecryptedPrivateKeyAndPassword(key, a.passwordGetter); err != nil {
		return err
	} else {
		// copy the value of privateKey to replace key
		*key = *privateKey
		*returnPassword = password
		return nil
	}

}

func (a LoadPrivateKeyFromGnupg) SortOrder() int {
	return 0 // unimportant since actions are already sorted
}

type PushIntoGnupg struct {
}

func (a PushIntoGnupg) String() string {
	return "Push key back into GnuPG"
}

type passwordMap = map[fingerprint.Fingerprint]string

func (a PushIntoGnupg) Enact(key *pgpkey.PgpKey, now time.Time, password *string) error {
	if password == nil {
		return fmt.Errorf("password was nil, but it's required")
	}

	return pushPrivateKeyBackToGpg(key, *password, &gpg)
}

func (a PushIntoGnupg) SortOrder() int {
	return 0 // unimportant since actions are already sorted
}

type UpdateBackupZIP struct {
}

func (a UpdateBackupZIP) String() string {
	return "Create full key backup ZIP file"
}

func (a UpdateBackupZIP) Enact(key *pgpkey.PgpKey, now time.Time, password *string) error {
	if password == nil {
		return fmt.Errorf("password was nil, but it's required")
	}
	_, err := backupzip.OutputZipBackupFile(fluidkeysDirectory, key, *password)
	return err
}

func (a UpdateBackupZIP) SortOrder() int {
	return 0 // unimportant since actions are already sorted
}
