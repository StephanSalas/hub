package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubeshark/base/pkg/models"
	"github.com/kubeshark/hub/pkg/dependency"
	"github.com/kubeshark/hub/pkg/entries"
	"github.com/kubeshark/hub/pkg/validation"
	"github.com/rs/zerolog/log"
)

func HandleEntriesError(c *gin.Context, err error) bool {
	if err != nil {
		log.Error().Err(err).Msg("Couldn't get the entry!")
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     true,
			"type":      "error",
			"autoClose": "5000",
			"msg":       err.Error(),
		})
		return true // signal that there was an error and the caller should return
	}
	return false // no error, can continue
}

func GetEntries(c *gin.Context) {
	entriesRequest := &models.EntriesRequest{}

	if err := c.BindQuery(entriesRequest); err != nil {
		c.JSON(http.StatusBadRequest, err)
	}
	validationError := validation.Validate(entriesRequest)
	if validationError != nil {
		c.JSON(http.StatusBadRequest, validationError)
	}

	if entriesRequest.TimeoutMs == 0 {
		entriesRequest.TimeoutMs = 3000
	}

	entriesProvider := dependency.GetInstance(dependency.EntriesProvider).(entries.EntriesProvider)
	entryWrappers, metadata, err := entriesProvider.GetEntries(entriesRequest)
	if !HandleEntriesError(c, err) {
		baseEntries := make([]interface{}, 0)
		for _, entry := range entryWrappers {
			baseEntries = append(baseEntries, entry.Base)
		}
		c.JSON(http.StatusOK, models.EntriesResponse{
			Data: baseEntries,
			Meta: metadata,
		})
	}
}

func GetEntry(c *gin.Context) {
	singleEntryRequest := &models.SingleEntryRequest{}

	if err := c.BindQuery(singleEntryRequest); err != nil {
		c.JSON(http.StatusBadRequest, err)
	}
	validationError := validation.Validate(singleEntryRequest)
	if validationError != nil {
		c.JSON(http.StatusBadRequest, validationError)
	}

	id := c.Param("id")

	entriesProvider := dependency.GetInstance(dependency.EntriesProvider).(entries.EntriesProvider)
	entry, err := entriesProvider.GetEntry(singleEntryRequest, id)

	if !HandleEntriesError(c, err) {
		c.JSON(http.StatusOK, entry)
	}
}
