package mq

// MqClient - type for taking in an MQ client's data
type MqClient struct {
	ID       string
	Text     string
	Password string
	Networks []string
}

// ModifyClient - modifies an existing client's network roles
func ModifyClient(client *MqClient) error {

	roles := []MqDynSecRole{
		{
			Rolename: HostRole,
			Priority: -1,
		},
	}

	for i := range client.Networks {
		roles = append(roles, MqDynSecRole{
			Rolename: client.Networks[i],
			Priority: -1,
		},
		)
	}

	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  ModifyClientCmd,
				Username: client.ID,
				Textname: client.Text,
				Roles:    roles,
				Groups:   make([]MqDynSecGroup, 0),
			},
		},
	}

	return publishEventToDynSecTopic(event)
}

// DeleteMqClient - removes a client from the DynSec system
func DeleteMqClient(hostID string) error {
	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  DeleteClientCmd,
				Username: hostID,
			},
		},
	}
	return publishEventToDynSecTopic(event)
}

// CreateMqClient - creates an MQ DynSec client
func CreateMqClient(client *MqClient) error {

	roles := []MqDynSecRole{
		{
			Rolename: HostRole,
			Priority: -1,
		},
	}

	for i := range client.Networks {
		roles = append(roles, MqDynSecRole{
			Rolename: client.Networks[i],
			Priority: -1,
		},
		)
	}

	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  CreateClientCmd,
				Username: client.ID,
				Password: client.Password,
				Textname: client.Text,
				Roles:    roles,
				Groups:   make([]MqDynSecGroup, 0),
			},
		},
	}

	return publishEventToDynSecTopic(event)
}
