package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Hello struct {
	ID   string
	Name string
}

func (h Hello) MarshalBinary() ([]byte, error) {
	return json.Marshal(h)
}

func (h *Hello) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, h)
}

type CreateHelloRequestDTO struct {
	Name string `json:"name" validate:"required,min=3"`
}

func (h *CreateHelloRequestDTO) toHello() Hello {
	return Hello{
		Name: h.Name,
	}
}

type HelloResponseDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h Hello) ToHelloResponseDTO() HelloResponseDTO {
	return HelloResponseDTO{
		ID:   h.ID,
		Name: h.Name,
	}
}

type ErrorResponseDTO struct {
	Message string `json:"message"`
}

var (
	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"), //"localhost:6379"
		Password: "",                     // no password set
		DB:       0,                      // use default DB
	})
	Validate = validator.New()
)

const (
	HelloKey = "hello:"
)

var (
	ErrCannotCreateHello        = errors.New("failed to create hello in db")
	ErrCannotGetHello           = errors.New("failed to get hello from db")
	ErrCannotUnmarshalHello     = errors.New("failed to unmarshal hello")
	ErrCannotMarshalHello       = errors.New("failed to marshal hello")
	ErrCannotMarshalHelloBinary = errors.New("failed to marshal hello binary")
	ErrHelloNotFound            = errors.New("hello not found")
)

func main() {
	setRedis()
	slog.SetLogLoggerLevel(slog.LevelDebug)
	server := http.NewServeMux()
	server.Handle("GET /hellos/{id}", http.HandlerFunc(getHelloByID))
	server.Handle("GET /hellos", http.HandlerFunc(getHelloList))
	server.Handle("POST /hellos", http.HandlerFunc(setHello))
	slog.Info("Server start at :45000")
	http.ListenAndServe(":45000", server)
}

func setRedis() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
}

func shouldBindJSON(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	if err := Validate.Struct(v); err != nil {
		return err
	}
	return nil
}

func getHelloByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	val, err := rdb.Get(r.Context(), HelloKey+id).Result()
	if err != nil {
		if err == redis.Nil {
			responseNotFound(w, errors.Join(ErrCannotGetHello, err, errors.New("id: "+id)))
			return
		}
		responseInternalServerError(w, err)
		return
	}
	var res Hello
	err = json.Unmarshal([]byte(val), &res)
	if err != nil {
		responseInternalServerError(w, err)
		return
	}
	responseOK(w, res.ToHelloResponseDTO())
}

func getHelloList(w http.ResponseWriter, r *http.Request) {
	keys, err := rdb.Keys(r.Context(), HelloKey+"*").Result()
	if err != nil {
		if err == redis.Nil {
			responseNoContent(w)
			return
		}
		responseInternalServerError(w, err)
		return
	}
	var hellos []Hello
	for _, id := range keys {
		val, err := rdb.Get(r.Context(), id).Result()
		if err != nil {
			responseInternalServerError(w, errors.Join(ErrHelloNotFound, errors.New("id: "+id)))
			return
		}
		var hello Hello
		err = json.Unmarshal([]byte(val), &hello)
		if err != nil {
			responseInternalServerError(w, err)
			return
		}
		hellos = append(hellos, hello)
	}
	var helloResponseList []HelloResponseDTO
	for _, hello := range hellos {
		helloResponseList = append(helloResponseList, hello.ToHelloResponseDTO())
	}
	responseOK(w, helloResponseList)
}

func setHello(w http.ResponseWriter, r *http.Request) {
	var createHelloRequestDTO CreateHelloRequestDTO
	if err := shouldBindJSON(r, &createHelloRequestDTO); err != nil {
		responseBadRequest(w, err)
		return
	}
	hello := createHelloRequestDTO.toHello()
	hello.ID = uuid.NewString()
	newHello, err := storeCreateHello(r.Context(), hello)
	if err != nil {
		responseInternalServerError(w, err)
		return
	}
	responseOK(w, newHello.ToHelloResponseDTO())
}

func storeCreateHello(ctx context.Context, hello Hello) (Hello, error) {
	helloBin, err := hello.MarshalBinary()
	if err != nil {
		return hello, errors.Join(ErrCannotMarshalHelloBinary, err)
	}
	err = rdb.Set(ctx, HelloKey+hello.ID, helloBin, 0).Err()
	if err != nil {
		return hello, errors.Join(ErrCannotCreateHello, err)
	}
	rs, err := rdb.Get(ctx, HelloKey+hello.ID).Result()
	if err != nil {
		return hello, errors.Join(ErrCannotGetHello, err)
	}
	var newHello Hello
	err = json.Unmarshal([]byte(rs), &newHello)
	if err != nil {
		return newHello, errors.Join(ErrCannotUnmarshalHello, err)
	}
	return newHello, nil
}

func responseOK(w http.ResponseWriter, i interface{}) {
	responseJSON, err := json.Marshal(i)
	if err != nil {
		responseInternalServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/json")
	w.Write(responseJSON)
}

func responseErr(w http.ResponseWriter, statusCode int, err error) {
	_, file, line, _ := runtime.Caller(2)
	slog.Error(fmt.Sprint("file: ", file, ":", line, ": ", err.Error()))
	var e ErrorResponseDTO
	if err != nil {
		e.Message = err.Error()
	}
	resp, _ := json.Marshal(e)
	w.WriteHeader(statusCode)
	w.Header().Add("Content-Type", "application/json")
	w.Write(resp)
}

func responseBadRequest(w http.ResponseWriter, err error) {
	responseErr(w, http.StatusBadRequest, err)
}

func responseNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func responseNotFound(w http.ResponseWriter, err error) {
	responseErr(w, http.StatusNotFound, err)
}

func responseInternalServerError(w http.ResponseWriter, err error) {
	responseErr(w, http.StatusInternalServerError, err)
}
