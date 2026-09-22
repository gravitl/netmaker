package logic

import (
	"context"
	"errors"
	"sort"

	"github.com/gravitl/netmaker/models"
)

// SortExtClient - Sorts slice of ExtClients by their ClientID alphabetically with numbers first
func SortExtClient(unsortedExtClient []models.ExtClient) {
	sort.Slice(unsortedExtClient, func(i, j int) bool {
		return unsortedExtClient[i].ClientID < unsortedExtClient[j].ClientID
	})
}

// GetExtClientByName - gets an ext client by name
func GetExtClientByName(ctx context.Context, ID string) (models.ExtClient, error) {
	clients, err := GetAllExtClients(ctx)
	if err != nil {
		return models.ExtClient{}, err
	}
	for i := range clients {
		if clients[i].ClientID == ID {
			return clients[i], nil
		}
	}
	return models.ExtClient{}, errors.New("client not found")
}
