package entries

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	baseApi "github.com/kubeshark/base/pkg/api"
	"github.com/kubeshark/base/pkg/models"
	"github.com/kubeshark/hub/pkg/app"
	"github.com/kubeshark/hub/pkg/db"
	"github.com/rs/zerolog/log"
	basenine "github.com/up9inc/basenine/client/go"
)

type EntriesProvider interface {
	GetEntries(entriesRequest *models.EntriesRequest) ([]*baseApi.EntryWrapper, *basenine.Metadata, error)
	GetEntry(singleEntryRequest *models.SingleEntryRequest, entryId string) (*baseApi.EntryWrapper, error)
}

type BasenineEntriesProvider struct{}

func (e *BasenineEntriesProvider) GetEntries(entriesRequest *models.EntriesRequest) ([]*baseApi.EntryWrapper, *basenine.Metadata, error) {
	data, _, lastMeta, err := basenine.Fetch(db.BasenineHost, db.BaseninePort,
		entriesRequest.LeftOff, entriesRequest.Direction, entriesRequest.Query,
		entriesRequest.Limit, time.Duration(entriesRequest.TimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, nil, err
	}

	var dataSlice []*baseApi.EntryWrapper

	for _, row := range data {
		var entry *baseApi.Entry
		err = json.Unmarshal(row, &entry)
		if err != nil {
			return nil, nil, err
		}

		protocol, ok := app.ProtocolsMap[entry.Protocol.ToString()]
		if !ok {
			return nil, nil, fmt.Errorf("protocol not found, protocol: %v", protocol)
		}

		extension, ok := app.ExtensionsMap[protocol.Name]
		if !ok {
			return nil, nil, fmt.Errorf("extension not found, extension: %v", protocol.Name)
		}

		base := extension.Dissector.Summarize(entry)

		dataSlice = append(dataSlice, &baseApi.EntryWrapper{
			Protocol: *protocol,
			Data:     entry,
			Base:     base,
		})
	}

	var metadata *basenine.Metadata
	err = json.Unmarshal(lastMeta, &metadata)
	if err != nil {
		log.Error().Err(err).Msg("While recieving metadata:")
	}

	return dataSlice, metadata, nil
}

func (e *BasenineEntriesProvider) GetEntry(singleEntryRequest *models.SingleEntryRequest, entryId string) (*baseApi.EntryWrapper, error) {
	var entry *baseApi.Entry
	bytes, err := basenine.Single(db.BasenineHost, db.BaseninePort, entryId, singleEntryRequest.Query)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &entry)
	if err != nil {
		return nil, errors.New(string(bytes))
	}

	protocol, ok := app.ProtocolsMap[entry.Protocol.ToString()]
	if !ok {
		return nil, fmt.Errorf("protocol not found, protocol: %v", protocol)
	}

	extension, ok := app.ExtensionsMap[protocol.Name]
	if !ok {
		return nil, fmt.Errorf("extension not found, extension: %v", protocol.Name)
	}

	base := extension.Dissector.Summarize(entry)
	var representation []byte
	representation, err = extension.Dissector.Represent(entry.Request, entry.Response)
	if err != nil {
		return nil, err
	}

	return &baseApi.EntryWrapper{
		Protocol:       *protocol,
		Representation: string(representation),
		Data:           entry,
		Base:           base,
	}, nil
}
