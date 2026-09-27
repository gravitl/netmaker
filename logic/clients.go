package logic

import (
	"sort"

	"github.com/gravitl/netmaker/models"
)

// SortExtClient - Sorts slice of ExtClients by their ClientID alphabetically with numbers first
func SortExtClient(unsortedExtClient []models.ExtClient) {
	sort.Slice(unsortedExtClient, func(i, j int) bool {
		return unsortedExtClient[i].ClientID < unsortedExtClient[j].ClientID
	})
}
