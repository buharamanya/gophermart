package handlers

import (
	"fmt"
	"net/http"
)

func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "тест")
}
