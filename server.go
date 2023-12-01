package main

import (
	"bytes"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strconv"

	"remote_reality/modules/site"

	"github.com/gorilla/mux"
)

// Получение IP, по которому можно подключиться к серверу
func GetLocalIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddress := conn.LocalAddr().(*net.UDPAddr)

	return localAddress.IP
}

// Сервер, на котором вся магия и работает
func InitServer(addr string, li chan *bytes.Buffer) {

	router := mux.NewRouter()

	router.HandleFunc("/", site.HomePage)
	router.HandleFunc("/stop", site.Stop)
	router.HandleFunc("/script.js", site.HomePage)
	router.HandleFunc("/blank.png", site.HomePage)
	router.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
		log.Println("connect from", r.RemoteAddr, r.URL)

		const boundary = `frame`
		w.Header().Set("Content-Type", `multipart/x-mixed-replace;boundary=`+boundary)
		multipartWriter := multipart.NewWriter(w)
		if err := multipartWriter.SetBoundary(boundary); err != nil {
			log.Println(err)
			return
		}
		for {
			img := <-li
			image := img.Bytes()
			iw, err := multipartWriter.CreatePart(textproto.MIMEHeader{
				"Content-type":   []string{"image/jpeg"},
				"Content-length": []string{strconv.Itoa(len(image))},
			})
			if err != nil {
				log.Println(err)
				return
			}
			_, err = iw.Write(image)
			if err != nil {
				log.Println(err)
				return
			}
		}
	})
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe(addr, nil))
}
