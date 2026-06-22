package logic

import (
	"errors"
	"sort"

	"github.com/gravitl/netmaker/schema"
)

// SortExtClient - Sorts slice of ExtClients by their ClientID alphabetically with numbers first
func SortExtClient(unsortedExtClient []schema.ExtClient) {
	sort.Slice(unsortedExtClient, func(i, j int) bool {
		return unsortedExtClient[i].ClientID < unsortedExtClient[j].ClientID
	})
}

// GetExtClientByName - gets an ext client by name
func GetExtClientByName(ID string) (schema.ExtClient, error) {
	clients, err := GetAllExtClients()
	if err != nil {
		return schema.ExtClient{}, err
	}
	for i := range clients {
		if clients[i].ClientID == ID {
			return clients[i], nil
		}
	}
	return schema.ExtClient{}, errors.New("client not found")
}
