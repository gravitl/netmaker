package mq

// MqClient - type for taking in an MQ client's data
type MqClient struct {
	ID       string
	Text     string
	Password string
	Networks []string
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

	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  CreateClientCmd,
				Username: client.ID,
				Password: client.Password,
				Textname: client.Text,
				Roles: []MqDynSecRole{
					{
						Rolename: genericRole,
						Priority: -1,
					},
				},
				Groups: make([]MqDynSecGroup, 0),
			},
		},
	}

	return publishEventToDynSecTopic(event)
}
