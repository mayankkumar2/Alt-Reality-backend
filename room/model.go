package room

import (
	"encoding/json"
	"github.com/TeamRekursion/Alt-Reality-backend/participant"
	"github.com/google/uuid"
	"time"
)

type Room struct {
	RoomID       uuid.UUID                 `json:"room_id"`
	Name         string                    `json:"name"`
	OfferStr     string                    `json:"offer_str"`
	Participants []participant.Participant `json:"participants"`
	StartedAt    int64                     `json:"started_at"`
	Active       []uuid.UUID               `json:"active"`
}

func (r Room) MarshalBinary() (data []byte, err error) {
	data, err = json.Marshal(r)
	return data, err
}
func (r *Room) UnmarshalBinary(data []byte) (err error) {
	err = json.Unmarshal(data, r)
	return err
}
func (r *Room) AddParticipants(p ...participant.Participant) {
	if r.Participants == nil {
		r.Participants = make([]participant.Participant, 0, len(p))
	}
	r.Participants = append(r.Participants, p...)
}

func (r *Room) AddActiveParticipant(p ...uuid.UUID) {
	if r.Active == nil {
		r.Active = make([]uuid.UUID, 0, len(p))
	}
	r.Active = append(r.Active, p...)
}

func (r *Room) RemoveActiveParticipant(pID uuid.UUID) {
	f := make([]uuid.UUID, 0, len(r.Active))
	for _, e := range r.Active {
		if e != pID {
			f = append(f, e)
		}
	}
	r.Active = f
}

func CreateRoom(n string, offer string) Room {
	uID := uuid.New()
	ts := time.Now().Unix()
	p := make([]participant.Participant, 0, 100)
	ac := make([]uuid.UUID, 0, 100)
	return Room{
		RoomID:       uID,
		Name:         n,
		OfferStr:     offer,
		StartedAt:    ts,
		Participants: p,
		Active:       ac,
	}
}
