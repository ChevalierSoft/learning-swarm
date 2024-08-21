package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"), //"localhost:6379"
		Password: "",                     // no password set
		DB:       0,                      // use default DB
	})
	uid = 0
)

func main() {
	setRedis()
	server := http.NewServeMux()
	server.Handle("GET /hello/{id}", http.HandlerFunc(getHello))
	server.Handle("POST /hello", http.HandlerFunc(setHello))
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

func getHello(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	val, err := rdb.Get(r.Context(), id).Result()
	if err != nil {
		w.Write([]byte(err.Error()))
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Hello " + val + " !"))
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}

func setHello(w http.ResponseWriter, r *http.Request) {
	storeHello(r)
	w.Write([]byte("registered !"))
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}

func storeHello(r *http.Request) {
	b := make([]byte, r.ContentLength)
	r.Body.Read(b)
	fmt.Println("Request Body: ", string(b))
	err := rdb.Set(r.Context(), strconv.Itoa(uid), string(b), 0).Err()
	if err != nil {
		panic(err)
	}
	uid++
}
