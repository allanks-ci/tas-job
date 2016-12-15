package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

var fatalLog = log.New(os.Stdout, "FATAL: ", log.LstdFlags)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))
	fatalLog.Fatal(http.ListenAndServe(":8080", r))
}
