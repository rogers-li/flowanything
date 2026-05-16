package mockbusiness

import (
	"net/http"
	"strings"
	"time"

	"flow-anything/internal/platform/kernel/httpserver"
)

type weatherSnapshot struct {
	City         string
	Country      string
	Condition    string
	TemperatureC float64
	Humidity     int
	WindKPH      float64
}

// RegisterRoutes installs deterministic mock business endpoints for local
// connector debugging and integration tests.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /orders/{order_id}", func(w http.ResponseWriter, r *http.Request) {
		orderID := r.PathValue("order_id")
		if orderID == "" {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_argument",
					"message": "order_id is required",
				},
			})
			return
		}

		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"order_id": orderID,
			"status":   "paid",
			"amount":   199.0,
			"currency": "CNY",
		})
	})

	mux.HandleFunc("GET /weather/current", func(w http.ResponseWriter, r *http.Request) {
		city := strings.TrimSpace(r.URL.Query().Get("city"))
		if city == "" {
			httpserver.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "invalid_argument",
					"message": "city is required",
				},
			})
			return
		}

		snapshot := lookupWeather(city)
		httpserver.WriteJSON(w, http.StatusOK, map[string]any{
			"city":          snapshot.City,
			"country":       snapshot.Country,
			"condition":     snapshot.Condition,
			"temperature_c": snapshot.TemperatureC,
			"humidity":      snapshot.Humidity,
			"wind_kph":      snapshot.WindKPH,
			"updated_at":    time.Now().UTC().Format(time.RFC3339),
			"source":        "mock-business-api",
		})
	})
}

func lookupWeather(city string) weatherSnapshot {
	normalized := strings.ToLower(strings.TrimSpace(city))
	normalized = strings.ReplaceAll(normalized, "市", "")
	known := map[string]weatherSnapshot{
		"深圳": {
			City: "深圳", Country: "CN", Condition: "多云", TemperatureC: 27.5, Humidity: 72, WindKPH: 13.2,
		},
		"shenzhen": {
			City: "Shenzhen", Country: "CN", Condition: "Cloudy", TemperatureC: 27.5, Humidity: 72, WindKPH: 13.2,
		},
		"北京": {
			City: "北京", Country: "CN", Condition: "晴", TemperatureC: 22.0, Humidity: 38, WindKPH: 9.4,
		},
		"beijing": {
			City: "Beijing", Country: "CN", Condition: "Sunny", TemperatureC: 22.0, Humidity: 38, WindKPH: 9.4,
		},
		"上海": {
			City: "上海", Country: "CN", Condition: "小雨", TemperatureC: 20.8, Humidity: 81, WindKPH: 16.5,
		},
		"shanghai": {
			City: "Shanghai", Country: "CN", Condition: "Light rain", TemperatureC: 20.8, Humidity: 81, WindKPH: 16.5,
		},
		"广州": {
			City: "广州", Country: "CN", Condition: "雷阵雨", TemperatureC: 28.2, Humidity: 78, WindKPH: 11.7,
		},
		"guangzhou": {
			City: "Guangzhou", Country: "CN", Condition: "Thunderstorm", TemperatureC: 28.2, Humidity: 78, WindKPH: 11.7,
		},
		"杭州": {
			City: "杭州", Country: "CN", Condition: "阴", TemperatureC: 19.6, Humidity: 69, WindKPH: 8.8,
		},
		"hangzhou": {
			City: "Hangzhou", Country: "CN", Condition: "Overcast", TemperatureC: 19.6, Humidity: 69, WindKPH: 8.8,
		},
	}
	if snapshot, ok := known[normalized]; ok {
		return snapshot
	}

	return weatherSnapshot{
		City:         city,
		Country:      "MOCK",
		Condition:    "晴",
		TemperatureC: 24.0,
		Humidity:     55,
		WindKPH:      10.0,
	}
}
