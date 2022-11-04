package helper

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

/*
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
*/

type Payload struct {
	Errors []ErrorPayload `json:"errors"`
}

type Detail struct {
	Type   string `json:"Type"`
	Name   string `json:"Name"`
	Action string `json:"Action"`
}

func NewDetail(dType, dName, dAction string) Detail {
	return Detail{
		Type:   dType,
		Name:   dName,
		Action: dAction,
	}
}

func NewDetailFromReq(req types.Request) Detail {
	// TODO: Better logic here
	return Detail{
		Type:   "repository",
		Name:   req.Repo,
		Action: "pull",
	}
}

type ErrorPayload struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"detail"`
}

func NewErrPayload(code, msg string, det Detail) ErrorPayload {
	return ErrorPayload{
		Code:    code,
		Message: msg,
		Details: []Detail{det},
	}
}

func (p Payload) ToBytes() []byte {
	b, err := json.Marshal(p)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}

func WriteOCIErrror(w http.ResponseWriter, statusCode int, errPl ...ErrorPayload) {
	w.WriteHeader(statusCode)
	payload := Payload{
		Errors: errPl,
	}
	w.Write(payload.ToBytes())
}

//[{"Type":"repository","Name":"samalba/my-app","Action":"pull"}
