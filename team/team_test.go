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

package team

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluidkeys/fluidkeys/assert"
	"github.com/fluidkeys/fluidkeys/exampledata"
	fpr "github.com/fluidkeys/fluidkeys/fingerprint"
	"github.com/fluidkeys/fluidkeys/pgpkey"
	"github.com/fluidkeys/fluidkeys/testhelpers"
	"github.com/gofrs/uuid"
)

func TestLoadTeams(t *testing.T) {
	person1 := Person{
		Email:       "test2@example.com",
		Fingerprint: exampledata.ExampleFingerprint2,
		IsAdmin:     false,
	}

	person2 := Person{
		Email:       "test3@example.com",
		Fingerprint: exampledata.ExampleFingerprint3,
		IsAdmin:     true, // <-- admin
	}

	team1 := Team{
		Name:   "Team 1",
		UUID:   uuid.Must(uuid.NewV4()),
		People: []Person{person1, person2},
	}

	team2 := Team{
		Name:   "Team 2",
		UUID:   uuid.Must(uuid.NewV4()),
		People: []Person{person1, person2},
	}

	fluidkeysDir := testhelpers.Maketemp(t)

	saveTeam(t, &team1, fluidkeysDir)
	saveTeam(t, &team2, fluidkeysDir)

	gotTeams, err := LoadTeams(fluidkeysDir)
	assert.NoError(t, err)

	team1Roster, err := team1.PreviewRoster()
	assert.NoError(t, err)

	team2Roster, err := team2.PreviewRoster()
	assert.NoError(t, err)

	expected := []Team{
		{
			Name:      team1.Name,
			UUID:      team1.UUID,
			People:    team1.People,
			roster:    team1Roster, // roster and signature get added
			signature: "fake signature",
		},
		{
			Name:      team2.Name,
			UUID:      team2.UUID,
			People:    team2.People,
			roster:    team2Roster, // roster and signature get added
			signature: "fake signature",
		},
	}

	assert.Equal(t, expected, gotTeams)

}

func TestLoad(t *testing.T) {
	roster := `# Fluidkeys CIC team roster. Everyone in the team has a copy of this file.
#
# It is used to look up which key to use for an email address and fetch keys
# automatically.
uuid = "38be2a70-23d8-11e9-bafd-7f97f2e239a3"
name = "Fluidkeys CIC"

[[person]]
email = "paul@fluidkeys.com"
fingerprint = "B79F 0840 DEF1 2EBB A72F  F72D 7327 A44C 2157 A758"
is_admin = true

[[person]]
email = "ian@fluidkeys.com"
fingerprint = "E63A F0E7 4EB5 DE3F B72D  C981 C991 7093 18EC BDE7"
is_admin = false

[[person]]
email = "ray@fluidkeys.com"
fingerprint = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
# missing is_admin
`

	key, err := pgpkey.LoadFromArmoredEncryptedPrivateKey(
		exampledata.ExamplePrivateKey4, "test4",
	)
	assert.NoError(t, err)

	signature, err := key.MakeArmoredDetachedSignature([]byte(roster))

	team, err := Load(roster, signature)

	assert.NoError(t, err)
	expectedPeople := []Person{
		{
			Email:       "paul@fluidkeys.com",
			Fingerprint: fpr.MustParse("B79F0840DEF12EBBA72FF72D7327A44C2157A758"),
			IsAdmin:     true,
		},
		{
			Email:       "ian@fluidkeys.com",
			Fingerprint: fpr.MustParse("E63AF0E74EB5DE3FB72DC981C991709318ECBDE7"),
			IsAdmin:     false,
		},
		{
			Email:       "ray@fluidkeys.com",
			Fingerprint: fpr.MustParse("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
			IsAdmin:     false,
		},
	}
	assert.Equal(t, expectedPeople, team.People)

	assert.Equal(t, uuid.Must(uuid.FromString("38be2a70-23d8-11e9-bafd-7f97f2e239a3")), team.UUID)
	assert.Equal(t, "Fluidkeys CIC", team.Name)
	assert.Equal(t, roster, team.roster)
	assert.Equal(t, signature, team.signature)
}

func TestFindTeamSubdirectories(t *testing.T) {

	tmpdir := testhelpers.Maketemp(t)

	good := filepath.Join(tmpdir, "good")
	empty := filepath.Join(tmpdir, "empty")
	missingRoster := filepath.Join(tmpdir, "missing-roster")
	missingSignature := filepath.Join(tmpdir, "missing-signature")

	makeEmptyFile := func(t *testing.T, filename string) {
		assert.NoError(t, ioutil.WriteFile(filename, []byte{}, 0600))
	}

	assert.NoError(t, os.Mkdir(good, 0700))
	makeEmptyFile(t, filepath.Join(good, "roster.toml"))
	makeEmptyFile(t, filepath.Join(good, "roster.toml.asc"))

	assert.NoError(t, os.Mkdir(empty, 0700))

	assert.NoError(t, os.Mkdir(missingRoster, 0700))
	makeEmptyFile(t, filepath.Join(missingRoster, "roster.toml.asc"))

	assert.NoError(t, os.Mkdir(missingSignature, 0700))
	makeEmptyFile(t, filepath.Join(missingSignature, "roster.toml"))

	t.Run("returns subdirectory with roster.toml and roster.toml.asc", func(t *testing.T) {
		got, err := findTeamSubdirectories(tmpdir)
		assert.NoError(t, err)

		assert.Equal(t, []string{good}, got)
	})

}

func TestRoster(t *testing.T) {
	t.Run("function simply returns content of roster and signature fields", func(t *testing.T) {
		testTeam := Team{
			roster:    "fake roster",
			signature: "fake signature",
		}

		gotRoster, gotSig := testTeam.Roster()
		assert.Equal(t, testTeam.roster, gotRoster)
		assert.Equal(t, testTeam.signature, gotSig)
	})
}

func TestAdmins(t *testing.T) {
	person1 := Person{
		Email:       "test2@example.com",
		Fingerprint: exampledata.ExampleFingerprint2,
		IsAdmin:     false,
	}
	person2 := Person{
		Email:       "test3@example.com",
		Fingerprint: exampledata.ExampleFingerprint3,
		IsAdmin:     true, // <-- admin
	}
	person3 := Person{
		Email:       "test4@example.com",
		Fingerprint: exampledata.ExampleFingerprint4,
		IsAdmin:     true, // <-- admin
	}
	team := Team{
		Name:   "Kiffix",
		UUID:   uuid.Must(uuid.FromString("74bb40b4-3510-11e9-968e-53c38df634be")),
		People: []Person{person1, person2, person3},
	}

	assert.Equal(t, []Person{person2, person3}, team.Admins())
}

func TestUpdateRoster(t *testing.T) {
	signingKey, err := pgpkey.LoadFromArmoredEncryptedPrivateKey(
		exampledata.ExamplePrivateKey2, "test2")
	assert.NoError(t, err)

	validTeam := Team{
		Name: "Kiffix",
		UUID: uuid.Must(uuid.FromString("74bb40b4-3510-11e9-968e-53c38df634be")),
		People: []Person{
			{
				Email:       "test@example.com",
				Fingerprint: signingKey.Fingerprint(),
				IsAdmin:     true,
			},
		},
		roster:    "",
		signature: "",
	}

	t.Run("for a valid team", func(t *testing.T) {
		expectedRoster := `# Kiffix team roster. Everyone in the team has a copy of this file.
#
# It is used to look up which key to use for an email address and fetch keys
# automatically.
uuid = "74bb40b4-3510-11e9-968e-53c38df634be"
version = 0
name = "Kiffix"

[[person]]
  email = "test@example.com"
  fingerprint = "5C78E71F6FEFB55829654CC5343CC240D350C30C"
  is_admin = true
`

		err := validTeam.UpdateRoster(signingKey)
		assert.NoError(t, err)

		t.Run("sets team.roster", func(t *testing.T) {
			assert.Equal(t, expectedRoster, validTeam.roster)
		})

		t.Run("sets a valid signature", func(t *testing.T) {
			err := VerifyRoster(
				validTeam.roster, validTeam.signature, []*pgpkey.PgpKey{signingKey},
			)

			assert.NoError(t, err)
		})
	})

	t.Run("returns an error for invalid team", func(t *testing.T) {
		invalidTeam := Team{
			Name: "Missing UUID",
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				},
			},
		}

		err := invalidTeam.UpdateRoster(signingKey)
		assert.Equal(t, fmt.Errorf("invalid team: invalid roster: invalid UUID"), err)
	})

	t.Run("returns an error if signing key isn't an admin", func(t *testing.T) {
		notAdminKey, err := pgpkey.LoadFromArmoredEncryptedPrivateKey(
			exampledata.ExamplePrivateKey3, "test3")
		assert.NoError(t, err)

		err = validTeam.UpdateRoster(notAdminKey)
		assert.Equal(t,
			fmt.Errorf(
				"can't sign with key 7C18 DE4D E478 1356 8B24  3AC8 719B D63E F03B DC20 "+
					"that's not an admin of the team"),
			err,
		)
	})
}

func TestValidate(t *testing.T) {
	t.Run("with valid roster, returns no error", func(t *testing.T) {
		team := Team{
			Name: "Kiffix",
			UUID: uuid.Must(uuid.NewV4()),
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
					IsAdmin:     true,
				},
			},
		}

		err := team.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing UUID", func(t *testing.T) {
		team := Team{
			Name: "Kiffix",
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				},
			},
		}

		err := team.Validate()
		assert.Equal(t, fmt.Errorf("invalid roster: invalid UUID"), err)
	})

	t.Run("with duplicated email address", func(t *testing.T) {
		team := Team{
			Name: "Kiffix",
			UUID: uuid.Must(uuid.NewV4()),
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				},
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
				},
			},
		}

		err := team.Validate()
		assert.Equal(t, fmt.Errorf("email listed more than once: test@example.com"), err)
	})

	t.Run("with duplicated fingerprint", func(t *testing.T) {
		team := Team{
			Name: "Kiffix",
			UUID: uuid.Must(uuid.NewV4()),
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				},
				{
					Email:       "another@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				},
			},
		}

		err := team.Validate()
		assert.Equal(t, fmt.Errorf("fingerprint listed more than once: "+
			"AAAA BBBB AAAA BBBB AAAA  AAAA BBBB AAAA BBBB AAAA"), err)
	})

	t.Run("with no members", func(t *testing.T) {
		team := Team{
			Name:   "Kiffix",
			UUID:   uuid.Must(uuid.NewV4()),
			People: []Person{},
		}

		err := team.Validate()
		assert.Equal(t, fmt.Errorf("team has no members"), err)
	})

	t.Run("with no admins", func(t *testing.T) {
		team := Team{
			Name: "Kiffix",
			UUID: uuid.Must(uuid.NewV4()),
			People: []Person{
				{
					Email:       "test@example.com",
					Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
					IsAdmin:     false,
				},
				{
					Email:       "another@example.com",
					Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
					IsAdmin:     false,
				},
			},
		}

		err := team.Validate()
		assert.Equal(t, fmt.Errorf("team has no administrators"), err)
	})
}

func TestVerifyRoster(t *testing.T) {
	key, err := pgpkey.LoadFromArmoredEncryptedPrivateKey(
		exampledata.ExamplePrivateKey4, "test4",
	)
	assert.NoError(t, err)

	roster := "hello"

	goodSignature, err := key.MakeArmoredDetachedSignature([]byte(roster))
	assert.NoError(t, err)

	t.Run("verifies a good signature", func(t *testing.T) {
		err := VerifyRoster(roster, goodSignature, []*pgpkey.PgpKey{key})
		assert.NoError(t, err)
	})

	t.Run("returns an error for a bad signature", func(t *testing.T) {
		err := VerifyRoster(roster+"tampered", goodSignature, []*pgpkey.PgpKey{key})
		assert.GotError(t, err)
		assert.Equal(t, "openpgp: invalid signature: hash tag doesn't match", err.Error())
	})

	t.Run("rejects empty signature", func(t *testing.T) {
		err := VerifyRoster(roster, "", []*pgpkey.PgpKey{key})
		assert.GotError(t, err)
		assert.Equal(t, fmt.Errorf("empty signature"), err)
	})

}

func TestIsAdmin(t *testing.T) {
	adminPerson := Person{
		Email:       "admin@example.com",
		Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
		IsAdmin:     true,
	}
	normalPerson := Person{
		Email:       "normal@example.com",
		Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
		IsAdmin:     false,
	}

	team := Team{
		Name:   "Kiffix",
		UUID:   uuid.Must(uuid.NewV4()),
		People: []Person{adminPerson, normalPerson},
	}

	t.Run("IsAdmin returns true for admin person", func(t *testing.T) {
		got := team.IsAdmin(adminPerson.Fingerprint)

		assert.Equal(t, true, got)
	})

	t.Run("IsAdmin returns false for normal person", func(t *testing.T) {
		got := team.IsAdmin(normalPerson.Fingerprint)

		assert.Equal(t, false, got)
	})
}

func TestTeamContains(t *testing.T) {
	pesonInTeam := Person{
		Email:       "admin@example.com",
		Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
	}
	personNotInTeam := Person{
		Email:       "normal@example.com",
		Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
	}

	team := Team{
		Name:   "Kiffix",
		UUID:   uuid.Must(uuid.NewV4()),
		People: []Person{pesonInTeam},
	}

	t.Run("team.Contains returns true for person in the team", func(t *testing.T) {
		got := team.Contains(pesonInTeam.Fingerprint)

		assert.Equal(t, true, got)
	})

	t.Run("team.Contains returns false for person not in the team", func(t *testing.T) {
		got := team.Contains(personNotInTeam.Fingerprint)

		assert.Equal(t, false, got)
	})
}

func TestGetPersonForFingerprint(t *testing.T) {
	personOne := Person{
		Email:       "test@example.com",
		Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
	}
	personTwo := Person{
		Email:       "another@example.com",
		Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
	}

	team := Team{
		Name:   "Kiffix",
		UUID:   uuid.Must(uuid.NewV4()),
		People: []Person{personOne, personTwo},
	}

	t.Run("with a team member with matching fingerprint", func(t *testing.T) {
		got, err := team.GetPersonForFingerprint(fpr.MustParse(
			"AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"))

		assert.NoError(t, err)
		assert.Equal(t, &personOne, got)
	})

	t.Run("with no matching fingerprints", func(t *testing.T) {
		_, err := team.GetPersonForFingerprint(fpr.MustParse(
			"EEEEFFFFEEEEFFFFEEEEFFFFEEEEFFFFEEEEFFFF"))

		assert.Equal(t, fmt.Errorf("person not found"), err)
	})
}

func TestGetUpsertPersonWarnings(t *testing.T) {

	var tests = []struct {
		name          string
		person        Person
		team          Team
		expectedError error
		expectedTeam  Team
	}{
		{
			"adding a new person",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     false,
			},
			Team{
				UUID:   uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name:   "Kiffix",
				People: nil,
			},
			nil,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
		},
		{
			"adding a person with email that's already in roster",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     false,
			},
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("CCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDDCCCCDDDD"),
						IsAdmin:     false,
					},
				},
			},
			ErrKeyWouldBeUpdated,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					Person{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
		},
		{
			"adding a person with fingerprint that's already in roster",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     false,
			},
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "another@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
			ErrEmailWouldBeUpdated,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
		},
		{
			"adding a person who already is in roster",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     false,
			},
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
			ErrPersonWouldNotBeChanged,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
		},
		{
			"adding a non admin who already is in roster as an admin",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     false,
			},
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     true,
					},
				},
			},
			ErrPersonWouldBeDemotedAsAdmin,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
		},
		{
			"adding an admin who already is in roster but not as an admin",
			Person{
				Email:       "person@example.com",
				Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
				IsAdmin:     true,
			},
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     false,
					},
				},
			},
			ErrPersonWouldBePromotedToAdmin,
			Team{
				UUID: uuid.Must(uuid.FromString("8e26e4df0d474f7f9a07a37b2aa92104")),
				Name: "Kiffix",
				People: []Person{
					{
						Email:       "person@example.com",
						Fingerprint: fpr.MustParse("AAAABBBBAAAABBBBAAAAAAAABBBBAAAABBBBAAAA"),
						IsAdmin:     true,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run("GetUpsertPersonWarnings for "+test.name, func(t *testing.T) {
			_, err := test.team.GetUpsertPersonWarnings(test.person)
			assert.Equal(t, test.expectedError, err)
		})
	}

	for _, test := range tests {
		t.Run("UpsertPerson for "+test.name, func(t *testing.T) {
			test.team.UpsertPerson(test.person)
			assert.Equal(t, test.expectedTeam.People, test.team.People)
		})
	}
}

func TestSlugify(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{
			"Hello world",
			"hello-world",
		},
		{
			"Marks & Spencers",
			"marks-and-spencers",
		},
		{
			"Digit@l Wizards",
			"digital-wizards",
		},
		{
			"Between [Worlds]",
			"between-worlds",
		},
		{
			"--Future--",
			"future",
		},
		{
			"😁 Happy Cleaners 💦",
			"happy-cleaners",
		},
		{
			"déjà vu",
			"d-j-vu",
		},
		{
			"\n\000\037 \041\176\177\200\377\n",
			"",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("slugifying `%s`", test.input), func(t *testing.T) {
			assert.Equal(t, test.expected, slugify(test.input))
		})
	}
}

func TestSubDirectory(t *testing.T) {
	var tests = []struct {
		team     Team
		expected string
	}{
		{
			Team{
				Name: "kiffix",
				UUID: uuid.Must(uuid.FromString("6caa3730-2ca3-47b9-b671-5dc326100431")),
			},
			"kiffix-6caa3730-2ca3-47b9-b671-5dc326100431",
		},
		{
			Team{
				Name: "😁 Happy Cleaners 💦",
				UUID: uuid.Must(uuid.FromString("6caa3730-2ca3-47b9-b671-5dc326100431")),
			},
			"happy-cleaners-6caa3730-2ca3-47b9-b671-5dc326100431",
		},
		{
			Team{
				Name: "😁",
				UUID: uuid.Must(uuid.FromString("6caa3730-2ca3-47b9-b671-5dc326100431")),
			},
			"6caa3730-2ca3-47b9-b671-5dc326100431",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("get directory for `%s`", test.team.Name), func(t *testing.T) {

			assert.Equal(t, test.expected, test.team.subDirectory())
		})
	}
}

func saveTeam(t *testing.T, theTeam *Team, fluidkeysDirectory string) {
	teamSubdir, err := Directory(*theTeam, fluidkeysDirectory)
	assert.NoError(t, err)

	saver := RosterSaver{Directory: teamSubdir}

	roster, err := theTeam.PreviewRoster()
	assert.NoError(t, err)
	saver.Save(roster, "fake signature")
}
