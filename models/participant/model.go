package participant

import "github.com/google/uuid"

type Participant struct {
	ParticipantID uuid.UUID `json:"participant_id"`
	AtX           float64   `json:"at_x"`
	AtY           float64   `json:"at_y"`
}
