package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"

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
func InitServer(addr string, tls bool, li chan *bytes.Buffer) {

	router := mux.NewRouter()

	router.HandleFunc("/", HomePage)
	router.HandleFunc("/script.js", HomePage)
	router.HandleFunc("/orient", Orientation)
	router.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
		log.Println("connect from", r.RemoteAddr, r.URL)

		const boundary = `frame`
		w.Header().Set("Content-Type", `multipart/x-mixed-replace;boundary=`+boundary)
		multipartWriter := multipart.NewWriter(w)
		multipartWriter.SetBoundary(boundary)
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
	if tls {
		log.Fatal(http.ListenAndServeTLS(":443", "server.crt", "server.key", nil))
	} else {
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}

type DeviceOrientation struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Gamma float64 `json:"gamma"`
}

// Поворот повортоной головы
func Orientation(w http.ResponseWriter, r *http.Request) {
	log.Println("send orientation from", r.RemoteAddr)
	var deviceData DeviceOrientation
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&deviceData); err != nil {
		log.Println(err)
		return
	}

	var X, Y int16
	if deviceData.Gamma > 0 {
		Y = int16(deviceData.Gamma)
		X = 270 - int16(deviceData.Alpha)
	} else {
		Y = 180 + int16(deviceData.Gamma)
		if deviceData.Alpha < 180 {
			X = 90 - int16(deviceData.Alpha)
		} else {
			X = 360 - int16(deviceData.Alpha) + 90
		}
	}

	lim := false
	if isRaspberry {
		lim = xServo.Set(X)
		lim = lim || yServo.Set(Y)
	}

	fmt.Printf(
		"A=%f, B=%f, G=%f, X=%d, Y=%d\n",
		deviceData.Alpha, deviceData.Beta, deviceData.Gamma, X, Y,
	)

	// Передача кода 210, если вылезли за пределы угла поворота (для включения вибрации)
	// TODO: сделать это отправкой нормального пакета
	if lim {
		w.WriteHeader(210)
	} else {
		w.WriteHeader(http.StatusOK)
	}

}

// Хомяк
func HomePage(w http.ResponseWriter, r *http.Request) {
	log.Println("connect from", r.RemoteAddr, r.URL)
	var file []byte
	var err error
	if r.URL.String() == "/" {
		file, err = os.ReadFile("./site/index.html")
		if err != nil {
			http.Error(w, "Error reading index", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")

	} else if r.URL.String() == "/script.js" {
		file, err = os.ReadFile("./site/script.js")
		if err != nil {
			http.Error(w, "Error reading script", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")

	}
	w.Write(file)

}
