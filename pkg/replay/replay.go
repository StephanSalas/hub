package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	baseApi "github.com/kubeshark/base/pkg/api"
	kubesharkhttp "github.com/kubeshark/base/pkg/extensions/http"
	"github.com/kubeshark/hub/pkg/app"
)

var (
	inProcessRequestsLocker = sync.Mutex{}
	inProcessRequests       = 0
)

const maxParallelAction = 5

type Details struct {
	Method  string            `json:"method"`
	Url     string            `json:"url"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type Response struct {
	Success      bool        `json:"status"`
	Data         interface{} `json:"data"`
	ErrorMessage string      `json:"errorMessage"`
}

func incrementCounter() bool {
	result := false
	inProcessRequestsLocker.Lock()
	if inProcessRequests < maxParallelAction {
		inProcessRequests++
		result = true
	}
	inProcessRequestsLocker.Unlock()
	return result
}

func decrementCounter() {
	inProcessRequestsLocker.Lock()
	inProcessRequests--
	inProcessRequestsLocker.Unlock()
}

func getEntryFromRequestResponse(extension *baseApi.Extension, request *http.Request, response *http.Response) *baseApi.Entry {
	captureTime := time.Now()

	itemTmp := baseApi.OutputChannelItem{
		Protocol: *extension.Protocol,
		ConnectionInfo: &baseApi.ConnectionInfo{
			ClientIP:   "",
			ClientPort: "1",
			ServerIP:   "",
			ServerPort: "1",
			IsOutgoing: false,
		},
		Capture:   "",
		Timestamp: time.Now().UnixMilli(),
		Pair: &baseApi.RequestResponsePair{
			Request: baseApi.GenericMessage{
				IsRequest:   true,
				CaptureTime: captureTime,
				CaptureSize: 0,
				Payload: &kubesharkhttp.HTTPPayload{
					Type: kubesharkhttp.TypeHttpRequest,
					Data: request,
				},
			},
			Response: baseApi.GenericMessage{
				IsRequest:   false,
				CaptureTime: captureTime,
				CaptureSize: 0,
				Payload: &kubesharkhttp.HTTPPayload{
					Type: kubesharkhttp.TypeHttpResponse,
					Data: response,
				},
			},
		},
	}

	// Analyze is expecting an item that's marshalled and unmarshalled
	itemMarshalled, err := json.Marshal(itemTmp)
	if err != nil {
		return nil
	}
	var finalItem *baseApi.OutputChannelItem
	if err := json.Unmarshal(itemMarshalled, &finalItem); err != nil {
		return nil
	}

	return extension.Dissector.Analyze(finalItem, "", "", "")
}

func ExecuteRequest(replayData *Details, timeout time.Duration) *Response {
	if incrementCounter() {
		defer decrementCounter()

		client := &http.Client{
			Timeout: timeout,
		}

		request, err := http.NewRequest(strings.ToUpper(replayData.Method), replayData.Url, bytes.NewBufferString(replayData.Body))
		if err != nil {
			return &Response{
				Success:      false,
				Data:         nil,
				ErrorMessage: err.Error(),
			}
		}

		for headerKey, headerValue := range replayData.Headers {
			request.Header.Add(headerKey, headerValue)
		}
		request.Header.Add("x-kubeshark", uuid.New().String())
		response, requestErr := client.Do(request)

		if requestErr != nil {
			return &Response{
				Success:      false,
				Data:         nil,
				ErrorMessage: requestErr.Error(),
			}
		}

		extension := app.ExtensionsMap["http"] // # TODO: maybe pass the extension to the function so it can be tested
		entry := getEntryFromRequestResponse(extension, request, response)
		base := extension.Dissector.Summarize(entry)
		var representation []byte

		// Represent is expecting an entry that's marshalled and unmarshalled
		entryMarshalled, err := json.Marshal(entry)
		if err != nil {
			return &Response{
				Success:      false,
				Data:         nil,
				ErrorMessage: err.Error(),
			}
		}
		var entryUnmarshalled *baseApi.Entry
		if err := json.Unmarshal(entryMarshalled, &entryUnmarshalled); err != nil {
			return &Response{
				Success:      false,
				Data:         nil,
				ErrorMessage: err.Error(),
			}
		}

		representation, err = extension.Dissector.Represent(entryUnmarshalled.Request, entryUnmarshalled.Response)
		if err != nil {
			return &Response{
				Success:      false,
				Data:         nil,
				ErrorMessage: err.Error(),
			}
		}

		return &Response{
			Success: true,
			Data: &baseApi.EntryWrapper{
				Protocol:       *extension.Protocol,
				Representation: string(representation),
				Data:           entryUnmarshalled,
				Base:           base,
			},
			ErrorMessage: "",
		}
	} else {
		return &Response{
			Success:      false,
			Data:         nil,
			ErrorMessage: fmt.Sprintf("reached threshold of %d requests", maxParallelAction),
		}
	}
}
