package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/openfaas/faas-cli/stack"
)

var gatewayURL string

// Handle a serverless request
func Handle(req []byte) []byte {

	pushEvent := PushEvent{}
	err := json.Unmarshal(req, &pushEvent)
	if err != nil {
		log.Println(err.Error())
		os.Exit(-1)
	}

	clonePath, err := clone(pushEvent)
	if err != nil {
		log.Println("Clone ", err.Error())
		os.Exit(-1)
	}

	stack, err := parseYAML(pushEvent, clonePath)
	if err != nil {
		log.Println("parseYAML ", err.Error())
		os.Exit(-1)
	}

	gatewayURL = os.Getenv("gateway_url")
	if len(gatewayURL) == 0 {
		gatewayURL = "http://gateway:8080"
	}

	var shrinkWrapPath string
	shrinkWrapPath, err = shrinkwrap(pushEvent, clonePath)
	if err != nil {
		log.Println("Shrinkwrap ", err.Error())
		os.Exit(-1)
	}

	var tars []tarEntry
	tars, err = makeTar(pushEvent, shrinkWrapPath, stack)
	if err != nil {
		log.Println("Error creating tar(s): ", err.Error())
		os.Exit(-1)
	}

	err = deploy(tars, pushEvent.Repository.Owner.Login, pushEvent.Repository.Name)
	if err != nil {
		log.Println(err)
	}

	err = collect(pushEvent, stack)
	if err != nil {
		log.Println(err)
	}

	return []byte(fmt.Sprintf("Deployed tar from: %s", tars))
}

func collect(pushEvent PushEvent, stack *stack.Services) error {
	var err error

	garbageReq := GarbageRequest{
		Owner: pushEvent.Repository.Owner.Login,
		Repo:  pushEvent.Repository.Name,
	}

	for k := range stack.Functions {
		garbageReq.Functions = append(garbageReq.Functions, k)
	}

	c := http.Client{
		Timeout: time.Second * 3,
	}

	bytesReq, _ := json.Marshal(garbageReq)
	bufferReader := bytes.NewBuffer(bytesReq)

	request, _ := http.NewRequest(http.MethodPost, gatewayURL+"/function/garbage-collect", bufferReader)

	response, err := c.Do(request)

	if err == nil {
		if response.Body != nil {
			defer response.Body.Close()
			bodyBytes, bErr := ioutil.ReadAll(response.Body)
			if bErr != nil {
				log.Fatal(bErr)
			}
			log.Println(string(bodyBytes))
		}
	}

	return err
}

type GarbageRequest struct {
	Functions []string `json:"functions"`
	Repo      string   `json:"repo"`
	Owner     string   `json:"owner"`
}
