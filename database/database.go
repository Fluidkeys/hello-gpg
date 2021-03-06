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

package database

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	fpr "github.com/fluidkeys/fluidkeys/fingerprint"
	"github.com/fluidkeys/fluidkeys/pgpkey"
	"github.com/fluidkeys/fluidkeys/team"
	"github.com/gofrs/uuid"
)

// Database is the user's Fluidkeys database. It points at the filepath for the jsonFilename
type Database struct {
	jsonFilename string
}

// Message is the structure the database takes
type Message struct {
	KeysImportedIntoGnuPG []KeyImportedIntoGnuPGMessage
	RequestsToJoinTeams   []RequestToJoinTeamMessage
	EventTimes            map[string]time.Time
}

// KeyImportedIntoGnuPGMessage represents a key the user has imported into GnuPG from Fluidkeys
type KeyImportedIntoGnuPGMessage struct {
	Fingerprint fpr.Fingerprint
}

// RequestToJoinTeamMessage records a request to join a team.
type RequestToJoinTeamMessage struct {
	TeamUUID    uuid.UUID       `json: "TeamUUID"`
	TeamName    string          `json: "TeamName"`
	Fingerprint fpr.Fingerprint `json: "Fingerprint"`
	RequestedAt time.Time       `json: "RequestedAt"`
}

// New returns a database from the given fluidkeys directory
func New(fluidkeysDirectory string) Database {
	jsonFilename := filepath.Join(fluidkeysDirectory, "db.json")
	return Database{jsonFilename: jsonFilename}
}

// RecordFingerprintImportedIntoGnuPG takes a given fingperprint and records that it's been
// imported into GnuPG by writing an updated json database.
func (db *Database) RecordFingerprintImportedIntoGnuPG(newFingerprint fpr.Fingerprint) error {
	message, err := db.loadFromFile()
	if err != nil {
		return err
	}

	existingKeysImported := message.KeysImportedIntoGnuPG

	message.KeysImportedIntoGnuPG = deduplicateKeyImportedIntoGnuPGMessages(
		append(existingKeysImported, KeyImportedIntoGnuPGMessage{
			Fingerprint: newFingerprint,
		}),
	)

	return db.saveToFile(*message)
}

// RecordRequestToJoinTeam takes a given request to join a team and records that it's been
// sent by writing an updated json database.
func (db *Database) RecordRequestToJoinTeam(
	teamUUID uuid.UUID, teamName string, fingerprint fpr.Fingerprint, now time.Time) error {

	message, err := db.loadFromFile()
	if err != nil {
		return err
	}

	newRequest := RequestToJoinTeamMessage{
		TeamUUID:    teamUUID,
		TeamName:    teamName,
		Fingerprint: fingerprint,
		RequestedAt: now,
	}

	message.RequestsToJoinTeams = deduplicateRequests(
		append(message.RequestsToJoinTeams, newRequest),
	)

	return db.saveToFile(*message)
}

// RecordLast takes a verb and item and records the action in the database, e.g verb "fetched",
// item: key.
func (db *Database) RecordLast(verb string, item interface{}, now time.Time) error {
	message, err := db.loadFromFile()
	if err != nil {
		return err
	}

	if verb == "" {
		return fmt.Errorf("verb can't be empty")
	}

	mapKey, err := makeMapKey(verb, item)
	if err != nil {
		return fmt.Errorf("don't know how to handle %v", item)
	}

	message.EventTimes[mapKey] = now

	return db.saveToFile(*message)
}

// GetLast akes a verb and item and returns the last time it was recorded as being 'done'.
func (db *Database) GetLast(verb string, item interface{}) (EventTimes time.Time, err error) {
	message, err := db.loadFromFile()
	if err != nil {
		return time.Time{}, err
	}

	mapKey, err := makeMapKey(verb, item)
	if err != nil {
		return time.Time{}, fmt.Errorf("don't know how to handle %#v", item)
	}

	return message.EventTimes[mapKey], nil
}

func makeMapKey(verb string, item interface{}) (string, error) {
	var itemKey string

	switch i := item.(type) {
	case *pgpkey.PgpKey:
		itemKey = keyItem + ":" + i.Fingerprint().Uri()

	case pgpkey.PgpKey:
		itemKey = keyItem + ":" + i.Fingerprint().Uri()

	case *fpr.Fingerprint:
		itemKey = keyItem + ":" + i.Uri()

	case fpr.Fingerprint:
		itemKey = keyItem + ":" + i.Uri()

	case *team.Team:
		itemKey = teamItem + ":" + i.UUID.String()

	case team.Team:
		itemKey = teamItem + ":" + i.UUID.String()

	case fmt.Stringer: // does it have .String() ?
		itemKey = getType(i) + ":" + i.String()

	default:
		return "", fmt.Errorf("don't know how to handle %#v", item)
	}

	return verb + ":" + itemKey, nil
}

func getType(myvar interface{}) string {
	t := reflect.TypeOf(myvar)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

// IsOlderThan takes a verb and item and returns true if it was last recorded more than `age` ago.
func (db *Database) IsOlderThan(verb string, item interface{}, age time.Duration, now time.Time) (
	isOlder bool, err error) {

	t, err := db.GetLast(verb, item)
	if err != nil {
		return false, err
	} else if t.IsZero() {
		// never recorded, so the answer is always yes (it's older than age)
		return true, nil
	}

	return now.Sub(t) > age, nil
}

// GetFingerprintsImportedIntoGnuPG returns a slice of fingerprints that have
// been imported into GnuPG
func (db *Database) GetFingerprintsImportedIntoGnuPG() (fingerprints []fpr.Fingerprint, err error) {
	message, err := db.loadFromFile()
	if os.IsNotExist(err) {
		return []fpr.Fingerprint{}, nil
	}
	if err != nil {
		return nil, err
	}

	for _, v := range message.KeysImportedIntoGnuPG {
		fingerprints = append(fingerprints, v.Fingerprint)
	}

	return fingerprints, nil
}

// GetRequestsToJoinTeams returns a slice of requests to join teams the user has made.
func (db *Database) GetRequestsToJoinTeams() (requests []team.RequestToJoinTeam, err error) {
	message, err := db.loadFromFile()
	if os.IsNotExist(err) {
		return []team.RequestToJoinTeam{}, nil
	}
	if err != nil {
		return nil, err
	}

	for _, msg := range deduplicateRequests(message.RequestsToJoinTeams) {
		requests = append(requests, team.RequestToJoinTeam{
			TeamUUID:    msg.TeamUUID,
			TeamName:    msg.TeamName,
			Fingerprint: msg.Fingerprint,
			RequestedAt: msg.RequestedAt,
		})
	}
	return requests, nil
}

// GetExistingRequestToJoinTeam returns a single request based on the teamUUID and fingerprint.
func (db *Database) GetExistingRequestToJoinTeam(teamUUID uuid.UUID, fingerprint fpr.Fingerprint) (
	request *team.RequestToJoinTeam, err error) {

	message, err := db.loadFromFile()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	for _, msg := range message.RequestsToJoinTeams {
		if msg.TeamUUID == teamUUID && msg.Fingerprint == fingerprint {
			return &team.RequestToJoinTeam{
				TeamUUID:    msg.TeamUUID,
				TeamName:    msg.TeamName,
				Fingerprint: msg.Fingerprint,
				RequestedAt: msg.RequestedAt,
			}, nil
		}
	}

	return nil, nil
}

// DeleteRequestToJoinTeam deletes all requests to join the team matching the given team UUID and
// fingerprint.
func (db *Database) DeleteRequestToJoinTeam(teamUUID uuid.UUID, fingerprint fpr.Fingerprint) error {
	message, err := db.loadFromFile()
	if err != nil {
		return err
	}

	newRequests := []RequestToJoinTeamMessage{}

	for _, req := range message.RequestsToJoinTeams {
		if req.TeamUUID == teamUUID && req.Fingerprint == fingerprint {
			log.Printf("deleting request to join team: %v", req)
			continue
		}

		newRequests = append(newRequests, req)
	}
	message.RequestsToJoinTeams = newRequests
	return db.saveToFile(*message)
}

func (db *Database) loadFromFile() (message *Message, err error) {
	file, err := os.Open(db.jsonFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return &Message{
				EventTimes: map[string]time.Time{},
			}, nil
		}
		return nil, fmt.Errorf("couldn't open '%s': %v", db.jsonFilename, err)
	}

	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll(..) error: %v", err)
	}

	if err := json.Unmarshal(byteValue, &message); err != nil {
		return nil, fmt.Errorf("error loading json: %v", err)
	}

	if len(message.EventTimes) == 0 {
		message.EventTimes = make(map[string]time.Time)
	}

	return &Message{
		KeysImportedIntoGnuPG: deduplicateKeyImportedIntoGnuPGMessages(
			message.KeysImportedIntoGnuPG,
		),
		RequestsToJoinTeams: message.RequestsToJoinTeams,
		EventTimes:          message.EventTimes,
	}, nil
}

func (db Database) saveToFile(message Message) error {
	file, err := os.Create(db.jsonFilename)
	if err != nil {
		return fmt.Errorf("couldn't open '%s': %v", db.jsonFilename, err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	return encoder.Encode(message)
}

// deduplicateRequests returns a de-duplicated version of requests, where a duplicate is defined
// as having the same (TeamUUID + Fingerprint) pair.
// The *newest* RequestedAt defines the single request that's returned.
func deduplicateRequests(requests []RequestToJoinTeamMessage) (deduped []RequestToJoinTeamMessage) {
	mapHash := func(r RequestToJoinTeamMessage) string {
		return fmt.Sprintf("%s%s", r.TeamUUID, r.Fingerprint)
	}

	reqsMap := map[string][]RequestToJoinTeamMessage{}

	for _, r := range requests {
		hash := mapHash(r)

		if existing, ok := reqsMap[hash]; ok {
			reqsMap[hash] = append(existing, r)
		} else {
			reqsMap[hash] = []RequestToJoinTeamMessage{r}
		}
	}

	for _, dupeRequests := range reqsMap {
		sort.Sort(newestFirst(dupeRequests))
		deduped = append(deduped, dupeRequests[0] /* 0th is the earliest */)
	}

	sort.Sort(newestFirst(deduped))
	return deduped
}

func deduplicateKeyImportedIntoGnuPGMessages(slice []KeyImportedIntoGnuPGMessage,
) (deduped []KeyImportedIntoGnuPGMessage) {

	alreadySeen := make(map[KeyImportedIntoGnuPGMessage]bool)

	for _, v := range slice {
		if _, inMap := alreadySeen[v]; !inMap {
			deduped = append(deduped, v)
			alreadySeen[v] = true
		}
	}
	return deduped
}

var (
	// ErrRequestNotFound is returned when a request to join a team is not found matching the given
	// team UUID and fingerprint
	ErrRequestNotFound = fmt.Errorf("no request to join team with that fingerprint found")
)

const (
	keyItem  = "key"
	teamItem = "team"
)
