package servo

import (
	"log"

	"github.com/stianeikeland/go-rpio/v4"
)

// Сервопривод
type Servo struct {
	pin   *rpio.Pin // Пин управляющего сигнала
	angle int16     // Текущий угол
}

// Список пинов Raspberry Pi, имеющих поддержку ШИМ
// TODO: проверять, что не включены 12 и 18 или 13 и 19 одновременно
var supportPWM = map[int]bool{
	12: true,
	13: true,
	18: true,
	19: true,
}

const (
	ServoFreq  = 50   // Частота ШИМ, Гц
	ServoCycle = 2000 // Число разбиений
)

// Инициализация сервопривода
func (servo *Servo) Init(pinNumber int) {
	if !supportPWM[pinNumber] {
		log.Fatalf("Pin %d is not PWM! Check another", pinNumber)
		return
	}

	pin := rpio.Pin(18)
	pin.Mode(rpio.Pwm)
	pin.Freq(ServoFreq * ServoCycle)
	servo.pin = &pin
	servo.Set(90)
	servo.angle = 90
}

// Установка сервопривода в указанный угол (0-180 град.)
//
// При выходе за пределы возвращает true
func (servo *Servo) Set(angle int16) bool {
	servo.angle = angle
	isOver := LimitAngle(&angle)
	dutyCycle := uint32((float32(angle) / 180.0 * 100.0) + 100.0)
	servo.pin.DutyCycle(dutyCycle, 2000)

	return isOver
}

// Ограничение угла (0-180 град.)
//
// При выходе за пределы возвращает true
func LimitAngle(angle *int16) bool {
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
