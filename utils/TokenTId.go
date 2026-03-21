package utils

import (
	"encoding/json"
	"net/http"
)

func TokenTId(token string) string {
	meReq, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return ""
	}

	meReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()
	type T struct {
		Id string `json:"id"`
	}

	var dec T
	err = json.NewDecoder(resp.Body).Decode(&dec)
	if err != nil {
		return ""
	}

	return dec.Id
}
