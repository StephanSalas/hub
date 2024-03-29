package app

import (
	"fmt"
	"sort"
	"time"

	"github.com/antelman107/net-wait-go/wait"
	baseApi "github.com/kubeshark/base/pkg/api"
	amqpExt "github.com/kubeshark/base/pkg/extensions/amqp"
	httpExt "github.com/kubeshark/base/pkg/extensions/http"
	kafkaExt "github.com/kubeshark/base/pkg/extensions/kafka"
	redisExt "github.com/kubeshark/base/pkg/extensions/redis"
	"github.com/kubeshark/hub/pkg/api"
	"github.com/kubeshark/hub/pkg/providers"
	"github.com/kubeshark/hub/pkg/utils"
	"github.com/op/go-logging"
	"github.com/rs/zerolog/log"
	basenine "github.com/up9inc/basenine/client/go"
)

var (
	Extensions    []*baseApi.Extension          // global
	ExtensionsMap map[string]*baseApi.Extension // global
	ProtocolsMap  map[string]*baseApi.Protocol  //global
)

func LoadExtensions() {
	Extensions = make([]*baseApi.Extension, 0)
	ExtensionsMap = make(map[string]*baseApi.Extension)
	ProtocolsMap = make(map[string]*baseApi.Protocol)

	extensionHttp := &baseApi.Extension{}
	dissectorHttp := httpExt.NewDissector()
	dissectorHttp.Register(extensionHttp)
	extensionHttp.Dissector = dissectorHttp
	Extensions = append(Extensions, extensionHttp)
	ExtensionsMap[extensionHttp.Protocol.Name] = extensionHttp
	protocolsHttp := dissectorHttp.GetProtocols()
	for k, v := range protocolsHttp {
		ProtocolsMap[k] = v
	}

	extensionAmqp := &baseApi.Extension{}
	dissectorAmqp := amqpExt.NewDissector()
	dissectorAmqp.Register(extensionAmqp)
	extensionAmqp.Dissector = dissectorAmqp
	Extensions = append(Extensions, extensionAmqp)
	ExtensionsMap[extensionAmqp.Protocol.Name] = extensionAmqp
	protocolsAmqp := dissectorAmqp.GetProtocols()
	for k, v := range protocolsAmqp {
		ProtocolsMap[k] = v
	}

	extensionKafka := &baseApi.Extension{}
	dissectorKafka := kafkaExt.NewDissector()
	dissectorKafka.Register(extensionKafka)
	extensionKafka.Dissector = dissectorKafka
	Extensions = append(Extensions, extensionKafka)
	ExtensionsMap[extensionKafka.Protocol.Name] = extensionKafka
	protocolsKafka := dissectorKafka.GetProtocols()
	for k, v := range protocolsKafka {
		ProtocolsMap[k] = v
	}

	extensionRedis := &baseApi.Extension{}
	dissectorRedis := redisExt.NewDissector()
	dissectorRedis.Register(extensionRedis)
	extensionRedis.Dissector = dissectorRedis
	Extensions = append(Extensions, extensionRedis)
	ExtensionsMap[extensionRedis.Protocol.Name] = extensionRedis
	protocolsRedis := dissectorRedis.GetProtocols()
	for k, v := range protocolsRedis {
		ProtocolsMap[k] = v
	}

	sort.Slice(Extensions, func(i, j int) bool {
		return Extensions[i].Protocol.Priority < Extensions[j].Protocol.Priority
	})

	api.InitMaps(ExtensionsMap, ProtocolsMap)
	providers.InitProtocolToColor(ProtocolsMap)
}

func ConfigureBasenineServer(host string, port string, dbSize int64, logLevel logging.Level, insertionFilter string) {
	if !wait.New(
		wait.WithProto("tcp"),
		wait.WithWait(200*time.Millisecond),
		wait.WithBreak(50*time.Millisecond),
		wait.WithDeadline(20*time.Second),
		wait.WithDebug(logLevel == logging.DEBUG),
	).Do([]string{fmt.Sprintf("%s:%s", host, port)}) {
		log.Fatal().Msg("Basenine is not available!")
	}

	if err := basenine.Limit(host, port, dbSize); err != nil {
		log.Fatal().Err(err).Msg("While limiting the database size:")
	}

	// Define the macros
	for _, extension := range Extensions {
		macros := extension.Dissector.Macros()
		for macro, expanded := range macros {
			if err := basenine.Macro(host, port, macro, expanded); err != nil {
				log.Fatal().Err(err).Msg("While adding a macro:")
			}
		}
	}

	// Set the insertion filter that comes from the config
	if err := basenine.InsertionFilter(host, port, insertionFilter); err != nil {
		log.Error().Err(err).Str("filter", insertionFilter).Msg("While setting the insertion filter:")
	}

	utils.StartTime = time.Now().UnixNano() / int64(time.Millisecond)
}

func GetEntryInputChannel() chan *baseApi.OutputChannelItem {
	outputItemsChannel := make(chan *baseApi.OutputChannelItem)
	filteredOutputItemsChannel := make(chan *baseApi.OutputChannelItem)
	go FilterItems(outputItemsChannel, filteredOutputItemsChannel)
	go api.StartReadingEntries(filteredOutputItemsChannel, nil, ExtensionsMap)

	return outputItemsChannel
}

func FilterItems(inChannel <-chan *baseApi.OutputChannelItem, outChannel chan *baseApi.OutputChannelItem) {
	for message := range inChannel {
		if message.ConnectionInfo.IsOutgoing && api.CheckIsServiceIP(message.ConnectionInfo.ServerIP) {
			continue
		}

		outChannel <- message
	}
}
