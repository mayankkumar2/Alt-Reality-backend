package room

import (
	participantPkg "github.com/TeamRekursion/Alt-Reality-backend/models/participant"
	"github.com/google/uuid"
)

// TODO: use a BST Tree implementation for fast addition and removal of active users
// TODO: use a SET Data structure indexed with ParticipantID for fast updates of user coordinates

type Room struct {
	RoomID       uuid.UUID                    `json:"room_id"`
	Name         string                       `json:"name"`
	OfferStr     string                       `json:"offer_str"`
	Participants []participantPkg.Participant `json:"participants"`
	StartedAt    int64                        `json:"started_at"`
	Active       []uuid.UUID                  `json:"active"`
}


