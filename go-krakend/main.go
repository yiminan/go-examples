package main

import (
	"encoding/json"
	"net/http"
)

func usersHandler(w http.ResponseWriter, r *http.Request) {
	// 객체(object) 형태로 반환
	response := map[string]interface{}{
		"users": []map[string]string{{"id": "1", "name": "Alice"}},
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/users", usersHandler)
	http.ListenAndServe(":3001", nil)
}
