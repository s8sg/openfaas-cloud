package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

// status constant
const (
	Success = "success"
	Failure = "failure"
	Pending = "pending"
)

// context constant
const (
	Deploy         = "%s"
	Stack          = "stack-deploy"
	EmptyAuthToken = ""
	tokenString    = "token"
)

const authTokenPattern = "^[A-Za-z0-9-_.]*"

var validToken = regexp.MustCompile(authTokenPattern)

type CommitStatus struct {
	Status      string `json:"status"`
	Description string `json:"description"`
	Context     string `json:"context"`
}

// Status to post status to github-status function
type Status struct {
	CommitStatuses map[string]CommitStatus `json:"commit-statuses"`
	EventInfo      Event                   `json:"event"`
	AuthToken      string                  `json:"auth-token"`
}

// BuildStatus constructs a status object from event
func BuildStatus(event *Event, token string) *Status {

	status := Status{}
	status.EventInfo = *event
	status.CommitStatuses = make(map[string]CommitStatus)
	status.AuthToken = token

	return &status
}

// UnmarshalStatus unmardhal a status object from json
func UnmarshalStatus(data []byte) (*Status, error) {
	status := Status{}
	err := json.Unmarshal(data, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// AddStatus adds a commit status into a status object
//           a status can contain multiple commit status
func (status *Status) AddStatus(state string, desc string, context string) {
	if status.CommitStatuses == nil {
		status.CommitStatuses = make(map[string]CommitStatus)
	}
	// the status.CommitStatuses is a map hashed against the context
	// it replace the old commit status if added for same context
	status.CommitStatuses[context] = CommitStatus{Status: state, Description: desc, Context: context}
}

// marshal marshal a status into json
func (status *Status) Marshal() ([]byte, error) {
	return json.Marshal(status)
}

// ValidToken check if a token is in valid format
func ValidToken(token string) bool {
	match := validToken.FindString(token)
	// token should be the whole string
	if len(match) == len(token) {
		return true
	}
	return false
}

// MarshalToken mardhal a token into json
func MarshalToken(token string) string {
	marshalToken, _ := json.Marshal(map[string]string{tokenString: token})
	return string(marshalToken)
}

// UnmarshalToken unmarshal a token and validate
func UnmarshalToken(data []byte) string {
	tokenMap := make(map[string]string)

	err := json.Unmarshal(data, &tokenMap)
	if err != nil {
		log.Printf(`invalid auth token format received, %s.
error: %v
make sure combine_output is disabled for github-status`, err, data)
		return EmptyAuthToken
	}

	token := tokenMap[tokenString]
	if !ValidToken(token) {
		log.Printf(`invalid auth token received, token : ( %s ),
make sure combine_output is disabled for github-status`, token)
		return EmptyAuthToken
	}
	return token
}

// Report send a status update to github-status function
func (status *Status) Report(gateway string) (string, error) {
	body, _ := status.Marshal()

	c := http.Client{}
	bodyReader := bytes.NewBuffer(body)
	httpReq, _ := http.NewRequest(http.MethodPost, gateway+"function/github-status", bodyReader)

	res, err := c.Do(httpReq)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	resData, _ := ioutil.ReadAll(res.Body)
	if resData == nil {
		return "", fmt.Errorf("empty token received")
	}

	status.AuthToken = UnmarshalToken(resData)

	// reset old status
	status.CommitStatuses = make(map[string]CommitStatus)

	return status.AuthToken, nil
}

// FunctionContext build a github context for a function
//                 Example:
//                    sdk.FunctionContext(functionName)
func FunctionContext(function string) string {
	return fmt.Sprintf(Deploy, function)
}
