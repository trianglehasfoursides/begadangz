package util

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
)

func Error(r *resty.Response, body []byte) {
	if r.StatusCode() != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &errResp)

		if errResp.Error == jwt.ErrTokenExpired.Error() {
			Logger.Info("Your token has expired. Run mathrock auth login {provider}")
			os.Exit(0)
		}

		Logger.Fatal(errResp.Error)
	}
}
