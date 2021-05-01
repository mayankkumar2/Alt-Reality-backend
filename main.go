package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	participantPkg "github.com/TeamRekursion/Alt-Reality-backend/models/participant"
	roomPkg "github.com/TeamRekursion/Alt-Reality-backend/models/room"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var redisSafeUp sync.Mutex

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

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
			redisSafeUp.Lock()
			roomCmd := redisClient.Get(context.Background(), key)
			var room roomPkg.Room
			err = roomCmd.Scan(&room)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				redisSafeUp.Unlock()
				return
			}
			p := participantPkg.CreateParticipant()
			room.AddParticipants(p)
			err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				redisSafeUp.Unlock()
				return
			}
			redisSafeUp.Unlock()

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
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id or participant_id is not a valid UUID",
			})
			_ = c.Close()
			return
		}
		key := initPayload.RoomID.String() + ":Room"

		redisSafeUp.Lock()
		roomCmd := redisClient.Get(context.Background(), key)

		var room roomPkg.Room
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			_ = c.Close()
			redisSafeUp.Unlock()
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
			redisSafeUp.Unlock()
			return
		}

		room.AddActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		redisSafeUp.Unlock()

		_ = c.WriteJSON(map[string]interface{}{
			"message": "user set as active",
		})

		channelKey := key + ":CHANNEL"
		msg := redisClient.Subscribe(context.Background(), channelKey).Channel()
		var breakFlag = false
		for {
			select {
			case m := <-msg:
				err = c.WriteMessage(1, []byte(m.Payload))
				if err != nil {
					breakFlag = true
				}
			case _ = <-r.Context().Done():
				breakFlag = true
			}
			if breakFlag {
				break
			}
		}
		redisSafeUp.Lock()
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		room.RemoveActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		redisSafeUp.Unlock()

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
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id or participant_id is not a valid UUID",
			})
			_ = c.Close()
			return
		}
		key := initPayload.RoomID.String() + ":Room"
		roomCmd := redisClient.Get(context.Background(), key)
		redisSafeUp.Lock()
		var room roomPkg.Room
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			fmt.Println(err)
			_ = c.Close()
			redisSafeUp.Unlock()
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
			redisSafeUp.Unlock()
			return
		}
		room.AddActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		redisSafeUp.Unlock()


		_ = c.WriteJSON(map[string]interface{}{
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
			_ = json.NewEncoder(&b).Encode(map[string] interface{}{
				"co_ordinates": message,
				"participant_id": initPayload.ParticipantID,
			})
			_ = redisClient.Publish(r.Context(), channelKey, string(b.Bytes())).Err()
			go HandlePositionUpdates(initPayload.RoomID, initPayload.ParticipantID, message.AtX, message.AtY)
		}
		redisSafeUp.Lock()
		err = roomCmd.Scan(&room)
		if err != nil {
			_ = c.WriteJSON(map[string]interface{}{
				"error":   true,
				"message": "room_id doesn't exist",
			})
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		room.RemoveActiveParticipant(initPayload.ParticipantID)
		err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
		if err != nil {
			_ = c.Close()
			redisSafeUp.Unlock()
			return
		}
		redisSafeUp.Unlock()


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
				"body":    room,
			})

		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	corsMiddleware := cors.Default()
	loggerMiddleware := handlers.LoggingHandler(os.Stdout, corsMiddleware.Handler(router))
	log.Println("Binding to.. 0.0.0.0:" + os.Getenv("PORT"))
	err = http.ListenAndServe("0.0.0.0:" + os.Getenv("PORT"), loggerMiddleware)
	if err != nil {
		log.Fatalln("Error occurred in binding to port " + os.Getenv("PORT"))
	}
}

func HandlePositionUpdates(rID uuid.UUID, participantID uuid.UUID, AtX float64, AtY float64) {
	key := rID.String() + ":Room"
	var room roomPkg.Room

	redisSafeUp.Lock()
	defer redisSafeUp.Unlock()
	roomCmd := redisClient.Get(context.Background(), key)
	err := roomCmd.Scan(&room)
	if err != nil {
		return
	}
	for i, e := range room.Participants {
		if e.ParticipantID == participantID {
			room.Participants[i].AtX = AtX
			room.Participants[i].AtY = AtY
		}
	}
	err = redisClient.Set(context.Background(), key, room, time.Hour).Err()
	if err != nil {
		return
	}
}