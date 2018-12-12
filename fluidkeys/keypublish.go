// Copyright 2018 Paul Furley and Ian Drysdale
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

package main

import (
	"fmt"

	"github.com/fluidkeys/fluidkeys/pgpkey"
)

func publishKeyToAPI(privateKey *pgpkey.PgpKey) error {
	armoredPublicKey, err := privateKey.Armor()
	if err != nil {
		return fmt.Errorf("Couldn't load armored key: %s\n", err)
	}
	if err = client.UpsertPublicKey(armoredPublicKey, privateKey); err != nil {
		return fmt.Errorf("Failed to upload public key: %s", err)

	}
	return nil
}
