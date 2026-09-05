# Microcontrollers C/C++ — Arduino/ESP32/STM32

> **Version**: 1.0.0 | **Stack**: C17/C++20, FreeRTOS, Arduino, ESP-IDF, PlatformIO

## ESP32 + FreeRTOS

```cpp
// main.cpp — ESP32 WiFi + MQTT sensor with FreeRTOS tasks
#include <WiFi.h>
#include <PubSubClient.h>
#include <DHT.h>

#define DHT_PIN 4
#define DHT_TYPE DHT22

DHT dht(DHT_PIN, DHT_TYPE);
WiFiClient wifiClient;
PubSubClient mqtt(wifiClient);

TaskHandle_t sensorTaskHandle;

void sensorTask(void *pvParameters) {
  TickType_t lastWake = xTaskGetTickCount();
  for (;;) {
    float temp = dht.readTemperature();
    float hum = dht.readHumidity();

    if (!isnan(temp) && !isnan(hum)) {
      char payload[64];
      snprintf(payload, sizeof(payload), "{\"temp\":%.1f,\"hum\":%.1f}", temp, hum);
      mqtt.publish("sensors/dht22", payload);
    }
    vTaskDelayUntil(&lastWake, pdMS_TO_TICKS(5000)); // 5s interval
  }
}

void setup() {
  Serial.begin(115200);
  dht.begin();
  WiFi.begin("SSID", "PASSWORD");

  xTaskCreatePinnedToCore(sensorTask, "sensor", 4096, NULL, 1, &sensorTaskHandle, 0);
}

void loop() {
  if (!mqtt.connected()) mqtt.connect("esp32-sensor");
  mqtt.loop();
}
```

## Safety Patterns

```cpp
// Watchdog timer — reset if task hangs
#include <esp_task_wdt.h>
esp_task_wdt_init(10, true);  // 10 second timeout
esp_task_wdt_add(sensorTaskHandle);

// Critical sections with mutex for shared resources
portMUX_TYPE mux = portMUX_INITIALIZER_UNLOCKED;
portENTER_CRITICAL(&mux);
// ... protected code ...
portEXIT_CRITICAL(&mux);

// Brown-out detection: ESP32 has built-in BOD — enable in menuconfig
```

## Security

```bash
pio run -t upload    # PlatformIO build + flash
pio test             # Run unit tests
```

- No hardcoded WiFi credentials — use `Preferences` or LittleFS config file
- OTA updates over HTTPS with certificate verification
- Firmware signing with ECDSA — reject unsigned updates
