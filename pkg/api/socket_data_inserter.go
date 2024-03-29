package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kubeshark/base/pkg/api"
	"github.com/kubeshark/hub/pkg/db"
	"github.com/rs/zerolog/log"
	basenine "github.com/up9inc/basenine/client/go"
)

type EntryInserter interface {
	Insert(entry *api.Entry) error
}

type BasenineEntryInserter struct {
	connection *basenine.Connection
}

var instance *BasenineEntryInserter
var once sync.Once

func GetBasenineEntryInserterInstance() *BasenineEntryInserter {
	once.Do(func() {
		instance = &BasenineEntryInserter{}
	})

	return instance
}

func (e *BasenineEntryInserter) Insert(entry *api.Entry) error {
	if e.connection == nil {
		e.connection = initializeConnection()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("error marshling entry, err: %v", err)
	}

	if err := e.connection.SendText(string(data)); err != nil {
		e.connection.Close()
		e.connection = nil

		return fmt.Errorf("error sending text to database, err: %v", err)
	}

	return nil
}

func initializeConnection() *basenine.Connection {
	for {
		connection, err := basenine.NewConnection(db.BasenineHost, db.BaseninePort)
		if err != nil {
			log.Error().Err(err).Msg("Can't establish a new connection to Basenine server:")
			time.Sleep(db.BasenineReconnectInterval * time.Second)
			continue
		}

		if err = connection.InsertMode(); err != nil {
			log.Error().Err(err).Msg("Insert mode call failed:")
			connection.Close()
			time.Sleep(db.BasenineReconnectInterval * time.Second)
			continue
		}

		return connection
	}
}
