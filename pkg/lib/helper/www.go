package helper

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Payload struct {
	Message string `json:"message"`
	Details string `json:"details"`
}

func (p Payload) ToBytes() []byte {
	b, err := json.Marshal(p)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}

func WriteOCIErrror(w http.ResponseWriter, statusCode int, usrMsg, usrDetail string) {
	w.WriteHeader(statusCode)
	payload := Payload{
		Message: usrMsg,
		Details: usrDetail,
	}
	w.Write(payload.ToBytes())
}
