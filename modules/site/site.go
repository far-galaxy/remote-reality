package site

import (
	"log"
	"net/http"
	"os"
)

// Хомяк
func HomePage(w http.ResponseWriter, r *http.Request) {
	log.Println("connect from", r.RemoteAddr, r.URL)
	var file []byte
	var err error
	if r.URL.String() == "/" {
		file, err = os.ReadFile("./modules/site/index.html")
		if err != nil {
			http.Error(w, "Error reading index", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")

	} else if r.URL.String() == "/script.js" {
		file, err = os.ReadFile("./modules/site/script.js")
		if err != nil {
			http.Error(w, "Error reading script", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")

	}
	if _, err := w.Write(file); err != nil {
		log.Println("error:", err)
	}

}

func Stop(w http.ResponseWriter, r *http.Request) {
	log.Println("stop from", r.RemoteAddr)
	os.Exit(0)
}
