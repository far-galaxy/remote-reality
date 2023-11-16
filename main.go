package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/blackjack/webcam"
	"github.com/gorilla/mux"
	"github.com/stianeikeland/go-rpio/v4"
)

func servo(pin *rpio.Pin, angle int16) {
	dutyCycle := uint32((float32(angle) / 180.0 * 100.0) + 100.0)
	//fmt.Println(dutyCycle, 1000000.0/50.0*(dutyCycle/2000.0))
	pin.DutyCycle(dutyCycle, 2000)
}

const (
	V4L2_PIX_FMT_PJPG = 0x47504A50
	V4L2_PIX_FMT_YUYV = 0x56595559
)

type FrameSizes []webcam.FrameSize

func (slice FrameSizes) Len() int {
	return len(slice)
}

// For sorting purposes
func (slice FrameSizes) Less(i, j int) bool {
	ls := slice[i].MaxWidth * slice[i].MaxHeight
	rs := slice[j].MaxWidth * slice[j].MaxHeight
	return ls < rs
}

// For sorting purposes
func (slice FrameSizes) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

var supportedFormats = map[webcam.PixelFormat]bool{
	V4L2_PIX_FMT_PJPG: true,
	V4L2_PIX_FMT_YUYV: true,
}

var xServo, yServo rpio.Pin
var isRaspberry = true

func main() {
	dev := flag.String("d", "/dev/video0", "video device to use")
	fmtstr := flag.String("f", "", "video format to use, default first supported")
	szstr := flag.String("s", "", "frame size to use, default largest one")
	addr := flag.String("l", ":8000", "addr to listien")
	fps := flag.Bool("p", false, "print fps info")
	flag.Parse()

	cam, err := webcam.Open(*dev)
	if err != nil {
		panic(err.Error())
	}
	defer cam.Close()

	// select pixel format
	format_desc := cam.GetSupportedFormats()

	fmt.Println("Available formats:")
	for _, s := range format_desc {
		fmt.Fprintln(os.Stderr, s)
	}

	var format webcam.PixelFormat
FMT:
	for f, s := range format_desc {
		if *fmtstr == "" {
			if supportedFormats[f] {
				format = f
				break FMT
			}

		} else if *fmtstr == s {
			if !supportedFormats[f] {
				log.Println(format_desc[f], "format is not supported, exiting")
				return
			}
			format = f
			break
		}
	}
	if format == 0 {
		log.Println("No format found, exiting")
		return
	}

	// select frame size
	frames := FrameSizes(cam.GetSupportedFrameSizes(format))
	sort.Sort(frames)

	fmt.Fprintln(os.Stderr, "Supported frame sizes for format", format_desc[format])
	for _, f := range frames {
		fmt.Fprintln(os.Stderr, f.GetString())
	}
	var size *webcam.FrameSize
	if *szstr == "" {
		size = &frames[len(frames)-1]
	} else {
		for _, f := range frames {
			if *szstr == f.GetString() {
				size = &f
				break
			}
		}
	}
	if size == nil {
		log.Println("No matching frame size, exiting")
		return
	}

	fmt.Fprintln(os.Stderr, "Requesting", format_desc[format], size.GetString())
	f, w, h, err := cam.SetImageFormat(format, uint32(size.MaxWidth), uint32(size.MaxHeight))
	if err != nil {
		log.Println("SetImageFormat return error", err)
		return

	}
	fmt.Fprintf(os.Stderr, "Resulting image format: %s %dx%d\n", format_desc[f], w, h)

	// start streaming
	err = cam.StartStreaming()
	if err != nil {
		log.Println(err)
		return
	}

	var (
		li   chan *bytes.Buffer = make(chan *bytes.Buffer)
		fi   chan []byte        = make(chan []byte)
		back chan struct{}      = make(chan struct{})
	)
	go encodeToImage(cam, back, fi, li, w, h, f)
	go httpVideo(*addr, li)

	if err := rpio.Open(); err != nil {
		if err.Error() == "open /dev/gpiomem: no such file or directory" {
			isRaspberry = false
			fmt.Println("It is hot Raspberry, Servo disable")
		} else {
			log.Fatal(err)
		}
	}
	defer rpio.Close()

	if isRaspberry {
		fmt.Println("It is Raspberry")
		xServo = rpio.Pin(18)
		xServo.Mode(rpio.Pwm)
		xServo.Freq(50 * 2000)
		servo(&xServo, 90)

		yServo = rpio.Pin(19)
		yServo.Mode(rpio.Pwm)
		yServo.Freq(50 * 2000)
		servo(&yServo, 90)
	}

	timeout := uint32(5) //5 seconds
	start := time.Now()
	var fr time.Duration

	for {
		err = cam.WaitForFrame(timeout)
		if err != nil {
			log.Println(err)
			return
		}

		switch err.(type) {
		case nil:
		case *webcam.Timeout:
			log.Println(err)
			continue
		default:
			log.Println(err)
			return
		}

		frame, err := cam.ReadFrame()
		if err != nil {
			log.Println(err)
			return
		}
		if len(frame) != 0 {

			// print framerate info every 10 seconds
			fr++
			if *fps {
				if d := time.Since(start); d > time.Second*10 {
					fmt.Println(float64(fr)/(float64(d)/float64(time.Second)), "fps")
					start = time.Now()
					fr = 0
				}
			}

			select {
			case fi <- frame:
				<-back
			default:
			}
		}
	}
}

func encodeToImage(wc *webcam.Webcam, back chan struct{}, fi chan []byte, li chan *bytes.Buffer, w, h uint32, format webcam.PixelFormat) {

	var (
		frame []byte
		img   image.Image
	)
	for {
		bframe := <-fi
		// copy frame
		if len(frame) < len(bframe) {
			frame = make([]byte, len(bframe))
		}
		copy(frame, bframe)
		back <- struct{}{}

		switch format {
		case V4L2_PIX_FMT_YUYV:
			yuyv := image.NewYCbCr(image.Rect(0, 0, int(w), int(h)), image.YCbCrSubsampleRatio422)
			for i := range yuyv.Cb {
				ii := i * 4
				yuyv.Y[i*2] = frame[ii]
				yuyv.Y[i*2+1] = frame[ii+2]
				yuyv.Cb[i] = frame[ii+1]
				yuyv.Cr[i] = frame[ii+3]

			}
			img = yuyv
		default:
			log.Fatal("invalid format ?")
		}
		//convert to jpeg
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, img, nil); err != nil {
			log.Fatal(err)
			return
		}

		const N = 50
		// broadcast image up to N ready clients
		nn := 0
	FOR:
		for ; nn < N; nn++ {
			select {
			case li <- buf:
			default:
				break FOR
			}
		}
		if nn == 0 {
			li <- buf
		}

	}
}

// DeviceData структура для разбора данных, полученных от клиента
type DeviceData struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Gamma float64 `json:"gamma"`
}

func home(w http.ResponseWriter, r *http.Request) {
	htmlContent, err := os.ReadFile("./site/index.html")
	if err != nil {
		http.Error(w, "Error reading HTML file", http.StatusInternalServerError)
		return
	}

	// Отправка HTML-страницы в качестве ответа
	w.Header().Set("Content-Type", "text/html")
	w.Write(htmlContent)
}

func limit(angle *float64) bool {
	isOver := false
	if *angle < 0 {
		*angle = 0
		isOver = true
	}
	if *angle > 180 {
		*angle = 180
		isOver = true
	}
	return isOver
}

func httpVideo(addr string, li chan *bytes.Buffer) {

	router := mux.NewRouter()

	err := http.Dir("./site/index.html")
	log.Println(err)

	router.HandleFunc("/", home)
	router.HandleFunc("/orient", func(w http.ResponseWriter, r *http.Request) {
		log.Println("orient", r.RemoteAddr, r.URL)
		// Разбор данных из тела запроса
		var deviceData DeviceData
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&deviceData); err != nil {
			log.Println(err)
			return
		}

		var X, Y float64
		if deviceData.Gamma > 0 {
			Y = deviceData.Gamma
			X = 270.0 - deviceData.Alpha
		} else {
			Y = 180.0 + deviceData.Gamma
			if deviceData.Alpha < 180.0 {
				X = 90.0 - deviceData.Alpha
			} else {
				X = 360.0 - deviceData.Alpha + 90.0
			}
		}
		lim := limit(&X)
		lim = lim || limit(&Y)
		if isRaspberry {
			servo(&xServo, int16(X))
			servo(&yServo, int16(Y))
		}

		// Вывод данных в консоль
		fmt.Printf("Получены данные: Alpha=%f, Beta=%f, Gamma=%f\n", X, deviceData.Beta, Y)

		// Отправка успешного ответа
		if lim {
			w.WriteHeader(210)
		} else {
			w.WriteHeader(http.StatusOK)
		}

	})
	router.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		log.Println("connect from", r.RemoteAddr, r.URL)
		/*if r.URL.Path != "/a" {
			http.NotFound(w, r)
			return
		}*/

		//remove stale image
		<-li
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
	//log.Fatal(http.ListenAndServe(":8000", nil))
	log.Fatal(http.ListenAndServeTLS(":443", "server.crt", "server.key", nil))
}
