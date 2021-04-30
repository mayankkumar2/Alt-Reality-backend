package room

import (
	participantPkg "github.com/TeamRekursion/Alt-Reality-backend/models/participant"
	"github.com/google/uuid"
)

type Room struct {
	RoomID       uuid.UUID                    `json:"room_id"`
	Name         string                       `json:"name"`
	OfferStr     string                       `json:"offer_str"`
	Participants []participantPkg.Participant `json:"participants"`
	StartedAt    int64                        `json:"started_at"`
	Active       []uuid.UUID                  `json:"active"`
}


