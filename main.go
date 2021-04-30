package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/TeamRekursion/Alt-Reality-backend/participant"
	roomPkg "github.com/TeamRekursion/Alt-Reality-backend/room"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{}
var redisClient *redis.Client

func main() {
	// TODO: Put content-type checks
	// TODO: Code refactor to hexagon
	// TODO: move to .env
	redisClient = redis.NewClient(&redis.Options{
		Addr: "0.0.0.0:6379",
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		log.Fatalln("error connecting to redis database:", err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/rooms/create", func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost == r.Method {

			reqBody := struct {
				OfferStr string `json:"offer_str"`
				Name     string `json:"name"`
			}{}

			err := json.NewDecoder(r.Body).Decode(&reqBody)
			if err != nil || reqBody.Name == "" || reqBody.OfferStr == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			room := roomPkg.CreateRoom(reqBody.Name, reqBody.OfferStr)

			key := room.RoomID.String() + ":Room"

			err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_ =json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "created a room",
				"body":    room,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	router.HandleFunc("/rooms/join", func(w http.ResponseWriter, r *http.Request) {

		if http.MethodPost == r.Method {
			reqBody := struct {
				RoomID uuid.UUID `json:"room_id"`
			}{}

			err := json.NewDecoder(r.Body).Decode(&reqBody)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			key := reqBody.RoomID.String() + ":Room"
			roomCmd := redisClient.Get(context.Background(), key)

			var room roomPkg.Room
			err = roomCmd.Scan(&room)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			p := participant.CreateParticipant()
			room.AddParticipants(p)
			err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "joined a room",
				"body": map[string]interface{}{
					"participant": p,
					"room":        room,
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	router.HandleFunc("/rooms/broadcast/receive", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		initPayload := struct {
			RoomID        uuid.UUID `json:"room_id"`
			ParticipantID uuid.UUID `json:"participant_id"`
		}{}

		err = c.ReadJSON(&initPayload)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{} {
				"error": true,
				"message": "room_id or participant_id is not a valid UUID",
			})
			_ = c.Close()
			return
		}
		key := initPayload.RoomID.String() + ":Room"
		roomCmd := redisClient.Get(context.Background(), key)

		var room roomPkg.Room
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			_ = c.Close()
			return
		}

		var doesExist = false
		for _, e := range room.Participants {
			if e.ParticipantID == initPayload.ParticipantID {
				doesExist = true
			}
		}

		if !doesExist {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "you must join the channel first",
			})
			_ = c.Close()
			return
		}


		room.AddActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			return
		}

		_ = c.WriteJSON(map[string]interface{} {
			"message": "user set as active",
		})

		channelKey := key + ":CHANNEL"
		msg := redisClient.Subscribe(context.Background(), channelKey).Channel()
		var breakFlag = false
		for {
			select {
			case m := <- msg:
				err = c.WriteMessage(1, []byte(m.Payload))
				if err != nil {
					breakFlag = true
				}
			case _ = <- r.Context().Done():
				breakFlag = true
			}
			if breakFlag {
				break
			}
		}
		room.RemoveActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			return
		}
	})
	router.HandleFunc("/rooms/broadcast/send", func(w http.ResponseWriter, r *http.Request) {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		initPayload := struct {
			RoomID        uuid.UUID `json:"room_id"`
			ParticipantID uuid.UUID `json:"participant_id"`
		}{}

		err = c.ReadJSON(&initPayload)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{} {
				"error": true,
				"message": "room_id or participant_id is not a valid UUID",
			})
			_ = c.Close()
			return
		}
		key := initPayload.RoomID.String() + ":Room"
		roomCmd := redisClient.Get(context.Background(), key)

		var room roomPkg.Room
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			_ = c.Close()
			return
		}

		var doesExist = false
		for _, e := range room.Participants {
			if e.ParticipantID == initPayload.ParticipantID {
				doesExist = true
			}
		}

		if !doesExist {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "you must join the channel first",
			})
			_ = c.Close()
			return
		}


		room.AddActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			return
		}

		_ = c.WriteJSON(map[string]interface{} {
			"message": "user set as active",
		})


		channelKey := key + ":CHANNEL"
		for {
			message := struct {
				AtX float64 `json:"at_x"`
				AtY float64 `json:"at_y"`
			}{}
			err = c.ReadJSON(&message)
			if err != nil {
				break
			}

			var b bytes.Buffer
			_ = json.NewEncoder(&b).Encode(message)
			_ = redisClient.Publish(r.Context(), channelKey, string(b.Bytes())).Err()
		}

		room.RemoveActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			return
		}



	})

	router.HandleFunc("/rooms/info", func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost == r.Method {
			reqBody := struct {
				RoomID uuid.UUID `json:"room_id"`
			}{}

			err = json.NewDecoder(r.Body).Decode(&reqBody)

			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			key := reqBody.RoomID.String() + ":Room"
			roomCmd := redisClient.Get(context.Background(), key)

			var room roomPkg.Room
			err = roomCmd.Scan(&room)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   false,
				"message": "team info",
				"body": room,
			})


		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	err = http.ListenAndServe("0.0.0.0:8080", router)
	if err != nil {
		log.Fatalln("Error occurred in binding to port 8080")
	}
}
