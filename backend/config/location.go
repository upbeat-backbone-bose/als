package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func updateLocation(cfg *ALSConfig) {
	log.Default().Println("Updating server location from internet...")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ipapi.co/json/")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Default().Printf("parse location failed: %v", err)
		return
	}
	if _, ok := data["country_name"]; !ok {
		return
	}

	if _, ok := data["city"]; !ok {
		return
	}

	cfg.Location = fmt.Sprintf("%s, %s", data["city"], data["country_name"])
	log.Default().Println("Server location: " + cfg.Location)
	log.Default().Println("Updating server location from internet successed, from ipapi.co")
}
