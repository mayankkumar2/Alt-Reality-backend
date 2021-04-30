package participant

import "github.com/google/uuid"

func CreateParticipant() Participant {
	return Participant{
		ParticipantID: uuid.New(),
		AtX:           -1,
		AtY:           -1,
	}
}
