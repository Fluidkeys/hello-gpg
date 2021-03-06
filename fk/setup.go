// Copyright 2019 Paul Furley and Ian Drysdale
//
// This file is part of Fluidkeys Client which makes it simple to use OpenPGP.
//
// Fluidkeys Client is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Fluidkeys Client is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Fluidkeys Client.  If not, see <https://www.gnu.org/licenses/>.

package fk

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/fluidkeys/fluidkeys/colour"
	"github.com/fluidkeys/fluidkeys/out"
)

func setup(email string) exitCode {
	out.Print("\n")

	out.Print(colour.Greeting(paulAndIanGreeting) + "\n")
	out.Print("\n")

	out.Print("Fluidkeys makes it easy to send end-to-end encrypted secrets using PGP.\n")

	exitCode, pgpKey := keyCreate(email)
	if exitCode != 0 {
		return exitCode
	}

	encryptedSecret, err := encryptSecret(secretSquirrelMessage(), "", pgpKey)
	if err != nil {
		printFailed("Couldn't encrypt a test secret message:")
		out.Print("Error: " + err.Error() + "\n")
		return 1
	}

	email, err = pgpKey.Email()
	if err != nil {
		printFailed("Couldn't get email address for key:")
		out.Print("Error: " + err.Error() + "\n")
		return 1
	}

	err = api.CreateSecret(pgpKey.Fingerprint(), encryptedSecret)
	if err != nil {
		printFailed("Couldn't send a test secret to " + email)
		out.Print("Error: " + err.Error() + "\n")
		return 1
	}

	time.Sleep(3 * time.Second)

	out.Print("🛎️  You've got a new secret. Read it by running:\n\n")
	out.Print(colour.Cmd("fk secret receive") + "\n\n")

	return 0
}

func secretSquirrelMessage() (message string) {
	rand.Seed(time.Now().Unix())
	codeName := fmt.Sprintf("%s %s", adjectives[rand.Intn(len(adjectives))], nouns[rand.Intn(len(nouns))])

	message = "🐿️ This is Secret Squirrel calling " + strings.Title(codeName) + "\n"
	message = message + `   Do you copy?
   Let me know by sending me a response:
   squirrel@fluidkeys.com`
	return message
}

const (
	paulAndIanGreeting = `👋  Hello and welcome to Fluidkeys!

    We're trying to make the world more safe and secure
    by simplifying powerful PGP encryption tools.
    
    We'd love to hear what you make of this version.
    You can always reach us at hello@fluidkeys.com
    
    Paul & Ian, Fluidkeys`
)

var (
	adjectives = []string{
		"dusty", "past", "amazing", "agreeable", "faded", "solid", "true", "wistful", "dear",
		"didactic", "spiky", "interesting", "jagged", "obedient", "amused", "furry", "rapid",
		"infamous", "succinct", "ethereal", "sable", "fantastic", "perpetual", "puzzled",
		"sneaky", "familiar", "inquisitive", "fine", "halting", "useful", "salty", "bright",
		"zesty", "gleaming", "graceful", "satisfying", "magnificent",
	}
	nouns = []string{
		"brick", "guitar", "monster", "notebook", "thunderstorm", "snowflake", "vineyard",
		"bacon", "canteen", "engineer", "fly", "raven", "bicycle", "crow", "eyelash", "bowtie",
		"ankle", "glove", "champion", "rose", "tin", "shirt", "wall", "stick", "holiday", "earth",
		"eye", "road", "cake", "sink", "brass", "sun", "stage", "table", "brake", "chair", "moon",
	}
)
