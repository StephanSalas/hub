package api

import (
	"context"
	"encoding/json"
	"log"
	"time"

	baseApi "github.com/kubeshark/base/pkg/api"
	"github.com/kubeshark/hub/pkg/db"
	"github.com/kubeshark/hub/pkg/dependency"
	basenine "github.com/up9inc/basenine/client/go"
)

type EntryStreamer interface {
	Get(ctx context.Context, socketId int, params *WebSocketParams) error
}

type BasenineEntryStreamer struct{}

func (e *BasenineEntryStreamer) Get(ctx context.Context, socketId int, params *WebSocketParams) error {
	var connection *basenine.Connection

	entryStreamerSocketConnector := dependency.GetInstance(dependency.EntryStreamerSocketConnector).(EntryStreamerSocketConnector)

	connection, err := basenine.NewConnection(db.BasenineHost, db.BaseninePort)
	if err != nil {
		log.Printf("Failed to establish a connection to Basenine: %v", err)
		entryStreamerSocketConnector.CleanupSocket(socketId)
		return err
	}

	data := make(chan []byte)
	meta := make(chan []byte)

	query := params.Query
	if err = basenine.Validate(db.BasenineHost, db.BaseninePort, query); err != nil {
		if err := entryStreamerSocketConnector.SendToastError(socketId, err); err != nil {
			return err
		}

		entryStreamerSocketConnector.CleanupSocket(socketId)
		return err
	}

	leftOff, err := e.fetch(socketId, params, entryStreamerSocketConnector)
	if err != nil {
		log.Printf("Fetch error: %v", err)
	}

	handleDataChannel := func(c *basenine.Connection, data chan []byte) {
		for {
			bytes := <-data

			if string(bytes) == basenine.CloseChannel {
				return
			}

			var entry *baseApi.Entry
			if err = json.Unmarshal(bytes, &entry); err != nil {
				log.Printf("Error unmarshalling entry: %v", err)
				continue
			}

			if err := entryStreamerSocketConnector.SendEntry(socketId, entry, params); err != nil {
				log.Printf("Error sending entry to socket, err: %v", err)
				return
			}
		}
	}

	handleMetaChannel := func(c *basenine.Connection, meta chan []byte) {
		for {
			bytes := <-meta

			if string(bytes) == basenine.CloseChannel {
				return
			}

			var metadata *basenine.Metadata
			if err = json.Unmarshal(bytes, &metadata); err != nil {
				log.Printf("Error unmarshalling metadata: %v", err)
				continue
			}

			if err := entryStreamerSocketConnector.SendMetadata(socketId, metadata); err != nil {
				log.Printf("Error sending metadata to socket, err: %v", err)
				return
			}
		}
	}

	go handleDataChannel(connection, data)
	go handleMetaChannel(connection, meta)

	if err = connection.Query(leftOff, query, data, meta); err != nil {
		log.Printf("Query mode call failed: %v", err)
		entryStreamerSocketConnector.CleanupSocket(socketId)
		return err
	}

	go func() {
		<-ctx.Done()
		data <- []byte(basenine.CloseChannel)
		meta <- []byte(basenine.CloseChannel)
		connection.Close()
	}()

	return nil
}

// Reverses a []byte slice.
func (e *BasenineEntryStreamer) fetch(socketId int, params *WebSocketParams, connector EntryStreamerSocketConnector) (leftOff string, err error) {
	if params.Fetch <= 0 {
		leftOff = params.LeftOff
		return
	}

	var data [][]byte
	var firstMeta []byte
	var lastMeta []byte
	data, firstMeta, lastMeta, err = basenine.Fetch(
		db.BasenineHost,
		db.BaseninePort,
		params.LeftOff,
		-1,
		params.Query,
		params.Fetch,
		time.Duration(params.TimeoutMs)*time.Millisecond,
	)
	if err != nil {
		return
	}

	var firstMetadata *basenine.Metadata
	if err = json.Unmarshal(firstMeta, &firstMetadata); err != nil {
		return
	}

	leftOff = firstMetadata.LeftOff

	var lastMetadata *basenine.Metadata
	if err = json.Unmarshal(lastMeta, &lastMetadata); err != nil {
		return
	}

	if err = connector.SendMetadata(socketId, lastMetadata); err != nil {
		return
	}

	data = e.reverseBytesSlice(data)
	for _, row := range data {
		var entry *baseApi.Entry
		if err = json.Unmarshal(row, &entry); err != nil {
			break
		}

		if err = connector.SendEntry(socketId, entry, params); err != nil {
			return
		}
	}
	return
}

// Reverses a []byte slice.
func (e *BasenineEntryStreamer) reverseBytesSlice(arr [][]byte) (newArr [][]byte) {
	for i := len(arr) - 1; i >= 0; i-- {
		newArr = append(newArr, arr[i])
	}
	return newArr
}
