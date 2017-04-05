package cluster

// Manager is the interface that manages cluster state.
type Manager interface {

	// Open startup the manager.
	Open() error

	// Close closes the manager underlying connection.
	Close()

	// RegisterResource register a resource for an input plugin.
	RegisterResource(resource Resource) error

	// RegisteredResources returns all the registered resource in the cluster.
	// The return map is in the form of {input: []resource}
	RegisteredResources() ([]Resource, error)

	// LiveParticipants returns currently online participants.
	LiveParticipants() ([]Participant, error)

	// Controller returns the controller participant.
	Controller() (Participant, error)

	// Rebalance triggers a new leader election across the cluster.
	Rebalance() error
}
