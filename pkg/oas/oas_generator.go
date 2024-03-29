package oas

import (
	"encoding/json"
	"log"
	"net/url"
	"sync"

	"github.com/kubeshark/base/pkg/api"
	"github.com/kubeshark/hub/pkg/har"
)

var (
	syncOnce sync.Once
	instance *defaultOasGenerator
)

type OasGeneratorSink interface {
	HandleEntry(kubesharkEntry *api.Entry)
}

type OasGenerator interface {
	Start()
	Stop()
	IsStarted() bool
	GetServiceSpecs() *sync.Map
}

type defaultOasGenerator struct {
	started       bool
	serviceSpecs  *sync.Map
	maxExampleLen int
}

func GetDefaultOasGeneratorInstance(maxExampleLen int) *defaultOasGenerator {
	syncOnce.Do(func() {
		instance = NewDefaultOasGenerator(maxExampleLen)
		log.Print("OAS Generator Initialized")
	})
	return instance
}

func (g *defaultOasGenerator) Start() {
	g.started = true
}

func (g *defaultOasGenerator) Stop() {
	if !g.started {
		return
	}

	g.started = false

	g.reset()
}

func (g *defaultOasGenerator) IsStarted() bool {
	return g.started
}

func (g *defaultOasGenerator) HandleEntry(kubesharkEntry *api.Entry) {
	if !g.started {
		return
	}

	if kubesharkEntry.Protocol.Name == "http" {
		dest := kubesharkEntry.Destination.Name
		if dest == "" {
			log.Printf("OAS: Unresolved entry %s", kubesharkEntry.Id)
			return
		}

		entry, err := har.NewEntry(kubesharkEntry.Request, kubesharkEntry.Response, kubesharkEntry.StartTime, kubesharkEntry.ElapsedTime)
		if err != nil {
			log.Printf("Failed to turn KubesharkEntry %s into HAR Entry: %v", kubesharkEntry.Id, err)
			return
		}

		entryWSource := &EntryWithSource{
			Entry:       *entry,
			Source:      kubesharkEntry.Source.Name,
			Destination: dest,
			Id:          kubesharkEntry.Id,
		}

		g.handleHARWithSource(entryWSource)
	} else {
		log.Printf("OAS: Unsupported protocol in entry %s: %s", kubesharkEntry.Id, kubesharkEntry.Protocol.Name)
	}
}

func (g *defaultOasGenerator) handleHARWithSource(entryWSource *EntryWithSource) {
	entry := entryWSource.Entry
	gen := g.getGen(entryWSource.Destination, entry.Request.URL)

	opId, err := gen.feedEntry(entryWSource)
	if err != nil {
		txt, suberr := json.Marshal(entry)
		if suberr == nil {
			log.Printf("Problematic entry: %s", txt)
		}

		log.Printf("Failed processing entry %s: %v", entryWSource.Id, err)
		return
	}

	log.Printf("Handled entry %s as opId: %s", entryWSource.Id, opId) // TODO: set opId back to entry?
}

func (g *defaultOasGenerator) getGen(dest string, urlStr string) *SpecGen {
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Printf("Failed to parse entry URL: %v, err: %v", urlStr, err)
	}

	val, found := g.serviceSpecs.Load(dest)
	var gen *SpecGen
	if !found {
		gen = NewGen(u.Scheme + "://" + dest)
		gen.MaxExampleLen = g.maxExampleLen
		g.serviceSpecs.Store(dest, gen)
	} else {
		gen = val.(*SpecGen)
	}
	return gen
}

func (g *defaultOasGenerator) reset() {
	g.serviceSpecs = &sync.Map{}
}

func (g *defaultOasGenerator) GetServiceSpecs() *sync.Map {
	return g.serviceSpecs
}

func NewDefaultOasGenerator(maxExampleLen int) *defaultOasGenerator {
	return &defaultOasGenerator{
		started:       false,
		serviceSpecs:  &sync.Map{},
		maxExampleLen: maxExampleLen,
	}
}
