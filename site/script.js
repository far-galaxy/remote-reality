var start = Date.now();
const timeout = 0; // Временной интервал между снятиями показаний

function sendData(deviceData) {

    fetch('/orient', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(deviceData)
        })
        .then(response => {
            if (response.status === 210) {
                try {
                    navigator.vibrate(200);
                } catch (err) {
                    document.querySelector("#debug").innerHTML = err.message
                }
            } else if (!response.ok) {
                throw new Error('Error');
            }
            return response.json();
        })
        .then(data => {
            console.log('Answer', data);
        })
        .catch(error => {
            console.error('Error:', error.message);
        });
}

// Передача ориентации устройства
function handleDeviceOrientation(event) {
    now = Date.now();
    if (now - start > timeout) {
        start = now
        var deviceData = {
            alpha: event.alpha,
            beta: event.beta,
            gamma: event.gamma
        };

        // Используем наклон головы для подстройки параметров
        //document.querySelector("img").style["margin-right"] = (event.gamma*3)+"px"

        sendData(deviceData);
    }
}

try {
    window.addEventListener('deviceorientation', handleDeviceOrientation);
} catch (err) {
    document.querySelector("#debug").innerHTML = err.message
}

function stop() {
    fetch('/stop', {method: 'GET',})
}