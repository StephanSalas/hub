package servicemap

import (
	baseApi "github.com/kubeshark/base/pkg/api"
)

type ServiceMapStatus struct {
	Status                string `json:"status"`
	EntriesProcessedCount int    `json:"entriesProcessedCount"`
	NodeCount             int    `json:"nodeCount"`
	EdgeCount             int    `json:"edgeCount"`
}

type ServiceMapResponse struct {
	Status ServiceMapStatus `json:"status"`
	Nodes  []ServiceMapNode `json:"nodes"`
	Edges  []ServiceMapEdge `json:"edges"`
}

type ServiceMapNode struct {
	Id       int          `json:"id"`
	Name     string       `json:"name"`
	Entry    *baseApi.TCP `json:"entry"`
	Count    int          `json:"count"`
	Resolved bool         `json:"resolved"`
}

type ServiceMapEdge struct {
	Source      ServiceMapNode    `json:"source"`
	Destination ServiceMapNode    `json:"destination"`
	Count       int               `json:"count"`
	Protocol    *baseApi.Protocol `json:"protocol"`
}
